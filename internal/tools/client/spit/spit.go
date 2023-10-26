package spit

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"bitbucket.org/crgw/supplier-hub/internal/tools/client"
	"bitbucket.org/crgw/supplier-hub/internal/tools/client/spit/openapi"
	"github.com/rs/zerolog"
)

type Client struct {
	client *openapi.ClientWithResponses
}

func NewClient(logger *zerolog.Logger, optionFuncs ...client.OptionFunc) (*Client, error) {
	baseURL := os.Getenv("CRG_URL_SPIT")
	clientOptions := []client.OptionFunc{client.WithBaseURL(baseURL)}
	clientOptions = append(clientOptions, optionFuncs...)

	options, err := client.NewOptions(clientOptions...)
	if err != nil {
		return nil, err
	}

	// Add header to all requests
	addHeaders := func(ctx context.Context, req *http.Request) error {
		req.Header.Add("User-Agent", fmt.Sprintf("spit-local-client via %s", options.Name()))
		return nil
	}

	client, err := openapi.NewClientWithResponses(
		options.BaseURL("spit", ""),
		openapi.WithHTTPClient(&http.Client{
			Timeout:   options.Timeout(),
			Transport: client.NewOutgoingLoggerRoundTripper(logger, "spit"),
		}),
		openapi.WithRequestEditorFn(addHeaders),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		client: client,
	}, nil

}
