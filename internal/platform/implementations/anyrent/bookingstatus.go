package anyrent

import (
	"context"
	jsonEncoding "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent/json"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type bookingStatusRequest struct {
	cache         *caching.Cacher
	params        schema.BookingStatusRequestParams
	configuration schema.AnyRentConfiguration
	logger        *zerolog.Logger
}

func (b *bookingStatusRequest) Execute(httpTransport *http.Transport) (schema.BookingStatusResponse, error) {
	bookingStatus := schema.BookingStatusResponse{}
	bookingStatus.Status = schema.BookingStatusResponseStatusFAILED

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	bookingStatus.SupplierRequests = requestsBucket.SupplierRequests()
	bookingStatus.Errors = errorsBucket.Errors()

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
		return bookingStatus, err
	}

	if auth.Token == nil {
		return bookingStatus, nil
	}

	timeout := b.params.Timeouts.Default

	// prepare client
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
		return bookingStatus, nil
	}

	bookingStatus.Status = response.GetBookingStatus()

	return bookingStatus, nil
}

func (b *bookingStatusRequest) makeRequest(
	client *http.Client,
	token string,
) (json.BookingStatusRS, error) {
	url := fmt.Sprintf("%v/v1/bookings/%v", b.configuration.SupplierApiUrl, b.params.SupplierBookingReference)
	c := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.BookingStatus)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodGet, url, http.NoBody)
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("x-lang", "en")

	rs, err := requesting.RequestErrors(client.Do(httpRequest))
	if err != nil {
		return json.BookingStatusRS{}, errors.New(err.Message)
	}
	defer rs.Body.Close()

	// bind the response body to the json
	bodyBytes, _ := io.ReadAll(rs.Body)
	rs.Body.Close()

	var jsonBookingStatusResponse json.BookingStatusRS
	jsonEncodeErr := jsonEncoding.Unmarshal(bodyBytes, &jsonBookingStatusResponse)
	if jsonEncodeErr != nil {
		return json.BookingStatusRS{}, errors.New(jsonEncodeErr.Error())
	}

	message := jsonBookingStatusResponse.ErrorMessage()
	if message != "" {
		return json.BookingStatusRS{}, errors.New(message)
	}

	return jsonBookingStatusResponse, nil
}
