package grouping

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type CachedValue struct {
	Code    int                 `json:"code"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

type storage struct {
	redis   *redis.Client
	log     *zerolog.Logger
	slowLog slowlog.Logger
}

func (s *storage) AcquireLock(ctx context.Context, cacheKey string) (bool, error) {
	response := s.redis.SetNX(ctx, cacheKey, "", 1*time.Minute)
	lockAcquired, err := response.Result()
	return lockAcquired, err
}

func (s *storage) ReleaseLock(ctx context.Context, cacheKey string) {
	s.redis.Del(context.Background(), cacheKey)
}

func (s *storage) StoreResponse(ctx context.Context, responseKey string, response *Response, duration time.Duration) {
	s.slowLog.Start("grouping:compression:compress")
	bytes, _ := json.Marshal(CachedValue{
		Code:    response.Code,
		Body:    response.Body,
		Headers: response.Headers,
	})
	compressed, err := deflate(bytes)
	s.slowLog.Stop("grouping:compression:compress")

	if err != nil {
		s.log.Err(err).Msg("Unable to compress the response body")
		return
	}

	s.redis.Set(ctx, responseKey, compressed, duration)
}

func (s *storage) FetchResponse(ctx context.Context, responseKey string) (*CachedValue, error) {
	response, err := s.redis.Get(context.Background(), responseKey).Bytes()

	// actual error
	if err != nil && err != redis.Nil {
		return nil, err
	}

	// no cache hit
	if err == redis.Nil {
		return nil, nil
	}

	s.slowLog.Start("grouping:compression:decompress")
	decompressed, err := inflate(response)
	if err != nil {
		return nil, err
	}

	value := CachedValue{}
	err = json.Unmarshal(decompressed, &value)
	s.slowLog.Stop("grouping:compression:decompress")

	if err != nil {
		return nil, err
	}

	return &value, err
}

func deflate(uncompressed []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, _ := flate.NewWriter(&buffer, flate.BestSpeed)

	_, err := writer.Write(uncompressed)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func inflate(compressed []byte) ([]byte, error) {
	buffer := bytes.NewReader(compressed)
	reader := flate.NewReader(buffer)
	defer reader.Close()

	var out bytes.Buffer
	_, err := out.ReadFrom(reader)
	if err != nil {
		return []byte{}, err
	}

	return out.Bytes(), nil
}
