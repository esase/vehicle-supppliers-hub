package schema

import (
	"sync"
)

type errorsBucket struct {
	errors SupplierResponseErrors
	sync.Mutex
}

func NewErrorsBucket() errorsBucket {
	return errorsBucket{
		errors: []SupplierResponseError{},
	}
}

func (e *errorsBucket) AddErrors(errors []SupplierResponseError) {
	e.Lock()
	e.errors = append(e.errors, errors...)
	e.Unlock()
}

func (e *errorsBucket) AddError(err SupplierResponseError) {
	e.Lock()
	e.errors = append(e.errors, err)
	e.Unlock()
}

func (e *errorsBucket) Errors() *SupplierResponseErrors {
	return &e.errors
}

func NewSupplierError(msg string) SupplierResponseError {
	return SupplierResponseError{
		Code:    SupplierError,
		Message: msg,
	}
}

func NewTimeoutError(msg string) SupplierResponseError {
	return SupplierResponseError{
		Code:    TimeoutError,
		Message: msg,
	}
}

func NewConnectionError(msg string) SupplierResponseError {
	return SupplierResponseError{
		Code:    ConnectionError,
		Message: msg,
	}
}
