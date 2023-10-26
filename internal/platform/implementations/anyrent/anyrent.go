package anyrent

import (
	"context"
	"encoding/json"
	"net/http"

	"bitbucket.org/crgw/supplier-hub/internal/platform/errors"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type anyRent struct {
	redis         *redis.Client
	httpTransport *http.Transport
}

func (a *anyRent) GetLocations(ctx context.Context, params schema.LocationsRequestParams, logger *zerolog.Logger) (schema.LocationsResponse, error) {
	configuration, _ := params.Configuration.AsAnyRentConfiguration()
	slowLogger := slowlog.CreateLogger(logger)

	locationsRequest := locationsRequest{
		cache:         caching.NewRedisCache(a.redis),
		params:        params,
		configuration: configuration,
		logger:        logger,
		slowLogger:    slowLogger,
	}

	locations, err := locationsRequest.Execute(ctx, a.httpTransport)
	if err != nil {
		return locations, err
	}

	return locations, nil
}

func (a *anyRent) GetRates(ctx context.Context, params schema.RatesRequestParams, logger *zerolog.Logger) (schema.RatesResponse, error) {
	configuration, _ := params.Configuration.AsAnyRentConfiguration()
	slowLogger := slowlog.CreateLogger(logger)

	ratesRequest := ratesRequest{
		cache:         caching.NewRedisCache(a.redis),
		params:        params,
		configuration: configuration,
		logger:        logger,
		slowLogger:    slowLogger,
	}

	rates, err := ratesRequest.Execute(ctx, a.httpTransport)
	if err != nil {
		return rates, err
	}

	return rates, nil
}

func (a *anyRent) CreateBooking(ctx context.Context, params schema.BookingRequestParams, logger *zerolog.Logger) (schema.BookingResponse, error) {
	configuration, _ := params.Configuration.AsAnyRentConfiguration()

	var supplierRateReference mapping.SupplierRateReference
	err := json.Unmarshal([]byte(params.SupplierRateReference), &supplierRateReference)
	if err != nil {
		return schema.BookingResponse{}, errors.ErrorInvalidRateReference
	}

	bookingRequest := bookingRequest{
		cache:                 caching.NewRedisCache(a.redis),
		params:                params,
		configuration:         configuration,
		supplierRateReference: supplierRateReference,
		logger:                logger,
	}

	return bookingRequest.Execute(a.httpTransport)
}

func (a *anyRent) GetBookingStatus(ctx context.Context, params schema.BookingStatusRequestParams, logger *zerolog.Logger) (schema.BookingStatusResponse, error) {
	configuration, _ := params.Configuration.AsAnyRentConfiguration()

	bookingStatusRequest := bookingStatusRequest{
		cache:         caching.NewRedisCache(a.redis),
		params:        params,
		configuration: configuration,
		logger:        logger,
	}

	return bookingStatusRequest.Execute(a.httpTransport)
}

func (a *anyRent) CancelBooking(ctx context.Context, params schema.CancelRequestParams, logger *zerolog.Logger) (schema.CancelResponse, error) {
	configuration, _ := params.Configuration.AsAnyRentConfiguration()

	bookingCancel := cancelRequest{
		cache:         caching.NewRedisCache(a.redis),
		params:        params,
		configuration: configuration,
		logger:        logger,
	}

	return bookingCancel.Execute(a.httpTransport)
}

func New(redisClient *redis.Client) *anyRent {
	transport := http.DefaultTransport.(*http.Transport)
	// improves durations a lot
	transport.DisableKeepAlives = true

	return &anyRent{
		redis:         redisClient,
		httpTransport: transport,
	}
}
