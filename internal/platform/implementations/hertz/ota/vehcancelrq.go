package ota

import "encoding/xml"

type VehCancelRQ struct {
	XMLName           xml.Name        `xml:"OTA_VehCancelRQ"`
	Xmlns             string          `xml:"xmlns,attr"`
	XmlnsXsi          string          `xml:"xmlns:xsi,attr"`
	XmlnsXsd          string          `xml:"xmlns:xsd,attr"`
	XsiSchemaLocation string          `xml:"xsi:schemaLocation,attr"`
	Version           string          `xml:"Version,attr"`
	POS               POS             `xml:"POS"`
	VehCancelRQCore   VehCancelRQCore `xml:"VehCancelRQCore"`
}

type VehCancelRQCore struct {
	CancelType string            `xml:"CancelType,attr"`
	UniqueID   *UniqueID         `xml:"UniqueID"`
	PersonName *CancelPersonName `xml:"PersonName"`
}

type CancelPersonName struct {
	GivenName string `xml:"GivenName,omitempty"`
	Surname   string `xml:"Surname"`
}

type UniqueID struct {
	Type string `xml:"Type,attr"`
	ID   string `xml:"ID,attr"`
}
