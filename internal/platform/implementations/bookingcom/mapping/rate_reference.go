package mapping

import "bitbucket.org/crgw/supplier-hub/internal/schema"

type SupplierRateReference struct {
	VehicleId         string              `json:"vehicleId"`
	PickUpLocationId  string              `json:"pickUpLocationId"`
	DropOffLocationId string              `json:"dropOffLocationId"`
	BasePrice         schema.RoundedFloat `json:"basePrice"`
	BaseCurrency      string              `json:"baseCurrency"`
}
