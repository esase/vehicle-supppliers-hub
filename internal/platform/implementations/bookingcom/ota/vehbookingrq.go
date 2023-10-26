package ota

import "bitbucket.org/crgw/supplier-hub/internal/schema"

type MakeBookingRQ struct {
	Version
	CorrelationId    string `xml:"correlationId,attr"`
	InsuranceVersion string `xml:"insuranceVersion,attr"`
	ResidenceCountry string `xml:"cor,attr"`
	Credentials
	Booking MakeBookingRQBooking `xml:"Booking"`
}

type MakeBookingRQBooking struct {
	PickUp        PickUp                     `xml:"PickUp"`
	DropOff       DropOff                    `xml:"DropOff"`
	ExtraList     *MakeBookingRQExtraList    `xml:"ExtraList,omitempty"`
	Vehicle       MakeBookingRQVehicle       `xml:"Vehicle"`
	DriverInfo    MakeBookingRQDriverInfo    `xml:"DriverInfo"`
	PaymentInfo   MakeBookingRQPaymentInfo   `xml:"PaymentInfo"`
	AirlineInfo   *MakeBookingRQAirlineInfo  `xml:"AirlineInfo,omitempty"`
	AcceptedPrice MakeBookingRQAcceptedPrice `xml:"AcceptedPrice"`
}

type MakeBookingRQExtraList struct {
	Extra []MakeBookingRQExtra `xml:"Extra"`
}

type MakeBookingRQExtra struct {
	Id     string `xml:"id,attr"`
	Amount int    `xml:"amount,attr"`
}

type MakeBookingRQVehicle struct {
	Id string `xml:"id,attr"`
}

type MakeBookingRQDriverInfo struct {
	DriverName MakeBookingRQDriverName `xml:"DriverName"`
	Address    MakeBookingRQAddress    `xml:"Address"`
	Email      string                  `xml:"Email"`
	Telephone  string                  `xml:"Telephone"`
	DriverAge  int                     `xml:"DriverAge"`
}

type MakeBookingRQDriverName struct {
	Title     *string `xml:"title,attr,omitempty"`
	FirstName string  `xml:"firstname,attr"`
	LastName  string  `xml:"lastname,attr"`
}

type MakeBookingRQAddress struct {
	Country  string  `xml:"country,attr"`
	City     *string `xml:"city,attr,omitempty"`
	Street   *string `xml:"street,attr,omitempty"`
	PostCode *string `xml:"postcode,attr,omitempty"`
}

type MakeBookingRQPaymentInfo struct {
	DepositPayment bool                      `xml:"depositPayment,attr"`
	CardVaultToken string                    `xml:"CardVaultToken"`
	ThreeDSecure   MakeBookingRQThreeDSecure `xml:"ThreeDSecure"`
}

type MakeBookingRQThreeDSecure struct {
	Eci       string `xml:"Eci"`
	Cavv      string `xml:"Cavv"`
	Aav       string `xml:"Aav"`
	Aevv      string `xml:"Aevv"`
	DsTransId string `xml:"DsTransId"`
}

type MakeBookingRQAirlineInfo struct {
	FlightNo string `xml:"flightNo,attr"`
}

type MakeBookingRQAcceptedPrice struct {
	Price    schema.RoundedFloat `xml:"price,attr"`
	Currency string              `xml:"currency,attr"`
}
