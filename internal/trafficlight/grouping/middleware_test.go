package grouping_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"bitbucket.org/crgw/service-helpers/middleware"
	m "bitbucket.org/crgw/supplier-hub/internal/platform/middleware"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/trafficlight/grouping"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

type factoryMock struct{}

func (f *factoryMock) GetPlatform(name string) (any, error) {
	return &mockPlatform{}, nil
}

type groupingManagerMock struct {
	handleRequestMock func(ctx context.Context, requester func() (*grouping.Response, error)) (*grouping.Response, error)
}

func (m *groupingManagerMock) HandleRequest(ctx context.Context, requester func() (*grouping.Response, error)) (*grouping.Response, error) {
	return m.handleRequestMock(ctx, requester)
}

type mockPlatform struct{}

func (m *mockPlatform) TrafficLightGroupingCacheKey(ctx context.Context, params schema.RatesRequestParams, log *zerolog.Logger) string {
	return "cache_key"
}

func TestGroupingMiddleware(t *testing.T) {
	out := &bytes.Buffer{}
	log := zerolog.New(out)

	t.Run("should return the response from the next handler", func(t *testing.T) {
		redisClient, _ := redismock.NewClientMock()

		createManager := func(
			redis *redis.Client,
			log *zerolog.Logger,
			cacheKey string,
		) grouping.RequestManager {
			assert.Equal(t, "cache_key", cacheKey)

			return &groupingManagerMock{
				handleRequestMock: func(ctx context.Context, requester func() (*grouping.Response, error)) (*grouping.Response, error) {
					response, err := requester()
					assert.NoError(t, err)
					return &grouping.Response{Code: response.Code, Body: response.Body}, nil
				},
			}
		}

		response := httptest.NewRecorder()

		router := gin.Default()

		router.Use(middleware.CorrelationId)
		router.Use(middleware.RegisterLogger(&log))

		router.Use(m.PreparePlatform(&factoryMock{}))
		router.Use(m.PrepareParams(schema.RatesRequestParams{}))

		handleRates := func(c *gin.Context) {
			c.Header("Content-Type", c.ContentType())
			c.Status(http.StatusOK)
			io.Copy(c.Writer, bytes.NewReader([]byte("response from supplier")))
		}

		router.POST("/rates", grouping.Middleware(
			grouping.MiddlewareOptions{CreateManager: createManager, RedisClient: redisClient},
		), handleRates)

		reader := bytes.NewReader([]byte(""))

		request, err := http.NewRequest(http.MethodPost, "/rates", reader)
		assert.NoError(t, err)

		router.ServeHTTP(response, request)

		assert.Equal(t, http.StatusOK, response.Code)
	})

	t.Run("should provide from manager and not call the next handler", func(t *testing.T) {
		redisClient, _ := redismock.NewClientMock()

		createManager := func(
			redis *redis.Client,
			log *zerolog.Logger,
			cacheKey string,
		) grouping.RequestManager {
			assert.Equal(t, "cache_key", cacheKey)

			return &groupingManagerMock{
				handleRequestMock: func(ctx context.Context, requester func() (*grouping.Response, error)) (*grouping.Response, error) {
					return &grouping.Response{Code: http.StatusOK, Body: "response from cache"}, nil
				},
			}
		}

		response := httptest.NewRecorder()

		router := gin.Default()

		router.Use(middleware.CorrelationId)
		router.Use(middleware.RegisterLogger(&log))

		router.Use(m.PreparePlatform(&factoryMock{}))
		router.Use(m.PrepareParams(schema.RatesRequestParams{}))

		router.POST("/rates", grouping.Middleware(
			grouping.MiddlewareOptions{CreateManager: createManager, RedisClient: redisClient},
		), func(c *gin.Context) {
			assert.Fail(t, "Should not call supplier")
		})

		reader := bytes.NewReader([]byte(""))

		request, err := http.NewRequest(http.MethodPost, "/rates", reader)
		assert.NoError(t, err)

		router.ServeHTTP(response, request)
	})
}
