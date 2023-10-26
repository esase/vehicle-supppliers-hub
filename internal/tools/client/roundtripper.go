package client

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

type OutgoingLoggerRoundTripper struct {
	destination string
	logger      *zerolog.Logger
}

func NewOutgoingLoggerRoundTripper(logger *zerolog.Logger, destination string) *OutgoingLoggerRoundTripper {
	return &OutgoingLoggerRoundTripper{
		destination: destination,
		logger:      logger,
	}
}

func (r OutgoingLoggerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	startTime := time.Now()

	message := r.logger.Info().
		Str("label", "outgoing-request").
		Str("method", req.Method).
		Str("url", req.URL.String()).
		Str("destination", r.destination).
		Str("userAgent", req.UserAgent())

	defer func(startTime time.Time) {
		message.Float64("duration", time.Since(startTime).Seconds()).Msg("")
	}(startTime)

	res, err := http.DefaultTransport.RoundTrip(req)

	if err != nil {
		message.Str("error", err.Error())

		if res != nil {
			message.Int("code", res.StatusCode)
		} else {
			message.Int("code", 0)
		}
		return nil, err
	}

	message.Int("code", res.StatusCode)

	return res, nil
}
