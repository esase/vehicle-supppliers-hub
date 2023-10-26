package ota

import "encoding/xml"

type VehRetResRQ struct {
	XMLName           xml.Name        `xml:"OTA_VehRetResRQ"`
	Xmlns             string          `xml:"xmlns,attr"`
	XmlnsXsi          string          `xml:"xmlns:xsi,attr"`
	XsiSchemaLocation string          `xml:"xsi:schemaLocation,attr"`
	Version           string          `xml:"Version,attr"`
	Target            string          `xml:"Target,attr"`
	POS               POS             `xml:"POS"`
	VehRetResRQCore   VehRetResRQCore `xml:"VehRetResRQCore"`
}

type VehRetResRQCore struct {
	UniqueID   UniqueID   `xml:"UniqueID"`
	PersonName PersonName `xml:"PersonName"`
}
