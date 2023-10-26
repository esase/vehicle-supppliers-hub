package hertz_test

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

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"github.com/go-redis/redismock/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func bookingDefaultConfiguration() schema.HertzConfiguration {
	return schema.HertzConfiguration{
		VendorCode:       "ZE",
		Taco:             converting.PointerToValue("91266313"),
		Vc:               converting.PointerToValue("5E24X16P9IA"),
		Cp:               converting.PointerToValue("3X93"),
		Vn:               converting.PointerToValue("T744"),
		LastName:         converting.PointerToValue("TESTNAME"),
		ResidenceCountry: converting.PointerToValue("GB"),
	}
}

func mergeBookingParamsAndConfiguration(params schema.BookingRequestParams, configuration schema.HertzConfiguration) schema.BookingRequestParams {
	b, _ := json.Marshal(configuration)

	var cp schema.BookingRequestParams_Configuration
	json.Unmarshal(b, &cp)

	params.Configuration = cp

	return params
}

func rateReference(ref mapping.SupplierRateReference) string {
	bytes, _ := json.Marshal(ref)
	return string(bytes)
}

func bookingDefaultParams() schema.BookingRequestParams {
	pickup, _ := time.Parse(schema.DateTimeFormat, "2023-07-10T10:00:00")
	dropOff, _ := time.Parse(schema.DateTimeFormat, "2023-07-17T10:00:00")

	return schema.BookingRequestParams{
		PickUp: schema.RequestBranchWithTimeZone{
			Code:     "QRY",
			DateTime: pickup,
		},
		DropOff: schema.RequestBranchWithTimeZone{
			Code:     "QRY",
			DateTime: dropOff,
		},
		Customer: schema.Customer{
			Phone:            "53535353",
			ResidenceCountry: "EE",
			FirstName:        "First",
			LastName:         "Last",
			Email:            "asd@example.com",
		},
		SupplierRateReference: rateReference(mapping.SupplierRateReference{
			FromRates:                    "rates",
			FromQuote:                    "quote",
			EstimatedTotalAmount:         "100.01",
			EstimatedTotalAmountCurrency: "EUR",
		}),
		Timeouts: schema.Timeouts{Default: 8000},
	}
}

func defaultSupplierBookingRequest() ota.VehResRQ {
	return ota.VehResRQ{
		Xmlns:             "http://www.opentravel.org/OTA/2003/05",
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
		XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehResRS.xsd",
		Version:           "1.008",
		MaxResponses:      10,
		POS: ota.POS{
			Source: []ota.Source{
				{
					ISOCountry:    "EE",
					AgentDutyCode: "5E24X16P9IA",
					RequestorID: ota.RequestorID{
						Type: "4",
						ID:   "T744",
						CompanyName: &ota.CompanyName{
							Code:        "CP",
							CodeContext: "3X93",
						},
					},
				},
				{
					RequestorID: ota.RequestorID{
						Type: "8",
						ID:   "ZE",
					},
				},
				{
					RequestorID: ota.RequestorID{
						Type: "5",
						ID:   "91266313",
					},
				},
			},
		},
		VehResRQCore: ota.VehResRQCore{
			Status: "All",
			VehRentalCore: ota.VehRentalCore{
				PickUpDateTime: "2023-07-10T10:00:00",
				ReturnDateTime: "2023-07-17T10:00:00",
				PickUpLocation: ota.Location{
					LocationCode: "QRY",
				},
				ReturnLocation: &ota.Location{
					LocationCode: "QRY",
				},
			},
			Customer: ota.BookingCustomer{
				Primary: ota.BookingPrimary{
					PersonName: ota.PersonName{
						GivenName: "First",
						Surname:   "Last",
					},
					Telephone: &ota.Telephone{
						PhoneNumber:   "53535353",
						PhoneTechType: 1,
					},
					Email: "asd@example.com",
				},
			},
			SpecialEquipPrefs: &ota.SpecialEquipPrefs{},
		},
		VehResRQInfo: ota.VehResRQInfo{
			Reference: &ota.Reference{
				Type: "16",
				ID:   "rates",
			},
		},
	}
}

func TestBookingRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build supplier booking requests correctly", func(t *testing.T) {
		tests := []struct {
			name                    string
			requestParams           schema.BookingRequestParams
			configuration           schema.HertzConfiguration
			expectedSupplierRequest ota.VehResRQ
		}{
			{
				name:                    "general",
				requestParams:           bookingDefaultParams(),
				configuration:           bookingDefaultConfiguration(),
				expectedSupplierRequest: defaultSupplierBookingRequest(),
			},
			{
				name: "AirlineCode walk-in",
				requestParams: func() schema.BookingRequestParams {
					p := bookingDefaultParams()
					p.AirlineCode = converting.PointerToValue("Walk-In")
					return p
				}(),
				configuration: bookingDefaultConfiguration(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.VehResRQInfo.ArrivalDetails = &ota.ArrivalDetails{
						TransportationCode: "24",
					}
					return r
				}(),
			},
			{
				name: "AirlineCode and FlightNumber",
				requestParams: func() schema.BookingRequestParams {
					p := bookingDefaultParams()
					p.FlightNo = converting.PointerToValue("FOO")
					p.AirlineCode = converting.PointerToValue("BAR")
					return p
				}(),
				configuration: bookingDefaultConfiguration(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.VehResRQInfo.ArrivalDetails = &ota.ArrivalDetails{
						TransportationCode: "14",
						Number:             "FOO",
						OperatingCompany: &ota.OperatingCompany{
							Code: "BAR",
						},
					}
					return r
				}(),
			},
			{
				name: "BillingNumber",
				requestParams: func() schema.BookingRequestParams {
					p := bookingDefaultParams()
					p.BrokerReference = "123"
					p.SupplierSpecificInformation = &schema.SupplierSpecificInformation{
						HertzBillingNumber: converting.PointerToValue("555"),
					}
					return p
				}(),
				configuration: func() schema.HertzConfiguration {
					c := bookingDefaultConfiguration()
					c.SendAgencyBillingNumberWithBooking = converting.PointerToValue(true)
					return c
				}(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.POS.Source = append(r.POS.Source, ota.Source{
						RequestorID: ota.RequestorID{
							Type: "16",
							ID:   "123",
						},
					})
					r.VehResRQInfo.RentalPaymentPref = &ota.RentalPaymentPref{
						Voucher: &ota.Voucher{
							SeriesCode:    "123",
							BillingNumber: "555",
						},
					}
					return r
				}(),
			},
			{
				name: "VoucherContractBillingType",
				requestParams: func() schema.BookingRequestParams {
					p := bookingDefaultParams()
					p.BrokerReference = "123"
					return p
				}(),
				configuration: func() schema.HertzConfiguration {
					c := bookingDefaultConfiguration()
					c.VoucherContractBillingType = converting.PointerToValue("Partial")
					return c
				}(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.POS.Source = append(r.POS.Source, ota.Source{
						RequestorID: ota.RequestorID{
							Type: "16",
							ID:   "123",
						},
					})
					r.VehResRQInfo.RentalPaymentPref = &ota.RentalPaymentPref{
						PaymentAmount: &ota.PaymentAmount{
							Amount:       "100.01",
							CurrencyCode: "EUR",
						},
					}
					return r
				}(),
			},
			{
				name: "BillingNumber and VoucherContractBillingType",
				requestParams: func() schema.BookingRequestParams {
					p := bookingDefaultParams()
					p.BrokerReference = "123"
					p.SupplierSpecificInformation = &schema.SupplierSpecificInformation{
						HertzBillingNumber: converting.PointerToValue("555"),
					}
					return p
				}(),
				configuration: func() schema.HertzConfiguration {
					c := bookingDefaultConfiguration()
					c.SendAgencyBillingNumberWithBooking = converting.PointerToValue(true)
					c.VoucherContractBillingType = converting.PointerToValue("Partial")
					return c
				}(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.POS.Source = append(r.POS.Source, ota.Source{
						RequestorID: ota.RequestorID{
							Type: "16",
							ID:   "123",
						},
					})
					r.VehResRQInfo.RentalPaymentPref = &ota.RentalPaymentPref{
						Voucher: &ota.Voucher{
							SeriesCode:    "123",
							BillingNumber: "555",
						},
						PaymentAmount: &ota.PaymentAmount{
							Amount:       "100.01",
							CurrencyCode: "EUR",
						},
					}
					return r
				}(),
			},
			{
				name: "sendVoucher only",
				requestParams: func() schema.BookingRequestParams {
					p := bookingDefaultParams()
					p.BrokerReference = "123"
					p.SupplierSpecificInformation = &schema.SupplierSpecificInformation{
						HertzBillingNumber: converting.PointerToValue("555"),
					}
					return p
				}(),
				configuration: func() schema.HertzConfiguration {
					c := bookingDefaultConfiguration()
					c.SendVoucher = converting.PointerToValue(true)
					c.SendAgencyBillingNumberWithBooking = converting.PointerToValue(false)
					return c
				}(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.POS.Source = append(r.POS.Source, ota.Source{
						RequestorID: ota.RequestorID{
							Type: "16",
							ID:   "123",
						},
					})
					r.VehResRQInfo.RentalPaymentPref = &ota.RentalPaymentPref{
						Voucher: &ota.Voucher{
							SeriesCode: "123",
						},
					}
					return r
				}(),
			},
			{
				name: "UseDirectSell",
				requestParams: func() schema.BookingRequestParams {
					p := bookingDefaultParams()
					p.VehicleClass = "ECMR"
					return p
				}(),
				configuration: func() schema.HertzConfiguration {
					c := bookingDefaultConfiguration()
					c.UseDirectSell = converting.PointerToValue(true)
					return c
				}(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.VehResRQCore.VehPref = &ota.VehPref{
						Code:        "ECMR",
						CodeContext: "SIPP",
					}
					r.VehResRQInfo.Reference = nil
					return r
				}(),
			},
			{
				name: "extras and fees",
				requestParams: func() schema.BookingRequestParams {
					p := bookingDefaultParams()
					p.ExtrasAndFees = &[]schema.BookingExtraOrFee{
						{
							Code:     "ABC",
							Quantity: converting.PointerToValue(1),
						},
					}
					return p
				}(),
				configuration: bookingDefaultConfiguration(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.VehResRQCore.SpecialEquipPrefs = &ota.SpecialEquipPrefs{
						SpecialEquipPref: []ota.SpecialEquipPref{
							{
								EquipType: "ABC",
								Quantity:  1,
							},
						},
					}
					return r
				}(),
			},
			{
				name: "comments",
				requestParams: func() schema.BookingRequestParams {
					p := bookingDefaultParams()
					p.Comments = &struct {
						Customer *string "json:\"customer,omitempty\""
					}{
						Customer: converting.PointerToValue("Hello ⱻ"),
					}
					return p
				}(),
				configuration: bookingDefaultConfiguration(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.VehResRQInfo.SpecialReqPref = "Hello E"
					return r
				}(),
			},
			{
				name:          "isoCountry mapping",
				requestParams: bookingDefaultParams(),
				configuration: func() schema.HertzConfiguration {
					c := bookingDefaultConfiguration()
					c.ResidenceCountryMapping = &map[string]string{
						"EE": "YY",
					}
					return c
				}(),
				expectedSupplierRequest: func() ota.VehResRQ {
					r := defaultSupplierBookingRequest()
					r.POS.Source[0] = ota.Source{
						ISOCountry:    "YY",
						AgentDutyCode: "5E24X16P9IA",
						RequestorID: ota.RequestorID{
							Type: "4",
							ID:   "T744",
							CompanyName: &ota.CompanyName{
								Code:        "CP",
								CodeContext: "3X93",
							},
						},
					}
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
				handlerFuncCalled := false
				handlerFunc = func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "application/xml; charset=utf-8", r.Header.Get("Content-Type"))

					body, _ := io.ReadAll(r.Body)
					expected, _ := xml.Marshal(test.expectedSupplierRequest)

					actual := strings.ReplaceAll(string(body), "\n", "")
					actual = strings.ReplaceAll(actual, "    ", "")

					assert.Equal(t, string(expected), actual)

					w.WriteHeader(http.StatusNoContent)
					handlerFuncCalled = true
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := mergeBookingParamsAndConfiguration(test.requestParams, test.configuration)

				redisClient, _ := redismock.NewClientMock()
				service := hertz.New(redisClient)
				ctx := context.Background()

				service.CreateBooking(ctx, params, &log)

				assert.True(t, handlerFuncCalled)
			})
		}
	})

	t.Run("should craft correct supplierData", func(t *testing.T) {
		validResponse, _ := xml.Marshal(ota.VehResRS{
			VehResRSCore: ota.VehResRSCore{
				VehReservation: ota.VehReservation{
					VehSegmentCore: ota.VehSegmentCore{
						ConfID: ota.ConfID{
							ID: "id",
						},
					},
				},
			},
		})

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(validResponse)
		}))
		defer testServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeBookingParamsAndConfiguration(bookingDefaultParams(), configuration)

		rateReference := mapping.SupplierRateReference{
			FromRates:                    "A",
			EstimatedTotalAmount:         "100",
			EstimatedTotalAmountCurrency: "EUR",
		}

		reference, _ := json.Marshal(rateReference)
		params.SupplierRateReference = string(reference)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		bookingResponse, _ := service.CreateBooking(ctx, params, &log)

		supplierData := &mapping.SupplierData{
			LastName:         "Last",
			ResidenceCountry: "EE",
		}

		assert.Equal(t, string(schema.BookingResponseStatusOK), string(bookingResponse.Status))
		assert.Equal(t, supplierData.AsMap(), bookingResponse.SupplierData)
	})

	t.Run("should handle configuration timeouts", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL

		p := bookingDefaultParams()
		p.Timeouts = schema.Timeouts{
			Booking: converting.PointerToValue(1),
			Default: 1,
		}

		params := mergeBookingParamsAndConfiguration(p, configuration)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		bookingResponse, _ := service.CreateBooking(ctx, params, &log)

		supplierError := converting.Unwrap(bookingResponse.Errors)[0]

		assert.Len(t, *bookingResponse.Errors, 1)
		assert.Equal(t, schema.TimeoutError, supplierError.Code)
		assert.True(t, len(supplierError.Message) > 0)
		assert.Equal(t, bookingResponse.Status, schema.BookingResponseStatusFAILED)
	})

	t.Run("should handle connection errors", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeBookingParamsAndConfiguration(bookingDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)

		channel := make(chan schema.BookingResponse, 1)

		go func() {
			ctx := context.Background()
			bookingResponse, _ := service.CreateBooking(ctx, params, &log)
			channel <- bookingResponse
		}()
		time.Sleep(5 * time.Millisecond)
		testServer.CloseClientConnections()

		bookingResponse := <-channel

		supplierError := converting.Unwrap(bookingResponse.Errors)[0]

		assert.Len(t, *bookingResponse.Errors, 1)
		assert.Equal(t, schema.ConnectionError, supplierError.Code)
		assert.True(t, len(supplierError.Message) > 0)
	})

	t.Run("should handle status != 200 error", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer testServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeBookingParamsAndConfiguration(bookingDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		bookingResponse, _ := service.CreateBooking(ctx, params, &log)

		supplierError := converting.Unwrap(bookingResponse.Errors)[0]

		assert.Len(t, *bookingResponse.Errors, 1)
		assert.Equal(t, schema.SupplierError, supplierError.Code)
		assert.Equal(t, "supplier returned status code 404", supplierError.Message)
		assert.Equal(t, bookingResponse.Status, schema.BookingResponseStatusFAILED)
	})

	t.Run("should return errors from supplier response", func(t *testing.T) {
		errorResponse, _ := xml.Marshal(ota.VehResRS{
			ErrorsMixin: ota.ErrorsMixin{
				Errors: ota.Errors{
					Error: []ota.Error{
						{
							Type:      "003",
							ShortText: "INCORRECT SPECIAL EQUIPMENT CODE",
							Code:      "250",
							RecordID:  "026",
						},
					},
				},
			},
		})

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(errorResponse)
		}))
		defer testServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeBookingParamsAndConfiguration(bookingDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		bookingResponse, _ := service.CreateBooking(ctx, params, &log)

		supplierError := converting.Unwrap(bookingResponse.Errors)[0]

		assert.Len(t, converting.Unwrap(bookingResponse.Errors), 1)
		assert.Equal(t, schema.SupplierError, supplierError.Code)
		assert.Equal(t, "INCORRECT SPECIAL EQUIPMENT CODE", supplierError.Message)
		assert.Equal(t, bookingResponse.Status, schema.BookingResponseStatusFAILED)
	})

	t.Run("should return build supplier requests history array", func(t *testing.T) {
		errorResponse, _ := xml.Marshal(ota.VehResRS{
			ErrorsMixin: ota.ErrorsMixin{
				Errors: ota.Errors{
					Error: []ota.Error{
						{
							Type:      "003",
							ShortText: "INCORRECT SPECIAL EQUIPMENT CODE",
							Code:      "250",
							RecordID:  "026",
						},
					},
				},
			},
		})

		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write(errorResponse)
		}))
		defer testServer.Close()

		configuration := bookingDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeBookingParamsAndConfiguration(bookingDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		bookingResponse, _ := service.CreateBooking(ctx, params, &log)

		assert.Len(t, (*bookingResponse.SupplierRequests), 1)

		supplierRequest := converting.Unwrap(bookingResponse.SupplierRequests)[0]

		assert.Equal(t, testServer.URL, converting.Unwrap(supplierRequest.RequestContent.Url))
		assert.Equal(t, http.MethodPost, converting.Unwrap(supplierRequest.RequestContent.Method))
		assert.Len(t, converting.Unwrap(supplierRequest.RequestContent.Headers), 1)

		assert.Equal(t, http.StatusOK, converting.Unwrap(supplierRequest.ResponseContent.StatusCode))
		assert.Len(t, converting.Unwrap(supplierRequest.ResponseContent.Headers), 3)
	})
}
