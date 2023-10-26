package bookingcom

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/rs/zerolog"
)

type RatesRequest struct {
	cache         *caching.Cacher
	params        schema.RatesRequestParams
	configuration schema.BookingComConfiguration
	logger        *zerolog.Logger
	slowLogger    slowlog.Logger
}

func (r *RatesRequest) Execute(ctx context.Context, httpTransport *http.Transport) (schema.RatesResponse, error) {
	r.slowLogger.Start("booking-com:rates:execute:client")

	rates := schema.RatesResponse{
		Vehicles: []schema.Vehicle{},
	}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	rates.SupplierRequests = requestsBucket.SupplierRequests()
	rates.Errors = errorsBucket.Errors()

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

	r.slowLogger.Stop("booking-com:rates:execute:client")

	r.slowLogger.Start("booking-com:rates:execute:requests")

	ratesResChannel := make(chan ota.SearchRS, 1)
	ratesErrChannel := make(chan schema.SupplierResponseError, 1)
	defer close(ratesResChannel)
	defer close(ratesErrChannel)

	r.ratesRequest(ctx, client, ratesResChannel, ratesErrChannel)

	var vehAvailRateRS ota.SearchRS

	select {
	case vehAvailRateRS = <-ratesResChannel:
		break

	case ratesErr := <-ratesErrChannel:
		errorsBucket.AddError(ratesErr)
		return rates, nil
	}

	r.slowLogger.Stop("booking-com:rates:execute:requests")

	r.slowLogger.Start("booking-com:rates:execute:mapVehicles")

	for _, vehAvail := range vehAvailRateRS.MatchList.Match {
		// skip not checked vehicles and filter be the supplier name
		if !vehAvail.Vehicle.AvailabilityCheck || strings.ToLower(vehAvail.Supplier.SupplierName) != strings.ToLower(r.configuration.SupplierName) {
			continue
		}

		vehicle, err := r.parseVehicle(vehAvail)
		if err != nil {
			errorsBucket.AddError(schema.NewSupplierError(err.Error()))
			continue
		}

		rates.Vehicles = append(rates.Vehicles, vehicle)
	}

	r.slowLogger.Stop("booking-com:rates:execute:mapVehicles")

	return rates, nil
}

func (r *RatesRequest) ratesRequest(
	ctx context.Context,
	client *http.Client,
	resChannel chan<- ota.SearchRS,
	errChannel chan<- schema.SupplierResponseError,
) {
	requestBody := r.requestBody()

	c := context.WithValue(ctx, schema.RequestingTypeKey, schema.Rates)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodPost, r.configuration.SupplierApiUrl, bytes.NewBuffer([]byte(requestBody)))
	httpRequest.Header.Set("Content-Type", "application/xml")
	httpRequest.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36")

	go func() {
		defer r.recoverPanic(errChannel)

		rs, e := requesting.RequestErrors(client.Do(httpRequest))
		if e != nil {
			errChannel <- *e
			return
		}
		defer rs.Body.Close()

		var ratesResponse ota.SearchRS
		bodyBytes, _ := io.ReadAll(rs.Body)
		rs.Body.Close()

		err := xml.Unmarshal(bodyBytes, &ratesResponse)
		if err != nil {
			errChannel <- schema.NewSupplierError("unable to parse the body")
			return
		}

		message := ratesResponse.ErrorMessage()
		if message != "" {
			errChannel <- schema.NewSupplierError(message)
			return
		}

		resChannel <- ratesResponse
	}()
}

func (r *RatesRequest) requestBody() string {
	pickupLocationCode := r.params.PickUp.Code
	if r.params.PickUp.Iata != nil {
		pickupLocationCode = *r.params.PickUp.Iata
	}

	dropOffLocationCode := r.params.DropOff.Code
	if r.params.DropOff.Iata != nil {
		dropOffLocationCode = *r.params.DropOff.Iata
	}

	requestBody := ota.SearchRQ{
		Version: ota.Version{
			Version: "1.1",
		},
		ReturnExtras:     true,
		SupplierInfo:     true,
		ResidenceCountry: r.params.ResidenceCountry,
		Credentials: ota.Credentials{
			Credentials: ota.CredentialsInfo{
				Username: r.configuration.Username,
				Password: r.configuration.Password,
			},
		},
		PickUp: ota.PickUp{
			Location: ota.Location{
				Location: pickupLocationCode,
			},
			Date: ota.Date{
				Year:   r.params.PickUp.DateTime.Year(),
				Month:  int(r.params.PickUp.DateTime.Month()),
				Day:    r.params.PickUp.DateTime.Day(),
				Hour:   r.params.PickUp.DateTime.Hour(),
				Minute: r.params.PickUp.DateTime.Minute(),
			},
		},
		DropOff: ota.DropOff{
			Location: ota.Location{
				Location: dropOffLocationCode,
			},
			Date: ota.Date{
				Year:   r.params.DropOff.DateTime.Year(),
				Month:  int(r.params.DropOff.DateTime.Month()),
				Day:    r.params.DropOff.DateTime.Day(),
				Hour:   r.params.DropOff.DateTime.Hour(),
				Minute: r.params.DropOff.DateTime.Minute(),
			},
		},
		DriverAge: r.params.Age,
	}

	xmlString, _ := xml.MarshalIndent(requestBody, "", "    ")

	return string(xmlString)
}

func (r *RatesRequest) recoverPanic(errChannel chan<- schema.SupplierResponseError) {
	if err := recover(); err != nil {
		errChannel <- schema.NewConnectionError("requesting supplier failed")
		r.logger.Err(fmt.Errorf("%v", string(debug.Stack()))).Msg(fmt.Sprintf("Recovered from a panic: %v", err))
	}
}

func (r *RatesRequest) parseVehicle(vehAvail ota.SearchRSMatch) (schema.Vehicle, error) {
	seats, _ := strconv.Atoi(vehAvail.Vehicle.Seats)
	doors, _ := strconv.Atoi(vehAvail.Vehicle.Doors)
	bigSuitcases, _ := strconv.Atoi(vehAvail.Vehicle.BigSuitcase)
	smallSuitcases, _ := strconv.Atoi(vehAvail.Vehicle.SmallSuitcase)

	qualifier, err := json.Marshal(mapping.SupplierRateReference{
		VehicleId:         vehAvail.Vehicle.Id,
		PickUpLocationId:  vehAvail.Route.PickUp.Location.Id,
		DropOffLocationId: vehAvail.Route.DropOff.Location.Id,
		BasePrice:         schema.RoundedFloat(vehAvail.Price.BasePrice),
		BaseCurrency:      vehAvail.Price.BaseCurrency,
	})

	if err != nil {
		return schema.Vehicle{}, err
	}

	rateReference := string(qualifier)

	extrasAndFees := []schema.ExtraOrFee{}
	extrasAndFees = append(extrasAndFees, *vehAvail.Fees.DepositExcessFees.Coverages()...)
	extrasAndFees = append(extrasAndFees, *vehAvail.Fees.KnownFees.Fees()...)
	extrasAndFees = append(extrasAndFees, *vehAvail.ExtraInfoList.Extras()...)

	vehicle := schema.Vehicle{
		Name:  vehAvail.Vehicle.Name,
		Class: vehAvail.Vehicle.Group,
		Price: schema.PriceAmount{
			Amount:   schema.RoundedFloat(vehAvail.Price.BasePrice),
			Currency: vehAvail.Price.BaseCurrency,
		},
		AcrissCode:       &vehAvail.Vehicle.Group,
		HasAirco:         vehAvail.Vehicle.AirCondition(),
		Doors:            &doors,
		Seats:            &seats,
		BigSuitcases:     &bigSuitcases,
		SmallSuitcases:   &smallSuitcases,
		TransmissionType: vehAvail.Vehicle.Transmission(),
		FuelType:         vehAvail.Vehicle.Fuel(),
		ImageUrl:         &vehAvail.Vehicle.LargeImageURL,
		Mileage: &schema.Mileage{
			Unlimited: &vehAvail.Vehicle.UnlimitedMileage,
		},
		SupplierRateReference: &rateReference,
		Status:                schema.AVAILABLE,
		ExtrasAndFees:         &extrasAndFees,
	}

	return vehicle, nil
}
