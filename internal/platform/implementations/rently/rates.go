package rently

import (
	"context"
	jsonEncoding "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/rently/json"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/rently/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/google/go-querystring/query"
	"github.com/rs/zerolog"
)

type ratesRequest struct {
	params        schema.RatesRequestParams
	configuration schema.RentlyConfiguration
	logger        *zerolog.Logger
	slowLogger    slowlog.Logger
	cache         *caching.Cacher
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

	for _, vehicle := range response {
		vehicle, err := r.parseVehicle(vehicle)
		if err != nil {
			errorsBucket.AddError(schema.NewSupplierError(err.Error()))
			return rates, err
		}

		rates.Vehicles = append(rates.Vehicles, vehicle)
	}

	return rates, nil
}

func (r *ratesRequest) makeRequest(
	client *http.Client,
	token string,
) ([]json.PricesAndAvailabilityRS, error) {
	deliveryLocation, _ := strconv.Atoi(r.params.PickUp.Code)
	dropOffLocation, _ := strconv.Atoi(r.params.DropOff.Code)

	opt := json.PricesAndAvailabilityRQ{
		DeliveryLocation:        deliveryLocation,
		DropOffLocation:         dropOffLocation,
		From:                    r.params.PickUp.DateTime.Format(time.DateTime),
		To:                      r.params.DropOff.DateTime.Format(time.DateTime),
		DriverAge:               r.params.Age,
		CommercialAgreementCode: string(r.configuration.CommercialAgreementCode),
		ReturnAdditionalPrice:   true,
	}
	v, _ := query.Values(opt)

	url := fmt.Sprintf("%v/api/AvailabilityByPlace?%v", r.configuration.SupplierApiUrl, v.Encode())
	c := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Rates)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodGet, url, http.NoBody)
	httpRequest.Header.Set("Authorization", "Bearer "+token)

	rs, err := requesting.RequestErrors(client.Do(httpRequest))
	if err != nil {
		return []json.PricesAndAvailabilityRS{}, errors.New(err.Message)
	}
	defer rs.Body.Close()

	// bind the response body to the json
	bodyBytes, _ := io.ReadAll(rs.Body)
	rs.Body.Close()

	var jsonRatesResponse []json.PricesAndAvailabilityRS
	jsonEncodeErr := jsonEncoding.Unmarshal(bodyBytes, &jsonRatesResponse)
	if jsonEncodeErr != nil {
		return []json.PricesAndAvailabilityRS{}, errors.New(jsonEncodeErr.Error())
	}

	return jsonRatesResponse, nil
}

func (r *ratesRequest) parseVehicle(supplierVehicle json.PricesAndAvailabilityRS) (schema.Vehicle, error) {
	qualifier, err := jsonEncoding.Marshal(mapping.SupplierRateReference{
		Model: supplierVehicle.Model.Id,
	})

	if err != nil {
		return schema.Vehicle{}, err
	}
	rateReference := string(qualifier)

	unlimitedMilage := !supplierVehicle.LimitedKm

	extrasAndFees := []schema.ExtraOrFee{}
	extrasAndFees = append(extrasAndFees, supplierVehicle.Model.GetCoverages(supplierVehicle.Currency)...)
	extrasAndFees = append(extrasAndFees, supplierVehicle.GetFeesAndExtras()...)

	vehicle := schema.Vehicle{
		Name:  supplierVehicle.Model.GetName(),
		Class: supplierVehicle.Model.Sipp,
		Price: schema.PriceAmount{
			Amount:   schema.RoundedFloat(supplierVehicle.Price),
			Currency: supplierVehicle.Currency,
		},
		AcrissCode:       &supplierVehicle.Model.Sipp,
		HasAirco:         &supplierVehicle.Model.HasAirCondition,
		Doors:            &supplierVehicle.Model.Doors,
		Seats:            &supplierVehicle.Model.Passengers,
		BigSuitcases:     &supplierVehicle.Model.BigLuggage,
		SmallSuitcases:   &supplierVehicle.Model.SmallLuggage,
		TransmissionType: supplierVehicle.Model.GetTransmission(),
		ImageUrl:         &supplierVehicle.Model.ImagePath,
		Mileage: &schema.Mileage{
			Unlimited: &unlimitedMilage,
		},
		SupplierRateReference: &rateReference,
		Status:                schema.AVAILABLE,
		ExtrasAndFees:         &extrasAndFees,
	}

	return vehicle, nil
}
