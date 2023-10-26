package anyrent_test

import (
	"bytes"
	"context"
	"encoding/json"
	jsonEncoding "encoding/json"
	"io"
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

func TestBookingRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build booking request based on params", func(t *testing.T) {
		tests := []struct {
			name            string
			requestParams   func(url string) schema.BookingRequestParams
			expectedRequest []byte
		}{
			{
				"general",
				func(url string) schema.BookingRequestParams {
					configuration := bookingDefaultConfiguration()
					configuration.SupplierApiUrl = url
					return bookingParamsTemplate(configuration)
				},
				defaultSupplierBookingRequest(),
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

					// mock the booking response
					if handlerFuncCalledCount == 2 {
						assert.Equal(t, "/v1/bookings", r.RequestURI)
						assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
						assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
						assert.Equal(t, "en", r.Header.Get("x-lang"))
						assert.Equal(t, "POST", r.Method)

						w.Write([]byte(defaultSupplierBookingResponse()))

						body, _ := io.ReadAll(r.Body)
						assert.Equal(t, strings.ReplaceAll(string(test.expectedRequest), "    ", "\t"), strings.ReplaceAll(string(body), "    ", "\t"))
					}
				}

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				_, err := createBooking(test.requestParams(testServer.URL), &log, redisClient)

				assert.Nil(t, err)
				assert.True(t, handlerFuncCalled)
				assert.Equal(t, 2, handlerFuncCalledCount)
			})
		}
	})

	t.Run("should parse supplier responses correctly", func(t *testing.T) {
		tests := []struct {
			name                    string
			configuration           schema.AnyRentConfiguration
			supplierResponseCode    int
			supplierBookingResponse []byte
			expectedResponse        []byte
		}{
			{
				name:                    "confirmed",
				configuration:           bookingDefaultConfiguration(),
				supplierResponseCode:    http.StatusOK,
				supplierBookingResponse: defaultSupplierBookingResponse(),
				expectedResponse:        defaultBookingResponse(),
			},
			{
				name:                    "expired",
				configuration:           bookingDefaultConfiguration(),
				supplierResponseCode:    http.StatusOK,
				supplierBookingResponse: expiredSupplierBookingResponse(),
				expectedResponse:        failedBookingResponse(),
			},
			{
				name:                    "pending",
				configuration:           bookingDefaultConfiguration(),
				supplierResponseCode:    http.StatusOK,
				supplierBookingResponse: pendingSupplierBookingResponse(),
				expectedResponse:        pendingBookingResponse(),
			},
			{
				name:                    "canceled",
				configuration:           bookingDefaultConfiguration(),
				supplierResponseCode:    http.StatusOK,
				supplierBookingResponse: canceledSupplierBookingResponse(),
				expectedResponse:        canceledBookingResponse(),
			},
			{
				name:                    "no_show",
				configuration:           bookingDefaultConfiguration(),
				supplierResponseCode:    http.StatusOK,
				supplierBookingResponse: notShowedSupplierBookingResponse(),
				expectedResponse:        failedBookingResponse(),
			},
			{
				name:                    "fulfilled",
				configuration:           bookingDefaultConfiguration(),
				supplierResponseCode:    http.StatusOK,
				supplierBookingResponse: fulfilledSupplierBookingResponse(),
				expectedResponse:        failedBookingResponse(),
			},
			{
				name:                    "failed with http code",
				configuration:           bookingDefaultConfiguration(),
				supplierResponseCode:    http.StatusForbidden,
				supplierBookingResponse: defaultSupplierBookingResponse(),
				expectedResponse:        failedBookingResponseWithCode(),
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
					w.Write(test.supplierBookingResponse)
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := bookingParamsTemplate(test.configuration)

				redisClient, mock := redismock.NewClientMock()
				cachedKey, _ := getCachedAndCompressedAuthKey()
				mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
				mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

				service := anyrent.New(redisClient)
				ctx := context.Background()
				booking, err := service.CreateBooking(ctx, params, &log)
				assert.Nil(t, err)

				booking.SupplierRequests = nil
				actual, _ := jsonEncoding.MarshalIndent(booking, "", "	")

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

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingParamsTemplate(configuration)
		params.Timeouts.Default = 1

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		bookingResponse, err := createBooking(params, &log, redisClient)

		assert.Nil(t, err)
		assert.Len(t, *bookingResponse.Errors, 1)
		assert.Equal(t, schema.TimeoutError, (*bookingResponse.Errors)[0].Code)
		assert.True(t, len((*bookingResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle supplier connection errors", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		service := anyrent.New(redisClient)

		channel := make(chan schema.BookingResponse, 1)

		go func() {
			ctx := context.Background()
			bookingResponse, _ := service.CreateBooking(ctx, params, &log)
			channel <- bookingResponse
		}()
		time.Sleep(5 * time.Millisecond)
		testServer.CloseClientConnections() // close the connection to force transport level error

		bookingResponse := <-channel

		assert.Len(t, *bookingResponse.Errors, 1)
		assert.Equal(t, schema.ConnectionError, (*bookingResponse.Errors)[0].Code)
		assert.True(t, len((*bookingResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle status != 200 error from supplier", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound) // 404 for testing
		}))
		defer testServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		bookingResponse, _ := createBooking(params, &log, redisClient)

		assert.Len(t, *bookingResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, (*bookingResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*bookingResponse.Errors)[0].Message)
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

			// mock the booking response
			if handlerFuncCalledCount == 2 {
				w.Write([]byte(defaultSupplierBookingResponse()))
			}
		}))
		defer testServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingParamsTemplate(configuration)

		redisClient, mock := redismock.NewClientMock()
		cachedKey, _ := getCachedAndCompressedAuthKey()
		mock.ExpectGet(defaultAuthRedisCacheKey(testServer.URL)).RedisNil()
		mock.ExpectSetEx(defaultAuthRedisCacheKey(testServer.URL), cachedKey, time.Duration(3600)*time.Second).SetVal("")

		bookingResponse, _ := createBooking(params, &log, redisClient)

		assert.Len(t, *bookingResponse.SupplierRequests, 2)

		assert.Equal(t, testServer.URL+"/v1/authorize", *(*bookingResponse.SupplierRequests)[0].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*bookingResponse.SupplierRequests)[0].RequestContent.Method)
		assert.Len(t, *(*bookingResponse.SupplierRequests)[0].RequestContent.Headers, 1)
		assert.Equal(t, http.StatusOK, *(*bookingResponse.SupplierRequests)[0].ResponseContent.StatusCode)
		assert.Len(t, *(*bookingResponse.SupplierRequests)[0].ResponseContent.Headers, 3)

		assert.Equal(t, testServer.URL+"/v1/bookings", *(*bookingResponse.SupplierRequests)[1].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*bookingResponse.SupplierRequests)[1].RequestContent.Method)
		assert.Len(t, *(*bookingResponse.SupplierRequests)[1].RequestContent.Headers, 3)
		assert.Equal(t, http.StatusOK, *(*bookingResponse.SupplierRequests)[1].ResponseContent.StatusCode)
		assert.Len(t, *(*bookingResponse.SupplierRequests)[1].ResponseContent.Headers, 3)
	})
}

func bookingDefaultConfiguration() schema.AnyRentConfiguration {
	return schema.AnyRentConfiguration{
		ApiKey: "test-api-key",
	}
}

func bookingParamsTemplate(configuration schema.AnyRentConfiguration) schema.BookingRequestParams {
	b, _ := json.Marshal(configuration)

	var bp schema.BookingRequestParams_Configuration
	json.Unmarshal(b, &bp)

	pickup, _ := time.Parse(schema.DateTimeFormat, "2023-09-01T12:30:00")
	dropOff, _ := time.Parse(schema.DateTimeFormat, "2023-09-02T12:30:00")

	flightNo := "841"
	driverTitle := "Mr"
	extraQuantity := 10

	return schema.BookingRequestParams{
		ReservNumber: "82428499",
		PickUp: schema.RequestBranchWithTimeZone{
			Code:     "OPO",
			DateTime: pickup,
		},
		VehicleClass:          "CDAR",
		BrokerReference:       "15844563",
		SupplierRateReference: "{\"pickupStation\":\"OPO\",\"pickupDate\":\"2023-09-28 14:30:00\",\"dropOffStation\":\"OPO\",\"dropOffDate\":\"2023-09-29 14:30:00\",\"group\":\"C1\"}",
		DropOff: schema.RequestBranchWithTimeZone{
			Code:     "OPO",
			DateTime: dropOff,
		},
		ExtrasAndFees: &[]schema.BookingExtraOrFee{{
			Code: "607304703",
			Type: "Fee",
		}, {
			Code:     "607304702",
			Type:     "Extra",
			Quantity: &extraQuantity,
		}},
		FlightNo:      &flightNo,
		Timeouts:      schema.Timeouts{Default: 8000},
		Configuration: bp,
		Customer: schema.Customer{
			Title:            &driverTitle,
			FirstName:        "tester",
			LastName:         "tester",
			ResidenceCountry: "ee",
			Email:            "example@exampleemail.com",
			Age:              18,
		},
	}
}

func createBooking(params schema.BookingRequestParams, log *zerolog.Logger, redisClient *redis.Client) (schema.BookingResponse, error) {
	service := anyrent.New(redisClient)
	ctx := context.Background()
	return service.CreateBooking(ctx, params, log)
}

func defaultSupplierBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/supplier_response_default.json")

	return bookingBody
}

func defaultBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/response_default.json")

	return bookingBody
}

func expiredSupplierBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/supplier_response_expired.json")

	return bookingBody
}

func failedBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/response_failed.json")

	return bookingBody
}

func failedBookingResponseWithCode() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/response_failed_with_code.json")

	return bookingBody
}

func pendingSupplierBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/supplier_response_pending.json")

	return bookingBody
}

func pendingBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/response_pending.json")

	return bookingBody
}

func canceledSupplierBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/supplier_response_canceled.json")

	return bookingBody
}

func canceledBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/response_canceled.json")

	return bookingBody
}

func notShowedSupplierBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/supplier_response_not_showed.json")

	return bookingBody
}

func fulfilledSupplierBookingResponse() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/supplier_response_fulfilled.json")

	return bookingBody
}

func defaultSupplierBookingRequest() []byte {
	bookingBody, _ := os.ReadFile("./testdata/booking/request_default.json")

	return bookingBody
}
