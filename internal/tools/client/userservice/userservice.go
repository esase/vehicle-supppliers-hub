package userservice

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"bitbucket.org/crgw/supplier-hub/internal/tools/client"
	"bitbucket.org/crgw/supplier-hub/internal/tools/client/userservice/openapi"
	"github.com/rs/zerolog"
)

type Client struct {
	client *openapi.ClientWithResponses
}

func NewClient(logger *zerolog.Logger, optionFuncs ...client.OptionFunc) (*Client, error) {
	baseURL := os.Getenv("CRG_URL_USER_SERVICE")
	clientOptions := []client.OptionFunc{client.WithBaseURL(baseURL)}
	clientOptions = append(clientOptions, optionFuncs...)

	options, err := client.NewOptions(clientOptions...)
	if err != nil {
		return nil, err
	}

	// Add header to all requests
	addHeaders := func(ctx context.Context, req *http.Request) error {
		req.Header.Add("User-Agent", fmt.Sprintf("user-service-local-client via %s", options.Name()))
		return nil
	}

	client, err := openapi.NewClientWithResponses(
		options.BaseURL("user-service", ""),
		openapi.WithHTTPClient(&http.Client{
			Timeout:   options.Timeout(),
			Transport: client.NewOutgoingLoggerRoundTripper(logger, "user-service"),
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
