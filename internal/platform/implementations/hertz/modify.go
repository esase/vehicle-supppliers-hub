package hertz

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/ota"
	"github.com/rs/zerolog"
)

type modifyRequest struct {
	params                      schema.ModifyRequestParams
	configuration               schema.HertzConfiguration
	supplierSpecificInformation *schema.SupplierSpecificInformation
	supplierRateReference       mapping.SupplierRateReference // todo: move to hertz schema package
	logger                      *zerolog.Logger
	otaModifyResponse           ota.VehModifyRS
}

func (m *modifyRequest) requestBody(qualifier mapping.SupplierRateReference) []byte {
	vehRentalCore := ota.VehRentalCore{
		PickUpDateTime: m.params.PickUp.DateTime.Format(schema.DateTimeFormat),
		ReturnDateTime: m.params.DropOff.DateTime.Format(schema.DateTimeFormat),
		PickUpLocation: ota.Location{
			LocationCode: m.params.PickUp.Code,
		},
	}

	var arrivalDetails *ota.ArrivalDetails = nil
	if m.params.AirlineCode != nil && strings.ToLower(*m.params.AirlineCode) == "walk-in" {
		arrivalDetails = &ota.ArrivalDetails{
			TransportationCode: "24",
		}
	} else if m.params.FlightNumber != nil && m.params.AirlineCode != nil {
		arrivalDetails = &ota.ArrivalDetails{
			TransportationCode: "14",
			Number:             *m.params.FlightNo,
			OperatingCompany: &ota.OperatingCompany{
				Code: *m.params.AirlineCode,
			},
		}
	}

	var paymentPref *ota.RentalPaymentPref = nil
	voucher := ota.Voucher{}

	if m.params.SupplierSpecificInformation != nil && converting.Unwrap(m.configuration.SendAgencyBillingNumberWithBooking) {
		paymentPref = &ota.RentalPaymentPref{}
		voucher.BillingNumber = *m.params.SupplierSpecificInformation.HertzBillingNumber
		voucher.SeriesCode = m.params.BrokerReference
	} else if converting.Unwrap(m.configuration.SendVoucher) {
		if paymentPref == nil {
			paymentPref = &ota.RentalPaymentPref{}
		}
		voucher.SeriesCode = m.params.BrokerReference
	}

	if paymentPref != nil {
		paymentPref.Voucher = &voucher
	}

	if m.configuration.VoucherContractBillingType != nil && *m.configuration.VoucherContractBillingType == string(billingTypeVoucher) {
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

	if converting.Unwrap(m.configuration.UseDirectSell) {
		vehPref = &ota.VehPref{
			Code:        m.params.VehicleClass,
			CodeContext: "SIPP",
		}
	} else if qualifier.FromQuote != "" {
		reference = &ota.Reference{
			Type: "16",
			ID:   qualifier.FromQuote,
		}
	}

	extrasAndFees := []schema.BookingExtraOrFee{}
	if m.params.ExtrasAndFees != nil {
		extrasAndFees = *m.params.ExtrasAndFees
	}

	extras := make([]ota.SpecialEquipPref, len(extrasAndFees))

	for i, extra := range extrasAndFees {
		extras[i] = ota.SpecialEquipPref{
			EquipType: extra.Code,
			Quantity:  *extra.Quantity,
		}
	}

	var telephone *ota.Telephone = nil
	if m.params.Customer.Phone != "" {
		telephone = &ota.Telephone{
			PhoneNumber:   m.params.Customer.Phone,
			PhoneTechType: 1,
		}
	}

	var custLoyalty *ota.CustLoyalty = nil
	if m.configuration.ClubNumber != nil {
		custLoyalty = &ota.CustLoyalty{
			MembershipID: *m.configuration.ClubNumber,
			ProgramID:    "ZE",
			TravelSector: "2",
		}
	} else if m.configuration.FrequentTravellerNumber != nil {
		custLoyalty = &ota.CustLoyalty{
			MembershipID: *m.configuration.FrequentTravellerNumber,
			ProgramID:    *m.configuration.FrequentTravellerProgramId,
			TravelSector: *m.configuration.FrequentTravellerTravelSector,
		}
	}

	var comments string
	if m.params.Comments != nil && m.params.Comments.Customer != nil {
		comments = converting.LatinCharacters(*m.params.Comments.Customer)
	}

	xmlString, _ := xml.MarshalIndent(&ota.VehModifyRQ{
		Xmlns:             "http://www.opentravel.org/OTA/2003/05",
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
		XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehModifyRQ.xsd",
		Version:           "1.008",
		POS:               ota.NewPOS(mapping.MappedResidenceCountry(m.configuration, m.params.Customer.ResidenceCountry), m.configuration, m.params.BrokerReference),
		VehModifyRQCore: ota.VehModifyRQCore{
			Status:     "Confirmed",
			ModifyType: "Book",
			UniqueID: ota.UniqueID{
				Type: "14",
				ID:   m.params.SupplierBookingReference,
			},
			VehRentalCore: vehRentalCore,
			Customer: ota.BookingCustomer{
				Primary: ota.BookingPrimary{
					PersonName: ota.PersonName{
						GivenName: converting.LatinCharacters(m.params.Customer.FirstName),
						Surname:   converting.LatinCharacters(m.params.Customer.LastName),
					},
					Telephone:   telephone,
					Email:       string(m.params.Customer.Email),
					CustLoyalty: custLoyalty,
				},
			},
			VehPref: vehPref,
			SpecialEquipPrefs: &ota.SpecialEquipPrefs{
				SpecialEquipPref: extras,
			},
		},
		VehModifyRQInfo: ota.VehModifyRQInfo{
			SpecialReqPref:    comments,
			ArrivalDetails:    arrivalDetails,
			RentalPaymentPref: paymentPref,
			Reference:         reference,
		},
	}, "", "    ")

	return xmlString
}

func (m *modifyRequest) modifyRequest(client *http.Client) (*http.Response, error) {
	requestBody := m.requestBody(m.supplierRateReference)

	ctx := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Modify)
	httpRequest, _ := http.NewRequestWithContext(ctx, http.MethodPost, m.configuration.SupplierApiUrl, bytes.NewBuffer(requestBody))
	httpRequest.Header.Set("Content-Type", "application/xml; charset=utf-8")

	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (m *modifyRequest) Execute(httpTransport *http.Transport) (schema.ModifyResponse, error) {
	modify := schema.ModifyResponse{}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	modify.SupplierRequests = requestsBucket.SupplierRequests()
	modify.Errors = errorsBucket.Errors()

	timeout := m.params.Timeouts.Default
	if m.params.Timeouts.Rates != nil {
		timeout = *m.params.Timeouts.Booking
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: http.DefaultTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(m.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	response, e := requesting.RequestErrors(m.modifyRequest(client))
	if e != nil {
		errorsBucket.AddError(*e)
		return modify, nil
	}

	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	err := xml.Unmarshal(bodyBytes, &m.otaModifyResponse)
	if err != nil {
		return modify, err
	}

	message := m.otaModifyResponse.ErrorMessage()
	if message != "" {
		errorsBucket.AddError(schema.NewSupplierError(message))
		return modify, nil
	}

	status := schema.ModifyResponseStatusFAILED

	modify.SupplierBookingReference = &m.otaModifyResponse.VehModifyRSCore.VehReservation.VehSegmentCore.ConfID.ID

	if modify.SupplierBookingReference != nil && *modify.SupplierBookingReference != "" {
		status = schema.ModifyResponseStatusOK
	}

	modify.Status = &status

	return modify, nil
}
