package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TapLogger(c *gin.Context) {
	platform := c.Params.ByName("platform")
	logger := c.MustGet("logger").(*zerolog.Logger)

	requestLogger := logger.
		With().
		Str("platform", platform).
		Str("operationId", uuid.New().String()).
		Logger()

	c.Set("logger", &requestLogger)
}
