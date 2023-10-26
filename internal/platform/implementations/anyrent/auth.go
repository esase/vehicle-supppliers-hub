package anyrent

import (
	"bytes"
	"context"
	jsonEncoding "encoding/json"
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

type authRequest struct {
	configuration    schema.AnyRentConfiguration
	logger           *zerolog.Logger
	jsonAuthResponse json.AuthRS
	timeout          int
	cache            *caching.Cacher
}

type AuthResponse struct {
	Errors           *schema.SupplierResponseErrors `json:"errors,omitempty"`
	SupplierRequests *schema.SupplierRequests       `json:"supplierRequests,omitempty"`
	Token            *string                        `json:"token,omitempty"`
}

func (a *authRequest) Execute(httpTransport *http.Transport) (AuthResponse, error) {
	authResponse := AuthResponse{}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	authResponse.SupplierRequests = requestsBucket.SupplierRequests()
	authResponse.Errors = errorsBucket.Errors()

	ctx := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Auth)

	var cachedAuthToken string
	ok := a.cache.Fetch(ctx, a.getCacheKey(), &cachedAuthToken)
	if ok {
		authResponse.Token = &cachedAuthToken

		return authResponse, nil
	}

	// prepare client
	client := &http.Client{
		Timeout: time.Duration(a.timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: httpTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(a.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	response, e := requesting.RequestErrors(a.makeRequest(ctx, client))

	// handle response
	if e != nil {
		errorsBucket.AddError(*e)
		return authResponse, nil
	}

	// bind the response body to the json
	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	err := jsonEncoding.Unmarshal(bodyBytes, &a.jsonAuthResponse)
	if err != nil {
		return authResponse, err
	}

	errorMessage := a.jsonAuthResponse.ErrorMessage()

	if errorMessage != "" {
		errorsBucket.AddError(schema.NewSupplierError(errorMessage))

		return authResponse, nil
	}

	authResponse.Token = &a.jsonAuthResponse.Token

	err = a.cache.Store(ctx, a.getCacheKey(), a.jsonAuthResponse.Token, time.Duration(a.jsonAuthResponse.Expiration)*time.Second)
	if err != nil {
		return authResponse, err
	}

	return authResponse, nil
}

func (a *authRequest) makeRequest(ctx context.Context, client *http.Client) (*http.Response, error) {
	body := bytes.NewBuffer(a.requestBody())
	url := a.configuration.SupplierApiUrl + "/v1/authorize"

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return httpResponse, nil
}

func (a *authRequest) requestBody() []byte {
	json, _ := jsonEncoding.MarshalIndent(&json.AuthRQ{
		ApiKey: a.configuration.ApiKey,
	}, "", "	")

	return json
}

func (a *authRequest) getCacheKey() string {
	return fmt.Sprintf("anyrent-auth-token:%s-%s", a.configuration.SupplierApiUrl, a.configuration.ApiKey)
}
