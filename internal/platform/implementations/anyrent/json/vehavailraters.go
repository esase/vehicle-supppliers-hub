package json

import (
	jsonEncoding "encoding/json"
	"fmt"
	"strconv"
	"strings"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
)

type PricesAndAvailabilityRS struct {
	Errors
	Fleets []PricesAndAvailabilityRSFleet `json:"fleets"`
}

type PricesAndAvailabilityRSFleet struct {
	Id     int                                 `json:"id"`
	Code   string                              `json:"code"`
	Name   string                              `json:"name"`
	Groups []PricesAndAvailabilityRSFleetGroup `json:"groups"`
}

type VehicleStatus string

const (
	VehicleStatusAvailable VehicleStatus = "AVAILABLE"
	VehicleStatusOnRequest VehicleStatus = "ON_REQUEST"
)

type PricesAndAvailabilityRSFleetGroup struct {
	Code           string                                        `json:"code"`
	SippCode       string                                        `json:"sipp_code"`
	Name           string                                        `json:"name"`
	Brand          string                                        `json:"brand"`
	Model          string                                        `json:"model"`
	Features       PricesAndAvailabilityRSFeatures               `json:"features"`
	Status         VehicleStatus                                 `json:"status"`
	Rate           PricesAndAvailabilityRSFleetGroupRate         `json:"rate"`
	OptionalsRates PricesAndAvailabilityRSFleetGroupOptionalRate `json:"optionals_rates"`
}

func (r *PricesAndAvailabilityRSFleetGroup) GetName() string {
	return r.Brand + " " + r.Model
}

type PricesAndAvailabilityRSFeatures struct {
	Fuel         *any `json:"fuel,omitempty"`
	Transmission *any `json:"transmission,omitempty"`
	Seats        *any `json:"seats,omitempty"`
	Doors        *any `json:"doors,omitempty"`
	Suitcases    *any `json:"suitcases,omitempty"`
	AirCondition *any `json:"ac,omitempty"`
}

func (f *PricesAndAvailabilityRSFeatures) UnmarshalJSON(b []byte) error {
	if string(b) == "[]" {
		return nil
	}

	type PricesAndAvailabilityRSFeaturesAlias PricesAndAvailabilityRSFeatures

	features := &struct {
		*PricesAndAvailabilityRSFeaturesAlias
	}{
		PricesAndAvailabilityRSFeaturesAlias: (*PricesAndAvailabilityRSFeaturesAlias)(f),
	}

	if err := jsonEncoding.Unmarshal(b, features); err != nil {
		return err
	}

	return nil
}

func (f *PricesAndAvailabilityRSFeatures) GetAirCondition() *bool {
	var mapped *bool = nil

	if f.AirCondition != nil {
		hasAirCondition := *f.AirCondition != ""
		mapped = &hasAirCondition
	}

	return mapped
}

func (f *PricesAndAvailabilityRSFeatures) GetDoors() *int {
	var mapped *int = nil

	if f.Doors != nil {
		doors, _ := strconv.Atoi(fmt.Sprintf("%v", *f.Doors))

		mapped = converting.PointerToValue(doors)
	}

	return mapped
}

func (f *PricesAndAvailabilityRSFeatures) GetTransmission() *schema.VehicleTransmissionType {
	var mapped *schema.VehicleTransmissionType = nil

	if f.Transmission != nil {
		transmission := fmt.Sprintf("%v", *f.Transmission)

		switch strings.ToLower(transmission) {
		case "auto":
			mapped = converting.PointerToValue(schema.Automatic)

		default:
			mapped = converting.PointerToValue(schema.Manual)
		}
	}

	return mapped
}

func (f *PricesAndAvailabilityRSFeatures) GetFuel() *schema.VehicleFuelType {
	var mapped *schema.VehicleFuelType = nil

	if f.Fuel != nil {
		fuel := fmt.Sprintf("%v", *f.Fuel)

		switch strings.ToLower(fuel) {
		case "diesel":
			mapped = converting.PointerToValue(schema.Diesel)

		case "petrol":
			mapped = converting.PointerToValue(schema.Petrol)
		}
	}

	return mapped
}

func (f *PricesAndAvailabilityRSFeatures) GetSeats() *int {
	var mapped *int = nil

	if f.Seats != nil {
		seats, _ := strconv.Atoi(fmt.Sprintf("%v", *f.Seats))

		mapped = converting.PointerToValue(seats)
	}

	return mapped
}

func (f *PricesAndAvailabilityRSFeatures) GetSuitCases() *int {
	var mapped *int = nil

	if f.Suitcases != nil {
		suitcases, _ := strconv.Atoi(fmt.Sprintf("%v", *f.Suitcases))

		mapped = converting.PointerToValue(suitcases)
	}

	return mapped
}

type PricesAndAvailabilityRSFleetGroupRate struct {
	Code                 string                                     `json:"code"`
	RateChargeType       string                                     `json:"rate_charge_type"`
	TimeUnits            int                                        `json:"time_units"`
	RateBeforeTax        float32                                    `json:"rate_before_tax"`
	RateTax              float32                                    `json:"rate_tax"`
	RateAfterTax         float32                                    `json:"rate_after_tax"`
	TotalBeforeTax       float32                                    `json:"total_before_tax"`
	TotalTax             float32                                    `json:"total_tax"`
	TotalAfterTax        float32                                    `json:"total_after_tax"`
	DiscountRate         float32                                    `json:"discount_rate"`
	DiscountAfterTax     float32                                    `json:"discount_after_tax"`
	TaxRate              float32                                    `json:"tax_rate"`
	IncludedMileage      int                                        `json:"included_mileage"`
	PricePerMileageUnit  float32                                    `json:"price_per_mileage_unit"`
	ExcessValue          float32                                    `json:"excess_value"`
	SecurityDepositValue float32                                    `json:"security_deposit_value"`
	Currency             string                                     `json:"currency"`
	Extras               []PricesAndAvailabilityRSFleetRateExtra    `json:"extras"`
	Taxes                []PricesAndAvailabilityRSFleetRateExtra    `json:"taxes"`
	Insurances           []PricesAndAvailabilityRSFleetRateCoverage `json:"insurances"`
}

func (r *PricesAndAvailabilityRSFleetGroupRate) GetCustomCoverages() []schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	mapped = append(mapped, schema.ExtraOrFee{
		Code: "CollisionDamage",
		Name: "Collision damage",
		Price: schema.PriceAmount{
			Amount:   0,
			Currency: r.Currency,
		},
		IncludedInRate: true,
		PayLocal:       true,
		Mandatory:      true,
		Unit:           converting.PointerToValue(schema.PerRental),
		Type:           schema.VCT,
		Excess: &schema.PriceAmount{
			Amount:   schema.RoundedFloat(r.ExcessValue),
			Currency: r.Currency,
		},
	})

	return mapped
}

func (r *PricesAndAvailabilityRSFleetGroupRate) GetRequiredCoverages(currency string) []schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	for _, extraInfo := range r.Insurances {
		mapped = append(mapped, schema.ExtraOrFee{
			Code: extraInfo.Code,
			Name: extraInfo.Name,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(extraInfo.TotalAfterTax),
				Currency: currency,
			},
			IncludedInRate: extraInfo.Included,
			PayLocal:       true,
			Mandatory:      extraInfo.Required,
			Unit:           converting.PointerToValue(schema.PerRental),
			MaxQuantity:    &extraInfo.MaxAcceptedQty,
			Type:           schema.VCT,
			Excess: &schema.PriceAmount{
				Amount:   schema.RoundedFloat(extraInfo.ExcessValue),
				Currency: currency,
			},
		})
	}

	return mapped
}

func (r *PricesAndAvailabilityRSFleetGroupRate) GetRequiredFees(currency string) []schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	for _, extraInfo := range r.Taxes {
		mapped = append(mapped, schema.ExtraOrFee{
			Code: extraInfo.Code,
			Name: extraInfo.Name,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(extraInfo.TotalAfterTax),
				Currency: currency,
			},
			IncludedInRate: extraInfo.Included,
			PayLocal:       true,
			Mandatory:      extraInfo.Required,
			Unit:           converting.PointerToValue(schema.PerRental),
			MaxQuantity:    &extraInfo.MaxAcceptedQty,
			Type:           schema.VCP,
		})
	}

	return mapped
}

func (r *PricesAndAvailabilityRSFleetGroupRate) GetRequiredExtras(currency string) []schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	for _, extraInfo := range r.Extras {
		mapped = append(mapped, schema.ExtraOrFee{
			Code: extraInfo.Code,
			Name: extraInfo.Name,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(extraInfo.TotalAfterTax),
				Currency: currency,
			},
			IncludedInRate: extraInfo.Included,
			PayLocal:       true,
			Mandatory:      extraInfo.Required,
			Unit:           converting.PointerToValue(schema.PerRental),
			MaxQuantity:    &extraInfo.MaxAcceptedQty,
			Type:           schema.EQP,
		})
	}

	return mapped
}

type PricesAndAvailabilityRSFleetGroupOptionalRate struct {
	Extras     map[string]PricesAndAvailabilityRSFleetRateExtra    `json:"extras"`
	Taxes      map[string]PricesAndAvailabilityRSFleetRateExtra    `json:"taxes"`
	Insurances map[string]PricesAndAvailabilityRSFleetRateCoverage `json:"insurances"`
}

func (r *PricesAndAvailabilityRSFleetGroupOptionalRate) GetOptionalCoverages(currency string, collectedRequiredExtraAndFeeCodes map[string]struct{}) []schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	for _, extraInfo := range r.Insurances {
		if _, ok := collectedRequiredExtraAndFeeCodes[extraInfo.Code]; ok {
			continue
		}

		isAutoAssignable := false

		if extraInfo.AutoAssignable != nil {
			isAutoAssignable = *extraInfo.AutoAssignable
		}

		mapped = append(mapped, schema.ExtraOrFee{
			Code: extraInfo.Code,
			Name: extraInfo.Name,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(extraInfo.TotalAfterTax),
				Currency: currency,
			},
			IncludedInRate: extraInfo.Included || isAutoAssignable,
			PayLocal:       true,
			Mandatory:      extraInfo.Required,
			Unit:           converting.PointerToValue(schema.PerRental),
			MaxQuantity:    &extraInfo.MaxAcceptedQty,
			Type:           schema.VCT,
			Excess: &schema.PriceAmount{
				Amount:   schema.RoundedFloat(extraInfo.ExcessValue),
				Currency: currency,
			},
		})
	}

	return mapped
}

func (r *PricesAndAvailabilityRSFleetGroupOptionalRate) GetOptionalFees(currency string, collectedRequiredExtraAndFeeCodes map[string]struct{}) []schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	for _, extraInfo := range r.Taxes {
		if _, ok := collectedRequiredExtraAndFeeCodes[extraInfo.Code]; ok {
			continue
		}

		isAutoAssignable := false

		if extraInfo.AutoAssignable != nil {
			isAutoAssignable = *extraInfo.AutoAssignable
		}

		mapped = append(mapped, schema.ExtraOrFee{
			Code: extraInfo.Code,
			Name: extraInfo.Name,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(extraInfo.TotalAfterTax),
				Currency: currency,
			},
			IncludedInRate: extraInfo.Included || isAutoAssignable,
			PayLocal:       true,
			Mandatory:      extraInfo.Required,
			Unit:           converting.PointerToValue(schema.PerRental),
			MaxQuantity:    &extraInfo.MaxAcceptedQty,
			Type:           schema.VCP,
		})
	}

	return mapped
}

func (r *PricesAndAvailabilityRSFleetGroupOptionalRate) GetOptionalExtras(currency string, collectedRequiredExtraAndFeeCodes map[string]struct{}) []schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	for _, extraInfo := range r.Extras {
		if _, ok := collectedRequiredExtraAndFeeCodes[extraInfo.Code]; ok {
			continue
		}

		isAutoAssignable := false

		if extraInfo.AutoAssignable != nil {
			isAutoAssignable = *extraInfo.AutoAssignable
		}

		mapped = append(mapped, schema.ExtraOrFee{
			Code: extraInfo.Code,
			Name: extraInfo.Name,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(extraInfo.TotalAfterTax),
				Currency: currency,
			},
			IncludedInRate: extraInfo.Included || isAutoAssignable,
			PayLocal:       true,
			Mandatory:      extraInfo.Required,
			Unit:           converting.PointerToValue(schema.PerRental),
			MaxQuantity:    &extraInfo.MaxAcceptedQty,
			Type:           schema.EQP,
		})
	}

	return mapped
}

type ExtraUnit string

const (
	ExtraUnitTime   ExtraUnit = "time"
	ExtraUnitRental ExtraUnit = "rental"
)

type PricesAndAvailabilityRSFleetRateExtra struct {
	Id                int       `json:"id"`
	Code              string    `json:"code"`
	Name              string    `json:"name"`
	ChargeType        ExtraUnit `json:"charge_type"`
	MaxAcceptedQty    int       `json:"max_accepted_qty"`
	Required          bool      `json:"required"`
	Included          bool      `json:"included"`
	AutoAssignable    *bool     `json:"auto_assignable,omitempty"`
	Qty               int       `json:"qty"`
	TaxRate           float32   `json:"tax_rate"`
	PriceBeforeTax    float32   `json:"price_before_tax"`
	PriceAfterTax     float32   `json:"price_after_tax"`
	TotalBeforeTax    float32   `json:"total_before_tax"`
	TotalAfterTax     float32   `json:"total_after_tax"`
	TotalTax          float32   `json:"total_tax"`
	MinPriceBeforeTax float32   `json:"min_price_before_tax"`
	MaxPriceBeforeTax float32   `json:"max_price_before_tax"`
}

type PricesAndAvailabilityRSFleetRateCoverage struct {
	PricesAndAvailabilityRSFleetRateExtra
	ExcessValue          float32 `json:"excess_value"`
	SecurityDepositValue float32 `json:"security_deposit_value"`
}
