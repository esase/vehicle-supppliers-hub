package rently

import (
	"context"
	jsonEncoding "encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform/implementations/rently/json"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/caching"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/requesting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/rs/zerolog"
)

type locationsRequest struct {
	params        schema.LocationsRequestParams
	configuration schema.RentlyConfiguration
	logger        *zerolog.Logger
	slowLogger    slowlog.Logger
	cache         *caching.Cacher
}

func (l *locationsRequest) Execute(httpTransport *http.Transport) (schema.LocationsResponse, error) {
	locations := schema.LocationsResponse{
		Locations: &[]schema.Location{},
	}

	requestsBucket := schema.NewSupplierRequestsBucket()
	errorsBucket := schema.NewErrorsBucket()

	locations.SupplierRequests = requestsBucket.SupplierRequests()
	locations.Errors = errorsBucket.Errors()

	// fetch auth token
	authRequest := authRequest{
		configuration: l.configuration,
		logger:        l.logger,
		timeout:       l.params.Timeouts.Default,
		cache:         l.cache,
	}

	auth, err := authRequest.Execute(httpTransport)
	requestsBucket.AddRequests(*auth.SupplierRequests)
	errorsBucket.AddErrors(*auth.Errors)

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

	response, err := l.makeRequest(client, *auth.Token)

	if err != nil {
		errorsBucket.AddError(schema.NewSupplierError(err.Error()))
		return locations, nil
	}

	for _, place := range response {
		location, err := l.parseLocation(place)
		if err != nil {
			errorsBucket.AddError(schema.NewSupplierError(err.Error()))
			return locations, err
		}

		*locations.Locations = append(*locations.Locations, location)
	}

	return locations, nil
}

func (l *locationsRequest) makeRequest(
	client *http.Client,
	token string,
) ([]json.PlaceRS, error) {
	url := fmt.Sprintf("%v/api/Places", l.configuration.SupplierApiUrl)
	c := context.WithValue(context.Background(), schema.RequestingTypeKey, schema.Locations)

	httpRequest, _ := http.NewRequestWithContext(c, http.MethodGet, url, http.NoBody)
	httpRequest.Header.Set("Authorization", "Bearer "+token)

	rs, err := requesting.RequestErrors(client.Do(httpRequest))
	if err != nil {
		return []json.PlaceRS{}, errors.New(err.Message)
	}
	defer rs.Body.Close()

	// bind the response body to the json
	bodyBytes, _ := io.ReadAll(rs.Body)
	rs.Body.Close()

	var jsonPlacesResponse []json.PlaceRS
	jsonEncodeErr := jsonEncoding.Unmarshal(bodyBytes, &jsonPlacesResponse)
	if jsonEncodeErr != nil {
		return []json.PlaceRS{}, errors.New(jsonEncodeErr.Error())
	}

	return jsonPlacesResponse, nil
}

func (l *locationsRequest) parseLocation(place json.PlaceRS) (schema.Location, error) {
	location := schema.Location{
		Code:         strconv.Itoa(place.Id),
		Name:         place.Type,
		Country:      place.Country,
		City:         converting.PointerToValue(place.City),
		Address:      place.Address,
		Address2:     place.Address2,
		Latitude:     converting.PointerToValue(place.Latitude),
		Longitude:    converting.PointerToValue(place.Longitude),
		Phone:        converting.PointerToValue(place.Phone),
		Email:        converting.PointerToValue(openapi_types.Email(place.Email)),
		Iata:         place.Iata,
		OpeningHours: converting.PointerToValue(place.AttentionSchedule.Schedule.GetOpeningTime()),
		RawData:      place.GetRawData(),
	}

	return location, nil
}
