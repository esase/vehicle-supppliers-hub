package ota

import "encoding/xml"

type VehResRQ struct {
	XMLName           xml.Name     `xml:"OTA_VehResRQ"`
	Xmlns             string       `xml:"xmlns,attr"`
	XmlnsXsi          string       `xml:"xmlns:xsi,attr"`
	XsiSchemaLocation string       `xml:"xsi:schemaLocation,attr"`
	Version           string       `xml:"Version,attr"`
	MaxResponses      int          `xml:"MaxResponses,attr"`
	POS               POS          `xml:"POS"`
	VehResRQCore      VehResRQCore `xml:"VehResRQCore"`
	VehResRQInfo      VehResRQInfo `xml:"VehResRQInfo"`
}

type VehResRQCore struct {
	Status            string             `xml:"Status,attr"`
	VehRentalCore     VehRentalCore      `xml:"VehRentalCore"`
	Customer          BookingCustomer    `xml:"Customer"`
	VehPref           *VehPref           `xml:"VehPref,omitempty"`
	SpecialEquipPrefs *SpecialEquipPrefs `xml:"SpecialEquipPrefs,omitempty"`
}

type VehPref struct {
	Code        string `xml:"Code,attr"`
	CodeContext string `xml:"CodeContext,attr"`
}

type BookingCustomer struct {
	Primary BookingPrimary `xml:"Primary"`
}

type BookingPrimary struct {
	PersonName  PersonName   `xml:"PersonName"`
	Telephone   *Telephone   `xml:"Telephone,omitempty"`
	Email       string       `xml:"Email,omitempty"`
	CustLoyalty *CustLoyalty `xml:"CustLoyalty,omitempty"`
}

type PersonName struct {
	GivenName string `xml:"GivenName"`
	Surname   string `xml:"Surname"`
}

type Telephone struct {
	PhoneNumber   string `xml:"PhoneNumber,attr"`
	PhoneTechType int    `xml:"PhoneTechType,attr"`
}

type VehResRQInfo struct {
	SpecialReqPref    string             `xml:"SpecialReqPref,omitempty"`
	ArrivalDetails    *ArrivalDetails    `xml:"ArrivalDetails,omitempty"`
	RentalPaymentPref *RentalPaymentPref `xml:"RentalPaymentPref,omitempty"`
	Reference         *Reference         `xml:"Reference,omitempty"`
	TourInfo          *TourInfo          `xml:"TourInfo,omitempty"`
}

type RentalPaymentPref struct {
	PaymentAmount *PaymentAmount `xml:"PaymentAmount,omitempty"`
	PaymentCard   *PaymentCard   `xml:"PaymentCard,omitempty"`
	Voucher       *Voucher       `xml:"Voucher,omitempty"`
}

type Voucher struct {
	SeriesCode    string `xml:"SeriesCode,attr"`
	BillingNumber string `xml:"BillingNumber,attr,omitempty"`
}

type PaymentAmount struct {
	Amount       string `xml:"Amount,attr"`
	CurrencyCode string `xml:"CurrencyCode,attr"`
}

type PaymentCard struct {
	CardType   int    `xml:"CardType,attr"`
	CardCode   string `xml:"CardCode,attr"`
	CardNumber string `xml:"CardNumber,attr"`
	ExpireDate string `xml:"ExpireDate,attr"`
}

type ArrivalDetails struct {
	TransportationCode string            `xml:"ArrivalDetails,attr"`
	Number             string            `xml:"Number,attr,omitempty"`
	OperatingCompany   *OperatingCompany `xml:"OperatingCompany,omitempty"`
}

type OperatingCompany struct {
	Code string `xml:"Code,attr"`
}
