package ota

type BookingStatusRQ struct {
	Version
	Credentials
	Booking
	Email string `xml:"Email"`
}
