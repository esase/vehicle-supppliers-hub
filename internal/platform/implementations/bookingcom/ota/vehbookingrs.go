package ota

type MakeBookingRS struct {
	Version
	InsuranceVersion string `xml:"insuranceVersion,attr"`
	Booking
	Errors
}
