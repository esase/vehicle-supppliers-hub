package json

type BookingRQ struct {
	Model                   int                    `json:"model"`
	FromDate                string                 `json:"fromDate"`
	ToDate                  string                 `json:"toDate"`
	DeliveryPlace           int                    `json:"deliveryPlace"`
	DropOffPlace            int                    `json:"dropoffPlace"`
	Additionals             *[]BookingRQAdditional `json:"additionals,omitempty"`
	ExternalSystemBookingId string                 `json:"externalSystemBookingId"`
	CommercialAgreementCode string                 `json:"commercialAgreementCode"`
	Customer                BookingRQCustomer      `json:"customer"`
}

type BookingRQCustomer struct {
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	CellPhone    string `json:"cellPhone"`
	Country      string `json:"country"`
	Age          int    `json:"age"`
}

type BookingRQAdditional struct {
	AdditionalId int `json:"additionalId"`
	Quantity     int `json:"quantity"`
}
