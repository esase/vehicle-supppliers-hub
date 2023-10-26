package rently

import (
	"context"
	"encoding/json"
	"net/http"

	"bitbucket.org/crgw/supplier-hub/internal/platform/errors"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/rently/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type rentlyCar struct {
	redis         *redis.Client
	httpTransport *http.Transport
}

func (r *rentlyCar) GetLocations(ctx context.Context, params schema.LocationsRequestParams, logger *zerolog.Logger) (schema.LocationsResponse, error) {
	configuration, _ := params.Configuration.AsRentlyConfiguration()
	slowLogger := slowlog.CreateLogger(logger)

	locationsRequest := locationsRequest{
		params:        params,
		configuration: configuration,
		logger:        logger,
		slowLogger:    slowLogger,
		cache:         caching.NewRedisCache(r.redis),
	}

	locations, err := locationsRequest.Execute(r.httpTransport)
	if err != nil {
		return locations, err
	}

	return locations, nil
}

func (a *rentlyCar) GetRates(ctx context.Context, params schema.RatesRequestParams, logger *zerolog.Logger) (schema.RatesResponse, error) {
	configuration, _ := params.Configuration.AsRentlyConfiguration()
	slowLogger := slowlog.CreateLogger(logger)

	ratesRequest := ratesRequest{
		params:        params,
		configuration: configuration,
		logger:        logger,
		slowLogger:    slowLogger,
		cache:         caching.NewRedisCache(a.redis),
	}

	rates, err := ratesRequest.Execute(ctx, a.httpTransport)
	if err != nil {
		return rates, err
	}

	return rates, nil
}

func (a *rentlyCar) CreateBooking(ctx context.Context, params schema.BookingRequestParams, logger *zerolog.Logger) (schema.BookingResponse, error) {
	configuration, _ := params.Configuration.AsRentlyConfiguration()

	var supplierRateReference mapping.SupplierRateReference
	err := json.Unmarshal([]byte(params.SupplierRateReference), &supplierRateReference)
	if err != nil {
		return schema.BookingResponse{}, errors.ErrorInvalidRateReference
	}

	bookingRequest := bookingRequest{
		params:                params,
		configuration:         configuration,
		supplierRateReference: supplierRateReference,
		logger:                logger,
		cache:                 caching.NewRedisCache(a.redis),
	}

	return bookingRequest.Execute(a.httpTransport)
}

func (a *rentlyCar) CancelBooking(ctx context.Context, params schema.CancelRequestParams, logger *zerolog.Logger) (schema.CancelResponse, error) {
	configuration, _ := params.Configuration.AsRentlyConfiguration()

	bookingCancel := cancelRequest{
		params:        params,
		configuration: configuration,
		logger:        logger,
		cache:         caching.NewRedisCache(a.redis),
	}

	return bookingCancel.Execute(a.httpTransport)
}

func New(redisClient *redis.Client) *rentlyCar {
	transport := http.DefaultTransport.(*http.Transport)
	// improves durations a lot
	transport.DisableKeepAlives = true

	return &rentlyCar{
		redis:         redisClient,
		httpTransport: transport,
	}
}
