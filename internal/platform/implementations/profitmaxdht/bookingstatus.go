package profitmaxdht

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/profitmaxdht/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type bookingStatusRequest struct {
	params        schema.BookingStatusRequestParams
	configuration schema.ProfitMaxDHTConfiguration
	logger        *zerolog.Logger
}

func (b *bookingStatusRequest) requestBody() []byte {
	target := "Production"
	if converting.Unwrap(b.configuration.Test) {
		target = "Test"
	}

	xml, _ := xml.MarshalIndent(
		ota.SoapEnvelope{
			XmlnsSoapEnv:  "http://www.w3.org/2001/12/soap-envelope",
			XmlnsXsd:      "http://www.w3.org/1999/XMLSchema",
			XmlnsXsi:      "http://www.w3.org/1999/XMLSchema-instance",
			SoapEnvHeader: ota.SoapEnvHeaderBuilder(b.configuration),
			SoapEnvBody: ota.SoapEnvBody{
				VehRetResRQ: &ota.VehRetResRQ{
					Xmlns:             "http://www.opentravel.org/OTA/2003/05",
					XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
					XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehRetResRQ.xsd",
					Version:           "1.008",
					Target:            target,
					POS:               ota.POSBuiler(b.configuration),
					VehRetResRQCore: ota.VehRetResRQCore{
						UniqueID: ota.UniqueID{
							Type: "14",
							ID:   b.params.SupplierBookingReference,
						},
						PersonName: ota.PersonName{
							Surname: *b.configuration.LastName,
						},
					},
				},
			}}, "", "	")
	return xml
}

func (b *bookingStatusRequest) makeRequest(client *http.Client) (*http.Response, error) {
	body := bytes.NewBuffer(b.requestBody())
	url := b.configuration.SupplierApiUrl

	ctx := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Cancel)

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Content-Type", "application/xml; charset=utf-8")

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return httpResponse, nil
}

func (b *bookingStatusRequest) Execute(httpTransport *http.Transport) (schema.BookingStatusResponse, error) {
	bookingStatus := schema.BookingStatusResponse{}
	var faultResponse ota.FaultEnvelope

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	bookingStatus.SupplierRequests = requestsBucket.SupplierRequests()
	bookingStatus.Errors = errorsBucket.Errors()
	bookingStatus.Status = schema.BookingStatusResponseStatusFAILED

	timeout := b.params.Timeouts.Default
	if b.params.Timeouts.Cancel != nil {
		timeout = *b.params.Timeouts.Cancel
	}

	// prepare client
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

	response, e := requesting.RequestErrors(b.makeRequest(client))
	if e != nil {
		errorsBucket.AddError(*e)
		return bookingStatus, nil
	}

	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	_ = xml.Unmarshal(bodyBytes, &faultResponse)
	faultMessage := faultResponse.FaultMessage()
	if faultMessage != "" {
		errorsBucket.AddError(schema.NewSupplierError(faultMessage))
		return bookingStatus, nil
	}

	var otaBookingResponse ota.VehRetResRS
	err := xml.Unmarshal(bodyBytes, &otaBookingResponse)
	if err != nil {
		return bookingStatus, err
	}

	message := otaBookingResponse.ErrorMessage()
	if message != "" {
		errorsBucket.AddError(schema.NewSupplierError(message))
		return bookingStatus, nil
	}

	bookingStatus.SupplierBookingReference = &otaBookingResponse.VehRetResRSCore.VehReservation.VehSegmentCore.ConfID.ID

	if otaBookingResponse.VehRetResRSCore.VehReservation.VehSegmentCore.ConfID.ID != "" {
		bookingStatus.Status = schema.BookingStatusResponseStatusOK
	}

	return bookingStatus, nil
}
