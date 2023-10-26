package ota

import "encoding/xml"

type VehResRQ struct {
	XMLName           xml.Name     `xml:"OTA_VehResRQ"`
	Xmlns             string       `xml:"xmlns,attr"`
	XmlnsXsi          string       `xml:"xmlns:xsi,attr"`
	XsiSchemaLocation string       `xml:"xsi:schemaLocation,attr"`
	Version           string       `xml:"Version,attr"`
	Target            string       `xml:"Target,attr"`
	POS               POS          `xml:"POS"`
	VehResRQCore      VehResRQCore `xml:"VehResRQCore"`
	VehResRQInfo      VehResRQInfo `xml:"VehResRQInfo"`
}

type VehResRQCore struct {
	Status            string             `xml:"Status,attr"`
	VehRentalCore     VehRentalCore      `xml:"VehRentalCore"`
	Customer          BookingCustomer    `xml:"Customer"`
	SpecialEquipPrefs *SpecialEquipPrefs `xml:"SpecialEquipPrefs,omitempty"`
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

type CustLoyalty struct {
	MembershipID string `xml:"MembershipID,attr"`
	ProgramID    string `xml:"ProgramID,attr"`
	TravelSector string `xml:"TravelSector,attr"`
}

type PersonName struct {
	GivenName string `xml:"GivenName,omitempty"`
	Surname   string `xml:"Surname,omitempty"`
}

type Telephone struct {
	PhoneNumber   string `xml:"PhoneNumber,attr"`
	PhoneTechType int    `xml:"PhoneTechType,attr"`
}

type VehResRQInfo struct {
	SpecialReqPref    string             `xml:"SpecialReqPref,omitempty"`
	RentalPaymentPref *RentalPaymentPref `xml:"RentalPaymentPref,omitempty"`
	Reference         *Reference         `xml:"Reference,omitempty"`
	TourInfo          *TourInfo          `xml:"TourInfo,omitempty"`
}

type RentalPaymentPref struct {
	Voucher *Voucher `xml:"Voucher,omitempty"`
}

type Voucher struct {
	SeriesCode    string `xml:"SeriesCode,attr"`
	BillingNumber string `xml:"BillingNumber,attr,omitempty"`
}
