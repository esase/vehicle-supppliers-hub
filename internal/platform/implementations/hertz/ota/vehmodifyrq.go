package ota

import "encoding/xml"

type VehModifyRQ struct {
	XMLName           xml.Name        `xml:"OTA_VehModifyRQ"`
	Xmlns             string          `xml:"xmlns,attr"`
	XmlnsXsi          string          `xml:"xmlns:xsi,attr"`
	XsiSchemaLocation string          `xml:"xsi:schemaLocation,attr"`
	Version           string          `xml:"Version,attr"`
	POS               POS             `xml:"POS"`
	VehModifyRQCore   VehModifyRQCore `xml:"VehModifyRQCore"`
	VehModifyRQInfo   VehModifyRQInfo `xml:"VehModifyRQInfo"`
}

type VehModifyRQCore struct {
	Status            string             `xml:"Status,attr"`
	ModifyType        string             `xml:"ModifyType,attr"`
	UniqueID          UniqueID           `xml:"UniqueID"`
	VehRentalCore     VehRentalCore      `xml:"VehRentalCore"`
	Customer          BookingCustomer    `xml:"Customer"`
	VehPref           *VehPref           `xml:"VehPref,omitempty"`
	SpecialEquipPrefs *SpecialEquipPrefs `xml:"SpecialEquipPrefs,omitempty"`
}

type VehModifyRQInfo struct {
	SpecialReqPref    string             `xml:"SpecialReqPref,omitempty"`
	ArrivalDetails    *ArrivalDetails    `xml:"ArrivalDetails,omitempty"`
	RentalPaymentPref *RentalPaymentPref `xml:"RentalPaymentPref,omitempty"`
	Reference         *Reference         `xml:"Reference,omitempty"`
}
