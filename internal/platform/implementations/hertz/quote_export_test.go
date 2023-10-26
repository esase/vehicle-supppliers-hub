package hertz

import (
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/rs/zerolog"
)

func NewQuoteRequest(
	params schema.RatesRequestParams,
	logger *zerolog.Logger,
) quoteRequest {
	configuration, _ := params.Configuration.AsHertzConfiguration()

	return quoteRequest{
		params:        params,
		configuration: configuration,
		logger:        logger,
		slowLogger:    slowlog.CreateLogger(logger),
	}
}
