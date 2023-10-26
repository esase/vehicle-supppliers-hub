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
	"strconv"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/mapping"
	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/hertz/ota"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/rs/zerolog"
)

type ratesRequest struct {
	cache         *caching.Cacher
	params        schema.RatesRequestParams
	configuration schema.HertzConfiguration
	logger        *zerolog.Logger
	slowLogger    slowlog.Logger

	extras []ota.PricedEquip
}

func (r *ratesRequest) ratesRequestBodyWithExtras() ota.VehAvailRateRQ {
	body := r.ratesRequestBody()

	branchExtras := converting.Unwrap(r.params.BranchExtras)

	prefabs := make([]ota.SpecialEquipPref, len(branchExtras))
	for i, extra := range branchExtras {
		prefabs[i] = ota.SpecialEquipPref{
			EquipType: extra,
			Quantity:  1,
		}
	}

	body.VehAvailRQCore.SpecialEquipPrefs = &ota.SpecialEquipPrefs{
		SpecialEquipPref: prefabs,
	}

	return body
}

func (r *ratesRequest) ratesRequestBody() ota.VehAvailRateRQ {
	maxResponses := defaultMaxResponses
	if r.configuration.MaxResponses != nil {
		maxResponses, _ = strconv.Atoi(*r.configuration.MaxResponses)
	}

	pickUpDateTime := r.params.PickUp.DateTime
	dropOffDateTime := r.params.DropOff.DateTime

	var rateQualifier ota.RateQualifier
	if r.configuration.RateQualifier != nil ||
		r.configuration.PromotionCode != nil ||
		r.configuration.CorpDiscountNmbr != nil ||
		r.configuration.TravelPurpose != nil {
		rateQualifier = ota.RateQualifier{
			RateQualifier:    converting.Unwrap(r.configuration.RateQualifier),
			PromotionCode:    converting.Unwrap(r.configuration.PromotionCode),
			CorpDiscountNmbr: converting.Unwrap(r.configuration.CorpDiscountNmbr),
			TravelPurpose:    converting.Unwrap(r.configuration.TravelPurpose),
		}
	}

	var vehAvailRQInfo ota.VehAvailRQInfo
	if r.configuration.Tour != nil || r.configuration.ClubNumber != nil {
		if r.configuration.Tour != nil {
			vehAvailRQInfo.TourInfo = &ota.TourInfo{
				TourNumber: *r.configuration.Tour,
			}
		}

		if r.configuration.ClubNumber != nil {
			vehAvailRQInfo.Customer = &ota.Customer{
				Primary: &ota.Primary{
					CustLoyalty: &ota.CustLoyalty{
						MembershipID: *r.configuration.ClubNumber,
						ProgramID:    "ZE",
						TravelSector: "2",
					},
				},
			}
		}
	}

	return ota.VehAvailRateRQ{
		Xmlns:             "http://www.opentravel.org/OTA/2003/05",
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",
		XsiSchemaLocation: "http://www.opentravel.org/OTA/2003/05 OTA_VehAvailRateRQ.xsd",
		Version:           "1.008",
		MaxResponses:      maxResponses,
		POS:               ota.NewPOS(mapping.MappedResidenceCountry(r.configuration, r.params.ResidenceCountry), r.configuration, ""),
		VehAvailRQCore: ota.VehAvailRQCore{
			Status: "All",
			VehRentalCore: ota.VehRentalCore{
				PickUpDateTime: pickUpDateTime.Format(schema.DateTimeFormat),
				ReturnDateTime: dropOffDateTime.Format(schema.DateTimeFormat),
				PickUpLocation: ota.Location{
					LocationCode: r.params.PickUp.Code,
				},
				ReturnLocation: &ota.Location{
					LocationCode: r.params.DropOff.Code,
				},
			},
			RateQualifier: rateQualifier,
		},
		VehAvailRQInfo: vehAvailRQInfo,
	}
}

func (r *ratesRequest) requestBody(body ota.VehAvailRateRQ) string {
	xmlString, _ := xml.MarshalIndent(body, "", "    ")
	return string(xmlString)
}

func parseExtra(pricedEquip ota.PricedEquip, taxMultiplier float64) schema.ExtraOrFee {
	return schema.ExtraOrFee{
		Type: schema.EQP,
		Code: pricedEquip.Equipment.EquipType,
		Name: pricedEquip.Equipment.EquipType,
		Price: schema.PriceAmount{
			Amount:   schema.RoundedFloat(pricedEquip.Charge.Amount * taxMultiplier),
			Currency: pricedEquip.Charge.CurrencyCode,
		},
		IncludedInRate: pricedEquip.Charge.IncludedInRate,
		PayLocal:       !pricedEquip.Charge.IncludedInRate,
		Mandatory:      false,
	}
}

func (r *ratesRequest) parseVehicle(vehAvail ota.VehAvail, pricedEquips []ota.PricedEquip) (schema.Vehicle, string) {
	extrasAndFees := []schema.ExtraOrFee{}

	qualifier, _ := json.Marshal(mapping.SupplierRateReference{
		FromRates:                    vehAvail.VehAvailCore.Reference.ID,
		EstimatedTotalAmount:         fmt.Sprintf("%.2f", vehAvail.VehAvailCore.TotalCharge.EstimatedTotalAmount),
		EstimatedTotalAmountCurrency: vehAvail.VehAvailCore.TotalCharge.CurrencyCode,
	})

	vehiclePrice, err := vehAvail.VehAvailCore.TotalCharge.Price(r.params, vehAvail.VehAvailInfo.PaymentRules.PaymentRule)
	if err != "" {
		return schema.Vehicle{}, err
	}

	mileage := vehAvail.VehAvailCore.RentalRate.RateDistance.Mileage()

	taxMultiplier, taxCharge, taxIsPartOfTheVehiclePrice := vehAvail.VehAvailCore.RentalRate.VehicleCharges.TaxCharge(
		r.params,
		r.configuration,
		vehAvail.VehAvailInfo.PaymentRules.PaymentRule,
	)

	if taxIsPartOfTheVehiclePrice {
		vehiclePrice.Amount = schema.RoundedFloat(float64(vehiclePrice.Amount) * taxMultiplier)
	}

	charges := vehAvail.VehAvailCore.RentalRate.VehicleCharges.Charges()

	coverages, coveragePricePartOfVehiclePrice := vehAvail.VehAvailInfo.PricedCoverages.VehicleCoverages(
		taxMultiplier,
		vehAvail.VehAvailInfo.PaymentRules.PaymentRule,
		r.params,
		r.configuration,
	)

	vehiclePrice.Amount = schema.RoundedFloat(float64(vehiclePrice.Amount) + coveragePricePartOfVehiclePrice)

	fees := vehAvail.VehAvailCore.Fees.Fees(r.params, r.configuration, vehAvail.VehAvailInfo.PaymentRules.PaymentRule)

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

	rateReference := string(qualifier)

	vehicle := schema.Vehicle{
		Name:                  vehAvail.VehAvailCore.Vehicle.VehMakeModel.Name,
		Class:                 vehAvail.VehAvailCore.Vehicle.VehMakeModel.Code,
		Price:                 vehiclePrice,
		SupplierRateReference: &rateReference,
		ExtrasAndFees:         &extrasAndFees,
		AcrissCode:            &vehAvail.VehAvailCore.Vehicle.VehMakeModel.Code,
		HasAirco:              &vehAvail.VehAvailCore.Vehicle.AirConditionInd,
		Status:                schema.AVAILABLE,
		SmallSuitcases:        &vehAvail.VehAvailCore.Vehicle.BaggageQuantity,
		Doors:                 &vehAvail.VehAvailCore.Vehicle.VehType.DoorCount,
		Seats:                 &vehAvail.VehAvailCore.Vehicle.PassengerQuantity,
		TransmissionType:      mapping.Transmission(vehAvail.VehAvailCore.Vehicle.TransmissionType),
		FuelType:              mapping.FuelType(vehAvail.VehAvailCore.Vehicle.FuelType),
		DriveType:             mapping.DriveType(vehAvail.VehAvailCore.Vehicle.DriveType),
		Mileage:               &mileage,
	}

	return vehicle, ""
}

func (r *ratesRequest) recoverPanic(errChannel chan<- schema.SupplierResponseError) {
	if err := recover(); err != nil {
		errChannel <- schema.NewConnectionError("requesting supplier failed")
		r.logger.Err(fmt.Errorf("%v", string(debug.Stack()))).Msg(fmt.Sprintf("Recovered from a panic: %v", err))
	}
}

func (r *ratesRequest) ratesRequest(
	ctx context.Context,
	client *http.Client,
	resChannel chan<- ota.VehAvailRateRS,
	errChannel chan<- schema.SupplierResponseError,
) {
	requestBody := r.requestBody(r.ratesRequestBody())

	c := context.WithValue(ctx, schema.RequestingTypeKey, schema.Rates)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodPost, r.configuration.SupplierApiUrl, bytes.NewBuffer([]byte(requestBody)))
	httpRequest.Header.Set("Content-Type", "application/xml; charset=utf-8")

	go func() {
		defer r.recoverPanic(errChannel)

		rs, e := requesting.RequestErrors(client.Do(httpRequest))
		if e != nil {
			errChannel <- *e
			return
		}
		defer rs.Body.Close()

		var otaRatesResponse ota.VehAvailRateRS
		bodyBytes, _ := io.ReadAll(rs.Body)
		rs.Body.Close()

		err := xml.Unmarshal(bodyBytes, &otaRatesResponse)
		if err != nil {
			errChannel <- schema.NewSupplierError("unable to parse the body")
			return
		}

		message := otaRatesResponse.ErrorMessage()
		if message != "" {
			errChannel <- schema.NewSupplierError(message)
			return
		}

		resChannel <- otaRatesResponse
	}()
}

func (r *ratesRequest) extrasCacheKey() string {
	return fmt.Sprintf(
		"supplier-hertz:extras:2:%s:%s:%s:%d",
		r.configuration.VendorCode,
		converting.Unwrap(r.configuration.Taco),
		r.params.PickUp.Code,
		r.params.RentalDays,
	)
}

func (r *ratesRequest) extrasFromCache() ([]ota.PricedEquip, bool) {
	var pricedEquip []ota.PricedEquip

	// timeouts are enforced by the client
	ok := r.cache.Fetch(context.Background(), r.extrasCacheKey(), &pricedEquip)
	if !ok {
		return pricedEquip, false
	}

	return pricedEquip, true
}

func (r *ratesRequest) storeExtras(pricedEquips []ota.PricedEquip) {
	// timeouts are enforced by the client
	r.cache.Store(context.Background(), r.extrasCacheKey(), pricedEquips, 5*24*time.Hour)
}

func (r *ratesRequest) extrasRequest(
	ctx context.Context,
	client *http.Client,
	resChannel chan<- []ota.PricedEquip,
	errChannel chan<- schema.SupplierResponseError,
) {
	requestBody := r.requestBody(r.ratesRequestBodyWithExtras())

	c := context.WithValue(ctx, schema.RequestingTypeKey, schema.Extras)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodPost, r.configuration.SupplierApiUrl, bytes.NewBuffer([]byte(requestBody)))
	httpRequest.Header.Set("Content-Type", "application/xml; charset=utf-8")

	go func() {
		defer r.recoverPanic(errChannel)

		var pricedEquips []ota.PricedEquip

		pricedEquips, ok := r.extrasFromCache()

		if !ok {
			rs, e := requesting.RequestErrors(client.Do(httpRequest))
			if e != nil {
				errChannel <- *e
				return
			}
			defer rs.Body.Close()

			var otaExtrasResponse ota.VehAvailRateRS
			bodyBytes, _ := io.ReadAll(rs.Body)
			rs.Body.Close()

			err := xml.Unmarshal(bodyBytes, &otaExtrasResponse)
			if err != nil {
				errChannel <- schema.NewSupplierError("unable to parse the body")
				return
			}

			message := otaExtrasResponse.ErrorMessage()
			if message != "" {
				errChannel <- schema.NewSupplierError(message)
				return
			}

			if len(otaExtrasResponse.VehAvailRSCore.VehVendorAvails.VehVendorAvail.VehAvails.VehAvail) > 0 {
				pricedEquips = otaExtrasResponse.VehAvailRSCore.VehVendorAvails.VehVendorAvail.VehAvails.VehAvail[0].VehAvailCore.PricedEquips.PricedEquip

				r.storeExtras(pricedEquips)
			}
		}

		resChannel <- pricedEquips
	}()
}

func (r *ratesRequest) Extras() []ota.PricedEquip {
	return r.extras
}

func (r *ratesRequest) Execute(ctx context.Context, httpTransport *http.Transport) (schema.RatesResponse, error) {
	r.slowLogger.Start("hertz:rates:execute:client")

	rates := schema.RatesResponse{
		Vehicles: []schema.Vehicle{},
	}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	rates.SupplierRequests = requestsBucket.SupplierRequests()
	rates.Errors = errorsBucket.Errors()

	timeout := r.params.Timeouts.Default
	if r.params.Timeouts.Rates != nil {
		timeout = *r.params.Timeouts.Rates
	}

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: httpTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(r.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	r.slowLogger.Stop("hertz:rates:execute:client")

	r.slowLogger.Start("hertz:rates:execute:requests")

	r.slowLogger.Start("hertz:rates:execute:requests:prepare")

	ratesResChannel := make(chan ota.VehAvailRateRS, 1)
	ratesErrChannel := make(chan schema.SupplierResponseError, 1)
	defer close(ratesResChannel)
	defer close(ratesErrChannel)

	r.ratesRequest(ctx, client, ratesResChannel, ratesErrChannel)

	extrasResChannel := make(chan []ota.PricedEquip, 1)
	extrasErrChannel := make(chan schema.SupplierResponseError, 1)
	defer close(extrasResChannel)
	defer close(extrasErrChannel)

	r.extrasRequest(ctx, client, extrasResChannel, extrasErrChannel)

	r.slowLogger.Stop("hertz:rates:execute:requests:prepare")

	r.slowLogger.Start("hertz:rates:execute:requests:extras")

	pricedEquips := make([]ota.PricedEquip, 0)

	select {
	case pricedEquips = <-extrasResChannel:
		break

	case extrasErr := <-extrasErrChannel:
		errorsBucket.AddError(extrasErr)
		break
		// continue with no extras
	}

	r.extras = pricedEquips

	r.slowLogger.Stop("hertz:rates:execute:requests:extras")

	r.slowLogger.Start("hertz:rates:execute:requests:rates")

	var vehAvailRateRS ota.VehAvailRateRS

	select {
	case vehAvailRateRS = <-ratesResChannel:
		break

	case ratesErr := <-ratesErrChannel:
		errorsBucket.AddError(ratesErr)
		return rates, nil
	}

	r.slowLogger.Stop("hertz:rates:execute:requests:rates")

	r.slowLogger.Stop("hertz:rates:execute:requests")

	r.slowLogger.Start("hertz:rates:execute:mapVehicles")

	for _, vehAvail := range vehAvailRateRS.VehAvailRSCore.VehVendorAvails.VehVendorAvail.VehAvails.VehAvail {
		if vehAvail.VehAvailCore.Status != "Available" {
			continue
		}

		vehicle, err := r.parseVehicle(vehAvail, pricedEquips)
		if err != "" {
			errorsBucket.AddError(schema.NewSupplierError(err))
			continue
		}

		rates.Vehicles = append(rates.Vehicles, vehicle)
	}

	r.slowLogger.Stop("hertz:rates:execute:mapVehicles")

	rates.BranchVehicleWhereAt = vehAvailRateRS.VehAvailRSCore.VehVendorAvails.VehVendorAvail.Info.LocationDetails.AdditionalInfo.CounterLocation.Location

	return rates, nil
}
