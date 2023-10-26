package profitmaxdht

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/errors"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/profitmaxdht/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

const (
	defaultMaxResponses = 10
)

type profitmaxdht struct {
	redis         *redis.Client
	httpTransport *http.Transport
}

func (h *profitmaxdht) TrafficLightGroupingCacheKey(ctx context.Context, params schema.RatesRequestParams, logger *zerolog.Logger) string {
	configuration, _ := params.Configuration.AsProfitMaxDHTConfiguration()

	pickUpDateTime := params.PickUp.DateTime
	dropOffDateTime := params.DropOff.DateTime

	pickUpDate := pickUpDateTime.Format(time.DateOnly)
	duration := dropOffDateTime.Sub(pickUpDateTime).Minutes()

	keyPieces := [19]string{
		"grouping",
		"supplier-profitmaxdht",
		"5",
		params.PickUp.Code,
		params.DropOff.Code,
		pickUpDate,
		fmt.Sprintf("%.0f", duration),
		converting.Unwrap(configuration.Cp),
		converting.Unwrap(configuration.RateQualifier),
		converting.Unwrap(configuration.TravelPurpose),
		converting.Unwrap(configuration.Vn),
		converting.Unwrap(configuration.VendorCode),
		configuration.Destination,
	}

	return strings.ToLower(strings.Join(keyPieces[:], ":"))
}

func (h *profitmaxdht) GetRates(ctx context.Context, params schema.RatesRequestParams, logger *zerolog.Logger) (schema.RatesResponse, error) {
	configuration, _ := params.Configuration.AsProfitMaxDHTConfiguration()
	slowLogger := slowlog.CreateLogger(logger)

	ratesRequest := ratesRequest{
		cache:         caching.NewRedisCache(h.redis),
		params:        params,
		configuration: configuration,
		logger:        logger,
		slowLogger:    slowLogger,
	}

	rates, err := ratesRequest.Execute(ctx, h.httpTransport)
	if err != nil {
		return rates, err
	}

	return rates, nil
}

func (h *profitmaxdht) CancelBooking(ctx context.Context, params schema.CancelRequestParams, logger *zerolog.Logger) (schema.CancelResponse, error) {
	configuration, _ := params.Configuration.AsProfitMaxDHTConfiguration()

	bookingCancel := cancelRequest{
		params:        params,
		configuration: configuration,
		logger:        logger,
	}

	return bookingCancel.Execute(h.httpTransport)
}

func (h *profitmaxdht) GetBookingStatus(ctx context.Context, params schema.BookingStatusRequestParams, logger *zerolog.Logger) (schema.BookingStatusResponse, error) {
	configuration, _ := params.Configuration.AsProfitMaxDHTConfiguration()

	bookingStatus := bookingStatusRequest{
		params:        params,
		configuration: configuration,
		logger:        logger,
	}

	return bookingStatus.Execute(h.httpTransport)
}

func (h *profitmaxdht) CreateBooking(ctx context.Context, params schema.BookingRequestParams, logger *zerolog.Logger) (schema.BookingResponse, error) {
	configuration, _ := params.Configuration.AsProfitMaxDHTConfiguration()

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
	}

	return bookingRequest.Execute(h.httpTransport)
}

func New(redisClient *redis.Client) *profitmaxdht {
	transport := http.DefaultTransport.(*http.Transport)
	// improves durations a lot
	transport.DisableKeepAlives = true

	return &profitmaxdht{
		redis:         redisClient,
		httpTransport: transport,
	}
}
