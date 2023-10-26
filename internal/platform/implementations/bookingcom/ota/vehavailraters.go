package ota

import (
	"fmt"
	"strings"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
)

type SearchRS struct {
	MatchList SearchRSMatchList `xml:"MatchList"`
	Errors
}

type SearchRSMatchList struct {
	Match []SearchRSMatch `xml:"Match"`
}

type SearchRSMatch struct {
	Vehicle       SearchRSVehicle       `xml:"Vehicle"`
	Price         SearchRSPrice         `xml:"Price"`
	Fees          SearchRSFees          `xml:"Fees"`
	ExtraInfoList SearchRSExtraInfoList `xml:"ExtraInfoList"`
	Supplier      SearchRSSupplier      `xml:"Supplier"`
	Route         SearchRSRoute         `xml:"Route"`
}

type SearchRSVehicle struct {
	Id                string `xml:"id,attr"`
	AvailabilityCheck bool   `xml:"availabilityCheck,attr"`
	PropositionType   string `xml:"propositionType,attr"`
	Automatic         string `xml:"automatic,attr"`
	Aircon            string `xml:"aircon,attr"`
	Airbag            bool   `xml:"airbag,attr"`
	UnlimitedMileage  bool   `xml:"unlimitedMileage,attr"`
	Petrol            string `xml:"petrol,attr"`
	Group             string `xml:"group,attr"`
	Doors             string `xml:"doors,attr"`
	Seats             string `xml:"seats,attr"`
	BigSuitcase       string `xml:"bigSuitcase,attr"`
	SmallSuitcase     string `xml:"smallSuitcase,attr"`
	Name              string `xml:"Name"`
	ImageURL          string `xml:"ImageURL"`
	LargeImageURL     string `xml:"LargeImageURL"`
}

func (c *SearchRSVehicle) AirCondition() *bool {
	mapped := strings.ToLower(c.Aircon) == "yes"

	return &mapped
}

func (c *SearchRSVehicle) Transmission() *schema.VehicleTransmissionType {
	mapped := schema.Manual

	if strings.ToLower(c.Automatic) == "automatic" {
		mapped = schema.Automatic
	}

	return &mapped
}

func (c *SearchRSVehicle) Fuel() *schema.VehicleFuelType {
	var mapped schema.VehicleFuelType

	switch strings.ToLower(c.Petrol) {
	case "diesel":
		mapped = schema.Diesel

	case "petrol":
		mapped = schema.Petrol
	}

	if mapped == "" {
		return nil
	}

	return &mapped
}

type SearchRSPrice struct {
	Currency           string  `xml:"currency,attr"`
	BaseCurrency       string  `xml:"baseCurrency,attr"`
	BasePrice          float64 `xml:"basePrice,attr"`
	Discount           float64 `xml:"discount,attr"`
	DriveAwayPrice     float64 `xml:"driveAwayPrice,attr"`
	QuoteAllowed       string  `xml:"quoteAllowed,attr"`
	CreditCardRequired bool    `xml:"creditCardRequired,attr"`
}

type SearchRSFees struct {
	DepositExcessFees SearchRSDepositExcessFees `xml:"DepositExcessFees"`
	KnownFees         SearchRSKnownFees         `xml:"KnownFees"`
}

type SearchRSDepositExcessFees struct {
	TheftExcess  SearchRSDepositExcessFee `xml:"TheftExcess"`
	DamageExcess SearchRSDepositExcessFee `xml:"DamageExcess"`
	Deposit      SearchRSDepositExcessFee `xml:"Deposit"`
}

func (c *SearchRSDepositExcessFees) Coverages() *[]schema.ExtraOrFee {
	unit := schema.PerRental
	var mapped []schema.ExtraOrFee

	mapped = append(mapped, schema.ExtraOrFee{
		Code: "TheftExcess",
		Name: "TheftExcess",
		Price: schema.PriceAmount{
			Amount:   0,
			Currency: c.TheftExcess.Currency,
		},
		IncludedInRate: true,
		PayLocal:       false,
		Mandatory:      true,
		Unit:           &unit,
		Type:           schema.VCT,
		Excess: &schema.PriceAmount{
			Amount:   schema.RoundedFloat(c.TheftExcess.Amount),
			Currency: c.TheftExcess.Currency,
		},
	}, schema.ExtraOrFee{
		Code: "DamageExcess",
		Name: "DamageExcess",
		Price: schema.PriceAmount{
			Amount:   0,
			Currency: c.DamageExcess.Currency,
		},
		IncludedInRate: true,
		PayLocal:       false,
		Mandatory:      true,
		Unit:           &unit,
		Type:           schema.VCT,
		Excess: &schema.PriceAmount{
			Amount:   schema.RoundedFloat(c.DamageExcess.Amount),
			Currency: c.DamageExcess.Currency,
		},
	}, schema.ExtraOrFee{
		Code: "Deposit",
		Name: "Deposit",
		Price: schema.PriceAmount{
			Amount:   0,
			Currency: c.Deposit.Currency,
		},
		IncludedInRate: true,
		PayLocal:       false,
		Mandatory:      true,
		Unit:           &unit,
		Type:           schema.VCT,
		Excess: &schema.PriceAmount{
			Amount:   schema.RoundedFloat(c.Deposit.Amount),
			Currency: c.Deposit.Currency,
		},
	})

	return &mapped
}

type SearchRSDepositExcessFee struct {
	Amount      float64 `xml:"amount,attr"`
	Currency    string  `xml:"currency,attr"`
	TaxIncluded bool    `xml:"taxIncluded,attr"`
}

type SearchRSKnownFees struct {
	KnownFee []SearchRSKnownFee `xml:"Fee"`
}

func (c *SearchRSKnownFees) Fees() *[]schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	for _, knownFee := range c.KnownFee {
		unit := schema.PerDay

		if strings.ToLower(knownFee.PerDuration) == "rental" {
			unit = schema.PerRental
		}

		mapped = append(mapped, schema.ExtraOrFee{
			Code: knownFee.FeeTypeName,
			Name: knownFee.FeeTypeName,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(knownFee.Amount),
				Currency: knownFee.Currency,
			},
			IncludedInRate: false,
			PayLocal:       false,
			Mandatory:      knownFee.AlwaysPayable == true,
			Unit:           &unit,
			Type:           schema.VCP,
		})
	}

	return &mapped
}

type SearchRSKnownFee struct {
	FeeTypeName   string  `xml:"feeTypeName,attr"`
	Amount        float64 `xml:"amount,attr"`
	Currency      string  `xml:"currency,attr"`
	MinAmount     float64 `xml:"minAmount,attr"`
	AlwaysPayable bool    `xml:"alwaysPayable,attr"`
	PerDuration   string  `xml:"perDuration,attr"`
}

type SearchRSExtraInfoList struct {
	ExtraInfo []SearchRSExtraInfo `xml:"ExtraInfo"`
}

func (c *SearchRSExtraInfoList) Extras() *[]schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	for _, extraInfo := range c.ExtraInfo {
		if !extraInfo.Price.PriceAvailable {
			continue
		}

		unit := schema.PerDay

		if strings.ToLower(extraInfo.Price.PricePerWhat) == "per rental" {
			unit = schema.PerRental
		}

		mapped = append(mapped, schema.ExtraOrFee{
			Code: fmt.Sprint(extraInfo.Extra.Product),
			Name: extraInfo.Extra.Name,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(extraInfo.Price.BasePrice),
				Currency: extraInfo.Price.BaseCurrency,
			},
			IncludedInRate: false,
			PayLocal:       false,
			Mandatory:      false,
			Unit:           &unit,
			MaxQuantity:    &extraInfo.Extra.Available,
			Type:           schema.EQP,
		})
	}

	return &mapped
}

type SearchRSExtraInfo struct {
	Extra SearchRSExtra      `xml:"Extra"`
	Price SearchRSExtraPrice `xml:"Price"`
}

type SearchRSExtra struct {
	Available int    `xml:"available,attr"`
	Product   int    `xml:"product,attr"`
	Name      string `xml:"Name"`
	Comments  string `xml:"Comments"`
}

type SearchRSExtraPrice struct {
	Currency       string  `xml:"currency,attr"`
	BaseCurrency   string  `xml:"baseCurrency,attr"`
	BasePrice      float64 `xml:"basePrice,attr"`
	PrePayable     string  `xml:"prePayable,attr"`
	MaxPrice       float64 `xml:"maxPrice,attr"`
	MinPrice       float64 `xml:"minPrice,attr"`
	PricePerWhat   string  `xml:"pricePerWhat,attr"`
	PriceAvailable bool    `xml:"priceAvailable,attr"`
	DriveAwayPrice float64 `xml:"driveAwayPrice,attr"`
}

type SearchRSSupplier struct {
	SupplierName string `xml:"supplierName,attr"`
}

type SearchRSRoute struct {
	PickUp  SearchRSRouteLocation `xml:"PickUp"`
	DropOff SearchRSRouteLocation `xml:"DropOff"`
}

type SearchRSRouteLocation struct {
	Location SearchRSRouteLocationInfo `xml:"Location"`
}

type SearchRSRouteLocationInfo struct {
	Id        string `xml:"id,attr"`
	LocCode   string `xml:"locCode,attr"`
	LocName   string `xml:"locName,attr"`
	OnAirport string `xml:"onAirport,attr"`
}
