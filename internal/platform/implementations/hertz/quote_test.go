package hertz_test

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

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func quoteDefaultConfiguration() schema.HertzConfiguration {
	return schema.HertzConfiguration{
		VendorCode:    "ZT",
		Taco:          converting.PointerToValue("00295632"),
		Vc:            converting.PointerToValue("T20C3I9N14T"),
		Vn:            converting.PointerToValue("T007"),
		Cp:            converting.PointerToValue("D4D6"),
		Tour:          converting.PointerToValue("IT1000254TAM"),
		BookingAgent:  converting.PointerToValue("HZ00295632"),
		RateQualifier: converting.PointerToValue("L8TAM"),
	}
}

func quoteParamsTemplate(configuration schema.HertzConfiguration) schema.RatesRequestParams {
	b, _ := json.Marshal(configuration)

	var cp schema.RatesRequestParams_Configuration
	json.Unmarshal(b, &cp)

	pickup, _ := time.Parse(schema.DateTimeFormat, "2023-08-03T21:00:00")
	dropOff, _ := time.Parse(schema.DateTimeFormat, "2023-08-14T07:00:00")

	return schema.RatesRequestParams{
		PickUp: schema.RequestBranch{
			Code:     "SEA",
			DateTime: pickup,
		},
		DropOff: schema.RequestBranch{
			Code:     "SEA",
			DateTime: dropOff,
		},
		Booking: &schema.ExistingBooking{
			SupplierBookingReference: "bookingReference",
			FirstName:                converting.PointerToValue("FirstName"),
			LastName:                 converting.PointerToValue("LastName"),
			Phone:                    converting.PointerToValue("+372"),
			Email:                    converting.PointerToValue("email@example.com"),
			ReservNumber:             converting.PointerToValue("112233"),
		},
		Timeouts:         schema.Timeouts{Default: 8000},
		BranchExtras:     &[]string{"someExtra"},
		ResidenceCountry: "US",
		Configuration:    cp,
	}
}

func quoteRates(params schema.RatesRequestParams, log *zerolog.Logger, redisClient *redis.Client) (schema.RatesResponse, error) {
	service := hertz.New(redisClient)
	ctx := context.Background()
	return service.GetRates(ctx, params, log)
}

func TestQuoteRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	transport := http.DefaultTransport.(*http.Transport)

	t.Run("should build requests based on params", func(t *testing.T) {
		tests := []struct {
			name                  string
			ratesResponse         schema.RatesResponse
			extras                []ota.PricedEquip
			requestParams         func(url string) schema.RatesRequestParams
			expectedRequestFile   string
			expectedRequestsCount int
			expectedErrorsCount   int
		}{
			{
				"general",
				schema.RatesResponse{
					Vehicles: []schema.Vehicle{
						{Class: "ECAR"},
					},
				},
				[]ota.PricedEquip{},
				func(url string) schema.RatesRequestParams {
					configuration := quoteDefaultConfiguration()
					configuration.SupplierApiUrl = url
					params := quoteParamsTemplate(configuration)
					return params
				},
				"./testdata/quote/quote_general_quote_ECAR.xml",
				1,
				0,
			},
			{
				"does not add Voucher to quote request",
				schema.RatesResponse{
					Vehicles: []schema.Vehicle{
						{Class: "ECAR"},
					},
				},
				[]ota.PricedEquip{},
				func(url string) schema.RatesRequestParams {
					configuration := quoteDefaultConfiguration()
					configuration.SupplierApiUrl = url
					configuration.SendVoucher = converting.PointerToValue(false)
					params := quoteParamsTemplate(configuration)
					return params
				},
				"./testdata/quote/quote_general_quote_ECAR.xml",
				1,
				0,
			},
			{
				"adds Voucher to quote request",
				schema.RatesResponse{
					Vehicles: []schema.Vehicle{
						{Class: "ECAR"},
					},
				},
				[]ota.PricedEquip{},
				func(url string) schema.RatesRequestParams {
					configuration := quoteDefaultConfiguration()
					configuration.SupplierApiUrl = url
					configuration.SendVoucher = converting.PointerToValue(true)
					params := quoteParamsTemplate(configuration)
					return params
				},
				"./testdata/quote/quote_general_quote_ECAR_with_voucher.xml",
				1,
				0,
			},
			{
				"passes errors and supplier requests",
				schema.RatesResponse{
					SupplierRequests: &[]schema.SupplierRequest{
						{},
					},
					Errors: &[]schema.SupplierResponseError{
						{},
					},
					Vehicles: []schema.Vehicle{
						{Class: "ECAR"},
					},
				},
				[]ota.PricedEquip{},
				func(url string) schema.RatesRequestParams {
					configuration := quoteDefaultConfiguration()
					configuration.SupplierApiUrl = url
					params := quoteParamsTemplate(configuration)
					return params
				},
				"./testdata/quote/quote_general_quote_ECAR.xml",
				2,
				1,
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
					xmlBody, _ := os.ReadFile(test.expectedRequestFile)

					assert.Equal(t, "application/xml; charset=utf-8", r.Header.Get("Content-Type"))
					assert.Equal(t, string(xmlBody), strings.ReplaceAll(string(body), "    ", "\t"))

					w.WriteHeader(http.StatusOK)

					xmlBody, _ = os.ReadFile("./testdata/quote/quote_general_valid_response.xml")
					w.Write(xmlBody)

					handlerFuncCalled = true
				}

				quoteRequest := hertz.NewQuoteRequest(test.requestParams(testServer.URL), &log)
				ctx := context.TODO()
				response, err := quoteRequest.Execute(ctx, transport, test.ratesResponse, test.extras)

				assert.Nil(t, err)
				assert.True(t, handlerFuncCalled)
				assert.Equal(t, test.expectedErrorsCount, len(*response.Errors))
				assert.Equal(t, test.expectedRequestsCount, len(*response.SupplierRequests))
			})
		}
	})

	t.Run("should handle timeout from supplier", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond) // timeout in params is 1ms
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := quoteDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := quoteParamsTemplate(configuration)
		params.Timeouts.Default = 1

		redisClient, _ := redismock.NewClientMock()
		quoteResponse, err := quoteRates(params, &log, redisClient)

		assert.Nil(t, err)
		assert.Len(t, *quoteResponse.Errors, 2)
		assert.Equal(t, schema.TimeoutError, (*quoteResponse.Errors)[0].Code)
		assert.True(t, len((*quoteResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle supplier connection errors", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := quoteDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := quoteParamsTemplate(configuration)

		channel := make(chan schema.RatesResponse, 1)

		go func() {
			redisClient, _ := redismock.NewClientMock()
			quoteResponse, _ := quoteRates(params, &log, redisClient)
			channel <- quoteResponse
		}()
		time.Sleep(5 * time.Millisecond)
		testServer.CloseClientConnections() // close the connection to force transport level error

		quoteResponse := <-channel

		assert.Len(t, *quoteResponse.Errors, 2)
		assert.Equal(t, schema.ConnectionError, (*quoteResponse.Errors)[0].Code)
		assert.True(t, len((*quoteResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle status != 200 error from supplier", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound) // 404 for testing
		}))
		defer testServer.Close()

		configuration := quoteDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := quoteParamsTemplate(configuration)

		redisClient, _ := redismock.NewClientMock()
		quoteResponse, _ := quoteRates(params, &log, redisClient)

		assert.Len(t, *quoteResponse.Errors, 2)
		assert.Equal(t, schema.SupplierError, (*quoteResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*quoteResponse.Errors)[0].Message)
	})
}
