package web

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CorrelationId middleware adds a correlation id to the context from the request header
func CorrelationId(c *gin.Context) {
	correlationId := c.GetHeader("x-correlation-id")
	if correlationId == "" {
		correlationId = uuid.New().String()
	}

	c.Set("correlationId", correlationId)
}
