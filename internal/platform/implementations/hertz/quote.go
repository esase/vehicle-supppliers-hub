package hertz

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type quoteRequest struct {
	params        schema.RatesRequestParams
	configuration schema.HertzConfiguration
	logger        *zerolog.Logger
	slowLogger    slowlog.Logger
}

func (q *quoteRequest) requestBody(vehicleClass string) []byte {
	pickUpDateTime := q.params.PickUp.DateTime
	dropOffDateTime := q.params.DropOff.DateTime

	var telephone *ota.Telephone = nil
	if q.params.Booking.Phone != nil {
		telephone = &ota.Telephone{
			PhoneNumber:   *q.params.Booking.Phone,
			PhoneTechType: 1,
		}
	}

	var vehPref *ota.VehPref = nil
	if vehicleClass != "" {
		vehPref = &ota.VehPref{
			Code:        vehicleClass,
			CodeContext: "SIPP",
		}
	}

	var paymentPref *ota.RentalPaymentPref = nil
	if q.configuration.SendVoucher != nil && *q.configuration.SendVoucher {
		paymentPref = &ota.RentalPaymentPref{
			Voucher: &ota.Voucher{
				SeriesCode: *q.params.Booking.ReservNumber,
			},
		}
	}

	xmlString, _ := xml.MarshalIndent(&ota.VehModifyRQ{
		Xmlns:             "http://www.opentravel.org/OTA/2003/05",
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
		XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehResRS.xsd",
		Version:           "1.008",
		POS:               ota.NewPOS(mapping.MappedResidenceCountry(q.configuration, q.params.ResidenceCountry), q.configuration, ""),
		VehModifyRQCore: ota.VehModifyRQCore{
			Status:     "Confirmed",
			ModifyType: "Quote",
			UniqueID: ota.UniqueID{
				Type: "14",
				ID:   q.params.Booking.SupplierBookingReference,
			},
			VehRentalCore: ota.VehRentalCore{
				PickUpDateTime: pickUpDateTime.Format(schema.DateTimeFormat),
				ReturnDateTime: dropOffDateTime.Format(schema.DateTimeFormat),
				PickUpLocation: ota.Location{
					LocationCode: q.params.PickUp.Code,
				},
			},
			Customer: ota.BookingCustomer{
				Primary: ota.BookingPrimary{
					PersonName: ota.PersonName{
						GivenName: converting.LatinCharacters(converting.Unwrap(q.params.Booking.FirstName)),
						Surname:   converting.LatinCharacters(converting.Unwrap(q.params.Booking.LastName)),
					},
					Telephone: telephone,
					Email:     converting.Unwrap(q.params.Booking.Email),
				},
			},
			VehPref: vehPref,
		},
		VehModifyRQInfo: ota.VehModifyRQInfo{
			RentalPaymentPref: paymentPref,
		},
	}, "", "    ")

	return xmlString
}

func (r *quoteRequest) parseQuote(modify *ota.VehModifyRS, pricedEquips []ota.PricedEquip) (schema.Vehicle, string) {
	extrasAndFees := []schema.ExtraOrFee{}

	reservation := &modify.VehModifyRSCore.VehReservation

	qualifier, _ := json.Marshal(mapping.SupplierRateReference{
		FromRates:                    modify.EchoToken,
		FromQuote:                    modify.EchoToken,
		EstimatedTotalAmount:         fmt.Sprintf("%.2f", reservation.VehSegmentCore.TotalCharge.EstimatedTotalAmount),
		EstimatedTotalAmountCurrency: reservation.VehSegmentCore.TotalCharge.CurrencyCode,
	})

	vehiclePrice, err := reservation.VehSegmentCore.TotalCharge.Price(
		r.params,
		reservation.VehSegmentInfo.PaymentRules.PaymentRule,
	)
	if err != "" {
		return schema.Vehicle{}, err
	}

	mileage := reservation.VehSegmentCore.RentalRate.RateDistance.Mileage()

	taxMultiplier, taxCharge, taxIsPartOfTheVehiclePrice := reservation.VehSegmentCore.RentalRate.VehicleCharges.TaxCharge(
		r.params,
		r.configuration,
		reservation.VehSegmentInfo.PaymentRules.PaymentRule,
	)

	if taxIsPartOfTheVehiclePrice {
		vehiclePrice.Amount = schema.RoundedFloat(float64(vehiclePrice.Amount) * taxMultiplier)
	}

	charges := reservation.VehSegmentCore.RentalRate.VehicleCharges.Charges()

	coverages, coveragePricePartOfVehiclePrice := reservation.VehSegmentInfo.PricedCoverages.VehicleCoverages(
		taxMultiplier,
		reservation.VehSegmentInfo.PaymentRules.PaymentRule,
		r.params,
		r.configuration,
	)

	vehiclePrice.Amount += schema.RoundedFloat(coveragePricePartOfVehiclePrice)

	fees := reservation.VehSegmentCore.Fees.Fees(
		r.params,
		r.configuration,
		reservation.VehSegmentInfo.PaymentRules.PaymentRule,
	)

	extras := make([]schema.ExtraOrFee, len(pricedEquips))

	for i, pricedEquip := range pricedEquips {
		extras[i] = parseExtra(pricedEquip, taxMultiplier)
	}

	if taxCharge != nil {
		extrasAndFees = append(extrasAndFees, *taxCharge)
	}

	extrasAndFees = append(extrasAndFees, extras...)
	extrasAndFees = append(extrasAndFees, charges...)
	extrasAndFees = append(extrasAndFees, fees...)
	extrasAndFees = append(extrasAndFees, coverages...)

	rateQualifier := string(qualifier)

	vehicle := schema.Vehicle{
		Name:                  reservation.VehSegmentCore.Vehicle.VehMakeModel.Name,
		Class:                 reservation.VehSegmentCore.Vehicle.VehMakeModel.Code,
		Price:                 vehiclePrice,
		SupplierRateReference: &rateQualifier,
		ExtrasAndFees:         &extrasAndFees,
		AcrissCode:            &reservation.VehSegmentCore.Vehicle.VehMakeModel.Code,
		HasAirco:              &reservation.VehSegmentCore.Vehicle.AirConditionInd,
		Status:                schema.AVAILABLE,
		SmallSuitcases:        &reservation.VehSegmentCore.Vehicle.BaggageQuantity,
		Doors:                 &reservation.VehSegmentCore.Vehicle.VehType.DoorCount,
		Seats:                 &reservation.VehSegmentCore.Vehicle.PassengerQuantity,
		TransmissionType:      mapping.Transmission(reservation.VehSegmentCore.Vehicle.TransmissionType),
		FuelType:              mapping.FuelType(reservation.VehSegmentCore.Vehicle.FuelType),
		DriveType:             mapping.DriveType(reservation.VehSegmentCore.Vehicle.DriveType),
		Mileage:               &mileage,
	}

	return vehicle, ""
}

func (q *quoteRequest) quoteRequest(ctx context.Context, client *http.Client, vehicleClass string) (*http.Response, error) {
	requestBody := q.requestBody(vehicleClass)

	c := context.WithValue(ctx, schema.RequestingTypeKey, schema.Quote)
	httpRequest, _ := http.NewRequestWithContext(c, http.MethodPost, q.configuration.SupplierApiUrl, bytes.NewBuffer(requestBody))
	httpRequest.Header.Set("Content-Type", "application/xml; charset=utf-8")

	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return httpResponse, nil
}

func (q *quoteRequest) recoverPanic(errChannel chan<- schema.SupplierResponseError) {
	if err := recover(); err != nil {
		errChannel <- schema.NewConnectionError("requesting supplier failed")
		log.Err(fmt.Errorf("%v", string(debug.Stack()))).Msg(fmt.Sprintf("Recovered from a panic: %v", err))
	}
}

func (q *quoteRequest) Execute(
	ctx context.Context,
	httpTransport *http.Transport,
	rates schema.RatesResponse,
	extras []ota.PricedEquip,
) (schema.RatesResponse, error) {
	q.slowLogger.Start("hertz:quote:execute:client")

	quote := schema.RatesResponse{}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	requestsBucket.AddRequests(converting.Unwrap(rates.SupplierRequests))
	errorsBucket.AddErrors(converting.Unwrap(rates.Errors))

	quote.SupplierRequests = requestsBucket.SupplierRequests()
	quote.Errors = errorsBucket.Errors()

	timeout := q.params.Timeouts.Default
	if q.params.Timeouts.Rates != nil {
		timeout = *q.params.Timeouts.Rates
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: httpTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(q.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	q.slowLogger.Stop("hertz:quote:execute:client")

	vehicles := []schema.Vehicle{}

	q.slowLogger.Start("hertz:quote:execute:requests")

	wait := sync.WaitGroup{}

	vehicleQueue := make(chan schema.Vehicle, len(rates.Vehicles))
	errQueue := make(chan schema.SupplierResponseError, 1)

	for _, vehicle := range rates.Vehicles {
		wait.Add(1)
		go func(v schema.Vehicle) {
			defer q.recoverPanic(errQueue)

			k := fmt.Sprintf("hertz:quote:execute:request:%s", v.Class)

			q.slowLogger.Start(k)
			defer q.slowLogger.Stop(k)

			response, e := requesting.RequestErrors(q.quoteRequest(ctx, client, v.Class))
			if e != nil {
				errQueue <- *e
				return
			}

			k2 := fmt.Sprintf("%s:parse", k)
			q.slowLogger.Start(k2)
			defer q.slowLogger.Stop(k2)

			var body ota.VehModifyRS
			bodyBytes, _ := io.ReadAll(response.Body)
			response.Body.Close()

			err := xml.Unmarshal(bodyBytes, &body)
			if err != nil {
				errQueue <- schema.NewSupplierError("Invalid response from supplier")
				return
			}

			message := body.ErrorMessage()
			if message != "" {
				errQueue <- schema.NewSupplierError(message)
				return
			}

			parsedVehicle, errMessage := q.parseQuote(&body, extras)
			if errMessage != "" {
				errQueue <- schema.NewSupplierError(errMessage)
				return
			}

			vehicleQueue <- parsedVehicle
		}(vehicle)
	}

	go func() {
		for vehicle := range vehicleQueue {
			vehicles = append(vehicles, vehicle)
			wait.Done()
		}
	}()

	go func() {
		for err := range errQueue {
			errorsBucket.AddError(err)
			wait.Done()
		}
	}()

	wait.Wait()

	q.slowLogger.Stop("hertz:quote:execute:requests")

	close(vehicleQueue)
	close(errQueue)

	quote.Vehicles = vehicles

	return quote, nil
}
