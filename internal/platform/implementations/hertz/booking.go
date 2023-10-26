package hertz

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type billingType string

const (
	billingTypeVoucher billingType = "Partial"
)

type bookingRequest struct {
	params                schema.BookingRequestParams
	configuration         schema.HertzConfiguration
	supplierRateReference mapping.SupplierRateReference
	logger                *zerolog.Logger
}

func (b *bookingRequest) requestBody(qualifier mapping.SupplierRateReference) string {
	maxResponses := defaultMaxResponses
	if b.configuration.MaxResponses != nil {
		maxResponses, _ = strconv.Atoi(*b.configuration.MaxResponses)
	}

	vehRentalCore := ota.VehRentalCore{
		PickUpDateTime: b.params.PickUp.DateTime.Format(schema.DateTimeFormat),
		ReturnDateTime: b.params.DropOff.DateTime.Format(schema.DateTimeFormat),
		PickUpLocation: ota.Location{
			LocationCode: b.params.PickUp.Code,
		},
		ReturnLocation: &ota.Location{
			LocationCode: b.params.DropOff.Code,
		},
	}

	var arrivalDetails *ota.ArrivalDetails = nil
	if b.params.AirlineCode != nil && strings.ToLower(*b.params.AirlineCode) == "walk-in" {
		arrivalDetails = &ota.ArrivalDetails{
			TransportationCode: "24",
		}
	} else if b.params.FlightNo != nil && b.params.AirlineCode != nil {
		arrivalDetails = &ota.ArrivalDetails{
			TransportationCode: "14",
			Number:             *b.params.FlightNo,
			OperatingCompany: &ota.OperatingCompany{
				Code: *b.params.AirlineCode,
			},
		}
	}

	var paymentPref *ota.RentalPaymentPref = nil
	voucher := ota.Voucher{}

	if b.params.SupplierSpecificInformation != nil && converting.Unwrap(b.configuration.SendAgencyBillingNumberWithBooking) {
		paymentPref = &ota.RentalPaymentPref{}
		voucher.BillingNumber = *b.params.SupplierSpecificInformation.HertzBillingNumber
		voucher.SeriesCode = b.params.BrokerReference
	} else if converting.Unwrap(b.configuration.SendVoucher) {
		if paymentPref == nil {
			paymentPref = &ota.RentalPaymentPref{}
		}
		voucher.SeriesCode = b.params.BrokerReference
	}

	if paymentPref != nil {
		paymentPref.Voucher = &voucher
	}

	if b.configuration.VoucherContractBillingType != nil && *b.configuration.VoucherContractBillingType == string(billingTypeVoucher) {
		if paymentPref == nil {
			paymentPref = &ota.RentalPaymentPref{}
		}
		paymentPref.PaymentAmount = &ota.PaymentAmount{
			Amount:       qualifier.EstimatedTotalAmount,
			CurrencyCode: qualifier.EstimatedTotalAmountCurrency,
		}
	}

	var reference *ota.Reference = nil
	var vehPref *ota.VehPref = nil

	if converting.Unwrap(b.configuration.UseDirectSell) {
		vehPref = &ota.VehPref{
			Code:        b.params.VehicleClass,
			CodeContext: "SIPP",
		}
	} else if qualifier.FromRates != "" {
		reference = &ota.Reference{
			Type: "16",
			ID:   qualifier.FromRates,
		}
	}

	extrasAndFees := []schema.BookingExtraOrFee{}
	if b.params.ExtrasAndFees != nil {
		extrasAndFees = *b.params.ExtrasAndFees
	}

	extras := make([]ota.SpecialEquipPref, len(extrasAndFees))

	for i, extra := range extrasAndFees {
		extras[i] = ota.SpecialEquipPref{
			EquipType: extra.Code,
			Quantity:  *extra.Quantity,
		}
	}

	var tourInfo *ota.TourInfo = nil
	if b.configuration.Tour != nil {
		tourInfo = &ota.TourInfo{
			TourNumber: *b.configuration.Tour,
		}
	}

	var telephone *ota.Telephone = nil
	if b.params.Customer.Phone != "" {
		telephone = &ota.Telephone{
			PhoneNumber:   b.params.Customer.Phone,
			PhoneTechType: 1,
		}
	}

	var custLoyalty *ota.CustLoyalty = nil
	if b.configuration.ClubNumber != nil {
		custLoyalty = &ota.CustLoyalty{
			MembershipID: *b.configuration.ClubNumber,
			ProgramID:    "ZE",
			TravelSector: "2",
		}
	} else if b.configuration.FrequentTravellerNumber != nil {
		custLoyalty = &ota.CustLoyalty{
			MembershipID: *b.configuration.FrequentTravellerNumber,
			ProgramID:    *b.configuration.FrequentTravellerProgramId,
			TravelSector: *b.configuration.FrequentTravellerTravelSector,
		}
	}

	comments := ""
	if b.params.Comments != nil && b.params.Comments.Customer != nil {
		comments = *b.params.Comments.Customer
	}

	xmlString, _ := xml.MarshalIndent(&ota.VehResRQ{
		Xmlns:             "http://www.opentravel.org/OTA/2003/05",
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
		XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehResRS.xsd",
		Version:           "1.008",
		MaxResponses:      maxResponses,
		POS:               ota.NewPOS(mapping.MappedResidenceCountry(b.configuration, b.params.Customer.ResidenceCountry), b.configuration, b.params.BrokerReference),
		VehResRQInfo: ota.VehResRQInfo{
			SpecialReqPref:    converting.LatinCharacters(comments),
			ArrivalDetails:    arrivalDetails,
			RentalPaymentPref: paymentPref,
			Reference:         reference,
			TourInfo:          tourInfo,
		},
		VehResRQCore: ota.VehResRQCore{
			VehRentalCore: vehRentalCore,
			Status:        "All",
			Customer: ota.BookingCustomer{
				Primary: ota.BookingPrimary{
					PersonName: ota.PersonName{
						GivenName: converting.LatinCharacters(b.params.Customer.FirstName),
						Surname:   converting.LatinCharacters(b.params.Customer.LastName),
					},
					Telephone:   telephone,
					Email:       string(b.params.Customer.Email),
					CustLoyalty: custLoyalty,
				},
			},
			VehPref: vehPref,
			SpecialEquipPrefs: &ota.SpecialEquipPrefs{
				SpecialEquipPref: extras,
			},
		},
	}, "", "    ")

	return string(xmlString)
}

func (b *bookingRequest) bookingRequest(client *http.Client) (*http.Response, error) {
	requestBody := b.requestBody(b.supplierRateReference)

	ctx := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Booking)
	httpRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, b.configuration.SupplierApiUrl, bytes.NewBuffer([]byte(requestBody)))
	httpRequest.Header.Set("Content-Type", "application/xml; charset=utf-8")

	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (b *bookingRequest) Execute(httpTransport *http.Transport) (schema.BookingResponse, error) {
	booking := schema.BookingResponse{}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	booking.SupplierRequests = requestsBucket.SupplierRequests()
	booking.Errors = errorsBucket.Errors()
	booking.Status = schema.BookingResponseStatusFAILED

	timeout := b.params.Timeouts.Default
	if b.params.Timeouts.Booking != nil {
		timeout = *b.params.Timeouts.Booking
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: httpTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(b.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	response, e := requesting.RequestErrors(b.bookingRequest(client))
	if e != nil {
		errorsBucket.AddError(*e)
		return booking, nil
	}

	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	var otaBookingResponse ota.VehResRS
	err := xml.Unmarshal(bodyBytes, &otaBookingResponse)
	if err != nil {
		return booking, err
	}

	message := otaBookingResponse.ErrorMessage()
	if message != "" {
		errorsBucket.AddError(schema.NewSupplierError(message))
		return booking, nil
	}

	booking.SupplierBookingReference = &otaBookingResponse.VehResRSCore.VehReservation.VehSegmentCore.ConfID.ID

	if otaBookingResponse.VehResRSCore.VehReservation.VehSegmentCore.ConfID.ID != "" {
		booking.Status = schema.BookingResponseStatusOK

		supplierData := mapping.SupplierData{
			LastName:         b.params.Customer.LastName,
			ResidenceCountry: b.params.Customer.ResidenceCountry,
		}

		booking.SupplierData = supplierData.AsMap()
	}

	return booking, nil
}
