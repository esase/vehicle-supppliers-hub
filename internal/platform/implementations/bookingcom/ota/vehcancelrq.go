package ota

type CancelBookingRQ struct {
	Credentials
	Email  string `xml:"Email"`
	Reason string `xml:"Reason"`
	Booking
}
