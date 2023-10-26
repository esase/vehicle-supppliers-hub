package anyrent_test

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

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestCancelRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build cancel booking request based on params", func(t *testing.T) {
		tests := []struct {
			name          string
			requestParams func(url string) schema.CancelRequestParams
		}{
			{
				"general",
				func(url string) schema.CancelRequestParams {
					configuration := cancelDefaultConfiguration()
					configuration.SupplierApiUrl = url
					return cancelParamsTemplate(configuration)
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

					// mock the cancel response
					if handlerFuncCalledCount == 2 {
						assert.Equal(t, "/v1/bookings/10", r.RequestURI)
						assert.Equal(t, "DELETE", r.Method)
						assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
						assert.Equal(t, "en", r.Header.Get("x-lang"))
					}
				}

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				_, err := cancelBooking(test.requestParams(testServer.URL), &log, redisClient)

				assert.Nil(t, err)
				assert.True(t, handlerFuncCalled)
				assert.Equal(t, 2, handlerFuncCalledCount)
			})
		}
	})

	t.Run("should parse supplier responses correctly", func(t *testing.T) {
		tests := []struct {
			name                 string
			configuration        schema.AnyRentConfiguration
			supplierResponseCode int
			expectedResponse     []byte
		}{
			{
				name:                 "confirmed",
				configuration:        cancelDefaultConfiguration(),
				supplierResponseCode: http.StatusOK,
				expectedResponse:     defaultCancelResponse(),
			},
			{
				name:                 "failed",
				configuration:        cancelDefaultConfiguration(),
				supplierResponseCode: http.StatusForbidden,
				expectedResponse:     failedCancelResponse(),
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

					// mock the auth response
					if handlerFuncCalledCount == 1 {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(defaultSupplierAuthResponse()))
						return
					}

					w.WriteHeader(test.supplierResponseCode)
					w.Write([]byte("{}"))
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := cancelParamsTemplate(test.configuration)

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				service := anyrent.New(redisClient)
				ctx := context.Background()
				cancel, err := service.CancelBooking(ctx, params, &log)
				assert.Nil(t, err)

				cancel.SupplierRequests = nil
				actual, _ := jsonEncoding.MarshalIndent(cancel, "", "	")
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

		configuration := cancelDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := cancelParamsTemplate(configuration)
		params.Timeouts.Default = 1

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		cancelResponse, err := cancelBooking(params, &log, redisClient)

		assert.Nil(t, err)
		assert.Len(t, *cancelResponse.Errors, 1)
		assert.Equal(t, schema.TimeoutError, (*cancelResponse.Errors)[0].Code)
		assert.True(t, len((*cancelResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle supplier connection errors", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := cancelDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := cancelParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		service := anyrent.New(redisClient)

		channel := make(chan schema.CancelResponse, 1)

		go func() {
			ctx := context.Background()
			cancelResponse, _ := service.CancelBooking(ctx, params, &log)
			channel <- cancelResponse
		}()
		time.Sleep(5 * time.Millisecond)
		testServer.CloseClientConnections() // close the connection to force transport level error

		cancelResponse := <-channel

		assert.Len(t, *cancelResponse.Errors, 1)
		assert.Equal(t, schema.ConnectionError, (*cancelResponse.Errors)[0].Code)
		assert.True(t, len((*cancelResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle status != 200 error from supplier", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound) // 404 for testing
		}))
		defer testServer.Close()

		configuration := cancelDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := cancelParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		cancelResponse, _ := cancelBooking(params, &log, redisClient)

		assert.Len(t, *cancelResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, (*cancelResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*cancelResponse.Errors)[0].Message)
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
		}))
		defer testServer.Close()

		configuration := cancelDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := cancelParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		cancelResponse, _ := cancelBooking(params, &log, redisClient)

		assert.Len(t, *cancelResponse.SupplierRequests, 2)

		assert.Equal(t, testServer.URL+"/v1/authorize", *(*cancelResponse.SupplierRequests)[0].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*cancelResponse.SupplierRequests)[0].RequestContent.Method)
		assert.Len(t, *(*cancelResponse.SupplierRequests)[0].RequestContent.Headers, 1)
		assert.Equal(t, http.StatusOK, *(*cancelResponse.SupplierRequests)[0].ResponseContent.StatusCode)
		assert.Len(t, *(*cancelResponse.SupplierRequests)[0].ResponseContent.Headers, 3)

		assert.Equal(t, testServer.URL+"/v1/bookings/10", *(*cancelResponse.SupplierRequests)[1].RequestContent.Url)
		assert.Equal(t, http.MethodDelete, *(*cancelResponse.SupplierRequests)[1].RequestContent.Method)
		assert.Len(t, *(*cancelResponse.SupplierRequests)[1].RequestContent.Headers, 2)
		assert.Equal(t, http.StatusOK, *(*cancelResponse.SupplierRequests)[1].ResponseContent.StatusCode)
		assert.Len(t, *(*cancelResponse.SupplierRequests)[1].ResponseContent.Headers, 2)
	})
}

func cancelDefaultConfiguration() schema.AnyRentConfiguration {
	return schema.AnyRentConfiguration{
		ApiKey: "test-api-key",
	}
}

func cancelParamsTemplate(configuration schema.AnyRentConfiguration) schema.CancelRequestParams {
	b, _ := jsonEncoding.Marshal(configuration)

	var cp schema.CancelRequestParams_Configuration
	jsonEncoding.Unmarshal(b, &cp)

	cancelReason := 4

	return schema.CancelRequestParams{
		SupplierBookingReference: "10",
		Contact: schema.Contact{
			Email: "dumb@www.com",
		},
		CancelReason:  &cancelReason,
		Timeouts:      schema.Timeouts{Default: 8000},
		Configuration: cp,
	}
}

func cancelBooking(params schema.CancelRequestParams, log *zerolog.Logger, redisClient *redis.Client) (schema.CancelResponse, error) {
	service := anyrent.New(redisClient)
	ctx := context.Background()
	return service.CancelBooking(ctx, params, log)
}

func defaultCancelResponse() []byte {
	cancelBody, _ := os.ReadFile("./testdata/cancel/response_default.json")

	return cancelBody
}

func failedCancelResponse() []byte {
	cancelBody, _ := os.ReadFile("./testdata/cancel/response_failed.json")

	return cancelBody
}
