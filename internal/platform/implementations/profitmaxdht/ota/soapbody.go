package ota

type SoapEnvBody struct {
	VehAvailRateRQ *VehAvailRateRQ `xml:"OTA_VehAvailRateRQ,omitempty"`
	VehResRQ       *VehResRQ       `xml:"OTA_VehResRQ,omitempty"`
	VehRetResRQ    *VehRetResRQ    `xml:"OTA_VehRetResRQ,omitempty"`
	VehCancelRQ    *VehCancelRQ    `xml:"OTA_VehCancelRQ,omitempty"`
}
