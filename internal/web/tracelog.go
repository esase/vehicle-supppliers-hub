package web

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func TraceLog(c *gin.Context) {
	// Finish all others and then write trace log
	c.Next()

	logger := c.MustGet("logger").(*zerolog.Logger)
	startTime := c.MustGet("requestStartTime").(time.Time)

	logger.Info().
		Str("label", "trace").
		Str("method", c.Request.Method).
		Str("url", c.Request.URL.Path).
		Int("code", c.Writer.Status()).
		Float64("duration", time.Since(startTime).Seconds()).
		Msg("")
}
