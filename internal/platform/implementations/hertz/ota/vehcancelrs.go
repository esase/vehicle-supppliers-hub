package ota

import "encoding/xml"

type VehCancelRS struct {
	XMLName         xml.Name        `xml:"OTA_VehCancelRS"`
	Xmlns           string          `xml:"xmlns,attr"`
	Version         string          `xml:"Version,attr"`
	TargetName      string          `xml:"TargetName,attr"`
	VehCancelRSCore VehCancelRSCore `xml:"VehCancelRSCore"`
	ErrorsMixin
}

type VehCancelRSCore struct {
	CancelStatus CoreCancelStatus `xml:"CancelStatus,attr"`
	UniqueID     UniqueID
}

type CoreCancelStatus string

const CoreCancelStatusCancelled CoreCancelStatus = "Cancelled"
