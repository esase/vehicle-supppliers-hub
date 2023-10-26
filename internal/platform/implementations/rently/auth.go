package rently

import (
	"context"
	jsonEncoding "encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/rently/json"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type authRequest struct {
	configuration    schema.RentlyConfiguration
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

	jsonErr := jsonEncoding.Unmarshal(bodyBytes, &a.jsonAuthResponse)
	if jsonErr != nil {
		return authResponse, jsonErr
	}

	authResponse.Token = &a.jsonAuthResponse.AccessToken

	err := a.cache.Store(ctx, a.getCacheKey(), a.jsonAuthResponse.AccessToken, time.Duration(a.jsonAuthResponse.ExpiresIn)*time.Second)
	if err != nil {
		return authResponse, err
	}

	return authResponse, nil
}

func (a *authRequest) makeRequest(ctx context.Context, client *http.Client) (*http.Response, error) {
	body := strings.NewReader(a.requestBody())
	supplierUrl := a.configuration.SupplierApiUrl + "/connect/token"

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, supplierUrl, body)
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return httpResponse, nil
}

func (a *authRequest) requestBody() string {
	data := url.Values{}
	data.Set("username", a.configuration.Username)
	data.Set("password", a.configuration.Password)
	data.Set("grant_type", "password")
	data.Set("client_id", "RentlyAPI")

	return data.Encode()
}

func (a *authRequest) getCacheKey() string {
	return fmt.Sprintf("rently-auth-token:%s-%s-%s", a.configuration.SupplierApiUrl, a.configuration.Username, a.configuration.Password)
}
