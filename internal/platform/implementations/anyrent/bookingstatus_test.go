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

func TestBookingStatusRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build booking status request based on params", func(t *testing.T) {
		tests := []struct {
			name          string
			requestParams func(url string) schema.BookingStatusRequestParams
		}{
			{
				"general",
				func(url string) schema.BookingStatusRequestParams {
					configuration := bookingStatusDefaultConfiguration()
					configuration.SupplierApiUrl = url
					return bookingStatusParamsTemplate(configuration)
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

					// mock the booking status response
					if handlerFuncCalledCount == 2 {
						assert.Equal(t, "/v1/bookings/14", r.RequestURI)
						assert.Equal(t, "GET", r.Method)
						assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
						assert.Equal(t, "en", r.Header.Get("x-lang"))
					}
				}

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				_, err := getBookingStatus(test.requestParams(testServer.URL), &log, redisClient)

				assert.Nil(t, err)
				assert.True(t, handlerFuncCalled)
				assert.Equal(t, 2, handlerFuncCalledCount)
			})
		}
	})

	t.Run("should parse supplier responses correctly", func(t *testing.T) {
		tests := []struct {
			name                          string
			configuration                 schema.AnyRentConfiguration
			supplierResponseCode          int
			supplierBookingStatusResponse []byte
			expectedResponse              []byte
		}{
			{
				name:                          "confirmed",
				configuration:                 bookingStatusDefaultConfiguration(),
				supplierResponseCode:          http.StatusOK,
				supplierBookingStatusResponse: defaultSupplierBookingStatusResponse(),
				expectedResponse:              defaultBookingStatusResponse(),
			},
			{
				name:                          "expired",
				configuration:                 bookingStatusDefaultConfiguration(),
				supplierResponseCode:          http.StatusOK,
				supplierBookingStatusResponse: expiredSupplierBookingStatusResponse(),
				expectedResponse:              failedBookingStatusResponse(),
			},
			{
				name:                          "pending",
				configuration:                 bookingStatusDefaultConfiguration(),
				supplierResponseCode:          http.StatusOK,
				supplierBookingStatusResponse: pendingSupplierBookingStatusResponse(),
				expectedResponse:              pendingBookingStatusResponse(),
			},
			{
				name:                          "canceled",
				configuration:                 bookingStatusDefaultConfiguration(),
				supplierResponseCode:          http.StatusOK,
				supplierBookingStatusResponse: canceledSupplierBookingStatusResponse(),
				expectedResponse:              canceledBookingStatusResponse(),
			},
			{
				name:                          "no_show",
				configuration:                 bookingDefaultConfiguration(),
				supplierResponseCode:          http.StatusOK,
				supplierBookingStatusResponse: notShowedSupplierBookingStatusResponse(),
				expectedResponse:              failedBookingStatusResponse(),
			},
			{
				name:                          "fulfilled",
				configuration:                 bookingStatusDefaultConfiguration(),
				supplierResponseCode:          http.StatusOK,
				supplierBookingStatusResponse: fulfilledSupplierBookingStatusResponse(),
				expectedResponse:              failedBookingStatusResponse(),
			},
			{
				name:                          "failed with http code",
				configuration:                 bookingStatusDefaultConfiguration(),
				supplierResponseCode:          http.StatusForbidden,
				supplierBookingStatusResponse: defaultSupplierBookingStatusResponse(),
				expectedResponse:              failedBookingStatusResponseWithCode(),
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
					w.Write(test.supplierBookingStatusResponse)
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := bookingStatusParamsTemplate(test.configuration)

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				service := anyrent.New(redisClient)
				ctx := context.Background()
				bookingStatus, err := service.GetBookingStatus(ctx, params, &log)
				assert.Nil(t, err)

				bookingStatus.SupplierRequests = nil
				actual, _ := jsonEncoding.MarshalIndent(bookingStatus, "", "	")
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

		configuration := bookingStatusDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingStatusParamsTemplate(configuration)
		params.Timeouts.Default = 1

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		bookingStatusResponse, err := getBookingStatus(params, &log, redisClient)

		assert.Nil(t, err)
		assert.Len(t, *bookingStatusResponse.Errors, 1)
		assert.Equal(t, schema.TimeoutError, (*bookingStatusResponse.Errors)[0].Code)
		assert.True(t, len((*bookingStatusResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle supplier connection errors", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := bookingStatusDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingStatusParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		service := anyrent.New(redisClient)

		channel := make(chan schema.BookingStatusResponse, 1)

		go func() {
			ctx := context.Background()
			bookingStatusResponse, _ := service.GetBookingStatus(ctx, params, &log)
			channel <- bookingStatusResponse
		}()
		time.Sleep(5 * time.Millisecond)
		testServer.CloseClientConnections() // close the connection to force transport level error

		bookingStatusResponse := <-channel

		assert.Len(t, *bookingStatusResponse.Errors, 1)
		assert.Equal(t, schema.ConnectionError, (*bookingStatusResponse.Errors)[0].Code)
		assert.True(t, len((*bookingStatusResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle status != 200 error from supplier", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound) // 404 for testing
		}))
		defer testServer.Close()

		configuration := bookingStatusDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingStatusParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		bookingStatusResponse, _ := getBookingStatus(params, &log, redisClient)

		assert.Len(t, *bookingStatusResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, (*bookingStatusResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*bookingStatusResponse.Errors)[0].Message)
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

		configuration := bookingStatusDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingStatusParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		bookingStatusResponse, _ := getBookingStatus(params, &log, redisClient)

		assert.Len(t, *bookingStatusResponse.SupplierRequests, 2)

		assert.Equal(t, testServer.URL+"/v1/authorize", *(*bookingStatusResponse.SupplierRequests)[0].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*bookingStatusResponse.SupplierRequests)[0].RequestContent.Method)
		assert.Len(t, *(*bookingStatusResponse.SupplierRequests)[0].RequestContent.Headers, 1)
		assert.Equal(t, http.StatusOK, *(*bookingStatusResponse.SupplierRequests)[0].ResponseContent.StatusCode)
		assert.Len(t, *(*bookingStatusResponse.SupplierRequests)[0].ResponseContent.Headers, 3)

		assert.Equal(t, testServer.URL+"/v1/bookings/14", *(*bookingStatusResponse.SupplierRequests)[1].RequestContent.Url)
		assert.Equal(t, http.MethodGet, *(*bookingStatusResponse.SupplierRequests)[1].RequestContent.Method)
		assert.Len(t, *(*bookingStatusResponse.SupplierRequests)[1].RequestContent.Headers, 2)
		assert.Equal(t, http.StatusOK, *(*bookingStatusResponse.SupplierRequests)[1].ResponseContent.StatusCode)
		assert.Len(t, *(*bookingStatusResponse.SupplierRequests)[1].ResponseContent.Headers, 2)
	})
}

func bookingStatusDefaultConfiguration() schema.AnyRentConfiguration {
	return schema.AnyRentConfiguration{
		ApiKey: "test-api-key",
	}
}

func bookingStatusParamsTemplate(configuration schema.AnyRentConfiguration) schema.BookingStatusRequestParams {
	b, _ := jsonEncoding.Marshal(configuration)

	var cp schema.BookingStatusRequestParams_Configuration
	jsonEncoding.Unmarshal(b, &cp)

	return schema.BookingStatusRequestParams{
		SupplierBookingReference: "14",
		ReservNumber:             "K48730916F3",
		Contact: &schema.Contact{
			Email: "dumb@www.com",
		},
		Timeouts:      schema.Timeouts{Default: 8000},
		Configuration: cp,
	}
}

func getBookingStatus(params schema.BookingStatusRequestParams, log *zerolog.Logger, redisClient *redis.Client) (schema.BookingStatusResponse, error) {
	service := anyrent.New(redisClient)
	ctx := context.Background()
	return service.GetBookingStatus(ctx, params, log)
}

func defaultBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/response_default.json")

	return bookingStatusBody
}

func failedBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/response_failed.json")

	return bookingStatusBody
}

func failedBookingStatusResponseWithCode() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/response_failed_with_code.json")

	return bookingStatusBody
}

func defaultSupplierBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/supplier_response_default.json")

	return bookingStatusBody
}

func expiredSupplierBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/supplier_response_expired.json")

	return bookingStatusBody
}

func pendingSupplierBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/supplier_response_pending.json")

	return bookingStatusBody
}

func pendingBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/response_pending.json")

	return bookingStatusBody
}

func canceledSupplierBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/supplier_response_canceled.json")

	return bookingStatusBody
}

func canceledBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/response_canceled.json")

	return bookingStatusBody
}

func notShowedSupplierBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/supplier_response_not_showed.json")

	return bookingStatusBody
}

func fulfilledSupplierBookingStatusResponse() []byte {
	bookingStatusBody, _ := os.ReadFile("./testdata/bookingstatus/supplier_response_fulfilled.json")

	return bookingStatusBody
}
