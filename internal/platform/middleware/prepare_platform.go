package middleware

import (
	"net/http"

	"bitbucket.org/crgw/service-helpers/middleware"
	"github.com/gin-gonic/gin"
)

type factory interface {
	GetPlatform(string) (any, error)
}

const (
	PlatformKey string = "platform"
)

func PreparePlatform(f factory) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		platformFromPath := ctx.Params.ByName("platform")

		platform, err := f.GetPlatform(platformFromPath)
		if err != nil {
			middleware.HandleError(ctx, http.StatusNotFound, "Failed to find platform service", err)
			ctx.Abort()
			return
		}

		ctx.Set(PlatformKey, platform)
	}
}
