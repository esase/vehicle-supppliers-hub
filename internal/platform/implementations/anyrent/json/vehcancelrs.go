package json

type CancelBookingRS struct {
	Errors
	Message string `json:"message"`
}
