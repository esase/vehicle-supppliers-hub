package middleware

import (
	"net/http"
	"reflect"

	"bitbucket.org/crgw/service-helpers/middleware"
	"github.com/gin-gonic/gin"
)

const (
	ParamsKey string = "params"
)

func PrepareParams(val any) gin.HandlerFunc {
	value := reflect.ValueOf(val)
	if value.Kind() == reflect.Ptr {
		panic(`Bind struct can not be a pointer.`)
	}

	typ := value.Type()

	return func(ctx *gin.Context) {
		params := reflect.New(typ).Interface()

		err := ctx.ShouldBind(&params)
		if err != nil {
			middleware.HandleError(ctx, http.StatusBadRequest, "Failed to bind request params", err)
			ctx.Abort()
			return
		}

		ctx.Set(ParamsKey, params)
	}
}
