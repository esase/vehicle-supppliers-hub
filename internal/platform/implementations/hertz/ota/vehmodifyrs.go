package ota

import "encoding/xml"

type VehModifyRS struct {
	ErrorsMixin
	XMLName         xml.Name        `xml:"OTA_VehModifyRS"`
	EchoToken       string          `xml:"EchoToken,attr"`
	VehModifyRSCore VehModifyRSCore `xml:"VehModifyRSCore"`
}

type VehModifyRSCore struct {
	VehReservation VehReservation `xml:"VehReservation"`
}
