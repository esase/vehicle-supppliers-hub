package ota

import "encoding/xml"

type VehResRS struct {
	XMLName      xml.Name     `xml:"OTA_VehResRS"`
	VehResRSCore VehResRSCore `xml:"VehResRSCore"`
	ErrorsMixin
}

type VehResRSCore struct {
	VehReservation VehReservation `xml:"VehReservation"`
}

type VehReservation struct {
	Customer       Customer       `xml:"Customer"`
	VehSegmentCore VehSegmentCore `xml:"VehSegmentCore"`
	VehSegmentInfo VehSegmentInfo `xml:"VehSegmentInfo"`
}

type VehSegmentInfo struct {
	PaymentRules    PaymentRules    `xml:"PaymentRules"`
	PricedCoverages PricedCoverages `xml:"PricedCoverages"`
	LocationDetails LocationDetails `xml:"LocationDetails"`
}

type LocationDetails struct {
	Code                 string           `xml:"Code,attr"`
	Name                 string           `xml:"Name,attr"`
	ExtendedLocationCode string           `xml:"ExtendedLocationCode,attr"`
	Address              Address          `xml:"Address"`
	Telephone            BookingTelephone `xml:"Telephone"`
}

type BookingTelephone struct {
	PhoneLocationType int    `xml:"PhoneLocationType,attr"`
	PhoneTechType     int    `xml:"PhoneTechType,attr"`
	PhoneNumber       string `xml:"PhoneNumber,attr"`
	FormattedInd      bool   `xml:"FormattedInd,attr"`
}

type Address struct {
	FormattedInd bool     `xml:"FormattedInd,attr"`
	AddressLine  []string `xml:"AddressLine"`
	CityName     string   `xml:"CityName"`
	PostalCode   string   `xml:"PostalCode"`
	CountryName  string   `xml:"CountryName"`
}

type VehSegmentCore struct {
	ConfID        ConfID         `xml:"ConfID"`
	Vendor        Vendor         `xml:"Vendor"`
	VehRentalCore VehRentalCore  `xml:"VehRentalCore"`
	Vehicle       BookingVehicle `xml:"Vehicle"`
	RentalRate    RentalRate     `xml:"RentalRate"`
	Fees          Fees           `xml:"Fees"`
	TotalCharge   TotalCharge    `xml:"TotalCharge"`
}

type BookingVehicle struct {
	PassengerQuantity int          `xml:"PassengerQuantity,attr"`
	BaggageQuantity   int          `xml:"BaggageQuantity,attr"`
	AirConditionInd   bool         `xml:"AirConditionInd,attr"`
	TransmissionType  string       `xml:"TransmissionType,attr"`
	FuelType          string       `xml:"FuelType,attr"`
	DriveType         string       `xml:"DriveType,attr"`
	Code              string       `xml:"Code,attr"`
	CodeContext       string       `xml:"CodeContext,attr"`
	VehType           VehType      `xml:"VehType"`
	VehClass          VehClass     `xml:"VehClass"`
	VehMakeModel      VehMakeModel `xml:"VehMakeModel"`
	PictureURL        string       `xml:"PictureURL"`
}

type VehClass struct {
	Size string `xml:"Size,attr"`
}

type Vendor struct {
	Code string `xml:"Code,attr"`
}

type ConfID struct {
	Type string `xml:"Type,attr"`
	ID   string `xml:"ID,attr"`
}
