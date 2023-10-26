package bookingcom_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom"
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
			name                string
			requestParams       func(url string) schema.BookingStatusRequestParams
			expectedRequestFile string
		}{
			{
				"general",
				func(url string) schema.BookingStatusRequestParams {
					configuration := bookingStatusDefaultConfiguration()
					configuration.SupplierApiUrl = url
					return bookingStatusParamsTemplate(configuration)
				},
				"./testdata/bookingstatus/booking_status_request.xml",
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
				handlerFunc = func(w http.ResponseWriter, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					xmlBody, reqFileErr := os.ReadFile(test.expectedRequestFile)
					assert.Nil(t, reqFileErr)

					assert.Equal(t, "application/xml", r.Header.Get("Content-Type"))
					assert.Equal(t, "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36", r.Header.Get("User-Agent"))
					assert.Equal(t, strings.ReplaceAll(string(xmlBody), "    ", "\t"), string(body))

					w.WriteHeader(http.StatusOK)
					w.Write([]byte("<BookingStatusRS version=\"1.1\"><Booking id=\"123456789\"  status=\"booking confirmed\"  statusCode=\"1\"  /></BookingStatusRS>"))

					handlerFuncCalled = true
				}

				redisClient, _ := redismock.NewClientMock()
				_, err := bookingStatus(test.requestParams(testServer.URL), &log, redisClient)

				assert.Nil(t, err)
				assert.True(t, handlerFuncCalled)
			})
		}
	})

	t.Run("should return status, based on supplier response", func(t *testing.T) {
		var handlerFunc http.HandlerFunc
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFunc(w, r)
		}))
		defer testServer.Close()

		tests := []struct {
			name                 string
			expectedResponseFile string
			expectedStatus       schema.BookingStatusResponseStatus
			expectedErrorsCount  int
		}{
			{
				"success on status (confirmed)",
				"./testdata/bookingstatus/confirmed_response.xml",
				schema.BookingStatusResponseStatusOK,
				0,
			},
			{
				"success on status (completed)",
				"./testdata/bookingstatus/completed_response.xml",
				schema.BookingStatusResponseStatusOK,
				0,
			},
			{
				"pending on status (accepted)",
				"./testdata/bookingstatus/accepted_response.xml",
				schema.BookingStatusResponseStatusPENDING,
				0,
			},
			{
				"pending on status (manual confirmation required)",
				"./testdata/bookingstatus/manual_confirmation_response.xml",
				schema.BookingStatusResponseStatusPENDING,
				0,
			},
			{
				"pending on status (unconfirmed modification)",
				"./testdata/bookingstatus/unconfirmed_modification_response.xml",
				schema.BookingStatusResponseStatusPENDING,
				0,
			},
			{
				"pending on status (check)",
				"./testdata/bookingstatus/check_response.xml",
				schema.BookingStatusResponseStatusPENDING,
				0,
			},
			{
				"pending on status (payment failed)",
				"./testdata/bookingstatus/payment_failed_response.xml",
				schema.BookingStatusResponseStatusPENDING,
				0,
			},
			{
				"failed on status (cancelled)",
				"./testdata/bookingstatus/cancelled_response.xml",
				schema.BookingStatusResponseStatusFAILED,
				0,
			},
			{
				"failed on unexpected error",
				"./testdata/bookingstatus/failed_response.xml",
				schema.BookingStatusResponseStatusFAILED,
				1,
			},
		}

		configuration := bookingStatusDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingStatusParamsTemplate(configuration)

		for _, test := range tests {

			t.Run(test.name, func(t *testing.T) {
				handlerFunc = func(w http.ResponseWriter, r *http.Request) {
					xmlBody, reqFileErr := os.ReadFile(test.expectedResponseFile)
					assert.Nil(t, reqFileErr)

					w.WriteHeader(http.StatusOK)
					w.Write(xmlBody)
				}

				redisClient, _ := redismock.NewClientMock()
				bookingStatusResponse, err := bookingStatus(params, &log, redisClient)

				assert.Nil(t, err)
				assert.Equal(t, test.expectedStatus, *&bookingStatusResponse.Status)
				assert.Equal(t, test.expectedErrorsCount, len(*bookingStatusResponse.Errors))
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

		redisClient, _ := redismock.NewClientMock()
		bookingStatusResponse, err := bookingStatus(params, &log, redisClient)

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

		redisClient, _ := redismock.NewClientMock()
		service := bookingcom.New(redisClient)

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

		redisClient, _ := redismock.NewClientMock()
		bookingStatusResponse, _ := bookingStatus(params, &log, redisClient)

		assert.Len(t, *bookingStatusResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, (*bookingStatusResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*bookingStatusResponse.Errors)[0].Message)
	})

	t.Run("should return build supplier requests history array", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			body, err := os.ReadFile("./testdata/bookingstatus/confirmed_response.xml")
			assert.Nil(t, err)
			w.Write(body)
		}))
		defer testServer.Close()

		configuration := bookingStatusDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingStatusParamsTemplate(configuration)

		redisClient, _ := redismock.NewClientMock()
		bookingStatusResponse, _ := bookingStatus(params, &log, redisClient)

		assert.Len(t, *bookingStatusResponse.SupplierRequests, 1)

		assert.Equal(t, testServer.URL, *(*bookingStatusResponse.SupplierRequests)[0].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*bookingStatusResponse.SupplierRequests)[0].RequestContent.Method)
		assert.Len(t, *(*bookingStatusResponse.SupplierRequests)[0].RequestContent.Headers, 2)

		assert.Equal(t, http.StatusOK, *(*bookingStatusResponse.SupplierRequests)[0].ResponseContent.StatusCode)
		assert.Len(t, *(*bookingStatusResponse.SupplierRequests)[0].ResponseContent.Headers, 3)
	})
}

func bookingStatusDefaultConfiguration() schema.BookingComConfiguration {
	return schema.BookingComConfiguration{
		Username:     "test-user",
		Password:     "test-password",
		SupplierName: "test-supplier-name",
	}
}

func bookingStatusParamsTemplate(configuration schema.BookingComConfiguration) schema.BookingStatusRequestParams {
	b, _ := json.Marshal(configuration)

	var cp schema.BookingStatusRequestParams_Configuration
	json.Unmarshal(b, &cp)

	return schema.BookingStatusRequestParams{
		SupplierBookingReference: "",
		ReservNumber:             "K48730916F3",
		Contact: &schema.Contact{
			Email: "dumb@www.com",
		},
		Timeouts:      schema.Timeouts{Default: 8000},
		Configuration: cp,
	}
}

func bookingStatus(params schema.BookingStatusRequestParams, log *zerolog.Logger, redisClient *redis.Client) (schema.BookingStatusResponse, error) {
	service := bookingcom.New(redisClient)
	ctx := context.Background()
	return service.GetBookingStatus(ctx, params, log)
}
