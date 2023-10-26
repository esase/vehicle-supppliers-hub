package json

import "bitbucket.org/crgw/supplier-hub/internal/schema"

type BookingStatus string

const (
	BookingStatusConfirmed BookingStatus = "Confirmed"
	BookingStatusReserved  BookingStatus = "Reserved"
	BookingStatusCanceled  BookingStatus = "Canceled"
	BookingStatusDelivered BookingStatus = "Delivered"
	BookingStatusClosed    BookingStatus = "Closed"
	BookingStatusQuoted    BookingStatus = "Quoted"
)

type BookingRS struct {
	Id     string        `json:"id"`
	Status BookingStatus `json:"status"`
}

func (b *BookingRS) GetBookingStatus() schema.BookingStatusResponseStatus {
	switch *&b.Status {
	case BookingStatusConfirmed,
		BookingStatusReserved:
		return schema.BookingStatusResponseStatusOK

	case BookingStatusCanceled:
		return schema.BookingStatusResponseStatusCANCELLED

	default:
		return schema.BookingStatusResponseStatusFAILED
	}
}
