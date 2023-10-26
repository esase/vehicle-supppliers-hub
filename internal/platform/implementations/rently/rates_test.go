package rently_test

import (
	"bytes"
	"context"
	jsonEncoding "encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/rently"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestRatesRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build rates request based on params", func(t *testing.T) {
		tests := []struct {
			name          string
			requestParams func(url string) schema.RatesRequestParams
		}{
			{
				"general",
				func(url string) schema.RatesRequestParams {
					configuration := ratesDefaultConfiguration()
					configuration.SupplierApiUrl = url
					return ratesParamsTemplate(configuration)
				},
			},
		}

		var handlerFunc http.HandlerFunc
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFunc(w, r)
		}))
		defer testServer.Close()

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				handlerFuncCalled := false
				handlerFuncCalledCount := 0

				handlerFunc = func(w http.ResponseWriter, r *http.Request) {
					handlerFuncCalled = true
					handlerFuncCalledCount++

					w.WriteHeader(http.StatusOK)

					// mock the auth response
					if handlerFuncCalledCount == 1 {
						assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
						assert.Equal(t, "/connect/token", r.RequestURI)
						assert.Equal(t, "POST", r.Method)

						w.Write([]byte(defaultSupplierAuthResponse()))
					}

					// mock the rates response
					if handlerFuncCalledCount == 2 {
						assert.Equal(t, "/api/AvailabilityByPlace?CommercialAgreementCode=Prepaid&DeliveryLocation=1&DriverAge=39&DropoffLocation=2&From=2023-09-01+12%3A30%3A00&ReturnAdditionalsPrice=true&To=2023-09-02+12%3A30%3A00", r.RequestURI)
						assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
						assert.Equal(t, "GET", r.Method)

						w.Write([]byte(defaultSupplierLocationsResponse()))
					}
				}

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				_, err := getRates(test.requestParams(testServer.URL), &log, redisClient)

				assert.Nil(t, err)
				assert.True(t, handlerFuncCalled)
				assert.Equal(t, 2, handlerFuncCalledCount)
			})
		}
	})

	t.Run("should parse supplier responses correctly", func(t *testing.T) {
		tests := []struct {
			name                  string
			configuration         schema.RentlyConfiguration
			supplierRatesResponse []byte
			expectedResponse      []byte
		}{
			{
				name:                  "general",
				configuration:         ratesDefaultConfiguration(),
				supplierRatesResponse: defaultSupplierRatesResponse(),
				expectedResponse:      defaultRatesResponse(),
			},
		}

		var handlerFunc http.HandlerFunc
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFunc(w, r)
		}))
		defer testServer.Close()

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				handlerFuncCalledCount := 0
				handlerFunc = func(w http.ResponseWriter, req *http.Request) {
					handlerFuncCalledCount++

					w.WriteHeader(http.StatusOK)

					// mock the auth response
					if handlerFuncCalledCount == 1 {
						w.Write([]byte(defaultSupplierAuthResponse()))
						return
					}

					w.Write(test.supplierRatesResponse)
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := ratesParamsTemplate(test.configuration)

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				service := rently.New(redisClient)
				ctx := context.Background()
				vehicles, err := service.GetRates(ctx, params, &log)
				assert.Nil(t, err)

				vehicles.SupplierRequests = nil
				actual, _ := jsonEncoding.MarshalIndent(vehicles, "", "	")
				assert.Equal(t, strings.ReplaceAll(string(test.expectedResponse), "\t", ""), strings.ReplaceAll(string(actual), "\t", ""))
			})
		}
	})

	t.Run("should handle timeout from supplier", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond) // timeout in params is 1ms
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := ratesParamsTemplate(configuration)
		params.Timeouts.Default = 1

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		ratesResponse, err := getRates(params, &log, redisClient)

		assert.Nil(t, err)
		assert.Len(t, *ratesResponse.Errors, 1)
		assert.Equal(t, schema.TimeoutError, (*ratesResponse.Errors)[0].Code)
		assert.True(t, len((*ratesResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle supplier connection errors", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := ratesParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		service := rently.New(redisClient)

		channel := make(chan schema.RatesResponse, 1)

		go func() {
			ctx := context.Background()
			ratesResponse, _ := service.GetRates(ctx, params, &log)
			channel <- ratesResponse
		}()
		time.Sleep(5 * time.Millisecond)
		testServer.CloseClientConnections() // close the connection to force transport level error

		ratesResponse := <-channel

		assert.Len(t, *ratesResponse.Errors, 1)
		assert.Equal(t, schema.ConnectionError, (*ratesResponse.Errors)[0].Code)
		assert.True(t, len((*ratesResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle status != 200 error from supplier", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound) // 404 for testing
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := ratesParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		ratesResponse, _ := getRates(params, &log, redisClient)

		assert.Len(t, *ratesResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, (*ratesResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*ratesResponse.Errors)[0].Message)
	})

	t.Run("should return build supplier requests history array", func(t *testing.T) {
		handlerFuncCalledCount := 0

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFuncCalledCount++
			w.WriteHeader(http.StatusOK)

			// mock the auth response
			if handlerFuncCalledCount == 1 {
				w.Write([]byte(defaultSupplierAuthResponse()))
			}

			// mock the rates response
			if handlerFuncCalledCount == 2 {
				w.Write([]byte(defaultSupplierRatesResponse()))
			}
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := ratesParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		ratesResponse, _ := getRates(params, &log, redisClient)

		assert.Len(t, *ratesResponse.SupplierRequests, 2)

		assert.Equal(t, testServer.URL+"/connect/token", *(*ratesResponse.SupplierRequests)[0].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*ratesResponse.SupplierRequests)[0].RequestContent.Method)
		assert.Len(t, *(*ratesResponse.SupplierRequests)[0].RequestContent.Headers, 1)
		assert.Equal(t, http.StatusOK, *(*ratesResponse.SupplierRequests)[0].ResponseContent.StatusCode)
		assert.Len(t, *(*ratesResponse.SupplierRequests)[0].ResponseContent.Headers, 3)

		assert.Equal(t, testServer.URL+"/api/AvailabilityByPlace?CommercialAgreementCode=Prepaid&DeliveryLocation=1&DriverAge=39&DropoffLocation=2&From=2023-09-01+12%3A30%3A00&ReturnAdditionalsPrice=true&To=2023-09-02+12%3A30%3A00", *(*ratesResponse.SupplierRequests)[1].RequestContent.Url)
		assert.Equal(t, http.MethodGet, *(*ratesResponse.SupplierRequests)[1].RequestContent.Method)
		assert.Len(t, *(*ratesResponse.SupplierRequests)[1].RequestContent.Headers, 1)
		assert.Equal(t, http.StatusOK, *(*ratesResponse.SupplierRequests)[1].ResponseContent.StatusCode)
		assert.Len(t, *(*ratesResponse.SupplierRequests)[1].ResponseContent.Headers, 2)
	})
}

func ratesDefaultConfiguration() schema.RentlyConfiguration {
	return schema.RentlyConfiguration{
		Username:                "test-username",
		Password:                "test-password",
		CommercialAgreementCode: "Prepaid",
	}
}

func ratesParamsTemplate(configuration schema.RentlyConfiguration) schema.RatesRequestParams {
	b, _ := jsonEncoding.Marshal(configuration)

	var cp schema.RatesRequestParams_Configuration
	jsonEncoding.Unmarshal(b, &cp)

	pickup, _ := time.Parse(schema.DateTimeFormat, "2023-09-01T12:30:00")
	dropOff, _ := time.Parse(schema.DateTimeFormat, "2023-09-02T12:30:00")

	return schema.RatesRequestParams{
		PickUp: schema.RequestBranch{
			Code:     "1",
			DateTime: pickup,
		},
		DropOff: schema.RequestBranch{
			Code:     "2",
			DateTime: dropOff,
		},
		Contract: schema.Contract{
			PaymentType: 0,
		},
		Age:              39,
		RentalDays:       7,
		ResidenceCountry: "US",
		Timeouts:         schema.Timeouts{Default: 8000},
		Configuration:    cp,
	}
}

func getRates(params schema.RatesRequestParams, log *zerolog.Logger, redisClient *redis.Client) (schema.RatesResponse, error) {
	service := rently.New(redisClient)
	ctx := context.Background()
	return service.GetRates(ctx, params, log)
}

func defaultSupplierRatesResponse() []byte {
	ratesBody, _ := os.ReadFile("./testdata/rates/supplier_response_default.json")

	return ratesBody
}

func defaultRatesResponse() []byte {
	ratesBody, _ := os.ReadFile("./testdata/rates/response_default.json")

	return ratesBody
}
