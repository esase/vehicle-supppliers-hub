package bookingcom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/errors"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type bookingCom struct {
	redis         *redis.Client
	httpTransport *http.Transport
}

func (h *bookingCom) TrafficLightGroupingCacheKey(ctx context.Context, params schema.RatesRequestParams, logger *zerolog.Logger) string {
	configuration, _ := params.Configuration.AsBookingComConfiguration()

	pickUpDateTime := params.PickUp.DateTime
	dropOffDateTime := params.DropOff.DateTime

	pickUpDate := pickUpDateTime.Format(time.DateOnly)
	duration := dropOffDateTime.Sub(pickUpDateTime).Minutes()

	keyPieces := [8]string{
		"grouping",
		"supplier-booking-com",
		"5",
		params.PickUp.Code,
		params.DropOff.Code,
		pickUpDate,
		fmt.Sprintf("%.0f", duration),
		configuration.SupplierName,
	}

	return strings.ToLower(strings.Join(keyPieces[:], ":"))
}

func (h *bookingCom) GetRates(ctx context.Context, params schema.RatesRequestParams, logger *zerolog.Logger) (schema.RatesResponse, error) {
	configuration, _ := params.Configuration.AsBookingComConfiguration()
	slowLogger := slowlog.CreateLogger(logger)

	ratesRequest := RatesRequest{
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

func (h *bookingCom) CreateBooking(ctx context.Context, params schema.BookingRequestParams, logger *zerolog.Logger) (schema.BookingResponse, error) {
	configuration, _ := params.Configuration.AsBookingComConfiguration()

	var supplierRateReference mapping.SupplierRateReference
	err := json.Unmarshal([]byte(params.SupplierRateReference), &supplierRateReference)
	if err != nil {
		return schema.BookingResponse{}, errors.ErrorInvalidRateReference
	}

	if params.SupplierPassthrough == nil {
		return schema.BookingResponse{}, errors.ErrorMissingSupplierPassthroughToken
	}

	bookingRequest := bookingRequest{
		params:                params,
		configuration:         configuration,
		supplierRateReference: supplierRateReference,
		logger:                logger,
	}

	return bookingRequest.Execute(h.httpTransport)
}

func (h *bookingCom) GetBookingStatus(ctx context.Context, params schema.BookingStatusRequestParams, logger *zerolog.Logger) (schema.BookingStatusResponse, error) {
	configuration, _ := params.Configuration.AsBookingComConfiguration()

	bookingStatusRequest := bookingStatusRequest{
		params:        params,
		configuration: configuration,
		logger:        logger,
	}

	return bookingStatusRequest.Execute(h.httpTransport)
}

func (h *bookingCom) CancelBooking(ctx context.Context, params schema.CancelRequestParams, logger *zerolog.Logger) (schema.CancelResponse, error) {
	configuration, _ := params.Configuration.AsBookingComConfiguration()

	bookingCancel := cancelRequest{
		params:        params,
		configuration: configuration,
		logger:        logger,
	}

	return bookingCancel.Execute(h.httpTransport)
}

func New(redisClient *redis.Client) *bookingCom {
	transport := http.DefaultTransport.(*http.Transport)
	// improves durations a lot
	transport.DisableKeepAlives = true

	return &bookingCom{
		redis:         redisClient,
		httpTransport: transport,
	}
}
