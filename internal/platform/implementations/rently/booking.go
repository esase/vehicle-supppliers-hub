package rently

import (
	"bytes"
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
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type bookingRequest struct {
	params                schema.BookingRequestParams
	configuration         schema.RentlyConfiguration
	supplierRateReference mapping.SupplierRateReference
	logger                *zerolog.Logger
	cache                 *caching.Cacher
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

	booking.Status = schema.BookingResponseStatus(response.GetBookingStatus())
	booking.SupplierBookingReference = converting.PointerToValue(response.Id)

	return booking, nil
}

func (b *bookingRequest) makeRequest(
	client *http.Client,
	token string,
) (json.BookingRS, error) {
	body := bytes.NewBuffer(b.requestBody())

	url := b.configuration.SupplierApiUrl + "/api/Booking"
	c := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Booking)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodPost, url, body)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+token)

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

	return jsonBookingResponse, nil
}

func (b *bookingRequest) requestBody() []byte {
	name := fmt.Sprintf("%v %v", converting.LatinCharacters(b.params.Customer.FirstName), converting.LatinCharacters(b.params.Customer.LastName))

	deliveryLocation, _ := strconv.Atoi(b.params.PickUp.Code)
	dropOffLocation, _ := strconv.Atoi(b.params.DropOff.Code)

	var additionals *[]json.BookingRQAdditional = nil

	if b.params.ExtrasAndFees != nil {
		additionals := []json.BookingRQAdditional{}

		for _, extra := range *b.params.ExtrasAndFees {
			quantity := 1

			if extra.Quantity != nil {
				quantity = *extra.Quantity
			}

			code, _ := strconv.Atoi(extra.Code)

			additionals = append(additionals, json.BookingRQAdditional{
				AdditionalId: code,
				Quantity:     quantity,
			})
		}
	}

	json, _ := jsonEncoding.MarshalIndent(&json.BookingRQ{
		Model:                   b.supplierRateReference.Model,
		FromDate:                b.params.PickUp.DateTime.Format(time.RFC3339),
		ToDate:                  b.params.DropOff.DateTime.Format(time.RFC3339),
		DeliveryPlace:           deliveryLocation,
		DropOffPlace:            dropOffLocation,
		ExternalSystemBookingId: b.params.ReservNumber,
		CommercialAgreementCode: string(b.configuration.CommercialAgreementCode),
		Additionals:             additionals,
		Customer: json.BookingRQCustomer{
			Name:         name,
			EmailAddress: string(b.params.Customer.Email),
			CellPhone:    b.params.Customer.Phone,
			Country:      b.params.Customer.ResidenceCountry,
			Age:          b.params.Customer.Age,
		},
	}, "", "	")

	return json
}
