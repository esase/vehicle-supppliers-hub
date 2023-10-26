package schema

import "fmt"

type RoundedFloat float64

func (f RoundedFloat) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.2f", f)), nil
}

type PaymentType int

const (
	PaymentTypeFullPrepay    PaymentType = 0
	PaymentTypePartialPrepay PaymentType = 1
)
