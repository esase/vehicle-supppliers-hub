package interfaces

import (
	"context"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"github.com/rs/zerolog"
)

type WithTrafficLightRatesGrouping interface {
	TrafficLightGroupingCacheKey(context.Context, schema.RatesRequestParams, *zerolog.Logger) string
}

type WithGetRates interface {
	GetRates(context.Context, schema.RatesRequestParams, *zerolog.Logger) (schema.RatesResponse, error)
}

type WithCreateBooking interface {
	CreateBooking(context.Context, schema.BookingRequestParams, *zerolog.Logger) (schema.BookingResponse, error)
}

type WithModifyBooking interface {
	ModifyBooking(context.Context, schema.ModifyRequestParams, *zerolog.Logger) (schema.ModifyResponse, error)
}

type WithCancelBooking interface {
	CancelBooking(context.Context, schema.CancelRequestParams, *zerolog.Logger) (schema.CancelResponse, error)
}

type WithBookingStatus interface {
	GetBookingStatus(context.Context, schema.BookingStatusRequestParams, *zerolog.Logger) (schema.BookingStatusResponse, error)
}

type WithLocations interface {
	GetLocations(context.Context, schema.LocationsRequestParams, *zerolog.Logger) (schema.LocationsResponse, error)
}
