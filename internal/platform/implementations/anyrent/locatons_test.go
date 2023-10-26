package anyrent_test

import (
	"bytes"
	"compress/flate"
	"context"
	jsonEncoding "encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestLocationsRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build locations request based on params", func(t *testing.T) {
		tests := []struct {
			name          string
			requestParams func(url string) schema.LocationsRequestParams
		}{
			{
				"general",
				func(url string) schema.LocationsRequestParams {
					configuration := locationsDefaultConfiguration()
					configuration.SupplierApiUrl = url
					return locationsParamsTemplate(configuration)
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
						assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
						assert.Equal(t, "/v1/authorize", r.RequestURI)
						assert.Equal(t, "POST", r.Method)

						w.Write([]byte(defaultSupplierAuthResponse()))
					}

					// mock the locations response
					if handlerFuncCalledCount == 2 {
						assert.Equal(t, "/v1/stations?page=1", r.RequestURI)
						assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
						assert.Equal(t, "en", r.Header.Get("x-lang"))
						assert.Equal(t, "GET", r.Method)

						w.Write([]byte(defaultSupplierLocationsResponse()))
					}
				}

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				_, err := getLocations(test.requestParams(testServer.URL), &log, redisClient)

				assert.Nil(t, err)
				assert.True(t, handlerFuncCalled)
				assert.Equal(t, 2, handlerFuncCalledCount)
			})
		}
	})

	t.Run("should parse supplier responses correctly", func(t *testing.T) {
		tests := []struct {
			name                       string
			configuration              schema.AnyRentConfiguration
			supplierLocationsResponse  []byte
			expectedResponse           []byte
			supplierLocationsResponse2 *[]byte
		}{
			{
				name:                      "general",
				configuration:             locationsDefaultConfiguration(),
				supplierLocationsResponse: defaultSupplierLocationsResponse(),
				expectedResponse:          defaultLocationsResponse(),
			},
			{
				name:                       "paginated",
				configuration:              locationsDefaultConfiguration(),
				supplierLocationsResponse:  paginatedSupplierLocationsResponse(1),
				supplierLocationsResponse2: converting.PointerToValue(paginatedSupplierLocationsResponse(2)),
				expectedResponse:           paginatedLocationsResponse(),
			},
			{
				name:                      "failed",
				configuration:             locationsDefaultConfiguration(),
				supplierLocationsResponse: failedSupplierLocationsResponse(),
				expectedResponse:          failedLocationsResponse(),
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

					if handlerFuncCalledCount == 2 {
						w.Write(test.supplierLocationsResponse)
						return
					}

					if handlerFuncCalledCount > 2 && test.supplierLocationsResponse2 != nil {
						w.Write(*test.supplierLocationsResponse2)

						return
					}
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := locationsParamsTemplate(test.configuration)

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				service := anyrent.New(redisClient)
				ctx := context.Background()
				locations, err := service.GetLocations(ctx, params, &log)
				assert.Nil(t, err)

				locations.SupplierRequests = nil
				actual, _ := jsonEncoding.MarshalIndent(locations, "", "	")

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

		configuration := locationsDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := locationsParamsTemplate(configuration)
		params.Timeouts.Default = 1

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		locationsResponse, err := getLocations(params, &log, redisClient)

		assert.Nil(t, err)
		assert.Len(t, *locationsResponse.Errors, 1)
		assert.Equal(t, schema.TimeoutError, (*locationsResponse.Errors)[0].Code)
		assert.True(t, len((*locationsResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle supplier connection errors", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := locationsDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := locationsParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		service := anyrent.New(redisClient)

		channel := make(chan schema.LocationsResponse, 1)

		go func() {
			ctx := context.Background()
			locationsResponse, _ := service.GetLocations(ctx, params, &log)
			channel <- locationsResponse
		}()
		time.Sleep(5 * time.Millisecond)
		testServer.CloseClientConnections() // close the connection to force transport level error

		locationsResponse := <-channel

		assert.Len(t, *locationsResponse.Errors, 1)
		assert.Equal(t, schema.ConnectionError, (*locationsResponse.Errors)[0].Code)
		assert.True(t, len((*locationsResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle status != 200 error from supplier", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound) // 404 for testing
		}))
		defer testServer.Close()

		configuration := locationsDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := locationsParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		locationsResponse, _ := getLocations(params, &log, redisClient)

		assert.Len(t, *locationsResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, (*locationsResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*locationsResponse.Errors)[0].Message)
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

			// mock the locations response
			if handlerFuncCalledCount == 2 {
				w.Write([]byte(defaultSupplierLocationsResponse()))
			}
		}))
		defer testServer.Close()

		configuration := locationsDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := locationsParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		locationsResponse, _ := getLocations(params, &log, redisClient)

		assert.Len(t, *locationsResponse.SupplierRequests, 2)

		assert.Equal(t, testServer.URL+"/v1/authorize", *(*locationsResponse.SupplierRequests)[0].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*locationsResponse.SupplierRequests)[0].RequestContent.Method)
		assert.Len(t, *(*locationsResponse.SupplierRequests)[0].RequestContent.Headers, 1)
		assert.Equal(t, http.StatusOK, *(*locationsResponse.SupplierRequests)[0].ResponseContent.StatusCode)
		assert.Len(t, *(*locationsResponse.SupplierRequests)[0].ResponseContent.Headers, 3)

		assert.Equal(t, testServer.URL+"/v1/stations?page=1", *(*locationsResponse.SupplierRequests)[1].RequestContent.Url)
		assert.Equal(t, http.MethodGet, *(*locationsResponse.SupplierRequests)[1].RequestContent.Method)
		assert.Len(t, *(*locationsResponse.SupplierRequests)[1].RequestContent.Headers, 2)
		assert.Equal(t, http.StatusOK, *(*locationsResponse.SupplierRequests)[1].ResponseContent.StatusCode)
		assert.Len(t, *(*locationsResponse.SupplierRequests)[1].ResponseContent.Headers, 3)
	})
}

func locationsDefaultConfiguration() schema.AnyRentConfiguration {
	return schema.AnyRentConfiguration{
		ApiKey: "test-api-key",
	}
}

func locationsParamsTemplate(configuration schema.AnyRentConfiguration) schema.LocationsRequestParams {
	b, _ := jsonEncoding.Marshal(configuration)

	var cp schema.LocationsRequestParams_Configuration
	jsonEncoding.Unmarshal(b, &cp)

	return schema.LocationsRequestParams{
		Timeouts:      schema.Timeouts{Default: 8000},
		Configuration: cp,
	}
}

func getLocations(params schema.LocationsRequestParams, log *zerolog.Logger, redisClient *redis.Client) (schema.LocationsResponse, error) {
	service := anyrent.New(redisClient)
	ctx := context.Background()
	return service.GetLocations(ctx, params, log)
}

func defaultSupplierAuthResponse() []byte {
	authSuccessfulBody, _ := os.ReadFile("./testdata/common/auth_response.json")

	return authSuccessfulBody
}

func defaultSupplierLocationsResponse() []byte {
	locationsBody, _ := os.ReadFile("./testdata/locations/supplier_response_default.json")

	return locationsBody
}

func failedSupplierLocationsResponse() []byte {
	locationsBody, _ := os.ReadFile("./testdata/locations/supplier_response_failed.json")

	return locationsBody
}

func paginatedSupplierLocationsResponse(page int) []byte {
	locationsBody, _ := os.ReadFile(fmt.Sprintf("./testdata/locations/supplier_response_page_%v.json", page))

	return locationsBody
}

func defaultLocationsResponse() []byte {
	locationsBody, _ := os.ReadFile("./testdata/locations/response_default.json")

	return locationsBody
}

func paginatedLocationsResponse() []byte {
	locationsBody, _ := os.ReadFile("./testdata/locations/response_paginated.json")

	return locationsBody
}

func failedLocationsResponse() []byte {
	locationsBody, _ := os.ReadFile("./testdata/locations/response_failed.json")

	return locationsBody
}

func defaultAuthRedisCacheKey(supplierApiUrl string) string {
	return fmt.Sprintf("anyrent-auth-token:%s-test-api-key", supplierApiUrl)
}

func getCachedAndCompressedAuthKey() ([]byte, error) {
	uncompressed, err := jsonEncoding.Marshal("test-token")
	if err != nil {
		return nil, err
	}

	var buffer bytes.Buffer
	writer, _ := flate.NewWriter(&buffer, flate.BestSpeed)

	_, err = writer.Write(uncompressed)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
