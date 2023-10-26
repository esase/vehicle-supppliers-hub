package rently

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type cancelRequest struct {
	params        schema.CancelRequestParams
	configuration schema.RentlyConfiguration
	logger        *zerolog.Logger
	cache         *caching.Cacher
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
	if c.params.Timeouts.Locations != nil {
		timeout = *c.params.Timeouts.Locations
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

	err = c.makeRequest(client, *auth.Token)

	if err != nil {
		errorsBucket.AddError(schema.NewSupplierError(err.Error()))
		return cancel, nil
	}

	status = schema.CancelResponseStatusOK
	cancel.Status = &status

	return cancel, nil
}

func (c *cancelRequest) makeRequest(
	client *http.Client,
	token string,
) error {
	url := fmt.Sprintf("%v/api/Booking/%v", c.configuration.SupplierApiUrl, c.params.SupplierBookingReference)
	ctx := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Cancel)

	httpRequest, _ := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	httpRequest.Header.Set("Authorization", "Bearer "+token)

	rs, err := requesting.RequestErrors(client.Do(httpRequest))
	if err != nil {
		return errors.New(err.Message)
	}
	defer rs.Body.Close()

	return nil
}
