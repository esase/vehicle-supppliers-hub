package bookingcom

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/bookingcom/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/client/spit"
	"bitbucket.org/crgw/supplier-hub/internal/tools/client/userservice"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"github.com/rs/zerolog"
)

type bookingRequest struct {
	params                schema.BookingRequestParams
	configuration         schema.BookingComConfiguration
	supplierRateReference mapping.SupplierRateReference
	logger                *zerolog.Logger
}

func (b *bookingRequest) Execute(httpTransport *http.Transport) (schema.BookingResponse, error) {
	booking := schema.BookingResponse{}
	booking.Status = schema.BookingResponseStatusFAILED

	ctx := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Booking)

	// fetch auth token
	user, err := b.requestUatToken(&ctx)
	if err != nil {
		return booking, err
	}

	// fetch card info
	cardInfo, err := b.requestCardInfo(&ctx, user.OriginalUAT)
	if err != nil {
		return booking, err
	}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	booking.SupplierRequests = requestsBucket.SupplierRequests()
	booking.Errors = errorsBucket.Errors()

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

	// make a booking
	response, e := requesting.RequestErrors(b.bookingRequest(&ctx, client, cardInfo))
	if e != nil {
		errorsBucket.AddError(*e)
		return booking, nil
	}

	bodyBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()

	var otaBookingResponse ota.MakeBookingRS
	err = xml.Unmarshal(bodyBytes, &otaBookingResponse)
	if err != nil {
		return booking, err
	}

	message := otaBookingResponse.ErrorMessage()
	if message != "" {
		errorsBucket.AddError(schema.NewSupplierError(message))
		return booking, nil
	}

	booking.SupplierBookingReference = &otaBookingResponse.Booking.Booking.Id

	var bookingStatus schema.BookingStatusResponse

	// fetch the booking's actual status
	bookingStatusRequest := bookingStatusRequest{
		params: schema.BookingStatusRequestParams{
			Timeouts: b.params.Timeouts,
			Contact: &schema.Contact{
				Email: b.params.Customer.Email,
			},
		},
		configuration: b.configuration,
		logger:        b.logger,
	}

	bookingStatusRequest.params.ReservNumber = *booking.SupplierBookingReference

	bookingStatus, err = bookingStatusRequest.Execute(httpTransport)
	requestsBucket.AddRequests(*bookingStatus.SupplierRequests)
	errorsBucket.AddErrors(*bookingStatus.Errors)

	if err != nil {
		return booking, err
	}

	booking.Status = schema.BookingResponseStatus(bookingStatus.Status)

	return booking, nil
}

func (b *bookingRequest) bookingRequest(ctx *context.Context, client *http.Client, cardInfo *spit.CardInfo) (*http.Response, error) {
	requestBody := b.requestBody(cardInfo)

	httpRequest, _ := http.NewRequestWithContext(*ctx, http.MethodPost, b.configuration.SupplierApiUrl, bytes.NewBuffer([]byte(requestBody)))
	httpRequest.Header.Set("Content-Type", "application/xml")
	httpRequest.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/101.0.4951.54 Safari/537.36")

	response, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (b *bookingRequest) requestBody(cardInfo *spit.CardInfo) string {
	var airlineInfo *ota.MakeBookingRQAirlineInfo = nil

	if b.params.FlightNo != nil {
		airlineInfo = &ota.MakeBookingRQAirlineInfo{
			FlightNo: *b.params.FlightNo,
		}
	}

	var extras *ota.MakeBookingRQExtraList = nil

	if b.params.ExtrasAndFees != nil {
		extras = &ota.MakeBookingRQExtraList{}

		for _, extra := range *b.params.ExtrasAndFees {
			amount := 1

			if extra.Quantity != nil {
				amount = *extra.Quantity
			}

			extras.Extra = append(extras.Extra, ota.MakeBookingRQExtra{
				Id:     extra.Code,
				Amount: amount,
			})
		}
	}

	xmlString, _ := xml.MarshalIndent(&ota.MakeBookingRQ{
		Version: ota.Version{
			Version: "1.1",
		},
		InsuranceVersion: "1.0",
		CorrelationId:    b.params.ReservNumber,
		ResidenceCountry: b.params.Customer.ResidenceCountry,
		Credentials: ota.Credentials{
			Credentials: ota.CredentialsInfo{
				Username: b.configuration.Username,
				Password: b.configuration.Password,
			},
		},
		Booking: ota.MakeBookingRQBooking{
			PickUp: ota.PickUp{
				Location: ota.Location{
					Location: b.supplierRateReference.PickUpLocationId,
				},
				Date: ota.Date{
					Year:   b.params.PickUp.DateTime.Year(),
					Month:  int(b.params.PickUp.DateTime.Month()),
					Day:    b.params.PickUp.DateTime.Day(),
					Hour:   b.params.PickUp.DateTime.Hour(),
					Minute: b.params.PickUp.DateTime.Minute(),
				},
			},
			DropOff: ota.DropOff{
				Location: ota.Location{
					Location: b.supplierRateReference.DropOffLocationId,
				},
				Date: ota.Date{
					Year:   b.params.DropOff.DateTime.Year(),
					Month:  int(b.params.DropOff.DateTime.Month()),
					Day:    b.params.DropOff.DateTime.Day(),
					Hour:   b.params.DropOff.DateTime.Hour(),
					Minute: b.params.DropOff.DateTime.Minute(),
				},
			},
			ExtraList: extras,
			Vehicle: ota.MakeBookingRQVehicle{
				Id: b.supplierRateReference.VehicleId,
			},
			DriverInfo: ota.MakeBookingRQDriverInfo{
				DriverName: ota.MakeBookingRQDriverName{
					Title:     b.params.Customer.Title,
					FirstName: converting.LatinCharacters(b.params.Customer.FirstName),
					LastName:  converting.LatinCharacters(b.params.Customer.LastName),
				},
				Address: ota.MakeBookingRQAddress{
					Country: b.params.Customer.ResidenceCountry,
				},
				Email:     string(b.params.Customer.Email),
				Telephone: b.params.Customer.Phone,
				DriverAge: b.params.Customer.Age,
			},
			PaymentInfo: ota.MakeBookingRQPaymentInfo{
				DepositPayment: false,
				CardVaultToken: cardInfo.CardVaultToken,
				ThreeDSecure: ota.MakeBookingRQThreeDSecure{
					Eci:       cardInfo.Eci,
					Cavv:      *cardInfo.Cavv,
					DsTransId: *cardInfo.TransactionId,
				},
			},
			AirlineInfo: airlineInfo,
			AcceptedPrice: ota.MakeBookingRQAcceptedPrice{
				Price:    b.supplierRateReference.BasePrice,
				Currency: b.supplierRateReference.BaseCurrency,
			},
		},
	}, "", "    ")

	return string(xmlString)
}

func (b *bookingRequest) requestUatToken(ctx *context.Context) (*userservice.User, error) {
	userServiceClient, err := userservice.NewClient(b.logger)
	if err != nil {
		return &userservice.User{}, err
	}

	result, err := userServiceClient.AuthUserViaPassword(*ctx, os.Getenv("CRG_USERNAME"), os.Getenv("CRG_PASSWORD"))
	if err != nil {
		return &userservice.User{}, err
	}

	return result, nil
}

func (b *bookingRequest) requestCardInfo(ctx *context.Context, uatToken string) (*spit.CardInfo, error) {
	spitClient, err := spit.NewClient(b.logger)
	if err != nil {
		return &spit.CardInfo{}, err
	}

	isTestEnv := false
	if b.configuration.Test != nil && *b.configuration.Test == true {
		isTestEnv = true
	}

	params := spit.ConvertTokenParams{
		Context:             ctx,
		AffiliateCode:       b.configuration.AffiliateCode,
		IsTestEnv:           isTestEnv,
		TransactionCurrency: b.supplierRateReference.BaseCurrency,
		UatToken:            uatToken,
		SpitToken:           b.params.SupplierPassthrough.Payment.SptToken,
	}

	result, err := spitClient.ConvertToken(&params)
	if err != nil {
		return &spit.CardInfo{}, err
	}

	return result, nil
}
