package grouping

import (
	"bytes"
	"context"
	"net/http"

	"bitbucket.org/crgw/service-helpers/middleware"
	"bitbucket.org/crgw/supplier-hub/internal/platform/interfaces"
	platformMiddleware "bitbucket.org/crgw/supplier-hub/internal/platform/middleware"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

type RequestManager interface {
	HandleRequest(context.Context, func() (*Response, error)) (*Response, error)
}

type MiddlewareOptions struct {
	CreateManager func(
		redis *redis.Client,
		log *zerolog.Logger,
		cacheKey string,
	) RequestManager
	RedisClient *redis.Client
}

func Middleware(o MiddlewareOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		log := c.MustGet("logger").(*zerolog.Logger)

		service, ok := c.MustGet(platformMiddleware.PlatformKey).(interfaces.WithTrafficLightRatesGrouping)
		if !ok {
			log.Warn().Msg("TrafficLight added to route, but not WithTrafficLightRatesGrouping compatible")
			c.Next()
			return
		}

		params := c.MustGet(platformMiddleware.ParamsKey).(*schema.RatesRequestParams)

		cacheKey := service.TrafficLightGroupingCacheKey(c.Request.Context(), *params, log)

		groupingManager := o.CreateManager(o.RedisClient, log, cacheKey)

		requester := func() (*Response, error) {
			bodyWriter := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
			c.Writer = bodyWriter

			// expects rates handler to be called
			c.Next()

			code := c.Writer.Status()
			body := bodyWriter.body.String()
			headers := bodyWriter.Header()
			err := c.Err()

			return &Response{
				Code:    code,
				Body:    body,
				Headers: headers,
			}, err
		}

		response, err := groupingManager.HandleRequest(c.Request.Context(), requester)

		if !c.Writer.Written() {
			if err != nil {
				middleware.HandleError(
					c,
					http.StatusBadRequest,
					"Error requesting rates",
					err,
				)
				return
			}

			for key, values := range response.Headers {
				for _, value := range values {
					c.Writer.Header().Add(key, value)
				}
			}

			c.Status(response.Code)
			c.Data(response.Code, gin.MIMEJSON, []byte(response.Body))
		}

		c.Abort()
	}
}
