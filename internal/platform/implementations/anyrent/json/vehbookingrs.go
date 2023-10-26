package json

import (
	"strconv"
)

type BookingRS struct {
	Errors
	Booking BookingRSBooking `json:"booking"`
}

type BookingRSBooking struct {
	BookingInfo
	Id         int     `json:"id"`
	Value      float32 `json:"value"`
	Currency   string  `json:"currency"`
	DriverId   int     `json:"driver_id"`
	CustomerId int     `json:"customer_id"`
}

func (b *BookingRSBooking) GetId() string {
	return strconv.Itoa(b.Id)
}
