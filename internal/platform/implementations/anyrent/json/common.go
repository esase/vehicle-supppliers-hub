package json

import "bitbucket.org/crgw/supplier-hub/internal/schema"

type PaginationMeta struct {
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Total       int `json:"total"`
	Count       int `json:"count"`
	PerPage     int `json:"per_page"`
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
}

type BookingStatus string

const (
	BookingStatusConfirmed BookingStatus = "CONFIRMED"
	BookingStatusPending   BookingStatus = "PENDING"
	BookingStatusExpired   BookingStatus = "EXPIRED"
	BookingStatusNoShow    BookingStatus = "NO_SHOW"
	BookingStatusFulfilled BookingStatus = "FULFILLED"
	BookingStatusCanceled  BookingStatus = "CANCELED"
)

type BookingInfo struct {
	Status BookingStatus `json:"status"`
}

func (b *BookingInfo) GetBookingStatus() schema.BookingStatusResponseStatus {
	switch b.Status {
	case BookingStatusConfirmed:
		return schema.BookingStatusResponseStatusOK

	case BookingStatusCanceled:
		return schema.BookingStatusResponseStatusCANCELLED

	case BookingStatusPending:
		return schema.BookingStatusResponseStatusPENDING

	default:
		return schema.BookingStatusResponseStatusFAILED
	}
}
