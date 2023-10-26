package web

import (
	"time"

	"github.com/gin-gonic/gin"
)

// CurrentTimeFunc Current time. Can be mocked for testing.
var CurrentTimeFunc = time.Now

func StartRequest(c *gin.Context) {
	c.Set("requestStartTime", CurrentTimeFunc())
}
