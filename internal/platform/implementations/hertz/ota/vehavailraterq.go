package ota

import "encoding/xml"

type CompanyName struct {
	Code        string `xml:"Code,attr"`
	CodeContext string `xml:"CodeContext,attr"`
}

type RequestorID struct {
	Type        string       `xml:"Type,attr"`
	ID          string       `xml:"ID,attr"`
	CompanyName *CompanyName `xml:"CompanyName,omitempty"`
}

type VehAvailRateRQ struct {
	XMLName           xml.Name       `xml:"OTA_VehAvailRateRQ"`
	Xmlns             string         `xml:"xmlns,attr"`
	XmlnsXsi          string         `xml:"xmlns:xsi,attr"`
	XsiSchemaLocation string         `xml:"xsi:schemaLocation,attr"`
	Version           string         `xml:"Version,attr"`
	MaxResponses      int            `xml:"MaxResponses,attr"`
	POS               POS            `xml:"POS"`
	VehAvailRQCore    VehAvailRQCore `xml:"VehAvailRQCore"`
	VehAvailRQInfo    VehAvailRQInfo `xml:"VehAvailRQInfo"`
}

type Location struct {
	LocationCode string `xml:"LocationCode,attr"`
}

type VehRentalCore struct {
	PickUpDateTime string    `xml:"PickUpDateTime,attr"`
	ReturnDateTime string    `xml:"ReturnDateTime,attr"`
	PickUpLocation Location  `xml:"PickUpLocation"`
	ReturnLocation *Location `xml:"ReturnLocation"`
}

type RateQualifier struct {
	CorpDiscountNmbr string `xml:"CorpDiscountNmbr,attr,omitempty"`
	RateQualifier    string `xml:"RateQualifier,attr,omitempty"`
	PromotionCode    string `xml:"PromotionCode,attr,omitempty"`
	TravelPurpose    string `xml:"TravelPurpose,attr,omitempty"`
}

type SpecialEquipPref struct {
	EquipType string `xml:"EquipType,attr"`
	Quantity  int    `xml:"Quantity,attr"`
}

type SpecialEquipPrefs struct {
	SpecialEquipPref []SpecialEquipPref `xml:"SpecialEquipPref"`
}

type VehAvailRQCore struct {
	Status            string             `xml:"Status,attr"`
	VehRentalCore     VehRentalCore      `xml:"VehRentalCore"`
	RateQualifier     RateQualifier      `xml:"RateQualifier,omitempty"`
	SpecialEquipPrefs *SpecialEquipPrefs `xml:"SpecialEquipPrefs,omitempty"`
}

type TourInfo struct {
	TourNumber string `xml:"TourNumber,attr"`
}

type CustLoyalty struct {
	MembershipID string `xml:"MembershipID,attr"`
	ProgramID    string `xml:"ProgramID,attr"`
	TravelSector string `xml:"TravelSector,attr"`
}

type Primary struct {
	CustLoyalty *CustLoyalty `xml:"CustLoyalty,omitempty"`
}

type Customer struct {
	Primary *Primary `xml:"Primary"`
}

type VehAvailRQInfo struct {
	TourInfo *TourInfo `xml:"TourInfo,omitempty"`
	Customer *Customer `xml:"Customer,omitempty"`
}
