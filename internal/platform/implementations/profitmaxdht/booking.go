package profitmaxdht

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/profitmaxdht/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/profitmaxdht/ota"
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
	configuration         schema.ProfitMaxDHTConfiguration
	supplierRateReference mapping.SupplierRateReference
	logger                *zerolog.Logger
}

func (b *bookingRequest) requestBody(qualifier mapping.SupplierRateReference) string {
	target := "Production"
	if converting.Unwrap(b.configuration.Test) {
		target = "Test"
	}

	vehRentalCore := ota.VehRentalCore{
		PickUpDateTime: b.params.PickUp.DateTime.Format(schema.DateTimeFormat),
		ReturnDateTime: b.params.DropOff.DateTime.Format(schema.DateTimeFormat),
		PickUpLocation: &ota.Location{
			LocationCode: b.params.PickUp.Code,
		},
		ReturnLocation: &ota.Location{
			LocationCode: b.params.DropOff.Code,
		},
	}

	var paymentPref *ota.RentalPaymentPref = nil
	if converting.Unwrap(b.configuration.SendVoucher) {
		paymentPref = &ota.RentalPaymentPref{}

		voucher := ota.Voucher{
			SeriesCode: b.params.BrokerReference,
		}

		paymentPref.Voucher = &voucher
	}

	var reference *ota.Reference = nil

	if qualifier.FromRates != "" {
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

	var telephone *ota.Telephone = nil
	if b.params.Customer.Phone != "" {
		telephone = &ota.Telephone{
			PhoneNumber:   b.params.Customer.Phone,
			PhoneTechType: 1,
		}
	}

	var tourInfo *ota.TourInfo = nil
	if b.configuration.TourNumber != nil {
		tourInfoEl := ota.TourInfo{
			TourNumber: *b.configuration.TourNumber,
		}

		tourInfo = &tourInfoEl
	}

	comments := ""
	if b.params.Comments != nil && b.params.Comments.Customer != nil {
		comments = *b.params.Comments.Customer
	}

	xmlString, _ := xml.MarshalIndent(
		ota.SoapEnvelope{
			XmlnsSoapEnv:  "http://www.w3.org/2001/12/soap-envelope",
			XmlnsXsd:      "http://www.w3.org/1999/XMLSchema",
			XmlnsXsi:      "http://www.w3.org/1999/XMLSchema-instance",
			SoapEnvHeader: ota.SoapEnvHeaderBuilder(b.configuration),
			SoapEnvBody: ota.SoapEnvBody{
				VehResRQ: &ota.VehResRQ{
					Xmlns:             "http://www.opentravel.org/OTA/2003/05",
					XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
					XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehResRS.xsd",
					Version:           "1.008",
					Target:            target,
					POS:               ota.POSBuiler(b.configuration),
					VehResRQInfo: ota.VehResRQInfo{
						SpecialReqPref:    converting.LatinCharacters(comments),
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
								Telephone: telephone,
								Email:     string(b.params.Customer.Email),
							},
						},
						SpecialEquipPrefs: &ota.SpecialEquipPrefs{
							SpecialEquipPref: extras,
						},
					},
				}}}, "", "    ")

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
	var faultResponse ota.FaultEnvelope

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

	_ = xml.Unmarshal(bodyBytes, &faultResponse)
	faultMessage := faultResponse.FaultMessage()
	if faultMessage != "" {
		errorsBucket.AddError(schema.NewSupplierError(faultMessage))
		return booking, nil
	}

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
