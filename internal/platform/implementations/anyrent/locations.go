package anyrent

import (
	"context"
	jsonEncoding "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/anyrent/json"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/google/go-querystring/query"
	"github.com/rs/zerolog"
)

type locationsRequest struct {
	cache         *caching.Cacher
	params        schema.LocationsRequestParams
	configuration schema.AnyRentConfiguration
	logger        *zerolog.Logger
	slowLogger    slowlog.Logger
}

func (l *locationsRequest) Execute(ctx context.Context, httpTransport *http.Transport) (schema.LocationsResponse, error) {
	locations := schema.LocationsResponse{
		Locations: &[]schema.Location{},
	}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	locations.SupplierRequests = requestsBucket.SupplierRequests()
	locations.Errors = errorsBucket.Errors()

	// fetch auth token
	l.slowLogger.Start("anyrent:locations:execute:auth")
	authRequest := authRequest{
		configuration: l.configuration,
		logger:        l.logger,
		timeout:       l.params.Timeouts.Default,
		cache:         l.cache,
	}

	auth, err := authRequest.Execute(httpTransport)
	requestsBucket.AddRequests(*auth.SupplierRequests)
	errorsBucket.AddErrors(*auth.Errors)
	l.slowLogger.Stop("anyrent:locations:execute:auth")

	if err != nil {
		return locations, err
	}

	if auth.Token == nil {
		return locations, nil
	}

	timeout := l.params.Timeouts.Default
	if l.params.Timeouts.Locations != nil {
		timeout = *l.params.Timeouts.Locations
	}

	// prepare client
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Transport: &requesting.InterceptorTransport{
			Transport: httpTransport,
			Middlewares: []requesting.TransportMiddleware{
				requesting.NewLoggingTransportMiddleware(l.logger),
				requesting.NewBucketTransportMiddleware(&requestsBucket),
			},
		},
	}

	// fetch the first page
	l.slowLogger.Start("anyrent:locations:execute:requests")
	response, err := l.makeRequest(client, 1, *auth.Token)
	l.slowLogger.Stop("anyrent:locations:execute:requests")

	if err != nil {
		errorsBucket.AddError(schema.NewSupplierError(err.Error()))
		return locations, nil
	}

	l.slowLogger.Start("anyrent:locations:execute:mapLocations")
	for _, station := range response.Stations {
		location, err := l.parseLocation(station)
		if err != nil {
			errorsBucket.AddError(schema.NewSupplierError(err.Error()))
			return locations, err
		}

		*locations.Locations = append(*locations.Locations, location)
	}
	l.slowLogger.Stop("anyrent:locations:execute:mapLocations")

	// fetch the rest of pages
	if response.Meta.Pagination.TotalPages > 1 {
		l.slowLogger.Start("anyrent:locations:execute:extraRequests")
		locationsDoneResultChannel := make(chan bool, 1)
		locationResultChannel := make(chan schema.Location)
		locationsErrChannel := make(chan schema.SupplierResponseError)
		defer close(locationResultChannel)
		defer close(locationsErrChannel)
		defer close(locationsDoneResultChannel)

		restOfPagesCount := response.Meta.Pagination.TotalPages - 1

		for page := 2; page <= restOfPagesCount+1; page++ {
			go l.makeExtraRequest(client, page, *auth.Token, locationResultChannel, locationsErrChannel, locationsDoneResultChannel)
		}

		finished := 0

		for ok := true; ok; ok = finished < restOfPagesCount {
			select {
			case location := <-locationResultChannel:
				*locations.Locations = append(*locations.Locations, location)

			case locationErr := <-locationsErrChannel:
				errorsBucket.AddError(locationErr)

			case <-locationsDoneResultChannel:
				finished++
			}
		}
		l.slowLogger.Stop("anyrent:locations:execute:extraRequests")
	}

	errorsCount := len(*errorsBucket.Errors())

	// cleanup collected locations in case of errors
	if errorsCount > 0 {
		locations.Locations = &[]schema.Location{}
	}

	return locations, nil
}

func (l *locationsRequest) makeExtraRequest(
	client *http.Client,
	pageNumber int,
	token string,
	locationResultChannel chan<- schema.Location,
	locationsErrChannel chan<- schema.SupplierResponseError,
	locationsDoneResultChannel chan<- bool,
) {
	response, err := l.makeRequest(client, pageNumber, token)

	if err != nil {
		locationsErrChannel <- schema.NewSupplierError(err.Error())
		locationsDoneResultChannel <- true
		return
	}

	l.slowLogger.Start("anyrent:locations:execute:mapExtraLocations")
	for _, station := range response.Stations {
		location, err := l.parseLocation(station)
		if err != nil {
			locationsErrChannel <- schema.NewSupplierError(err.Error())
			locationsDoneResultChannel <- true
			return
		}

		locationResultChannel <- location
	}
	l.slowLogger.Stop("anyrent:locations:execute:mapExtraLocations")

	locationsDoneResultChannel <- true
}

func (l *locationsRequest) makeRequest(
	client *http.Client,
	pageNumber int,
	token string,
) (json.LocationsRS, error) {
	opt := json.LocationsRQ{
		Page: pageNumber,
	}
	v, _ := query.Values(opt)

	url := fmt.Sprintf("%v/v1/stations?%v", l.configuration.SupplierApiUrl, v.Encode())
	c := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Locations)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodGet, url, http.NoBody)
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("x-lang", "en")

	rs, err := requesting.RequestErrors(client.Do(httpRequest))
	if err != nil {
		return json.LocationsRS{}, errors.New(err.Message)
	}
	defer rs.Body.Close()

	// bind the response body to the json
	bodyBytes, _ := io.ReadAll(rs.Body)
	rs.Body.Close()

	var jsonLocationsResponse json.LocationsRS
	jsonEncodeErr := jsonEncoding.Unmarshal(bodyBytes, &jsonLocationsResponse)
	if jsonEncodeErr != nil {
		return json.LocationsRS{}, errors.New(jsonEncodeErr.Error())
	}

	message := jsonLocationsResponse.ErrorMessage()
	if message != "" {
		return json.LocationsRS{}, errors.New(message)
	}

	return jsonLocationsResponse, nil
}

func (l *locationsRequest) parseLocation(station json.LocationsRSStation) (schema.Location, error) {
	location := schema.Location{
		Code:       station.Code,
		Name:       station.Name,
		Country:    station.Country.Code,
		City:       &station.City,
		Address:    &station.Address,
		PostalCode: &station.ZipCode,
		Latitude:   station.GetLatitude(),
		Longitude:  station.GetLongitude(),
		Phone:      &station.Phone,
		RawData:    station.GetRawData(),
		OpeningHours: &[]schema.OpeningTime{{
			Open:    true,
			Weekday: 0,
			Start:   station.Schedule.Week.GetOpenTime(),
			End:     station.Schedule.Week.GetCloseTime(),
		}, {
			Open:    true,
			Weekday: 0,
			Start:   station.Schedule.Week.GetOohOpenTime(),
			End:     station.Schedule.Week.GetOohCloseTime(),
			Ooh:     converting.PointerToValue(true),
		}, {
			Open:    true,
			Weekday: 6,
			Start:   station.Schedule.Saturday.GetOpenTime(),
			End:     station.Schedule.Saturday.GetCloseTime(),
		}, {
			Open:    true,
			Weekday: 6,
			Start:   station.Schedule.Saturday.GetOohOpenTime(),
			End:     station.Schedule.Saturday.GetOohCloseTime(),
			Ooh:     converting.PointerToValue(true),
		}, {
			Open:    true,
			Weekday: 7,
			Start:   station.Schedule.Sunday.GetOpenTime(),
			End:     station.Schedule.Sunday.GetCloseTime(),
		}, {
			Open:    true,
			Weekday: 7,
			Start:   station.Schedule.Sunday.GetOohOpenTime(),
			End:     station.Schedule.Sunday.GetOohCloseTime(),
			Ooh:     converting.PointerToValue(true),
		}},
	}

	return location, nil
}
