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

func ratesDefaultConfiguration() schema.HertzConfiguration {
	sendVoucher := true
	excluded := []string{
		"NO",
		"SE",
		"DK",
	}
	FpPayFeesLocally := false

	return schema.HertzConfiguration{
		VendorCode:               "ZE",
		VendorCodeImport:         converting.PointerToValue("ZE"),
		SendVoucher:              &sendVoucher,
		TaxExclCoverageCountries: &excluded,
		FpPayFeesLocally:         &FpPayFeesLocally,
		Taco:                     converting.PointerToValue("268287089992"),
		Vc:                       converting.PointerToValue("T20C3I9N14T"),
		Vn:                       converting.PointerToValue("T007"),
		Cp:                       converting.PointerToValue("D4D6"),
		Tour:                     converting.PointerToValue("ITHI00295540"),
	}
}

func mergeRatesParamsAndConfiguration(params schema.RatesRequestParams, configuration schema.HertzConfiguration) schema.RatesRequestParams {
	b, _ := json.Marshal(configuration)

	var cp schema.RatesRequestParams_Configuration
	json.Unmarshal(b, &cp)

	params.Configuration = cp

	return params
}

func ratesDefaultParams() schema.RatesRequestParams {
	pickup, _ := time.Parse(schema.DateTimeFormat, "2023-07-10T10:00:00")
	dropOff, _ := time.Parse(schema.DateTimeFormat, "2023-07-17T10:00:00")

	return schema.RatesRequestParams{
		PickUp: schema.RequestBranch{
			Code:     "QRY",
			DateTime: pickup,
		},
		DropOff: schema.RequestBranch{
			Code:     "QRY",
			DateTime: dropOff,
		},
		Contract: schema.Contract{
			PaymentType: 0,
		},
		RentalDays:       7,
		ResidenceCountry: "US",
		Timeouts:         schema.Timeouts{Default: 8000},
		BranchExtras:     &[]string{"7", "8", "9"},
	}
}

func defaultSupplierRatesRequest() ota.VehAvailRateRQ {
	return ota.VehAvailRateRQ{
		Xmlns:             "http://www.opentravel.org/OTA/2003/05",
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
		XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehAvailRateRQ.xsd",
		Version:           "1.008",
		MaxResponses:      10,
		POS: ota.POS{
			Source: []ota.Source{
				{
					ISOCountry:    "US",
					AgentDutyCode: "T20C3I9N14T",
					RequestorID: ota.RequestorID{
						Type: "4",
						ID:   "T007",
						CompanyName: &ota.CompanyName{
							Code:        "CP",
							CodeContext: "D4D6",
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
						ID:   "268287089992",
					},
				},
			},
		},
		VehAvailRQCore: ota.VehAvailRQCore{
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
			RateQualifier: ota.RateQualifier{},
		},
		VehAvailRQInfo: ota.VehAvailRQInfo{
			TourInfo: &ota.TourInfo{
				TourNumber: "ITHI00295540",
			},
		},
	}
}

func defaultSupplierExtrasRequest() ota.VehAvailRateRQ {
	return ota.VehAvailRateRQ{
		Xmlns:             "http://www.opentravel.org/OTA/2003/05",
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
		XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehAvailRateRQ.xsd",
		Version:           "1.008",
		MaxResponses:      10,
		POS: ota.POS{
			Source: []ota.Source{
				{
					ISOCountry:    "US",
					AgentDutyCode: "T20C3I9N14T",
					RequestorID: ota.RequestorID{
						Type: "4",
						ID:   "T007",
						CompanyName: &ota.CompanyName{
							Code:        "CP",
							CodeContext: "D4D6",
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
						ID:   "268287089992",
					},
				},
			},
		},
		VehAvailRQCore: ota.VehAvailRQCore{
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
			RateQualifier: ota.RateQualifier{},
			SpecialEquipPrefs: &ota.SpecialEquipPrefs{
				SpecialEquipPref: []ota.SpecialEquipPref{
					{
						EquipType: "7",
						Quantity:  1,
					},
					{
						EquipType: "8",
						Quantity:  1,
					},
					{
						EquipType: "9",
						Quantity:  1,
					},
				},
			},
		},
		VehAvailRQInfo: ota.VehAvailRQInfo{
			TourInfo: &ota.TourInfo{
				TourNumber: "ITHI00295540",
			},
		},
	}
}

func defaultSupplierRatesResponse() ota.VehAvailRateRS {
	return ota.VehAvailRateRS{
		VehAvailRSCore: ota.VehAvailRSCore{
			VehVendorAvails: ota.VehVendorAvails{
				VehVendorAvail: ota.VehVendorAvail{
					VehAvails: ota.VehAvails{
						VehAvail: []ota.VehAvail{
							{
								VehAvailCore: ota.VehAvailCore{
									Status: "Available",
									Vehicle: ota.Vehicle{
										PassengerQuantity: 4,
										BaggageQuantity:   2,
										AirConditionInd:   true,
										TransmissionType:  "Automatic",
										FuelType:          "Unspecified",
										DriveType:         "Unspecified",
										Code:              "ECAR",
										CodeContext:       "SIPP",
										VehMakeModel: ota.VehMakeModel{
											Name: "A CHEVROLET SPARK OR SIMILAR",
											Code: "ECAR",
										},
										VehType: ota.VehType{
											DoorCount:       5,
											VehicleCategory: 1,
										},
									},
									RentalRate: ota.RentalRate{
										RateDistance: ota.RateDistance{
											Unlimited:             true,
											DistUnitName:          "Mile",
											VehiclePeriodUnitName: "RentalPeriod",
										},
										VehicleCharges: ota.VehicleCharges{
											VehicleCharge: []ota.VehicleCharge{
												{
													Purpose:        1,
													TaxInclusive:   true,
													GuaranteedInd:  true,
													Amount:         463.03,
													CurrencyCode:   "USD",
													IncludedInRate: false,
													Calculation: []ota.ChargeCalculation{
														{
															UnitCharge: 295.03,
															UnitName:   "Week",
															Quantity:   1,
														},
														{
															UnitCharge: 42,
															UnitName:   "Day",
															Quantity:   4,
														},
													},
													TaxAmounts: ota.TaxAmounts{
														TaxAmount: []ota.TaxAmount{
															{
																Total:        30,
																CurrencyCode: "USD",
																Percentage:   10,
																Description:  "Tax",
															},
														},
													},
												},
												{
													Purpose:        2,
													TaxInclusive:   false,
													GuaranteedInd:  true,
													Amount:         75,
													CurrencyCode:   "USD",
													IncludedInRate: false,
												},
											},
										},
										RateQualifier: ota.RateQualifierRS{
											ArriveByFlight: false,
											RateQualifier:  "VAUW",
										},
									},
									TotalCharge: ota.TotalCharge{
										RateTotalAmount:      463.03,
										EstimatedTotalAmount: 863.86,
										CurrencyCode:         "USD",
									},
									Fees: ota.Fees{
										Fee: []ota.Fee{
											{
												Purpose:      "5",
												Description:  "MISCELLANEOUS TRF FEE",
												TaxInclusive: true,
												Amount:       0,
												CurrencyCode: "USD",
											},
											{
												Purpose:      "5",
												Description:  "VEHICLE LICENSE RECOVERY FEE:",
												TaxInclusive: true,
												Amount:       0,
												CurrencyCode: "USD",
											},
										},
									},
									Reference: ota.Reference{
										Type: "16",
										ID:   "LRV0IT41SV35543-6401",
									},
								},
								VehAvailInfo: ota.VehAvailInfo{
									PricedCoverages: ota.PricedCoverages{
										PricedCoverage: []ota.PricedCoverage{
											{
												Required: true,
												Coverage: ota.Coverage{
													CoverageType: "24",
												},
												Charge: ota.Charge{
													IncludedInRate: false,
													Amount:         converting.PointerToValue(50.0),
													CurrencyCode:   "USD",
												},
											},
											{
												Coverage: ota.Coverage{
													CoverageType: "27",
												},
												Charge: ota.Charge{
													IncludedInRate: true,
													Amount:         converting.PointerToValue(50.0),
													CurrencyCode:   "USD",
												},
											},
											{
												Required: false,
												Coverage: ota.Coverage{
													CoverageType: "38",
												},
												Charge: ota.Charge{
													TaxInclusive:   false,
													IncludedInRate: false,
													CurrencyCode:   "USD",
													Calculation: []ota.CoverageCalculation{
														{
															UnitCharge: 10,
															UnitName:   "Day",
															Quantity:   1,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func defaultSupplierExtrasResponse() ota.VehAvailRateRS {
	return ota.VehAvailRateRS{
		VehAvailRSCore: ota.VehAvailRSCore{
			VehVendorAvails: ota.VehVendorAvails{
				VehVendorAvail: ota.VehVendorAvail{
					VehAvails: ota.VehAvails{
						VehAvail: []ota.VehAvail{
							{
								VehAvailCore: ota.VehAvailCore{
									PricedEquips: ota.PricedEquips{
										PricedEquip: []ota.PricedEquip{
											{
												Equipment: ota.Equipment{
													EquipType: "7",
													Quantity:  1,
												},
												Charge: ota.PricedEquipCharge{
													Amount:         98,
													TaxInclusive:   true,
													CurrencyCode:   "USD",
													IncludedInRate: false,
												},
											},
											{
												Equipment: ota.Equipment{
													EquipType: "8",
													Quantity:  1,
												},
												Charge: ota.PricedEquipCharge{
													Amount:         98,
													TaxInclusive:   true,
													CurrencyCode:   "USD",
													IncludedInRate: false,
												},
											},
											{
												Equipment: ota.Equipment{
													EquipType: "9",
													Quantity:  1,
												},
												Charge: ota.PricedEquipCharge{
													Amount:         98,
													TaxInclusive:   true,
													CurrencyCode:   "USD",
													IncludedInRate: false,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func defaultRatesResponse() schema.RatesResponse {
	return schema.RatesResponse{
		Vehicles: []schema.Vehicle{
			{
				AcrissCode: converting.PointerToValue("ECAR"),
				Class:      "ECAR",
				Doors:      converting.PointerToValue(5),
				HasAirco:   converting.PointerToValue(true),
				Mileage: &schema.Mileage{
					DistanceUnit:     converting.PointerToValue(schema.Km),
					IncludedDistance: converting.PointerToValue(""),
					PeriodUnit:       converting.PointerToValue(schema.RentalPeriod),
					Unlimited:        converting.PointerToValue(true),
				},
				TransmissionType: converting.PointerToValue(schema.Automatic),
				Name:             "A CHEVROLET SPARK OR SIMILAR",
				Price: schema.PriceAmount{
					Amount:   463.03,
					Currency: "USD",
				},
				Seats:          converting.PointerToValue(4),
				SmallSuitcases: converting.PointerToValue(2),
				Status:         "AVAILABLE",
				SupplierRateReference: converting.PointerToValue(
					rateReference(mapping.SupplierRateReference{
						FromRates:                    "LRV0IT41SV35543-6401",
						FromQuote:                    "",
						EstimatedTotalAmount:         "863.86",
						EstimatedTotalAmountCurrency: "USD",
					}),
				),
				ExtrasAndFees: &[]schema.ExtraOrFee{
					{
						Code:           "7",
						IncludedInRate: false,
						Mandatory:      true,
						Name:           "Tax",
						PayLocal:       false,
						Price: schema.PriceAmount{
							Amount:   30.00,
							Currency: "USD",
						},
						Type: schema.VCP,
					},
					{
						Code:           "7",
						IncludedInRate: false,
						Mandatory:      false,
						Name:           "7",
						PayLocal:       true,
						Price: schema.PriceAmount{
							Amount:   107.80, // with tax
							Currency: "USD",
						},
						Type: schema.EQP,
					},
					{
						Code:           "8",
						IncludedInRate: false,
						Mandatory:      false,
						Name:           "8",
						PayLocal:       true,
						Price: schema.PriceAmount{
							Amount:   107.80, // with tax
							Currency: "USD",
						},
						Type: schema.EQP,
					},
					{
						Code:           "9",
						IncludedInRate: false,
						Mandatory:      false,
						Name:           "9",
						PayLocal:       true,
						Price: schema.PriceAmount{
							Amount:   107.80, // with tax
							Currency: "USD",
						},
						Type: schema.EQP,
					},
					{
						Code:           "2",
						IncludedInRate: false,
						Mandatory:      true,
						Name:           "",
						PayLocal:       true,
						Price: schema.PriceAmount{
							Amount:   75.00,
							Currency: "USD",
						},
						Type: schema.VCP,
					},
					{
						Code:           "5",
						IncludedInRate: false,
						Mandatory:      true,
						Name:           "MISCELLANEOUS TRF FEE",
						PayLocal:       false,
						Price: schema.PriceAmount{
							Amount:   0.00,
							Currency: "USD",
						},
						Type: schema.VCP,
					},
					{
						Code:           "5",
						IncludedInRate: false,
						Mandatory:      true,
						Name:           "VEHICLE LICENSE RECOVERY FEE:",
						PayLocal:       false,
						Price: schema.PriceAmount{
							Amount:   0.00,
							Currency: "USD",
						},
						Type: schema.VCP,
					},
					{
						Code:           "24",
						IncludedInRate: false,
						Mandatory:      true,
						Name:           "",
						PayLocal:       true,
						Price: schema.PriceAmount{
							Amount:   50.00,
							Currency: "USD",
						},
						Type: schema.VCT,
					},
					{
						Code:           "27",
						IncludedInRate: true,
						Mandatory:      true,
						Name:           "",
						PayLocal:       false,
						Price: schema.PriceAmount{
							Amount:   55.00, // with tax
							Currency: "USD",
						},
						Type: schema.VCT,
					},
					{
						Code:           "38",
						IncludedInRate: false,
						Mandatory:      false,
						Name:           "",
						PayLocal:       true,
						Price: schema.PriceAmount{
							Amount:   77.00, // with tax
							Currency: "USD",
						},
						Type: schema.VCT,
					},
				},
			},
		},
		Errors: &[]schema.SupplierResponseError{},
	}
}

func TestRatesRequest(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should build supplier rates & extras requests", func(t *testing.T) {
		tests := []struct {
			name                  string
			requestParams         schema.RatesRequestParams
			configuration         schema.HertzConfiguration
			expectedRatesRequest  ota.VehAvailRateRQ
			expectedExtrasRequest ota.VehAvailRateRQ
		}{
			{
				name:                  "general",
				requestParams:         ratesDefaultParams(),
				configuration:         ratesDefaultConfiguration(),
				expectedRatesRequest:  defaultSupplierRatesRequest(),
				expectedExtrasRequest: defaultSupplierExtrasRequest(),
			},
			{
				name:          "with custom max response size",
				requestParams: ratesDefaultParams(),
				configuration: func() schema.HertzConfiguration {
					c := ratesDefaultConfiguration()
					c.MaxResponses = converting.PointerToValue("15")
					return c
				}(),
				expectedRatesRequest: func() ota.VehAvailRateRQ {
					r := defaultSupplierRatesRequest()
					r.MaxResponses = 15
					return r
				}(),
				expectedExtrasRequest: func() ota.VehAvailRateRQ {
					r := defaultSupplierExtrasRequest()
					r.MaxResponses = 15
					return r
				}(),
			},
			{
				name:          "mapping residence by key 'ALL' to custom ISOCountry",
				requestParams: ratesDefaultParams(),
				configuration: func() schema.HertzConfiguration {
					c := ratesDefaultConfiguration()
					c.ResidenceCountryMapping = &map[string]string{"ALL": "BR", "US": "GB"}
					return c
				}(),
				expectedRatesRequest: func() ota.VehAvailRateRQ {
					r := defaultSupplierRatesRequest()
					r.POS = ota.NewPOS("BR", ratesDefaultConfiguration(), "")
					return r
				}(),
				expectedExtrasRequest: func() ota.VehAvailRateRQ {
					r := defaultSupplierExtrasRequest()
					r.POS = ota.NewPOS("BR", ratesDefaultConfiguration(), "")
					return r
				}(),
			},
			{
				name:          "mapping residence by residecCountryMapping",
				requestParams: ratesDefaultParams(),
				configuration: func() schema.HertzConfiguration {
					c := ratesDefaultConfiguration()
					c.ResidenceCountryMapping = &map[string]string{"US": "GB"}
					return c
				}(),
				expectedRatesRequest: func() ota.VehAvailRateRQ {
					r := defaultSupplierRatesRequest()
					r.POS = ota.NewPOS("GB", ratesDefaultConfiguration(), "")
					return r
				}(),
				expectedExtrasRequest: func() ota.VehAvailRateRQ {
					r := defaultSupplierExtrasRequest()
					r.POS = ota.NewPOS("GB", ratesDefaultConfiguration(), "")
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
					actual := strings.ReplaceAll(string(body), "\n", "")
					actual = strings.ReplaceAll(actual, "    ", "")

					assert.Equal(t, "application/xml; charset=utf-8", req.Header.Get("Content-Type"))

					var tmp ota.VehAvailRateRQ
					xml.Unmarshal(body, &tmp)

					var expected []byte

					if tmp.VehAvailRQCore.SpecialEquipPrefs != nil {
						expected, _ = xml.Marshal(test.expectedExtrasRequest)
					} else {
						expected, _ = xml.Marshal(test.expectedRatesRequest)
					}

					assert.Equal(t, string(expected), actual)

					w.WriteHeader(http.StatusOK)
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := mergeRatesParamsAndConfiguration(test.requestParams, test.configuration)

				redisClient, _ := redismock.NewClientMock()
				service := hertz.New(redisClient)
				ctx := context.Background()
				service.GetRates(ctx, params, &log)
			})
		}
	})
	t.Run("should parse supplier responses correctly", func(t *testing.T) {
		tests := []struct {
			name                   string
			configuration          schema.HertzConfiguration
			supplierRatesResponse  ota.VehAvailRateRS
			supplierExtrasResponse ota.VehAvailRateRS
			expectedResponse       schema.RatesResponse
		}{
			{
				name:                   "general",
				configuration:          ratesDefaultConfiguration(),
				supplierRatesResponse:  defaultSupplierRatesResponse(),
				supplierExtrasResponse: defaultSupplierExtrasResponse(),
				expectedResponse:       defaultRatesResponse(),
			},
			{
				name: "include coverage in vehicle price",
				configuration: func() schema.HertzConfiguration {
					c := ratesDefaultConfiguration()
					c.IncludeCoveragesInRate = converting.PointerToValue(true)
					return c
				}(),
				supplierRatesResponse:  defaultSupplierRatesResponse(),
				supplierExtrasResponse: defaultSupplierExtrasResponse(),
				expectedResponse: func() schema.RatesResponse {
					r := defaultRatesResponse()

					extrasAndFees := []schema.ExtraOrFee{}

					// filter out 24 and 27
					for _, extraOrFee := range *r.Vehicles[0].ExtrasAndFees {
						if extraOrFee.Code == "24" || extraOrFee.Code == "27" {
							extraOrFee.Mandatory = true
							extraOrFee.IncludedInRate = true
						}
						extrasAndFees = append(extrasAndFees, extraOrFee)
					}

					r.Vehicles[0].ExtrasAndFees = &extrasAndFees
					r.Vehicles[0].Price.Amount = schema.RoundedFloat(568.03)

					return r
				}(),
			},
			{
				name:          "should not return vehice if status is not 'Available'",
				configuration: ratesDefaultConfiguration(),
				supplierRatesResponse: func() ota.VehAvailRateRS {
					r := defaultSupplierRatesResponse()
					r.VehAvailRSCore.VehVendorAvails.VehVendorAvail.VehAvails.VehAvail[0].VehAvailCore.Status = "OnRequest"
					return r
				}(),
				expectedResponse: func() schema.RatesResponse {
					r := defaultRatesResponse()
					r.Vehicles = []schema.Vehicle{}
					return r
				}(),
			},
			{
				name: "add tax to coverages",
				configuration: func() schema.HertzConfiguration {
					c := ratesDefaultConfiguration()
					c.AddTaxToCoverages = converting.PointerToValue([]string{"24"})
					return c
				}(),
				supplierRatesResponse: func() ota.VehAvailRateRS {
					r := defaultSupplierRatesResponse()
					r.VehAvailRSCore.VehVendorAvails.VehVendorAvail.
						VehAvails.VehAvail[0].VehAvailCore.RentalRate.
						VehicleCharges.VehicleCharge[0].TaxAmounts.TaxAmount[0].Percentage = 100

					return r
				}(),
				supplierExtrasResponse: defaultSupplierExtrasResponse(),
				expectedResponse: func() schema.RatesResponse {
					r := defaultRatesResponse()

					extrasAndFees := []schema.ExtraOrFee{}

					for _, extraOrFee := range *r.Vehicles[0].ExtrasAndFees {
						switch {
						case extraOrFee.Code == "7" && extraOrFee.Type == schema.EQP,
							extraOrFee.Code == "8" && extraOrFee.Type == schema.EQP,
							extraOrFee.Code == "9" && extraOrFee.Type == schema.EQP:
							extraOrFee.Price.Amount = schema.RoundedFloat(196.0)

						case extraOrFee.Code == "24" && extraOrFee.Type == schema.VCT:
							extraOrFee.Price.Amount = schema.RoundedFloat(100.0)
						case extraOrFee.Code == "27" && extraOrFee.Type == schema.VCT:
							extraOrFee.Price.Amount = schema.RoundedFloat(100.0)
						case extraOrFee.Code == "38" && extraOrFee.Type == schema.VCT:
							extraOrFee.Price.Amount = schema.RoundedFloat(140.0)
						}

						extrasAndFees = append(extrasAndFees, extraOrFee)
					}

					r.Vehicles[0].ExtrasAndFees = &extrasAndFees

					return r
				}(),
			},
			{
				name: "FpPaynowVehiclePriceWithTax",
				configuration: func() schema.HertzConfiguration {
					c := ratesDefaultConfiguration()
					c.FpPaynowVehiclePriceWithTax = converting.PointerToValue(true)
					return c
				}(),
				supplierRatesResponse:  defaultSupplierRatesResponse(),
				supplierExtrasResponse: defaultSupplierExtrasResponse(),
				expectedResponse: func() schema.RatesResponse {
					r := defaultRatesResponse()

					withOutTax := (*r.Vehicles[0].ExtrasAndFees)[1:]
					r.Vehicles[0].ExtrasAndFees = &withOutTax
					r.Vehicles[0].Price.Amount = schema.RoundedFloat(509.33)

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
					var tmp ota.VehAvailRateRQ
					xml.Unmarshal(body, &tmp)

					var r []byte

					if tmp.VehAvailRQCore.SpecialEquipPrefs != nil {
						r, _ = xml.Marshal(test.supplierExtrasResponse)
					} else {
						r, _ = xml.Marshal(test.supplierRatesResponse)
					}

					w.WriteHeader(http.StatusOK)
					w.Write(r)
				}

				test.configuration.SupplierApiUrl = testServer.URL
				params := mergeRatesParamsAndConfiguration(ratesDefaultParams(), test.configuration)

				redisClient, _ := redismock.NewClientMock()
				service := hertz.New(redisClient)
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

	t.Run("should handle failing extras request", func(t *testing.T) {
		handlerFunc := func(w http.ResponseWriter, req *http.Request) {
			bodyBytes, _ := io.ReadAll(req.Body)
			var requested *ota.VehAvailRateRQ
			xml.Unmarshal(bodyBytes, &requested)

			if requested.VehAvailRQCore.SpecialEquipPrefs != nil {
				w.WriteHeader(http.StatusGatewayTimeout)
				w.Write([]byte(""))
				return
			}

			r, _ := xml.Marshal(defaultSupplierRatesResponse())

			w.WriteHeader(http.StatusOK)
			w.Write(r)
		}
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handlerFunc(w, r)
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeRatesParamsAndConfiguration(ratesDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()

		service := hertz.New(redisClient)
		ctx := context.Background()

		response, err := service.GetRates(ctx, params, &log)

		assert.Nil(t, err)

		assert.Equal(t, 1, len(response.Vehicles))
		assert.Equal(t, 1, len(*response.Errors))
		assert.Equal(t, 2, len(*response.SupplierRequests))
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
		service := hertz.New(redisClient)
		ctx := context.Background()

		ratesResponse, _ := service.GetRates(ctx, params, &log)

		assert.Len(t, *ratesResponse.Errors, 2)
		assert.Equal(t, schema.TimeoutError, (*ratesResponse.Errors)[0].Code)
		assert.True(t, len((*ratesResponse.Errors)[0].Message) > 0)
	})

	t.Run("should handle connection errors", func(t *testing.T) {
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeRatesParamsAndConfiguration(ratesDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)

		channel := make(chan schema.RatesResponse, 2)

		go func() {
			ctx := context.Background()
			ratesResponse, _ := service.GetRates(ctx, params, &log)
			channel <- ratesResponse
		}()
		time.Sleep(5 * time.Millisecond)
		testServer.CloseClientConnections()

		ratesResponse := <-channel

		assert.Len(t, *ratesResponse.Errors, 2)
		assert.Equal(t, schema.ConnectionError, (*ratesResponse.Errors)[0].Code)
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
		service := hertz.New(redisClient)
		ctx := context.Background()

		ratesResponse, _ := service.GetRates(ctx, params, &log)

		assert.Len(t, *ratesResponse.Errors, 2)
		assert.Equal(t, schema.SupplierError, (*ratesResponse.Errors)[0].Code)
		assert.Equal(t, "supplier returned status code 404", (*ratesResponse.Errors)[0].Message)
	})

	t.Run("should return errors from supplier response", func(t *testing.T) {
		errorResponse, _ := xml.Marshal(ota.VehAvailRateRS{
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

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeRatesParamsAndConfiguration(ratesDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		ratesResponse, _ := service.GetRates(ctx, params, &log)

		assert.Len(t, *ratesResponse.Errors, 2)
		assert.Equal(t, schema.SupplierError, (*ratesResponse.Errors)[0].Code)
		assert.Equal(t, "INCORRECT SPECIAL EQUIPMENT CODE", (*ratesResponse.Errors)[0].Message)
	})

	t.Run("should return build supplier requests history array", func(t *testing.T) {
		errorResponse, _ := xml.Marshal(ota.VehAvailRateRS{
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

		configuration := ratesDefaultConfiguration()
		configuration.SupplierApiUrl = testServer.URL
		params := mergeRatesParamsAndConfiguration(ratesDefaultParams(), configuration)

		redisClient, _ := redismock.NewClientMock()
		service := hertz.New(redisClient)
		ctx := context.Background()

		ratesResponse, _ := service.GetRates(ctx, params, &log)

		assert.Len(t, *ratesResponse.SupplierRequests, 2)

		r1 := converting.Unwrap(ratesResponse.SupplierRequests)[0]

		assert.Equal(t, testServer.URL, *r1.RequestContent.Url)
		assert.Equal(t, http.MethodPost, *r1.RequestContent.Method)
		assert.Len(t, *r1.RequestContent.Headers, 1)
		assert.Equal(t, http.StatusOK, *r1.ResponseContent.StatusCode)
		assert.Len(t, *r1.ResponseContent.Headers, 3)

		r2 := converting.Unwrap(ratesResponse.SupplierRequests)[1]

		assert.Equal(t, testServer.URL, *r2.RequestContent.Url)
		assert.Equal(t, http.MethodPost, *r2.RequestContent.Method)
		assert.Len(t, *r2.RequestContent.Headers, 1)
		assert.Equal(t, http.StatusOK, *r2.ResponseContent.StatusCode)
		assert.Len(t, *r2.ResponseContent.Headers, 3)

	})
}
