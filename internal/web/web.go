package web

import (
	"net/http"
	"os"
	"time"

	"bitbucket.org/crgw/supplier-hub/internal/platform"
	"bitbucket.org/crgw/supplier-hub/internal/platform/factory"
	"bitbucket.org/crgw/supplier-hub/internal/tools/redisfactory"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func SetupRouter(log *zerolog.Logger, redisFactory *redisfactory.Factory) *gin.Engine {
	var (
		startTime       = time.Now()
		openApiLocation = os.Getenv("OPENAPI_LOCATION")
	)

	if openApiLocation == "" {
		openApiLocation = "./api/openapi.json"
	}

	openApiContent, _ := os.ReadFile(openApiLocation)

	router := gin.New()

	if os.Getenv("ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router.
		Use(StartRequest).
		Use(CorrelationId).
		Use(RegisterLogger(log)).
		Use(TraceLog).
		Use(PanicRecovery).
		Use(OpenapiValidator())

	router.GET("/status", func(c *gin.Context) {
		response := struct {
			Uptime float64 `json:"uptime"`
		}{
			Uptime: time.Since(startTime).Seconds(),
		}

		c.JSON(http.StatusOK, response)
	})

	router.GET("/openapi.json", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.String(http.StatusOK, string(openApiContent))
	})

	pprof.Register(router)

	platform.RegisterRoutes(
		router,
		factory.NewFactory(redisFactory),
		redisFactory,
	)

	return router
}
