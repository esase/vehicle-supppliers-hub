package json

type BookingRQ struct {
	PickupStation  string            `json:"pickup_station"`
	PickupDate     string            `json:"pickup_date"`
	DropOffStation string            `json:"dropoff_station"`
	DropOffDate    string            `json:"dropoff_date"`
	Group          string            `json:"group"`
	DriverAge      int               `json:"driver_age"`
	ArrivalFlight  *string           `json:"arrival_flight,omitempty"`
	Reference      string            `json:"reference"`
	Extras         *string           `json:"extras,omitempty"`
	Taxes          *string           `json:"taxes,omitempty"`
	Drivers        []BookingRQDriver `json:"drivers"`
}

type BookingRQDriver struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Country string `json:"country"`
}
