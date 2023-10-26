package converting

// If nil returns the default value for the type
func Unwrap[T any](x *T) (r T) {
	if x != nil {
		r = *x
	}

	return
}

func PointerToValue[T any](v T) *T {
	return &v
}
