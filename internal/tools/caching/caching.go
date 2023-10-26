package caching

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type Engine interface {
	Store(ctx context.Context, key string, value any, ttl time.Duration) error
	Fetch(ctx context.Context, key string) ([]byte, error)
}

type Cacher struct {
	engine Engine
}

func NewRedisCache(redisClient *redis.Client) *Cacher {
	return &Cacher{
		engine: &redisCache{
			redis: redisClient,
		},
	}
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

func (c *Cacher) Store(ctx context.Context, key string, value any, ttl time.Duration) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}

	compressed, err := deflate(bytes)
	if err != nil {
		return err
	}

	return c.engine.Store(ctx, key, compressed, ttl)
}

func (c *Cacher) Fetch(ctx context.Context, key string, destination any) bool {
	value, err := c.engine.Fetch(ctx, key)
	if err != nil {
		return false
	}

	if value == nil {
		return false
	}

	uncompressed, err := inflate(value)
	if err != nil {
		return false
	}

	err = json.Unmarshal(uncompressed, destination)
	return err == nil
}
