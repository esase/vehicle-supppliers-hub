package ota

import "encoding/xml"

type VehRetResRS struct {
	XMLName         xml.Name        `xml:"OTA_VehRetResRS"`
	VehRetResRSCore VehRetResRSCore `xml:"VehRetResRSCore"`
	ErrorsMixin
}

type VehRetResRSCore struct {
	VehReservation VehReservation `xml:"VehReservation"`
}
