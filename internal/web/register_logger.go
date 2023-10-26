package web

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func RegisterLogger(logger *zerolog.Logger) func(c *gin.Context) {
	return func(c *gin.Context) {
		correlationId := c.MustGet("correlationId").(string)

		requestLogger := logger.
			With().
			Str("correlationId", correlationId).
			Logger()

		c.Set("logger", &requestLogger)
	}
}
