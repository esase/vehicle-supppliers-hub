package spit

import (
	"context"
	"fmt"
	"net/http"

	"bitbucket.org/crgw/supplier-hub/internal/tools/client/spit/openapi"
)

type ConvertTokenParams struct {
	Context             *context.Context
	AffiliateCode       string
	IsTestEnv           bool
	TransactionCurrency string
	UatToken            string
	SpitToken           string
}

func (c *Client) ConvertToken(params *ConvertTokenParams) (*CardInfo, error) {
	env := openapi.Production
	if params.IsTestEnv {
		env = openapi.Sandbox
	}

	requestBody := openapi.ConvertTokenToBookingDotComJSONRequestBody{
		AffiliateCode:       params.AffiliateCode,
		Environment:         env,
		TransactionCurrency: params.TransactionCurrency,
	}

	requestParams := openapi.ConvertTokenToBookingDotComParams{
		XUserAccessToken: params.UatToken,
		XSptToken:        params.SpitToken,
	}

	response, err := c.client.ConvertTokenToBookingDotComWithResponse(*params.Context, &requestParams, requestBody)

	if err != nil {
		return nil, err
	}

	if response.StatusCode() != http.StatusCreated {
		return nil, fmt.Errorf("invalid status code: %d", response.StatusCode())
	}

	if response.JSON201 == nil {
		return nil, fmt.Errorf("missing response body")
	}

	return &CardInfo{
		CardVaultToken: response.JSON201.CardVaultToken,
		Eci:            response.JSON201.ThreeDs.EciFlag,
		Cavv:           response.JSON201.ThreeDs.Cavv,
		TransactionId:  response.JSON201.ThreeDs.TransactionId,
	}, nil
}
