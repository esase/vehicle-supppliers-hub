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

func TestCancelRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build cancel request based on params", func(t *testing.T) {
		tests := []struct {
			name                string
			requestParams       func(url string) schema.CancelRequestParams
			expectedRequestFile string
		}{
			{
				"general",
				func(url string) schema.CancelRequestParams {
					configuration := cancelDefaultConfiguration()
					configuration.SupplierApiUrl = url
					return cancelParamsTemplate(configuration)
				},
				"./testdata/cancel/cancel_request.xml",
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
					w.Write([]byte("<CancelBookingRS><Status>OK</Status></CancelBookingRS>"))

					handlerFuncCalled = true
				}

				redisClient, _ := redismock.NewClientMock()
				_, err := cancelBooking(test.requestParams(testServer.URL), &log, redisClient)

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
			expectedStatus       schema.CancelResponseStatus
			expectedErrorsCount  int
		}{
			{
				"success on status",
				"./testdata/cancel/success_response.xml",
				schema.CancelResponseStatusOK,
				0,
			},
			{
				"failed on status",
				"./testdata/cancel/failed_response.xml",
				schema.CancelResponseStatusFAILED,
				0,
			},
			{
				"failed on unexpected error",
				"./testdata/cancel/failed2_response.xml",
				schema.CancelResponseStatusFAILED,
				1,
			},
		}

		configuration := cancelDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := cancelParamsTemplate(configuration)

		for _, test := range tests {

			t.Run(test.name, func(t *testing.T) {
				handlerFunc = func(w http.ResponseWriter, r *http.Request) {
					xmlBody, reqFileErr := os.ReadFile(test.expectedResponseFile)
					assert.Nil(t, reqFileErr)

					w.WriteHeader(http.StatusOK)
					w.Write(xmlBody)
				}

				redisClient, _ := redismock.NewClientMock()
				cancelResponse, err := cancelBooking(params, &log, redisClient)

				assert.Nil(t, err)
				assert.Equal(t, test.expectedStatus, *cancelResponse.Status)
				assert.Equal(t, test.expectedErrorsCount, len(*cancelResponse.Errors))
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

		redisClient, _ := redismock.NewClientMock()
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

		redisClient, _ := redismock.NewClientMock()
		service := bookingcom.New(redisClient)

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

		redisClient, _ := redismock.NewClientMock()
		cancelResponse, _ := cancelBooking(params, &log, redisClient)

		assert.Len(t, *cancelResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, (*cancelResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*cancelResponse.Errors)[0].Message)
	})

	t.Run("should return build supplier requests history array", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			body, err := os.ReadFile("./testdata/cancel/success_response.xml")
			assert.Nil(t, err)
			w.Write(body)
		}))
		defer testServer.Close()

		configuration := cancelDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := cancelParamsTemplate(configuration)

		redisClient, _ := redismock.NewClientMock()
		cancelResponse, _ := cancelBooking(params, &log, redisClient)

		assert.Len(t, *cancelResponse.SupplierRequests, 1)

		assert.Equal(t, testServer.URL, *(*cancelResponse.SupplierRequests)[0].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*cancelResponse.SupplierRequests)[0].RequestContent.Method)
		assert.Len(t, *(*cancelResponse.SupplierRequests)[0].RequestContent.Headers, 2)

		assert.Equal(t, http.StatusOK, *(*cancelResponse.SupplierRequests)[0].ResponseContent.StatusCode)
		assert.Len(t, *(*cancelResponse.SupplierRequests)[0].ResponseContent.Headers, 3)
	})
}

func cancelDefaultConfiguration() schema.BookingComConfiguration {
	return schema.BookingComConfiguration{
		Username:     "test-user",
		Password:     "test-password",
		SupplierName: "test-supplier-name",
	}
}

func cancelParamsTemplate(configuration schema.BookingComConfiguration) schema.CancelRequestParams {
	b, _ := json.Marshal(configuration)

	var cp schema.CancelRequestParams_Configuration
	json.Unmarshal(b, &cp)

	cancelReason := 4

	return schema.CancelRequestParams{
		SupplierBookingReference: "K48730916F3",
		Contact: schema.Contact{
			Email: "dumb@www.com",
		},
		CancelReason:  &cancelReason,
		Timeouts:      schema.Timeouts{Default: 8000},
		Configuration: cp,
	}
}

func cancelBooking(params schema.CancelRequestParams, log *zerolog.Logger, redisClient *redis.Client) (schema.CancelResponse, error) {
	service := bookingcom.New(redisClient)
	ctx := context.Background()
	return service.CancelBooking(ctx, params, log)
}
