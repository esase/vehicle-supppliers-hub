package bookingcom_test

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"github.com/go-redis/redismock/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestRatesRequestRR(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build supplier rates", func(t *testing.T) {
		tests := []struct {
			name                 string
			requestParams        schema.RatesRequestParams
			configuration        schema.BookingComConfiguration
			expectedRatesRequest ota.SearchRQ
		}{
			{
				name:                 "general",
				requestParams:        ratesDefaultParams(),
				configuration:        ratesDefaultConfiguration(),
				expectedRatesRequest: defaultSupplierRatesRequest(),
			},
		}

		var handlerFunc http.HandlerFunc
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFunc(w, r)
		}))
		defer testServer.Close()

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				handlerFunc = func(w http.ResponseWriter, req *http.Request) {
					body, _ := io.ReadAll(req.Body)
					actual := strings.ReplaceAll(string(body), "\n", "")
					actual = strings.ReplaceAll(actual, "    ", "")

					assert.Equal(t, "application/xml", req.Header.Get("Content-Type"))
					assert.Equal(t, "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36", req.Header.Get("User-Agent"))

					var tmp ota.SearchRQ
					xml.Unmarshal(body, &tmp)

					var expected []byte

					expected, _ = xml.Marshal(test.expectedRatesRequest)

					assert.Equal(t, string(expected), actual)

					w.WriteHeader(http.StatusOK)
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := mergeRatesParamsAndConfiguration(test.requestParams, test.configuration)

				redisClient, _ := redismock.NewClientMock()
				service := bookingcom.New(redisClient)
				ctx := context.Background()
				service.GetRates(ctx, params, &log)
			})
		}
	})

	t.Run("should parse supplier responses correctly", func(t *testing.T) {
		tests := []struct {
			name                  string
			configuration         schema.BookingComConfiguration
			supplierRatesResponse ota.SearchRS
			expectedResponse      schema.RatesResponse
		}{
			{
				name:                  "general",
				configuration:         ratesDefaultConfiguration(),
				supplierRatesResponse: defaultSupplierRatesResponse(),
				expectedResponse:      defaultRatesResponse(),
			},
			{
				name:          "should not return vehicle if AvailabilityCheck equals false",
				configuration: ratesDefaultConfiguration(),
				supplierRatesResponse: func() ota.SearchRS {
					r := defaultSupplierRatesResponse()
					r.MatchList.Match[0].Vehicle.AvailabilityCheck = false
					return r
				}(),
				expectedResponse: func() schema.RatesResponse {
					r := defaultRatesResponse()
					r.Vehicles = []schema.Vehicle{}
					return r
				}(),
			},
		}

		var handlerFunc http.HandlerFunc
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFunc(w, r)
		}))
		defer testServer.Close()

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				handlerFunc = func(w http.ResponseWriter, req *http.Request) {
					body, _ := io.ReadAll(req.Body)
					var tmp ota.SearchRQ
					xml.Unmarshal(body, &tmp)

					var r []byte

					r, _ = xml.Marshal(test.supplierRatesResponse)

					w.WriteHeader(http.StatusOK)
					w.Write(r)
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := mergeRatesParamsAndConfiguration(ratesDefaultParams(), test.configuration)

				redisClient, _ := redismock.NewClientMock()
				service := bookingcom.New(redisClient)
				ctx := context.Background()
				rates, err := service.GetRates(ctx, params, &log)

				assert.Nil(t, err)

				rates.SupplierRequests = nil
				test.expectedResponse.SupplierRequests = nil

				expected, _ := json.Marshal(test.expectedResponse)
				actual, _ := json.Marshal(rates)

				assert.Equal(t, string(expected), string(actual))
			})
		}
	})

	t.Run("should handle configuration timeouts", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL

		p := ratesDefaultParams()
		p.Timeouts = schema.Timeouts{
			Rates:   converting.PointerToValue(1),
			Default: 1,
		}

		params := mergeRatesParamsAndConfiguration(p, configuration)

		redisClient, _ := redismock.NewClientMock()
		service := bookingcom.New(redisClient)
		ctx := context.Background()

		ratesResponse, _ := service.GetRates(ctx, params, &log)

		assert.Len(t, *ratesResponse.Errors, 1)
		assert.Equal(t, schema.TimeoutError, (*ratesResponse.Errors)[0].Code)
		assert.True(t, len((*ratesResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle status != 200 error", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeRatesParamsAndConfiguration(ratesDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := bookingcom.New(redisClient)
		ctx := context.Background()

		ratesResponse, _ := service.GetRates(ctx, params, &log)

		assert.Len(t, *ratesResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, (*ratesResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*ratesResponse.Errors)[0].Message)
	})

	t.Run("should return errors from supplier response", func(t *testing.T) {
		errorResponse, _ := xml.Marshal(ota.SearchRS{
			Errors: ota.Errors{
				Error: ota.ErrorMessage{
					Message: "INCORRECT SPECIAL EQUIPMENT CODE",
				},
			},
		})

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(errorResponse)
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeRatesParamsAndConfiguration(ratesDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := bookingcom.New(redisClient)
		ctx := context.Background()

		ratesResponse, _ := service.GetRates(ctx, params, &log)

		assert.Len(t, *ratesResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, (*ratesResponse.Errors)[0].Code)
		assert.Equal(t, "INCORRECT SPECIAL EQUIPMENT CODE", (*ratesResponse.Errors)[0].Message)
	})

	t.Run("should return build supplier requests history array", func(t *testing.T) {
		errorResponse, _ := xml.Marshal(ota.SearchRS{
			Errors: ota.Errors{
				Error: ota.ErrorMessage{
					Message: "INCORRECT SPECIAL EQUIPMENT CODE",
				},
			},
		})

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(errorResponse)
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeRatesParamsAndConfiguration(ratesDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := bookingcom.New(redisClient)
		ctx := context.Background()

		ratesResponse, _ := service.GetRates(ctx, params, &log)

		assert.Len(t, *ratesResponse.SupplierRequests, 1)

		r1 := converting.Unwrap(ratesResponse.SupplierRequests)[0]

		assert.Equal(t, testServer.URL, *r1.RequestContent.Url)
		assert.Equal(t, http.MethodPost, *r1.RequestContent.Method)
		assert.Len(t, *r1.RequestContent.Headers, 2)
		assert.Equal(t, http.StatusOK, *r1.ResponseContent.StatusCode)
		assert.Len(t, *r1.ResponseContent.Headers, 3)
	})
}

func rateReference(ref mapping.SupplierRateReference) string {
	bytes, _ := json.Marshal(ref)
	return string(bytes)
}

func ratesDefaultConfiguration() schema.BookingComConfiguration {
	return schema.BookingComConfiguration{
		Username:     "test-user",
		Password:     "test-password",
		SupplierName: "test-supplier-name",
	}
}

func mergeRatesParamsAndConfiguration(params schema.RatesRequestParams, configuration schema.BookingComConfiguration) schema.RatesRequestParams {
	b, _ := json.Marshal(configuration)

	var cp schema.RatesRequestParams_Configuration
	json.Unmarshal(b, &cp)

	params.Configuration = cp

	return params
}

func ratesDefaultParams() schema.RatesRequestParams {
	pickup, _ := time.Parse(schema.DateTimeFormat, "2023-09-01T12:30:00")
	dropOff, _ := time.Parse(schema.DateTimeFormat, "2023-09-02T12:30:00")

	return schema.RatesRequestParams{
		PickUp: schema.RequestBranch{
			Code:     "MAN",
			DateTime: pickup,
		},
		DropOff: schema.RequestBranch{
			Code:     "MAN",
			DateTime: dropOff,
		},
		Contract: schema.Contract{
			PaymentType: 0,
		},
		Age:              39,
		RentalDays:       7,
		ResidenceCountry: "US",
		Timeouts:         schema.Timeouts{Default: 8000},
	}
}

func defaultSupplierRatesRequest() ota.SearchRQ {
	return ota.SearchRQ{
		Version: ota.Version{
			Version: "1.1",
		},
		ReturnExtras:     true,
		SupplierInfo:     true,
		ResidenceCountry: "US",
		Credentials: ota.Credentials{
			Credentials: ota.CredentialsInfo{
				Username: "test-user",
				Password: "test-password",
			},
		},
		PickUp: ota.PickUp{
			Location: ota.Location{
				Location: "MAN",
			},
			Date: ota.Date{
				Year:   2023,
				Month:  9,
				Day:    1,
				Hour:   12,
				Minute: 30,
			},
		},
		DropOff: ota.DropOff{
			Location: ota.Location{
				Location: "MAN",
			},
			Date: ota.Date{
				Year:   2023,
				Month:  9,
				Day:    2,
				Hour:   12,
				Minute: 30,
			},
		},
		DriverAge: 39,
	}
}

func defaultSupplierRatesResponse() ota.SearchRS {
	return ota.SearchRS{
		MatchList: ota.SearchRSMatchList{
			Match: []ota.SearchRSMatch{{
				Vehicle: ota.SearchRSVehicle{
					Id:                "100",
					Name:              "A CHEVROLET SPARK OR SIMILAR",
					Group:             "ECAR",
					AvailabilityCheck: true,
					Aircon:            "yes",
					Doors:             "4",
					Seats:             "5",
					BigSuitcase:       "6",
					SmallSuitcase:     "7",
					Automatic:         "Automatic",
					Petrol:            "Diesel",
					LargeImageURL:     "http://test.com",
					UnlimitedMileage:  true,
				},
				Price: ota.SearchRSPrice{
					BasePrice:    463.03,
					BaseCurrency: "USD",
				},
				Route: ota.SearchRSRoute{
					PickUp: ota.SearchRSRouteLocation{
						Location: ota.SearchRSRouteLocationInfo{
							Id: "A",
						},
					},
					DropOff: ota.SearchRSRouteLocation{
						Location: ota.SearchRSRouteLocationInfo{
							Id: "B",
						},
					},
				},
				Supplier: ota.SearchRSSupplier{
					SupplierName: "test-supplier-name",
				},
				Fees: ota.SearchRSFees{
					DepositExcessFees: ota.SearchRSDepositExcessFees{
						TheftExcess: ota.SearchRSDepositExcessFee{
							Currency: "USD",
							Amount:   1,
						},
						DamageExcess: ota.SearchRSDepositExcessFee{
							Currency: "USD",
							Amount:   2,
						},
						Deposit: ota.SearchRSDepositExcessFee{
							Currency: "USD",
							Amount:   3,
						},
					},
					KnownFees: ota.SearchRSKnownFees{
						KnownFee: []ota.SearchRSKnownFee{{
							FeeTypeName:   "testFee",
							Currency:      "USD",
							Amount:        4,
							AlwaysPayable: true,
							PerDuration:   "rental",
						}},
					},
				},
				ExtraInfoList: ota.SearchRSExtraInfoList{
					ExtraInfo: []ota.SearchRSExtraInfo{{
						Extra: ota.SearchRSExtra{
							Product:   500,
							Name:      "testExtra",
							Available: 10,
						},
						Price: ota.SearchRSExtraPrice{
							PriceAvailable: true,
							PricePerWhat:   "rental",
							BasePrice:      6,
							BaseCurrency:   "USD",
						},
					}},
				},
			}},
		},
	}
}

func defaultRatesResponse() schema.RatesResponse {
	return schema.RatesResponse{
		Vehicles: []schema.Vehicle{
			{
				AcrissCode: converting.PointerToValue("ECAR"),
				Class:      "ECAR",
				Doors:      converting.PointerToValue(4),
				HasAirco:   converting.PointerToValue(true),
				Mileage: &schema.Mileage{
					Unlimited: converting.PointerToValue(true),
				},
				TransmissionType: converting.PointerToValue(schema.Automatic),
				FuelType:         converting.PointerToValue(schema.Diesel),
				Name:             "A CHEVROLET SPARK OR SIMILAR",
				Price: schema.PriceAmount{
					Amount:   463.03,
					Currency: "USD",
				},
				ImageUrl:       converting.PointerToValue("http://test.com"),
				Seats:          converting.PointerToValue(5),
				BigSuitcases:   converting.PointerToValue(6),
				SmallSuitcases: converting.PointerToValue(7),
				Status:         "AVAILABLE",
				SupplierRateReference: converting.PointerToValue(
					rateReference(mapping.SupplierRateReference{
						VehicleId:         "100",
						PickUpLocationId:  "A",
						DropOffLocationId: "B",
						BasePrice:         463.03,
						BaseCurrency:      "USD",
					}),
				),
				ExtrasAndFees: &[]schema.ExtraOrFee{
					{
						Code:           "TheftExcess",
						IncludedInRate: true,
						Mandatory:      true,
						Name:           "TheftExcess",
						PayLocal:       false,
						Price: schema.PriceAmount{
							Amount:   0,
							Currency: "USD",
						},
						Type: schema.VCT,
						Unit: converting.PointerToValue(schema.PerRental),
						Excess: converting.PointerToValue(schema.PriceAmount{
							Amount:   1.00,
							Currency: "USD",
						}),
					},
					{
						Code:           "DamageExcess",
						IncludedInRate: true,
						Mandatory:      true,
						Name:           "DamageExcess",
						PayLocal:       false,
						Price: schema.PriceAmount{
							Amount:   0,
							Currency: "USD",
						},
						Type: schema.VCT,
						Unit: converting.PointerToValue(schema.PerRental),
						Excess: converting.PointerToValue(schema.PriceAmount{
							Amount:   2.00,
							Currency: "USD",
						}),
					},
					{
						Code:           "Deposit",
						IncludedInRate: true,
						Mandatory:      true,
						Name:           "Deposit",
						PayLocal:       false,
						Price: schema.PriceAmount{
							Amount:   0,
							Currency: "USD",
						},
						Type: schema.VCT,
						Unit: converting.PointerToValue(schema.PerRental),
						Excess: converting.PointerToValue(schema.PriceAmount{
							Amount:   3.00,
							Currency: "USD",
						}),
					},
					{
						Code:           "testFee",
						IncludedInRate: false,
						Mandatory:      true,
						Name:           "testFee",
						PayLocal:       false,
						Price: schema.PriceAmount{
							Amount:   4.00,
							Currency: "USD",
						},
						Type: schema.VCP,
						Unit: converting.PointerToValue(schema.PerRental),
					},
					{
						Code:           "500",
						IncludedInRate: false,
						Mandatory:      false,
						Name:           "testExtra",
						PayLocal:       false,
						Price: schema.PriceAmount{
							Amount:   6.00,
							Currency: "USD",
						},
						MaxQuantity: converting.PointerToValue(10),
						Type:        schema.EQP,
						Unit:        converting.PointerToValue(schema.PerDay),
					},
				},
			},
		},
		Errors: &[]schema.SupplierResponseError{},
	}
}
