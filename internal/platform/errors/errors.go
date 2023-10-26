package errors

import "errors"

var (
	ErrorNotImplemented                  = errors.New("not implemented")
	ErrorInvalidRateReference            = errors.New("invalid rate reference")
	ErrorMissingSupplierPassthroughToken = errors.New("supplier passthrough token missing")
)
