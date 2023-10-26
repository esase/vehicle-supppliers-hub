package bookingcom

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type cancelRequest struct {
	params                   schema.CancelRequestParams
	configuration            schema.BookingComConfiguration
	logger                   *zerolog.Logger
	otaCancelBookingResponse ota.CancelBookingRS
}

func (c *cancelRequest) Execute(httpTransport *http.Transport) (schema.CancelResponse, error) {
	cancel := schema.CancelResponse{}

	status := schema.CancelResponseStatusFAILED
	cancel.Status = &status

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

	if errorMessage != "" {
		errorsBucket.AddError(schema.NewSupplierError(errorMessage))

		return cancel, nil
	}

	if strings.ToLower(c.otaCancelBookingResponse.Status) == "ok" {
		status = schema.CancelResponseStatusOK
		cancel.Status = &status
	}

	return cancel, nil
}

func (c *cancelRequest) makeRequest(client *http.Client) (*http.Response, error) {
	body := bytes.NewBuffer(c.requestBody())

	url := c.configuration.SupplierApiUrl

	ctx := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Cancel)

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Content-Type", "application/xml")
	httpRequest.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36")

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return httpResponse, nil
}

func (c *cancelRequest) requestBody() []byte {
	cancelReason := "unknown"

	if c.params.CancelReason != nil {
		cancelReason = fmt.Sprintf("Cancel reason code is a %v", *c.params.CancelReason)
	}

	xml, _ := xml.MarshalIndent(&ota.CancelBookingRQ{
		Credentials: ota.Credentials{
			Credentials: ota.CredentialsInfo{
				Username: c.configuration.Username,
				Password: c.configuration.Password,
			},
		},
		Email:  string(c.params.Contact.Email),
		Reason: cancelReason,
		Booking: ota.Booking{
			Booking: ota.BookingInfo{
				Id: c.params.SupplierBookingReference,
			},
		},
	}, "", "	")

	return xml
}
