package userservice

import (
	"context"
	"fmt"
	"net/http"

	"bitbucket.org/crgw/supplier-hub/internal/tools/client/userservice/openapi"
	"github.com/golang-jwt/jwt/v4"
)

func (c *Client) AuthUserViaPassword(ctx context.Context, username string, password string) (*User, error) {
	requestBody := openapi.AuthViaPasswordJSONRequestBody{
		Username: username,
		Password: password,
	}

	response, err := c.client.AuthViaPasswordWithResponse(ctx, requestBody)

	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("invalid status code: %d", response.StatusCode())
	}

	if response.JSON200 == nil {
		return nil, fmt.Errorf("missing response body")
	}

	token, err := jwt.ParseWithClaims(*response.JSON200.UserAccessToken, &parsedClaims{}, func(token *jwt.Token) (interface{}, error) {
		return nil, nil
	})

	if token == nil {
		return nil, err
	}

	var parsed *parsedClaims
	if claims, ok := token.Claims.(*parsedClaims); ok {
		parsed = claims
	} else {
		return nil, fmt.Errorf("could not parse user access token")
	}

	return &User{
		UserID:        parsed.UserID,
		AgencyID:      parsed.AgencyID,
		TopAgencyID:   parsed.TopAgencyID,
		ViaUserID:     parsed.ViaUserID,
		CorrelationID: parsed.CorrelationID,
		OriginalUAT:   *response.JSON200.UserAccessToken,
		Username:      username,
	}, nil
}
