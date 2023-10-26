package web

import (
	"net/http"

	"bitbucket.org/crgw/service-helpers/middleware"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func PanicRecovery(c *gin.Context) {
	gin.CustomRecoveryWithWriter(&recoveryWriter{
		logger: c.MustGet("logger").(*zerolog.Logger),
	}, func(c *gin.Context, err any) {
		message, ok := err.(string)
		if !ok {
			message = "Unknown error, panic recovered"
		}
		middleware.HandleError(c, http.StatusInternalServerError, message, nil)
	})(c)
}

type recoveryWriter struct {
	logger *zerolog.Logger
}

func (r *recoveryWriter) Write(p []byte) (n int, err error) {
	str := string(p)
	r.
		logger.
		Error().
		Msg(str)

	return len(str), nil
}
