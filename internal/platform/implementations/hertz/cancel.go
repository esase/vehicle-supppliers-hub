package hertz

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type cancelRequest struct {
	params                   schema.CancelRequestParams
	configuration            schema.HertzConfiguration
	logger                   *zerolog.Logger
	otaCancelBookingResponse ota.VehCancelRS
}

func (c *cancelRequest) requestBody() []byte {
	var pos ota.POS = ota.NewPOS(*c.configuration.ResidenceCountry, c.configuration, c.params.BrokerReference)
	var core ota.VehCancelRQCore = ota.VehCancelRQCore{
		CancelType: "Book",
		UniqueID: &ota.UniqueID{
			Type: "14",
			ID:   c.params.SupplierBookingReference,
		},
		PersonName: &ota.CancelPersonName{
			Surname: *c.configuration.LastName,
		},
	}

	xml, _ := xml.MarshalIndent(&ota.VehCancelRQ{
		Xmlns:             "http://www.opentravel.org/OTA/2003/05",
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
		XmlnsXsd:          "http://www.w3.org/2001/XMLSchema",
		XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehCancelRQ.xsd",
		Version:           "1.008",
		POS:               pos,
		VehCancelRQCore:   core,
	}, "", "	")

	return xml
}

func (c *cancelRequest) makeRequest(client *http.Client) (*http.Response, error) {
	body := bytes.NewBuffer(c.requestBody())
	url := c.configuration.SupplierApiUrl

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

func (c *cancelRequest) alreadyCancelled(errors ota.Errors) bool {
	for _, e := range errors.Error {
		if e.ShortText == "UNABLE - RESERVATION CANCELLED" || e.Code == "095" {
			return true
		}
	}
	return false
}

func (c *cancelRequest) successfullyCancelled(cancelStatus ota.CoreCancelStatus) bool {
	return cancelStatus == ota.CoreCancelStatusCancelled
}

func (c *cancelRequest) Execute(httpTransport *http.Transport) (schema.CancelResponse, error) {
	cancel := schema.CancelResponse{}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	cancel.SupplierRequests = requestsBucket.SupplierRequests()
	cancel.Errors = errorsBucket.Errors()

	timeout := c.params.Timeouts.Default
	if c.params.Timeouts.Cancel != nil {
		timeout = *c.params.Timeouts.Cancel
	}

	// prepare client
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: httpTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(c.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	response, e := requesting.RequestErrors(c.makeRequest(client))

	// handle response
	if e != nil {
		errorsBucket.AddError(*e)
		return cancel, nil
	}

	// bind the response body to the xml
	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	err := xml.Unmarshal(bodyBytes, &c.otaCancelBookingResponse)
	if err != nil {
		return cancel, err
	}

	errorMessage := c.otaCancelBookingResponse.ErrorMessage()

	var status schema.CancelResponseStatus

	// decide status
	switch {
	case c.alreadyCancelled(c.otaCancelBookingResponse.Errors):
		status = schema.CancelResponseStatusOK
	case c.successfullyCancelled(c.otaCancelBookingResponse.VehCancelRSCore.CancelStatus):
		status = schema.CancelResponseStatusOK
	default:
		errorsBucket.AddError(schema.NewSupplierError(errorMessage))
		status = schema.CancelResponseStatusFAILED
	}

	cancel.Status = &status

	return cancel, nil
}
