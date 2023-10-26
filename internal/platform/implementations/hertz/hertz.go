package hertz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/errors"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/mapping"
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

type hertz struct {
	redis         *redis.Client
	httpTransport *http.Transport
}

func (h *hertz) TrafficLightGroupingCacheKey(ctx context.Context, params schema.RatesRequestParams, logger *zerolog.Logger) string {
	configuration, _ := params.Configuration.AsHertzConfiguration()

	pickUpDateTime := params.PickUp.DateTime
	dropOffDateTime := params.DropOff.DateTime

	pickUpDate := pickUpDateTime.Format(time.DateOnly)
	duration := dropOffDateTime.Sub(pickUpDateTime).Minutes()

	keyPieces := [19]string{
		"grouping",
		"supplier-hertz",
		"5",
		params.PickUp.Code,
		params.DropOff.Code,
		pickUpDate,
		fmt.Sprintf("%.0f", duration),
		converting.Unwrap(configuration.BookingAgent),
		converting.Unwrap(configuration.Cp),
		converting.Unwrap(configuration.ClubNumber),
		converting.Unwrap(configuration.CorpDiscountNmbr),
		converting.Unwrap(configuration.PromotionCode),
		converting.Unwrap(configuration.RateQualifier),
		converting.Unwrap(configuration.Taco),
		converting.Unwrap(configuration.Tour),
		converting.Unwrap(configuration.TravelPurpose),
		converting.Unwrap(configuration.Vc),
		converting.Unwrap(configuration.Vn),
		configuration.VendorCode,
	}

	return strings.ToLower(strings.Join(keyPieces[:], ":"))
}

func (h *hertz) GetRates(ctx context.Context, params schema.RatesRequestParams, logger *zerolog.Logger) (schema.RatesResponse, error) {
	configuration, _ := params.Configuration.AsHertzConfiguration()
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

	existingBooking := converting.Unwrap(params.Booking)

	if existingBooking.SupplierBookingReference != "" && len(rates.Vehicles) > 0 {
		quoteRequest := quoteRequest{
			params:        params,
			configuration: configuration,
			logger:        logger,
			slowLogger:    slowLogger,
		}

		return quoteRequest.Execute(ctx, h.httpTransport, rates, ratesRequest.Extras())
	}

	return rates, nil
}

func (h *hertz) CreateBooking(ctx context.Context, params schema.BookingRequestParams, logger *zerolog.Logger) (schema.BookingResponse, error) {
	configuration, _ := params.Configuration.AsHertzConfiguration()

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

func (h *hertz) CancelBooking(ctx context.Context, params schema.CancelRequestParams, logger *zerolog.Logger) (schema.CancelResponse, error) {
	configuration, _ := params.Configuration.AsHertzConfiguration()

	bookingCancel := cancelRequest{
		params:        params,
		configuration: configuration,
		logger:        logger,
	}

	return bookingCancel.Execute(h.httpTransport)
}

func (h *hertz) ModifyBooking(ctx context.Context, params schema.ModifyRequestParams, logger *zerolog.Logger) (schema.ModifyResponse, error) {
	configuration, _ := params.Configuration.AsHertzConfiguration()

	var supplierRateReference mapping.SupplierRateReference

	err := json.Unmarshal([]byte(params.SupplierRateReference), &supplierRateReference)
	if err != nil {
		return schema.ModifyResponse{}, errors.ErrorInvalidRateReference
	}

	modifyRequest := modifyRequest{
		params:                      params,
		configuration:               configuration,
		supplierRateReference:       supplierRateReference,
		supplierSpecificInformation: params.SupplierSpecificInformation,
		logger:                      logger,
	}

	return modifyRequest.Execute(h.httpTransport)
}

func New(redisClient *redis.Client) *hertz {
	transport := http.DefaultTransport.(*http.Transport)
	// improves durations a lot
	transport.DisableKeepAlives = true

	return &hertz{
		redis:         redisClient,
		httpTransport: transport,
	}
}
