package anyrent

import (
	"bytes"
	"context"
	jsonEncoding "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent/json"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type bookingRequest struct {
	cache                 *caching.Cacher
	params                schema.BookingRequestParams
	configuration         schema.AnyRentConfiguration
	supplierRateReference mapping.SupplierRateReference
	logger                *zerolog.Logger
}

func (b *bookingRequest) Execute(httpTransport *http.Transport) (schema.BookingResponse, error) {
	booking := schema.BookingResponse{}
	booking.Status = schema.BookingResponseStatusFAILED

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	booking.SupplierRequests = requestsBucket.SupplierRequests()
	booking.Errors = errorsBucket.Errors()

	// fetch auth token
	authRequest := authRequest{
		configuration: b.configuration,
		logger:        b.logger,
		timeout:       b.params.Timeouts.Default,
		cache:         b.cache,
	}

	auth, err := authRequest.Execute(httpTransport)
	requestsBucket.AddRequests(*auth.SupplierRequests)
	errorsBucket.AddErrors(*auth.Errors)

	if err != nil {
		return booking, err
	}

	if auth.Token == nil {
		return booking, nil
	}

	timeout := b.params.Timeouts.Default
	if b.params.Timeouts.Booking != nil {
		timeout = *b.params.Timeouts.Booking
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: httpTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(b.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	response, err := b.makeRequest(client, *auth.Token)

	if err != nil {
		errorsBucket.AddError(schema.NewSupplierError(err.Error()))
		return booking, nil
	}

	booking.Status = schema.BookingResponseStatus(response.Booking.GetBookingStatus())
	booking.SupplierBookingReference = converting.PointerToValue(response.Booking.GetId())

	return booking, nil
}

func (b *bookingRequest) makeRequest(
	client *http.Client,
	token string,
) (json.BookingRS, error) {
	body := bytes.NewBuffer(b.requestBody())

	url := b.configuration.SupplierApiUrl + "/v1/bookings"
	c := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Booking)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodPost, url, body)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("x-lang", "en")

	rs, err := requesting.RequestErrors(client.Do(httpRequest))
	if err != nil {
		return json.BookingRS{}, errors.New(err.Message)
	}
	defer rs.Body.Close()

	// bind the response body to the json
	bodyBytes, _ := io.ReadAll(rs.Body)
	rs.Body.Close()

	var jsonBookingResponse json.BookingRS
	jsonEncodeErr := jsonEncoding.Unmarshal(bodyBytes, &jsonBookingResponse)
	if jsonEncodeErr != nil {
		return json.BookingRS{}, errors.New(jsonEncodeErr.Error())
	}

	message := jsonBookingResponse.ErrorMessage()
	if message != "" {
		return json.BookingRS{}, errors.New(message)
	}

	return jsonBookingResponse, nil
}

func (b *bookingRequest) requestBody() []byte {
	name := fmt.Sprintf("%v %v", converting.LatinCharacters(b.params.Customer.FirstName), converting.LatinCharacters(b.params.Customer.LastName))

	var flightNo *string = nil
	if b.params.FlightNo != nil {
		flightNo = b.params.FlightNo
	}

	extras := ""
	taxes := ""

	// process list of taxes and extras and their quantity
	if b.params.ExtrasAndFees != nil {
		for _, extra := range *b.params.ExtrasAndFees {
			quantity := 1
			if extra.Quantity != nil {
				quantity = *extra.Quantity
			}

			codes := strings.Repeat(extra.Code+",", quantity)

			switch extra.Type {
			case schema.Extra:
				extras = extras + codes

			default:
				taxes = taxes + codes
			}
		}
	}

	var extraList *string = nil
	if extras != "" {
		extraList = converting.PointerToValue(strings.TrimRight(extras, ","))
	}

	var taxList *string = nil
	if taxes != "" {
		taxList = converting.PointerToValue(strings.TrimRight(taxes, ","))
	}

	json, _ := jsonEncoding.MarshalIndent(&json.BookingRQ{
		PickupStation:  b.supplierRateReference.PickupStation,
		PickupDate:     b.supplierRateReference.PickupDate,
		DropOffStation: b.supplierRateReference.DropOffStation,
		DropOffDate:    b.supplierRateReference.DropOffDate,
		Reference:      b.params.ReservNumber,
		DriverAge:      b.params.Customer.Age,
		ArrivalFlight:  flightNo,
		Group:          b.supplierRateReference.Group,
		Extras:         extraList,
		Taxes:          taxList,
		Drivers: []json.BookingRQDriver{{
			Name:    name,
			Phone:   b.params.Customer.Phone,
			Email:   string(b.params.Customer.Email),
			Country: b.params.Customer.ResidenceCountry,
		}},
	}, "", "	")

	return json
}
