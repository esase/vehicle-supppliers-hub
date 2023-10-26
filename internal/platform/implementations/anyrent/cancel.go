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

type cancelRequest struct {
	cache         *caching.Cacher
	params        schema.CancelRequestParams
	configuration schema.AnyRentConfiguration
	logger        *zerolog.Logger
}

func (c *cancelRequest) Execute(httpTransport *http.Transport) (schema.CancelResponse, error) {
	cancel := schema.CancelResponse{}

	status := schema.CancelResponseStatusFAILED
	cancel.Status = &status

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	cancel.SupplierRequests = requestsBucket.SupplierRequests()
	cancel.Errors = errorsBucket.Errors()

	// fetch auth token
	authRequest := authRequest{
		configuration: c.configuration,
		logger:        c.logger,
		timeout:       c.params.Timeouts.Default,
		cache:         c.cache,
	}

	auth, err := authRequest.Execute(httpTransport)
	requestsBucket.AddRequests(*auth.SupplierRequests)
	errorsBucket.AddErrors(*auth.Errors)

	if err != nil {
		return cancel, err
	}

	if auth.Token == nil {
		return cancel, nil
	}

	timeout := c.params.Timeouts.Default
	if c.params.Timeouts.Cancel != nil {
		timeout = *c.params.Timeouts.Cancel
	}

	// prepare client
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: httpTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(c.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	_, err = c.makeRequest(client, *auth.Token)

	if err != nil {
		errorsBucket.AddError(schema.NewSupplierError(err.Error()))
		return cancel, nil
	}

	status = schema.CancelResponseStatusOK
	cancel.Status = &status

	return cancel, nil
}

func (ca *cancelRequest) makeRequest(
	client *http.Client,
	token string,
) (json.CancelBookingRS, error) {
	url := fmt.Sprintf("%v/v1/bookings/%v", ca.configuration.SupplierApiUrl, ca.params.SupplierBookingReference)
	c := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Cancel)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodDelete, url, http.NoBody)
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("x-lang", "en")

	rs, err := requesting.RequestErrors(client.Do(httpRequest))
	if err != nil {
		return json.CancelBookingRS{}, errors.New(err.Message)
	}
	defer rs.Body.Close()

	// bind the response body to the json
	bodyBytes, _ := io.ReadAll(rs.Body)
	rs.Body.Close()

	var jsonCancelResponse json.CancelBookingRS
	jsonEncodeErr := jsonEncoding.Unmarshal(bodyBytes, &jsonCancelResponse)
	if jsonEncodeErr != nil {
		return json.CancelBookingRS{}, errors.New(jsonEncodeErr.Error())
	}

	message := jsonCancelResponse.ErrorMessage()
	if message != "" {
		return json.CancelBookingRS{}, errors.New(message)
	}

	return jsonCancelResponse, nil
}
