package json

type PricesAndAvailabilityRQ struct {
	PickupStation  string `url:"pickup_station"`
	PickupDate     string `url:"pickup_date"`
	DropOffStation string `url:"dropoff_station"`
	DropOffDate    string `url:"dropoff_date"`
	DriverAge      int    `url:"driver_age"`
}
