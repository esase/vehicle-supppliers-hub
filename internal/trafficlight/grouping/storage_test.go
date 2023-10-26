package grouping

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestCacheAcquireLock(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)
	slowLog := slowlog.CreateLogger(&log)
	redisClient, redisMock := redismock.NewClientMock()

	storage := storage{
		redis:   redisClient,
		log:     &log,
		slowLog: slowLog,
	}

	t.Run("should acquire lock successfully", func(t *testing.T) {
		redisMock.ExpectSetNX("cacheKey", "", 1*time.Minute).SetVal(true)

		lock, err := storage.AcquireLock(context.TODO(), "cacheKey")
		assert.Nil(t, err)
		assert.True(t, lock)
	})

	t.Run("should handle context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		cancel()

		lock, err := storage.AcquireLock(ctx, "cacheKey")
		assert.NotNil(t, err)
		assert.False(t, lock)
	})

	t.Run("should handle refused locking", func(t *testing.T) {
		redisMock.ExpectSetNX("cacheKey", "", 1*time.Minute).SetVal(false)

		lock, err := storage.AcquireLock(context.Background(), "cacheKey")
		assert.Nil(t, err)
		assert.False(t, lock)
	})
}

func TestCacheReleaseLock(t *testing.T) {
	redisClient, redisMock := redismock.NewClientMock()

	storage := storage{
		redis: redisClient,
		log:   nil,
	}

	t.Run("should release the lock", func(t *testing.T) {
		redisMock.ExpectDel("cacheKey")
		storage.ReleaseLock(context.TODO(), "cacheKey")
	})
}

func TestCacheFetchResponse(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)
	slowLog := slowlog.CreateLogger(&log)
	redisClient, redisMock := redismock.NewClientMock()

	storage := storage{
		redis:   redisClient,
		log:     &log,
		slowLog: slowLog,
	}

	t.Run("should fetch body from cache", func(t *testing.T) {
		bytes, _ := json.Marshal(CachedValue{
			Code: http.StatusOK,
			Body: "body",
		})
		compressed, _ := deflate(bytes)

		redisMock.ExpectGet("responseKey").SetVal(string(compressed))
		response, err := storage.FetchResponse(context.TODO(), "responseKey")

		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "body", response.Body)
	})

	t.Run("should handle not getting a cache hit", func(t *testing.T) {
		redisMock.ExpectGet("responseKey").SetErr(redis.Nil)
		responsee, err := storage.FetchResponse(context.TODO(), "responseKey")

		assert.Nil(t, err)
		assert.Nil(t, responsee)
	})

	t.Run("should handle error", func(t *testing.T) {
		redisMock.ExpectGet("responseKey").SetErr(assert.AnError)
		responsee, err := storage.FetchResponse(context.TODO(), "responseKey")

		assert.NotNil(t, err)
		assert.Nil(t, responsee)
	})
}
