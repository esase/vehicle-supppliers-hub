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

func TestBookingRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	bookingSuccessfulBody, _ := os.ReadFile("./testdata/booking/successful_response.xml")
	bookingStatusConfirmedBody, _ := os.ReadFile("./testdata/bookingstatus/confirmed_response.xml")

	t.Run("should build booking request based on params", func(t *testing.T) {
		tests := []struct {
			name                string
			requestParams       func(url string) schema.BookingRequestParams
			expectedRequestFile string
		}{
			{
				"general",
				func(url string) schema.BookingRequestParams {
					configuration := bookingDefaultConfiguration()
					configuration.SupplierApiUrl = url
					return bookingParamsTemplate(configuration)
				},
				"./testdata/booking/booking_request.xml",
			},
		}

		// mock the supplier
		var handlerFunc http.HandlerFunc
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFunc(w, r)
		}))
		defer testServer.Close()

		testUserServiceServer, testSpitServiceServer := mockApiServices()
		defer testUserServiceServer.Close()
		defer testSpitServiceServer.Close()

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				handlerFuncCalledCount := 0
				handlerFuncCalled := false

				handlerFunc = func(w http.ResponseWriter, r *http.Request) {
					handlerFuncCalled = true
					handlerFuncCalledCount++

					// check the booking request
					if handlerFuncCalledCount == 1 {
						body, _ := io.ReadAll(r.Body)
						xmlBody, reqFileErr := os.ReadFile(test.expectedRequestFile)
						assert.Nil(t, reqFileErr)

						assert.Equal(t, "application/xml", r.Header.Get("Content-Type"))
						assert.Equal(t, "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36", r.Header.Get("User-Agent"))
						assert.Equal(t, strings.ReplaceAll(string(xmlBody), "    ", "\t"), strings.ReplaceAll(string(body), "    ", "\t"))
					}

					w.WriteHeader(http.StatusOK)

					// mock the booking response
					if handlerFuncCalledCount == 1 {
						w.Write([]byte(bookingSuccessfulBody))
					}

					// mock the booking status response
					if handlerFuncCalledCount == 2 {
						w.Write([]byte(bookingStatusConfirmedBody))
					}
				}

				redisClient, _ := redismock.NewClientMock()
				_, err := createBooking(test.requestParams(testServer.URL), &log, redisClient)

				assert.Nil(t, err)
				assert.True(t, handlerFuncCalled)
				assert.Equal(t, 2, handlerFuncCalledCount)
			})
		}
	})

	t.Run("should return status, based on supplier response", func(t *testing.T) {
		var handlerFunc http.HandlerFunc
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFunc(w, r)
		}))
		defer testServer.Close()

		testUserServiceServer, testSpitServiceServer := mockApiServices()
		defer testUserServiceServer.Close()
		defer testSpitServiceServer.Close()

		tests := []struct {
			name                 string
			expectedResponseFile string
			expectedStatus       schema.BookingResponseStatus
			expectedErrorsCount  int
		}{
			{
				"success on status",
				"./testdata/booking/successful_response.xml",
				schema.BookingResponseStatusOK,
				0,
			},
			{
				"failed on unexpected error",
				"./testdata/booking/failed_response.xml",
				schema.BookingResponseStatusFAILED,
				1,
			},
		}

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingParamsTemplate(configuration)

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				handlerFuncCalledCount := 0

				handlerFunc = func(w http.ResponseWriter, r *http.Request) {
					handlerFuncCalledCount++

					// mock the booking response
					if handlerFuncCalledCount == 1 {
						xmlBody, reqFileErr := os.ReadFile(test.expectedResponseFile)
						assert.Nil(t, reqFileErr)
						w.Write(xmlBody)
					}

					// mock the booking status response
					if handlerFuncCalledCount == 2 {
						w.Write([]byte(bookingStatusConfirmedBody))
					}

					w.WriteHeader(http.StatusOK)
				}

				redisClient, _ := redismock.NewClientMock()
				bookingStatusResponse, err := createBooking(params, &log, redisClient)

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

		testUserServiceServer, testSpitServiceServer := mockApiServices()
		defer testUserServiceServer.Close()
		defer testSpitServiceServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingParamsTemplate(configuration)
		params.Timeouts.Default = 1

		redisClient, _ := redismock.NewClientMock()
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

		testUserServiceServer, testSpitServiceServer := mockApiServices()
		defer testUserServiceServer.Close()
		defer testSpitServiceServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingParamsTemplate(configuration)

		redisClient, _ := redismock.NewClientMock()
		service := bookingcom.New(redisClient)

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

		testUserServiceServer, testSpitServiceServer := mockApiServices()
		defer testUserServiceServer.Close()
		defer testSpitServiceServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingParamsTemplate(configuration)

		redisClient, _ := redismock.NewClientMock()
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

			if handlerFuncCalledCount == 1 {
				w.Write(bookingSuccessfulBody)
			}

			if handlerFuncCalledCount == 2 {
				w.Write(bookingStatusConfirmedBody)
			}
		}))
		defer testServer.Close()

		testUserServiceServer, testSpitServiceServer := mockApiServices()
		defer testUserServiceServer.Close()
		defer testSpitServiceServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := bookingParamsTemplate(configuration)

		redisClient, _ := redismock.NewClientMock()
		bookingResponse, _ := createBooking(params, &log, redisClient)

		assert.Len(t, *bookingResponse.SupplierRequests, 2)

		assert.Equal(t, testServer.URL, *(*bookingResponse.SupplierRequests)[0].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*bookingResponse.SupplierRequests)[0].RequestContent.Method)

		assert.Equal(t, testServer.URL, *(*bookingResponse.SupplierRequests)[1].RequestContent.Url)
		assert.Equal(t, http.MethodPost, *(*bookingResponse.SupplierRequests)[1].RequestContent.Method)

		assert.Len(t, *(*bookingResponse.SupplierRequests)[0].RequestContent.Headers, 2)
		assert.Len(t, *(*bookingResponse.SupplierRequests)[1].RequestContent.Headers, 2)

		assert.Equal(t, http.StatusOK, *(*bookingResponse.SupplierRequests)[0].ResponseContent.StatusCode)
		assert.Equal(t, http.StatusOK, *(*bookingResponse.SupplierRequests)[1].ResponseContent.StatusCode)

		assert.Len(t, *(*bookingResponse.SupplierRequests)[0].ResponseContent.Headers, 3)
		assert.Len(t, *(*bookingResponse.SupplierRequests)[1].ResponseContent.Headers, 3)
	})
}

func bookingDefaultConfiguration() schema.BookingComConfiguration {
	return schema.BookingComConfiguration{
		Username:      "test-user",
		Password:      "test-password",
		SupplierName:  "test-supplier-name",
		AffiliateCode: "test-affiliate-code",
	}
}

func bookingParamsTemplate(configuration schema.BookingComConfiguration) schema.BookingRequestParams {
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
			Code:     "MAN",
			DateTime: pickup,
		},
		SupplierPassthrough: &schema.SupplierPassthrough{
			Payment: schema.PassthroughPayment{
				Method:   "SptToken",
				SptToken: "test-token",
			},
		},
		VehicleClass:          "CDAR",
		BrokerReference:       "15844563",
		SupplierRateReference: "{\"vehicleId\":\"740171366\",\"baseCurrency\":\"USD\",\"basePrice\":42.61,\"pickUpLocationId\":\"4658225\",\"dropOffLocationId\":\"4658225\"}",
		DropOff: schema.RequestBranchWithTimeZone{
			Code:     "MAN",
			DateTime: dropOff,
		},
		ExtrasAndFees: &[]schema.BookingExtraOrFee{{
			Code: "607304703",
			Type: "Extra",
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
	service := bookingcom.New(redisClient)
	ctx := context.Background()
	return service.CreateBooking(ctx, params, log)
}

func mockApiServices() (testUserServiceServer *httptest.Server, testSpitServiceServer *httptest.Server) {
	// mock the user service
	testUserServiceServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		w.WriteHeader(http.StatusOK)
		uatBody, _ := os.ReadFile("./testdata/booking/uat_token_response.json")
		w.Write([]byte(uatBody))
	}))
	// defer testUserServiceServer.Close()
	os.Setenv("CRG_URL_USER_SERVICE", testUserServiceServer.URL)

	// mock the spit service
	testSpitServiceServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		w.WriteHeader(http.StatusCreated)
		spitBody, _ := os.ReadFile("./testdata/booking/spit_response.json")
		w.Write([]byte(spitBody))
	}))
	// defer testSpitServiceServer.Close()
	os.Setenv("CRG_URL_SPIT", testSpitServiceServer.URL)

	return testUserServiceServer, testSpitServiceServer
}
