package ota

import "bitbucket.org/crgw/supplier-hub/internal/schema"

type Credentials struct {
	Credentials CredentialsInfo `xml:"Credentials"`
}

type CredentialsInfo struct {
	Username string `xml:"username,attr"`
	Password string `xml:"password,attr"`
}

type PickUp struct {
	Location Location `xml:"Location"`
	Date     Date     `xml:"Date"`
}

type DropOff struct {
	Location Location `xml:"Location"`
	Date     Date     `xml:"Date"`
}

type Location struct {
	Location string `xml:"id,attr"`
}

type Date struct {
	Year   int `xml:"year,attr"`
	Month  int `xml:"month,attr"`
	Day    int `xml:"day,attr"`
	Hour   int `xml:"hour,attr"`
	Minute int `xml:"minute,attr"`
}

type Version struct {
	Version string `xml:"version,attr"`
}

type BookingStatus string

const (
	BookingStatusCancelled                  BookingStatus = "cancelled"
	BookingStatusConfirmed                  BookingStatus = "confirmed"
	BookingStatusAccepted                   BookingStatus = "accepted"
	BookingStatusManualConfirmationRequired BookingStatus = "manual confirmation required"
	BookingStatusUnconfirmedModification    BookingStatus = "unconfirmed modification"
	BookingStatusCheck                      BookingStatus = "check"
	BookingStatusPaymentFailed              BookingStatus = "payment failed"
	BookingStatusCompleted                  BookingStatus = "completed"
)

type Booking struct {
	Booking BookingInfo `xml:"Booking"`
}

type BookingInfo struct {
	Id            string         `xml:"id,attr"`
	StatusMessage *BookingStatus `xml:"status,attr,omitempty"`
	StatusCode    *int           `xml:"statusCode,attr,omitempty"`
}

func (c *BookingInfo) Status() schema.BookingStatusResponseStatus {
	switch *c.StatusMessage {
	case BookingStatusConfirmed,
		BookingStatusCompleted:
		return schema.BookingStatusResponseStatusOK

	case BookingStatusAccepted,
		BookingStatusManualConfirmationRequired,
		BookingStatusUnconfirmedModification,
		BookingStatusCheck,
		BookingStatusPaymentFailed:
		return schema.BookingStatusResponseStatusPENDING
	}

	return schema.BookingStatusResponseStatusFAILED
}
