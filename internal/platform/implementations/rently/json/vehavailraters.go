package json

import (
	"strconv"
	"strings"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
)

type PricesAndAvailabilityRS struct {
	Model            PricesAndAvailabilityRSModel              `json:"model"`
	Category         PricesAndAvailabilityRSCategory           `json:"category"`
	Price            float64                                   `json:"price"`
	Franchise        float64                                   `json:"franchise"`
	LimitedKm        bool                                      `json:"ilimitedKm"`
	AdditionalPrices []PricesAndAvailabilityRSAdditionalPrices `json:"additionalsPrices"`
	Currency         string                                    `json:"currency"`
}

func (r *PricesAndAvailabilityRS) GetFeesAndExtras() []schema.ExtraOrFee {
	var mapped []schema.ExtraOrFee

	for _, feeInfo := range r.AdditionalPrices {
		feeType := schema.VCP // fee
		if feeInfo.Type == Additional {
			feeType = schema.EQP // extra
		}

		mapped = append(mapped, schema.ExtraOrFee{
			Code: strconv.Itoa(feeInfo.Id),
			Name: feeInfo.Name,
			Price: schema.PriceAmount{
				Amount:   schema.RoundedFloat(feeInfo.Price),
				Currency: feeInfo.Currency,
			},
			IncludedInRate: feeInfo.IsRequired,
			PayLocal:       true,
			Mandatory:      feeInfo.IsRequired,
			Unit:           converting.PointerToValue(schema.PerRental),
			MaxQuantity:    &feeInfo.MaxQuantityPerBooking,
			Type:           feeType,
		})
	}

	return mapped
}

type PricesAndAvailabilityRSModel struct {
	Id                int      `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Sipp              string   `json:"sipp"`
	Brand             string   `json:"brand"`
	Passengers        int      `json:"passangers"`
	Doors             int      `json:"doors"`
	TransmissionType  string   `json:"transmissionType"`
	BigLuggage        int      `json:"bigLuggage"`
	SmallLuggage      int      `json:"smallLuggage"`
	HasAirCondition   bool     `json:"hasAirCondition"`
	ImagePath         string   `json:"imagePath"`
	Franchise         *float64 `json:"franchise,omitempty"`
	FranchiseDamage   *float64 `json:"franchiseDamage,omitempty"`
	FranchiseRollover *float64 `json:"franchiseRollover,omitempty"`
	FranchiseTheft    *float64 `json:"franchiseTheft,omitempty"`
}

func (m *PricesAndAvailabilityRSModel) GetName() string {
	return m.Brand + " " + m.Name
}

func (m *PricesAndAvailabilityRSModel) GetTransmission() *schema.VehicleTransmissionType {
	switch strings.ToLower(m.TransmissionType) {
	case "manual":
		return converting.PointerToValue(schema.Manual)

	default:
		return converting.PointerToValue(schema.Automatic)
	}
}

func (m *PricesAndAvailabilityRSModel) GetCoverages(currency string) []schema.ExtraOrFee {
	unit := schema.PerRental
	var mapped []schema.ExtraOrFee

	if m.FranchiseTheft != nil {
		mapped = append(mapped, schema.ExtraOrFee{
			Code: "TheftExcess",
			Name: "TheftExcess",
			Price: schema.PriceAmount{
				Amount:   0,
				Currency: currency,
			},
			IncludedInRate: true,
			PayLocal:       false,
			Mandatory:      true,
			Unit:           &unit,
			Type:           schema.VCT,
			Excess: &schema.PriceAmount{
				Amount:   schema.RoundedFloat(*m.FranchiseTheft),
				Currency: currency,
			},
		})
	}

	if m.FranchiseDamage != nil {
		mapped = append(mapped, schema.ExtraOrFee{
			Code: "DamageExcess",
			Name: "DamageExcess",
			Price: schema.PriceAmount{
				Amount:   0,
				Currency: currency,
			},
			IncludedInRate: true,
			PayLocal:       false,
			Mandatory:      true,
			Unit:           &unit,
			Type:           schema.VCT,
			Excess: &schema.PriceAmount{
				Amount:   schema.RoundedFloat(*m.FranchiseDamage),
				Currency: currency,
			},
		})
	}

	if m.FranchiseRollover != nil {
		mapped = append(mapped, schema.ExtraOrFee{
			Code: "RolloverExcess",
			Name: "RolloverExcess",
			Price: schema.PriceAmount{
				Amount:   0,
				Currency: currency,
			},
			IncludedInRate: true,
			PayLocal:       false,
			Mandatory:      true,
			Unit:           &unit,
			Type:           schema.VCT,
			Excess: &schema.PriceAmount{
				Amount:   schema.RoundedFloat(*m.FranchiseRollover),
				Currency: currency,
			},
		})
	}

	return mapped
}

type PricesAndAvailabilityRSCategory struct {
	Id    int    `json:"id"`
	Order int    `json:"order"`
	Name  string `json:"name"`
}

type AdditionalType string

const (
	Additional AdditionalType = "Additional"
	Insurance  AdditionalType = "Insurance"
)

type PricesAndAvailabilityRSAdditionalPrices struct {
	Id                    int            `json:"id"`
	Name                  string         `json:"name"`
	Description           string         `json:"description"`
	ImagePath             string         `json:"imagePath"`
	IsPriceByDay          bool           `json:"isPriceByDay"`
	Price                 float64        `json:"price"`
	Taxes                 float64        `json:"taxes"`
	PriceWithoutTaxes     float64        `json:"priceWithoutTaxes"`
	DailyPrice            float64        `json:"dailyPrice"`
	MaxQuantityPerBooking int            `json:"maxQuantityPerBooking"`
	Currency              string         `json:"currency"`
	AvailableStock        int            `json:"availableStock"`
	Type                  AdditionalType `json:"type"`
	IsRequired            bool           `json:"isRequired"`
	IsDefault             bool           `json:"isDefault"`
}
