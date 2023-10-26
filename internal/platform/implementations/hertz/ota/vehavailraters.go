package ota

import (
	"encoding/xml"
	"strconv"
	"strings"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
)

type VehAvailRateRS struct {
	XMLName        xml.Name       `xml:"OTA_VehAvailRateRS"`
	VehAvailRSCore VehAvailRSCore `xml:"VehAvailRSCore"`
	ErrorsMixin
}

type VehAvailRSCore struct {
	VehVendorAvails VehVendorAvails `xml:"VehVendorAvails"`
}

type VehVendorAvails struct {
	VehVendorAvail VehVendorAvail `xml:"VehVendorAvail"`
}

type VehVendorAvail struct {
	VehAvails VehAvails `xml:"VehAvails"`
	Info      Info      `xml:"Info"`
}

type Info struct {
	LocationDetails struct {
		AdditionalInfo struct {
			CounterLocation struct {
				Location string `xml:"Location,attr"`
			} `xml:"CounterLocation"`
		} `xml:"AdditionalInfo"`
	} `xml:"LocationDetails"`
}

type VehAvails struct {
	VehAvail []VehAvail `xml:"VehAvail"`
}

type VehAvail struct {
	VehAvailCore VehAvailCore `xml:"VehAvailCore"`
	VehAvailInfo VehAvailInfo `xml:"VehAvailInfo"`
}

type VehAvailCore struct {
	Status       string       `xml:"Status,attr"`
	Vehicle      Vehicle      `xml:"Vehicle"`
	RentalRate   RentalRate   `xml:"RentalRate"`
	TotalCharge  TotalCharge  `xml:"TotalCharge"`
	Fees         Fees         `xml:"Fees"`
	Reference    Reference    `xml:"Reference"`
	PricedEquips PricedEquips `xml:"PricedEquips"`
}

type PricedEquips struct {
	PricedEquip []PricedEquip `xml:"PricedEquip"`
}

type PricedEquip struct {
	Equipment Equipment         `xml:"Equipment"`
	Charge    PricedEquipCharge `xml:"Charge"`
}

type PricedEquipCharge struct {
	Amount         float64 `xml:"Amount,attr"`
	TaxInclusive   bool    `xml:"TaxInclusive,attr"`
	CurrencyCode   string  `xml:"CurrencyCode,attr"`
	IncludedInRate bool    `xml:"IncludedInRate,attr"`
}

type Equipment struct {
	EquipType string `xml:"EquipType,attr"`
	Quantity  int    `xml:"Quantity,attr"`
}

type Fees struct {
	Fee []Fee `xml:"Fee"`
}

func feeIncludedType(
	fee Fee,
	params schema.RatesRequestParams,
	configuration schema.HertzConfiguration,
	paymentRules []PaymentRule,
) feeIncludeType {
	feePayableLocally := make(map[string][]string)
	if configuration.FeePayableLocally != nil {
		feePayableLocally = *configuration.FeePayableLocally
	}

	feeNameUpper := strings.ToUpper(fee.Description)

	switch params.Contract.PaymentType {
	case int(schema.PaymentTypeFullPrepay):
		if mapContains(feePayableLocally, feeNameUpper, params.PickUp.Country) {
			return feesNotIncludedPayLocally
		}

		if len(paymentRules) > 0 && (paymentRules)[0].RuleType == hertzPaymentRulePrePay {
			return feesIncluded
		}

		if configuration.FpPayFeesLocally != nil && *configuration.FpPayFeesLocally {
			return feesNotIncludedPayLocally
		}

		return feesNotIncludedPayNow
	case int(schema.PaymentTypePartialPrepay):
		if fee.IncludedInRate {
			return feesIncluded
		}

		return feesUnknown

	default:
		return feesUnknown
	}
}

func (f *Fees) Fees(
	params schema.RatesRequestParams,
	configuration schema.HertzConfiguration,
	paymentRules []PaymentRule,
) []schema.ExtraOrFee {
	var fees = make([]schema.ExtraOrFee, len(f.Fee))

	i := 0
	for _, fee := range f.Fee {
		includedInRate := false
		payLocal := false

		switch feeIncludedType(fee, params, configuration, paymentRules) {
		case feesIncluded:
			includedInRate = true
			payLocal = false

		case feesNotIncludedPayNow:
			includedInRate = false
			payLocal = false

		case feesNotIncludedPayLocally, feesUnknown:
			includedInRate = false
			payLocal = true
		}

		fees[i] = schema.ExtraOrFee{
			Code:           fee.Purpose,
			Mandatory:      true,
			Name:           fee.Description,
			Type:           schema.VCP,
			IncludedInRate: includedInRate,
			PayLocal:       payLocal,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(fee.Amount),
				Currency: fee.CurrencyCode,
			},
		}
		i++
	}

	return fees
}

type Fee struct {
	Purpose        string  `xml:"Purpose,attr"`
	TaxInclusive   bool    `xml:"TaxInclusive,attr"`
	IncludedInRate bool    `xml:"IncludedInRate,attr"`
	Description    string  `xml:"Description,attr"`
	Amount         float64 `xml:"Amount,attr"`
	CurrencyCode   string  `xml:"CurrencyCode,attr"`
}

type TotalCharge struct {
	RateTotalAmount      float64 `xml:"RateTotalAmount,attr"`
	EstimatedTotalAmount float64 `xml:"EstimatedTotalAmount,attr"`
	CurrencyCode         string  `xml:"CurrencyCode,attr"`
}

func (c *TotalCharge) Price(params schema.RatesRequestParams, paymentRules []PaymentRule) (schema.PriceAmount, string) {
	var price schema.PriceAmount

	paymentsRules := paymentRules

	if isPartialPayment(params) || fullWithoutPaymentRules(params, paymentRules) {
		if c.CurrencyCode == "" {
			return schema.PriceAmount{}, "Cant parse price from TotalCharge"
		}

		price.Amount = schema.RoundedFloat(c.RateTotalAmount)
		price.Currency = c.CurrencyCode
	}

	if params.Contract.PaymentType == int(schema.PaymentTypeFullPrepay) && len(paymentRules) > 0 {
		rule := (paymentsRules)[0]

		if rule.RuleType == hertzPaymentRulePrePay {
			if rule.CurrencyCode == "" {
				return schema.PriceAmount{}, "Cant parse price from PaymentRule"
			}

			price.Amount = schema.RoundedFloat(rule.Amount)
			price.Currency = rule.CurrencyCode
		}
	}

	if price.Amount == 0 {
		return schema.PriceAmount{}, "Unable to parse price from vehicle"
	}

	return price, ""
}

type Reference struct {
	Type string `xml:"Type,attr"`
	ID   string `xml:"ID,attr"`
}

type RentalRate struct {
	RateDistance   RateDistance    `xml:"RateDistance"`
	VehicleCharges VehicleCharges  `xml:"VehicleCharges"`
	RateQualifier  RateQualifierRS `xml:"RateQualifier"`
}

type RateQualifierRS struct {
	ArriveByFlight bool   `xml:"ArriveByFlight,attr"`
	RateQualifier  string `xml:"RateQualifier,attr"`
}

type VehicleCharges struct {
	VehicleCharge []VehicleCharge `xml:"VehicleCharge"`
}

func (v *VehicleCharges) Charges() []schema.ExtraOrFee {
	var charges []schema.ExtraOrFee

	whitelist := []string{"2", "8", "23"}

	for _, charge := range v.VehicleCharge {
		if !contains(whitelist, strconv.Itoa(charge.Purpose)) {
			continue
		}

		includedInPrice := false
		if charge.IncludedInRate && charge.Purpose != int(schema.OtaVcpAdditionalDistance) {
			includedInPrice = true
		}

		price := schema.PriceAmount{
			Amount:   schema.RoundedFloat(charge.Amount),
			Currency: charge.CurrencyCode,
		}

		mandatory := charge.Purpose != int(schema.OtaVcpAdditionalDistance)

		charges = append(charges, schema.ExtraOrFee{
			Code:           strconv.Itoa(charge.Purpose),
			Name:           charge.Description,
			Price:          price,
			Mandatory:      mandatory,
			IncludedInRate: includedInPrice,
			PayLocal:       !includedInPrice,
			Type:           schema.VCP,
		})
	}

	return charges
}

func (v *VehicleCharges) TaxCharge(
	params schema.RatesRequestParams,
	configuration schema.HertzConfiguration,
	paymentRules []PaymentRule,
) (float64, *schema.ExtraOrFee, bool) {
	taxMultiplier := 1.0

	fpPaynowVehiclePriceWithTax := converting.Unwrap(configuration.FpPaynowVehiclePriceWithTax)

	for _, charge := range v.VehicleCharge {
		if charge.Purpose == int(schema.OtaVcpVehicleRentalFee) && len(charge.TaxAmounts.TaxAmount) > 0 {
			for _, tax := range charge.TaxAmounts.TaxAmount {
				if tax.Description == "Tax" {
					taxMultiplier = 1 + (tax.Percentage / 100)
					taxAmountForVehicle := tax.Total

					if fpPaynowVehiclePriceWithTax && fullWithoutPaymentRules(params, paymentRules) {
						return taxMultiplier, nil, true
					}

					payLocal := true
					if params.Contract.PaymentType == int(schema.PaymentTypeFullPrepay) {
						payLocal = converting.Unwrap(configuration.FpPayFeesLocally)
					}

					if isPartialPayment(params) || (!fpPaynowVehiclePriceWithTax && fullWithoutPaymentRules(params, paymentRules)) {
						return taxMultiplier, &schema.ExtraOrFee{
							Code:           strconv.Itoa(int(schema.OtaVcpTax)),
							Name:           "Tax",
							IncludedInRate: false,
							Mandatory:      true,
							PayLocal:       payLocal,
							Price: schema.PriceAmount{
								Amount:   schema.RoundedFloat(taxAmountForVehicle),
								Currency: tax.CurrencyCode,
							},
							Type: schema.VCP,
						}, false
					}
				}
			}
		}
	}

	return taxMultiplier, nil, false
}

type VehicleCharge struct {
	Purpose        int                 `xml:"Purpose,attr"`
	Description    string              `xml:"Description,attr"`
	TaxInclusive   bool                `xml:"TaxInclusive,attr"`
	GuaranteedInd  bool                `xml:"GuaranteedInd,attr"`
	Amount         float64             `xml:"Amount,attr"`
	CurrencyCode   string              `xml:"CurrencyCode,attr"`
	IncludedInRate bool                `xml:"IncludedInRate,attr"`
	TaxAmounts     TaxAmounts          `xml:"TaxAmounts"`
	Calculation    []ChargeCalculation `xml:"Calculation"`
}

type ChargeCalculation struct {
	UnitCharge float64 `xml:"UnitCharge,attr"`
	UnitName   string  `xml:"UnitName,attr"`
	Quantity   int     `xml:"Quantity,attr"`
}

type TaxAmounts struct {
	TaxAmount []TaxAmount `xml:"TaxAmount"`
}

type TaxAmount struct {
	Total        float64 `xml:"Total,attr"`
	CurrencyCode string  `xml:"CurrencyCode,attr"`
	Percentage   float64 `xml:"Percentage,attr"`
	Description  string  `xml:"Description,attr"`
}

type RateDistance struct {
	Unlimited             bool   `xml:"Unlimited,attr"`
	DistUnitName          string `xml:"DistUnitName,attr"`
	VehiclePeriodUnitName string `xml:"VehiclePeriodUnitName,attr"`
	Quantity              string `xml:"Quantity,attr"`
}

func (rateDistance *RateDistance) Mileage() schema.Mileage {
	mileage := schema.Mileage{
		Unlimited: &rateDistance.Unlimited,
	}

	mileage.DistanceUnit = mapping.DistanceUnit(rateDistance.DistUnitName)
	if mileage.DistanceUnit != nil {
		unit := schema.Km
		mileage.DistanceUnit = &unit
	}

	mileage.PeriodUnit = mapping.PeriodUnit(rateDistance.VehiclePeriodUnitName)
	if mileage.PeriodUnit != nil {
		unit := schema.Km
		mileage.DistanceUnit = &unit
	}

	mileage.IncludedDistance = &rateDistance.Quantity

	return mileage
}

type Vehicle struct {
	PassengerQuantity int          `xml:"PassengerQuantity,attr"`
	BaggageQuantity   int          `xml:"BaggageQuantity,attr"`
	AirConditionInd   bool         `xml:"AirConditionInd,attr"`
	TransmissionType  string       `xml:"TransmissionType,attr"`
	FuelType          string       `xml:"FuelType,attr"`
	DriveType         string       `xml:"DriveType,attr"`
	Code              string       `xml:"Code,attr"`
	CodeContext       string       `xml:"CodeContext,attr"`
	VehMakeModel      VehMakeModel `xml:"VehMakeModel"`
	VehType           VehType      `xml:"VehType"`
}

type VehType struct {
	VehicleCategory int `xml:"VehicleCategory,attr"`
	DoorCount       int `xml:"DoorCount,attr"`
}

type VehMakeModel struct {
	Name string `xml:"Name,attr"`
	Code string `xml:"Code,attr"`
}

type VehAvailInfo struct {
	PaymentRules    PaymentRules    `xml:"PaymentRules"`
	PricedCoverages PricedCoverages `xml:"PricedCoverages"`
}

type PricedCoverages struct {
	PricedCoverage []PricedCoverage `xml:"PricedCoverage"`
}

func (p *PricedCoverages) VehicleCoverages(
	taxMultiplier float64,
	paymentRules []PaymentRule,
	params schema.RatesRequestParams,
	configuration schema.HertzConfiguration,
) ([]schema.ExtraOrFee, float64) {
	priceToBeAddedToVehiclePrice := 0.0

	coverages := []schema.ExtraOrFee{}

	for _, coverage := range p.PricedCoverage {
		includedInRate := false
		payLocal := true

		mandatory := coverage.Charge.IncludedInRate || coverage.Required || coverage.Coverage.Required

		switch whichCoverageIncludedType(coverage, paymentRules, params, configuration) {
		case coverageIncluded:
			includedInRate = true
			payLocal = false

		case coverageNotIncludedPayNow:
			includedInRate = false
			payLocal = false

		case coverageNotIncluded, coverageUnknown:
			includedInRate = false
			payLocal = true
		}

		var price *schema.PriceAmount = nil

		if coverage.Charge.Amount != nil {
			price = &schema.PriceAmount{
				Amount:   schema.RoundedFloat(*coverage.Charge.Amount),
				Currency: coverage.Charge.CurrencyCode,
			}
		}

		if price == nil {
			for _, calculation := range coverage.Charge.Calculation {
				if calculation.UnitName == string(schema.Day) {
					price = &schema.PriceAmount{
						Amount:   schema.RoundedFloat(calculation.UnitCharge * float64(params.RentalDays)),
						Currency: coverage.Charge.CurrencyCode,
					}
					break
				} else if calculation.UnitName == string(schema.RentalPeriod) {
					price = &schema.PriceAmount{
						Amount:   schema.RoundedFloat(calculation.UnitCharge),
						Currency: coverage.Charge.CurrencyCode,
					}
					break
				}
			}
		}

		if price == nil {
			continue
		}

		excludedCountries := []string{}
		if configuration.TaxExclCoverageCountries != nil {
			excludedCountries = *configuration.TaxExclCoverageCountries
		}

		addTaxToCoverages := []string{}
		if configuration.AddTaxToCoverages != nil {
			addTaxToCoverages = *configuration.AddTaxToCoverages
		}

		if contains(addTaxToCoverages, coverage.Coverage.CoverageType) && !coverage.Charge.TaxInclusive {
			price.Amount = schema.RoundedFloat(taxMultiplier * float64(price.Amount))
		} else if !coverage.Charge.TaxInclusive && !coverage.Required && !contains(excludedCountries, params.PickUp.Country) {
			price.Amount = schema.RoundedFloat(taxMultiplier * float64(price.Amount))
		}

		if mandatory && converting.Unwrap(configuration.IncludeCoveragesInRate) {
			priceToBeAddedToVehiclePrice = priceToBeAddedToVehiclePrice + float64(price.Amount)
			includedInRate = true
			mandatory = true
		}

		coverages = append(coverages, schema.ExtraOrFee{
			Type:           schema.VCT,
			Name:           "",
			Code:           coverage.Coverage.CoverageType,
			IncludedInRate: includedInRate,
			PayLocal:       payLocal,
			Mandatory:      mandatory,
			Price:          *price,
		})
	}

	return coverages, priceToBeAddedToVehiclePrice
}

func whichCoverageIncludedType(
	coverage PricedCoverage,
	paymentRules []PaymentRule,
	params schema.RatesRequestParams,
	configuration schema.HertzConfiguration,
) coverageIncludedType {
	payNowCoverages := []string{}
	if configuration.PayNowCoverages != nil {
		payNowCoverages = *configuration.PayNowCoverages
	}

	if contains(payNowCoverages, coverage.Coverage.CoverageType) {
		return coverageNotIncludedPayNow
	}

	if coverage.Charge.IncludedInRate {
		return coverageIncluded
	}

	required := coverage.Coverage.Required || coverage.Required

	switch params.Contract.PaymentType {
	case int(schema.PaymentTypeFullPrepay):
		if len(paymentRules) > 0 {
			rule := (paymentRules)[0]
			if rule.RuleType == hertzPaymentRulePrePay && required {
				return coverageIncluded
			}
		}

		return coverageNotIncluded

	case int(schema.PaymentTypePartialPrepay):
		return coverageNotIncluded

	default:
		return coverageUnknown
	}
}

type PricedCoverage struct {
	Required bool     `xml:"Required,attr"`
	Coverage Coverage `xml:"Coverage"`
	Charge   Charge   `xml:"Charge"`
}

type Coverage struct {
	Required     bool   `xml:"Required,attr"`
	CoverageType string `xml:"CoverageType,attr"`
}

type Charge struct {
	TaxInclusive   bool                  `xml:"TaxInclusive,attr"`
	IncludedInRate bool                  `xml:"IncludedInRate,attr"`
	Amount         *float64              `xml:"Amount,attr"`
	CurrencyCode   string                `xml:"CurrencyCode,attr"`
	Calculation    []CoverageCalculation `xml:"Calculation"`
}

type CoverageCalculation struct {
	UnitCharge float64 `xml:"UnitCharge,attr"`
	UnitName   string  `xml:"UnitName,attr"`
	Quantity   int     `xml:"Quantity,attr"`
}

type PaymentRules struct {
	PaymentRule []PaymentRule `xml:"PaymentRule"`
}

type PaymentRule struct {
	RuleType     int     `xml:"RuleType,attr"`
	Amount       float64 `xml:"Amount,attr"`
	CurrencyCode string  `xml:"CurrencyCode,attr"`
}
