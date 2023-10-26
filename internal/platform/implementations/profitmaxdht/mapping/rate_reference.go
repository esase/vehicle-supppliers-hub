package mapping

type SupplierRateReference struct {
	FromRates                    string `json:"fromRates"`
	EstimatedTotalAmount         string `json:"estimatedTotalAmount"`
	EstimatedTotalAmountCurrency string `json:"estimatedTotalAmountCurrency"`
}

type SupplierData struct {
	LastName         string `json:"lastName"`
	ResidenceCountry string `json:"residenceCountry"`
}

func (s *SupplierData) AsMap() *map[string]interface{} {
	m := make(map[string]interface{})

	m["lastName"] = s.LastName
	m["residenceCountry"] = s.ResidenceCountry

	return &m
}
