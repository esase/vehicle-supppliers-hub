package requesting

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"github.com/rs/zerolog"
)

type TransportMiddleware func(http.RoundTripper) http.RoundTripper

type InterceptorTransport struct {
	Transport   http.RoundTripper
	Middlewares []TransportMiddleware
}

func (t *InterceptorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.Transport
	for _, middleware := range t.Middlewares {
		transport = middleware(transport)
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type LoggingTransportMiddleware struct {
	Transport http.RoundTripper
	log       *zerolog.Logger
}

func NewLoggingTransportMiddleware(log *zerolog.Logger) TransportMiddleware {
	return func(rt http.RoundTripper) http.RoundTripper {
		return &LoggingTransportMiddleware{
			log:       log,
			Transport: rt,
		}
	}
}

func (t *LoggingTransportMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	startTime := time.Now()

	message := t.log.Info().
		Str("label", "outgoing-request").
		Str("method", req.Method).
		Str("url", req.URL.String())

	defer func() {
		message.
			Float64("duration", time.Since(startTime).Seconds()).
			Msg("")
	}()

	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		message.Str("error", err.Error())
		return nil, err
	}

	message.Int("code", resp.StatusCode)

	return resp, nil
}

type RequestBucket interface {
	FinishedRequest(
		requestType schema.SupplierRequestName,
		startTime time.Time,
		statusCode int,
		method string,
		url string,
		requestBody string,
		requestHeaders http.Header,
		responseBody string,
		responseHeaders http.Header,
	)
}

type BucketTransportMiddleware struct {
	Transport http.RoundTripper
	Bucket    RequestBucket
}

func NewBucketTransportMiddleware(bucket RequestBucket) TransportMiddleware {
	return func(rt http.RoundTripper) http.RoundTripper {
		return &BucketTransportMiddleware{
			Transport: rt,
			Bucket:    bucket,
		}
	}
}

func (b *BucketTransportMiddleware) RoundTrip(request *http.Request) (*http.Response, error) {
	startTime := time.Now()

	requestType := request.Context().Value(schema.RequestingTypeKey).(schema.SupplierRequestName)

	requestBytes, _ := io.ReadAll(request.Body)
	request.Body.Close()
	request.Body = io.NopCloser(bytes.NewBuffer(requestBytes))

	status := 0
	resBody := ""
	resHeaders := make(http.Header)

	defer func() {
		b.Bucket.FinishedRequest(
			requestType,
			startTime,
			status,
			request.Method,
			request.URL.String(),
			string(requestBytes),
			request.Header,
			resBody,
			resHeaders,
		)
	}()

	response, err := b.Transport.RoundTrip(request)
	if err != nil {
		return nil, err
	}

	responseBytes, _ := io.ReadAll(response.Body)
	response.Body.Close()
	response.Body = io.NopCloser(bytes.NewBuffer(responseBytes))

	status = response.StatusCode
	resBody = string(responseBytes)
	resHeaders = response.Header

	return response, nil
}
