package bookingcom

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type bookingStatusRequest struct {
	params                   schema.BookingStatusRequestParams
	configuration            schema.BookingComConfiguration
	logger                   *zerolog.Logger
	otaBookingStatusResponse ota.BookingStatusRS
}

func (b *bookingStatusRequest) Execute(httpTransport *http.Transport) (schema.BookingStatusResponse, error) {
	bookingStatus := schema.BookingStatusResponse{}
	bookingStatus.Status = schema.BookingStatusResponseStatusFAILED

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	bookingStatus.SupplierRequests = requestsBucket.SupplierRequests()
	bookingStatus.Errors = errorsBucket.Errors()

	timeout := b.params.Timeouts.Default

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

	// handle response
	if e != nil {
		errorsBucket.AddError(*e)
		return bookingStatus, nil
	}

	// bind the response body to the xml
	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	err := xml.Unmarshal(bodyBytes, &b.otaBookingStatusResponse)
	if err != nil {
		return bookingStatus, err
	}

	errorMessage := b.otaBookingStatusResponse.ErrorMessage()

	if errorMessage != "" {
		errorsBucket.AddError(schema.NewSupplierError(errorMessage))

		return bookingStatus, nil
	}

	bookingStatus.Status = b.otaBookingStatusResponse.Booking.Booking.Status()

	return bookingStatus, nil
}

func (b *bookingStatusRequest) makeRequest(client *http.Client) (*http.Response, error) {
	body := bytes.NewBuffer(b.requestBody())

	url := b.configuration.SupplierApiUrl

	ctx := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.BookingStatus)

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

func (b *bookingStatusRequest) requestBody() []byte {
	xml, _ := xml.MarshalIndent(&ota.BookingStatusRQ{
		Version: ota.Version{
			Version: "1.1",
		},
		Credentials: ota.Credentials{
			Credentials: ota.CredentialsInfo{
				Username: b.configuration.Username,
				Password: b.configuration.Password,
			},
		},
		Email: string(b.params.Contact.Email),
		Booking: ota.Booking{
			Booking: ota.BookingInfo{
				Id: b.params.ReservNumber,
			},
		},
	}, "", "	")

	return xml
}
