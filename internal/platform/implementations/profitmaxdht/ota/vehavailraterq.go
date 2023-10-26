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
	XMLName        xml.Name       `xml:"OTA_VehAvailRateRQ"`
	Xmlns          string         `xml:"xmlns,attr"`
	XmlnsXsi       string         `xml:"xmlns:xsi,attr"`
	Version        string         `xml:"Version,attr"`
	Target         string         `xml:"Target,attr"`
	SequenceNmbr   string         `xml:"SequenceNmbr,attr"`
	MaxResponses   int            `xml:"MaxResponses,attr"`
	POS            POS            `xml:"POS"`
	VehAvailRQCore VehAvailRQCore `xml:"VehAvailRQCore"`
	VehAvailRQInfo VehAvailRQInfo `xml:"VehAvailRQInfo"`
}

type Location struct {
	LocationCode string `xml:"LocationCode,attr"`
	CodeContext  string `xml:"CodeContext,attr"`
}

type VehRentalCore struct {
	PickUpDateTime string    `xml:"PickUpDateTime,attr"`
	ReturnDateTime string    `xml:"ReturnDateTime,attr"`
	PickUpLocation *Location `xml:"PickUpLocation"`
	ReturnLocation *Location `xml:"ReturnLocation"`
}

type RateQualifier struct {
	RateQualifier string `xml:"RateQualifier,attr,omitempty"`
	TravelPurpose string `xml:"TravelPurpose,attr,omitempty"`
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
	VehPrefs          *VehPrefs          `xml:"VehPrefs"`
	RateQualifier     RateQualifier      `xml:"RateQualifier,omitempty"`
	SpecialEquipPrefs *SpecialEquipPrefs `xml:"SpecialEquipPrefs,omitempty"`
}

type VehPrefs struct {
	VehPref []VehPref `xml:"VehPref"`
}

type VehPref struct {
	AirConditionInd  string   `xml:"AirConditionInd,attr,omitempty"`
	TransmissionType string   `xml:"TransmissionType,attr,omitempty"`
	FuelType         string   `xml:"FuelType,attr,omitempty"`
	DriveType        string   `xml:"DriveType,attr,omitempty"`
	UpSellInd        string   `xml:"UpSellInd,attr,omitempty"`
	VehType          VehType  `xml:"VehType"`
	VehClass         VehClass `xml:"VehClass"`
}

type VehClass struct {
	Size string `xml:"Size,attr"`
}

type VehAvailRQInfo struct {
	TourInfo *TourInfo `xml:"TourInfo,omitempty"`
}

type TourInfo struct {
	TourNumber string `xml:"TourNumber,attr,omitempty"`
}
