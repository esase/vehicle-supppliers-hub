package json

type PricesAndAvailabilityRQ struct {
	DeliveryLocation        int    `url:"DeliveryLocation"`
	DropOffLocation         int    `url:"DropoffLocation"`
	From                    string `url:"From"`
	To                      string `url:"To"`
	DriverAge               int    `url:"DriverAge"`
	CommercialAgreementCode string `url:"CommercialAgreementCode"`
	ReturnAdditionalPrice   bool   `url:"ReturnAdditionalsPrice"`
}
