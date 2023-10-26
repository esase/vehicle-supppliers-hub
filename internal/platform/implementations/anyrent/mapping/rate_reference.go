package mapping

type SupplierRateReference struct {
	PickupStation  string `json:"pickupStation"`
	PickupDate     string `json:"pickupDate"`
	DropOffStation string `json:"dropOffStation"`
	DropOffDate    string `json:"dropOffDate"`
	Group          string `json:"group"`
}
