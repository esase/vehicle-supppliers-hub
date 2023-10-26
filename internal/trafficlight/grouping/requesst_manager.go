package grouping

import (
	"context"
	"encoding/json"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/converting"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type Response struct {
	Code    int
	Headers map[string][]string
	Body    string
}

type Storage interface {
	AcquireLock(ctx context.Context, cacheKey string) (bool, error)
	ReleaseLock(ctx context.Context, cacheKey string)
	StoreResponse(ctx context.Context, responseKey string, response *Response, duration time.Duration)
	FetchResponse(ctx context.Context, responseKey string) (*CachedValue, error)
}

type requestManager struct {
	groupingId string
	cache      Storage
	log        *zerolog.Logger
	slowLog    slowlog.Logger
	cacheKey   string
}

type ratesResponseObject struct {
	Errors *schema.SupplierResponseErrors `json:"errors"`
}

func isStatusCodeAcceptable(code int) bool {
	return code >= 200 && code < 300
}

func (m *requestManager) requestSupplierAndStore(
	responseKey string,
	requester func() (*Response, error),
) (*Response, error) {
	m.slowLog.Start("grouping:requestSupplierAndStore")
	defer m.slowLog.Stop("grouping:requestSupplierAndStore")

	response, err := requester()

	if err != nil {
		m.cache.ReleaseLock(context.Background(), m.cacheKey)
		m.log.Err(err).Msg("Unable to request supplier")
		return nil, err
	}

	duration := 10 * time.Minute

	var ratesResponse ratesResponseObject
	e := json.Unmarshal([]byte(response.Body), &ratesResponse)
	if e != nil || !isStatusCodeAcceptable(response.Code) || len(converting.Unwrap(ratesResponse.Errors)) != 0 {
		duration = 1 * time.Minute
	}

	m.cache.StoreResponse(context.Background(), responseKey, &Response{
		Code:    response.Code,
		Body:    response.Body,
		Headers: response.Headers,
	}, duration)

	m.cache.ReleaseLock(context.Background(), m.cacheKey)

	return response, err
}

func (m *requestManager) requestOrWait(ctx context.Context, requester func() (*Response, error)) (*Response, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
	}

	responseKey := "res:" + m.cacheKey

	m.slowLog.Start("grouping:fetchFromCache")
	response, err := m.cache.FetchResponse(ctx, responseKey)
	m.slowLog.Stop("grouping:fetchFromCache")

	if err != nil {
		m.log.Err(err).
			Str("label", "cache").
			Bool("hit", false).
			Str("key", responseKey).
			Msg("Error fetching from cache")

		return requester()
	}

	if response != nil {
		m.log.Info().
			Str("label", "cache").
			Bool("hit", true).
			Str("key", m.cacheKey).
			Msg("Used cache response")

		if response.Headers == nil {
			response.Headers = make(map[string][]string)
		}

		response.Headers["x-trafficlight-grouping-hit"] = []string{"hit"}

		return &Response{
			Code:    response.Code,
			Body:    response.Body,
			Headers: response.Headers,
		}, err
	}

	canMakeTheRequest, err := m.cache.AcquireLock(ctx, m.cacheKey)

	if err != nil || canMakeTheRequest {
		return m.requestSupplierAndStore(responseKey, requester)
	}

	time.Sleep(400 * time.Millisecond)

	return m.requestOrWait(ctx, requester)
}

func (m *requestManager) HandleRequest(ctx context.Context, requester func() (*Response, error)) (*Response, error) {
	m.slowLog.Start("grouping:HandleRequest")
	defer m.slowLog.Stop("grouping:HandleRequest")
	return m.requestOrWait(ctx, requester)
}

func NewRequestManager(
	redis *redis.Client,
	log *zerolog.Logger,
	cacheKey string,
) RequestManager {
	groupingId := uuid.New().String()
	logWithGroupingId := log.With().Str("groupingId", groupingId).Logger()
	slowLog := slowlog.CreateLogger(&logWithGroupingId)

	return &requestManager{
		groupingId: uuid.New().String(),
		cacheKey:   cacheKey,
		cache: &storage{
			redis:   redis,
			log:     &logWithGroupingId,
			slowLog: slowLog,
		},
		log:     &logWithGroupingId,
		slowLog: slowLog,
	}
}
