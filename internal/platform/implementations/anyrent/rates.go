package anyrent

import (
	"context"
	jsonEncoding "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent/json"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/google/go-querystring/query"
	"github.com/rs/zerolog"
)

type ratesRequest struct {
	cache         *caching.Cacher
	params        schema.RatesRequestParams
	configuration schema.AnyRentConfiguration
	logger        *zerolog.Logger
	slowLogger    slowlog.Logger
}

func (r *ratesRequest) Execute(ctx context.Context, httpTransport *http.Transport) (schema.RatesResponse, error) {
	rates := schema.RatesResponse{
		Vehicles: []schema.Vehicle{},
	}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	rates.SupplierRequests = requestsBucket.SupplierRequests()
	rates.Errors = errorsBucket.Errors()

	// fetch auth token
	authRequest := authRequest{
		configuration: r.configuration,
		logger:        r.logger,
		timeout:       r.params.Timeouts.Default,
		cache:         r.cache,
	}

	auth, err := authRequest.Execute(httpTransport)
	requestsBucket.AddRequests(*auth.SupplierRequests)
	errorsBucket.AddErrors(*auth.Errors)

	if err != nil {
		return rates, err
	}

	if auth.Token == nil {
		return rates, nil
	}

	timeout := r.params.Timeouts.Default
	if r.params.Timeouts.Rates != nil {
		timeout = *r.params.Timeouts.Rates
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: httpTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(r.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	response, err := r.makeRequest(client, *auth.Token)

	if err != nil {
		errorsBucket.AddError(schema.NewSupplierError(err.Error()))
		return rates, nil
	}

	for _, fleet := range response.Fleets {
		for _, group := range fleet.Groups {
			// skip not available vehicles
			if group.Status != json.VehicleStatusAvailable {
				continue
			}

			vehicle, err := r.parseVehicle(group)
			if err != nil {
				errorsBucket.AddError(schema.NewSupplierError(err.Error()))
				return rates, err
			}

			rates.Vehicles = append(rates.Vehicles, vehicle)
		}
	}

	return rates, nil
}

func (r *ratesRequest) makeRequest(
	client *http.Client,
	token string,
) (json.PricesAndAvailabilityRS, error) {
	pickupDate := strings.ReplaceAll(r.params.PickUp.DateTime.Format(time.RFC3339), "-", "")
	pickupDate = strings.ReplaceAll(pickupDate, ":", "")

	dropOffDate := strings.ReplaceAll(r.params.DropOff.DateTime.Format(time.RFC3339), "-", "")
	dropOffDate = strings.ReplaceAll(dropOffDate, ":", "")

	opt := json.PricesAndAvailabilityRQ{
		PickupStation:  r.params.PickUp.Code,
		PickupDate:     pickupDate,
		DropOffStation: r.params.DropOff.Code,
		DropOffDate:    dropOffDate,
		DriverAge:      r.params.Age,
	}
	v, _ := query.Values(opt)

	url := fmt.Sprintf("%v/v1/prices?%v", r.configuration.SupplierApiUrl, v.Encode())
	c := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Rates)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodGet, url, http.NoBody)
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("x-lang", "en")

	rs, err := requesting.RequestErrors(client.Do(httpRequest))
	if err != nil {
		return json.PricesAndAvailabilityRS{}, errors.New(err.Message)
	}
	defer rs.Body.Close()

	// bind the response body to the json
	bodyBytes, _ := io.ReadAll(rs.Body)
	rs.Body.Close()

	var jsonRatesResponse json.PricesAndAvailabilityRS
	jsonEncodeErr := jsonEncoding.Unmarshal(bodyBytes, &jsonRatesResponse)
	if jsonEncodeErr != nil {
		return json.PricesAndAvailabilityRS{}, errors.New(jsonEncodeErr.Error())
	}

	message := jsonRatesResponse.ErrorMessage()
	if message != "" {
		return json.PricesAndAvailabilityRS{}, errors.New(message)
	}

	return jsonRatesResponse, nil
}

func (r *ratesRequest) parseVehicle(group json.PricesAndAvailabilityRSFleetGroup) (schema.Vehicle, error) {
	qualifier, err := jsonEncoding.Marshal(mapping.SupplierRateReference{
		PickupStation:  r.params.PickUp.Code,
		PickupDate:     r.params.PickUp.DateTime.Format(time.DateTime),
		DropOffStation: r.params.DropOff.Code,
		DropOffDate:    r.params.DropOff.DateTime.Format(time.DateTime),
		Group:          group.Code,
	})

	if err != nil {
		return schema.Vehicle{}, err
	}

	rateReference := string(qualifier)

	// we ignore the `sipp` code if it less then 4 symbols
	var sipCode *string = nil
	if len(group.SippCode) == 4 {
		sipCode = &group.SippCode
	}

	var includedDistance *string = nil
	if group.Rate.IncludedMileage > 0 {
		distanceString := strconv.FormatInt(int64(group.Rate.IncludedMileage), 10)
		includedDistance = &distanceString
	}

	unlimitedDistance := group.Rate.IncludedMileage == 0

	// process both the required and optional extras and fees
	extrasAndFees := []schema.ExtraOrFee{}
	extrasAndFees = append(extrasAndFees, group.Rate.GetCustomCoverages()...)

	extrasAndFees = append(extrasAndFees, group.Rate.GetRequiredCoverages(group.Rate.Currency)...)
	extrasAndFees = append(extrasAndFees, group.Rate.GetRequiredFees(group.Rate.Currency)...)
	extrasAndFees = append(extrasAndFees, group.Rate.GetRequiredExtras(group.Rate.Currency)...)

	collectedRequiredExtraAndFeeCodes := make(map[string]struct{}, len(extrasAndFees))

	for _, extraAndFeeInfo := range extrasAndFees {
		collectedRequiredExtraAndFeeCodes[extraAndFeeInfo.Code] = struct{}{}
	}

	extrasAndFees = append(extrasAndFees, group.OptionalsRates.GetOptionalCoverages(group.Rate.Currency, collectedRequiredExtraAndFeeCodes)...)
	extrasAndFees = append(extrasAndFees, group.OptionalsRates.GetOptionalFees(group.Rate.Currency, collectedRequiredExtraAndFeeCodes)...)
	extrasAndFees = append(extrasAndFees, group.OptionalsRates.GetOptionalExtras(group.Rate.Currency, collectedRequiredExtraAndFeeCodes)...)

	vehicle := schema.Vehicle{
		Name:  group.GetName(),
		Class: group.Code,
		Price: schema.PriceAmount{
			Amount:   schema.RoundedFloat(group.Rate.RateAfterTax),
			Currency: group.Rate.Currency,
		},
		AcrissCode:       sipCode,
		HasAirco:         group.Features.GetAirCondition(),
		Doors:            group.Features.GetDoors(),
		Seats:            group.Features.GetSeats(),
		BigSuitcases:     group.Features.GetSuitCases(),
		TransmissionType: group.Features.GetTransmission(),
		FuelType:         group.Features.GetFuel(),
		Mileage: &schema.Mileage{
			IncludedDistance: includedDistance,
			Unlimited:        &unlimitedDistance,
		},
		SupplierRateReference: &rateReference,
		Status:                schema.AVAILABLE,
		ExtrasAndFees:         &extrasAndFees,
	}

	return vehicle, nil
}
