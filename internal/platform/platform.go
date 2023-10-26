package platform

import (
	"fmt"
	"net/http"

	"bitbucket.org/crgw/service-helpers/middleware"
	"bitbucket.org/crgw/supplier-hub/internal/platform/errors"
	"bitbucket.org/crgw/supplier-hub/internal/platform/factory"
	"bitbucket.org/crgw/supplier-hub/internal/platform/interfaces"
	platformMiddleware "bitbucket.org/crgw/supplier-hub/internal/platform/middleware"
	"bitbucket.org/crgw/supplier-hub/internal/schema"
	"bitbucket.org/crgw/supplier-hub/internal/tools/redisfactory"
	"bitbucket.org/crgw/supplier-hub/internal/tools/slowlog"
	"bitbucket.org/crgw/supplier-hub/internal/trafficlight/grouping"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func RegisterRoutes(
	router *gin.Engine,
	factory *factory.Factory,
	redisFactory *redisfactory.Factory,
) {
	group := router.Group(
		"/:platform",
		platformMiddleware.PreparePlatform(factory),
		platformMiddleware.TapLogger,
	)

	group.POST("/rates",
		platformMiddleware.PrepareParams(schema.RatesRequestParams{}),
		grouping.Middleware(grouping.MiddlewareOptions{
			CreateManager: grouping.NewRequestManager,
			RedisClient:   redisFactory.TrafficlightClient(),
		}),
		func(ctx *gin.Context) {
			logger := ctx.MustGet("logger").(*zerolog.Logger)

			slowLog := slowlog.CreateLogger(logger)
			key := fmt.Sprintf("%s:rates", ctx.Params.ByName("platform"))
			slowLog.Start(key)

			platformWithRatesRequest, ok := ctx.MustGet(platformMiddleware.PlatformKey).(interfaces.WithGetRates)
			if !ok {
				middleware.HandleError(ctx, http.StatusBadRequest, "Rates not implemented", errors.ErrorNotImplemented)
				return
			}

			params, ok := ctx.MustGet(platformMiddleware.ParamsKey).(*schema.RatesRequestParams)
			if !ok {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Bad request params", nil)
				return
			}

			response, err := platformWithRatesRequest.GetRates(ctx.Request.Context(), *params, logger)
			if err != nil {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Failed requesting rates", nil)
				return
			}

			ctx.JSON(http.StatusOK, response)

			slowLog.Stop(key)
		},
	)

	group.POST("/booking",
		platformMiddleware.PrepareParams(schema.BookingRequestParams{}),
		func(ctx *gin.Context) {
			platformWithRatesRequest, ok := ctx.MustGet(platformMiddleware.PlatformKey).(interfaces.WithCreateBooking)
			if !ok {
				middleware.HandleError(ctx, http.StatusBadRequest, "Create booking not implemented", errors.ErrorNotImplemented)
				return
			}

			params, ok := ctx.MustGet(platformMiddleware.ParamsKey).(*schema.BookingRequestParams)
			if !ok {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Bad request params", nil)
				return
			}

			logger := ctx.MustGet("logger").(*zerolog.Logger)

			response, err := platformWithRatesRequest.CreateBooking(ctx.Request.Context(), *params, logger)
			if err != nil {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Failed requesting booking", nil)
				return
			}

			ctx.JSON(http.StatusOK, response)
		},
	)

	group.POST("/booking-status",
		platformMiddleware.PrepareParams(schema.BookingStatusRequestParams{}),
		func(ctx *gin.Context) {
			platformWithBookingStatusRequest, ok := ctx.MustGet(platformMiddleware.PlatformKey).(interfaces.WithBookingStatus)
			if !ok {
				middleware.HandleError(ctx, http.StatusBadRequest, "Booking status not implemented", errors.ErrorNotImplemented)
				return
			}

			params, ok := ctx.MustGet(platformMiddleware.ParamsKey).(*schema.BookingStatusRequestParams)
			if !ok {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Bad request params", nil)
				return
			}

			logger := ctx.MustGet("logger").(*zerolog.Logger)

			response, err := platformWithBookingStatusRequest.GetBookingStatus(ctx.Request.Context(), *params, logger)
			if err != nil {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Failed requesting booking status", nil)
				return
			}

			ctx.JSON(http.StatusOK, response)
		},
	)

	group.POST("/modify",
		platformMiddleware.PrepareParams(schema.ModifyRequestParams{}),
		func(ctx *gin.Context) {
			platformWithRatesRequest, ok := ctx.MustGet(platformMiddleware.PlatformKey).(interfaces.WithModifyBooking)
			if !ok {
				middleware.HandleError(ctx, http.StatusBadRequest, "Modify not implemented", errors.ErrorNotImplemented)
				return
			}

			params, ok := ctx.MustGet(platformMiddleware.ParamsKey).(*schema.ModifyRequestParams)
			if !ok {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Bad request params", nil)
				return
			}

			logger := ctx.MustGet("logger").(*zerolog.Logger)

			response, err := platformWithRatesRequest.ModifyBooking(ctx.Request.Context(), *params, logger)
			if err != nil {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Failed requesting modifying", nil)
				return
			}

			ctx.JSON(http.StatusOK, response)
		},
	)

	group.POST("/cancel",
		platformMiddleware.PrepareParams(schema.CancelRequestParams{}),
		func(ctx *gin.Context) {
			platformWithRatesRequest, ok := ctx.MustGet(platformMiddleware.PlatformKey).(interfaces.WithCancelBooking)
			if !ok {
				middleware.HandleError(ctx, http.StatusBadRequest, "Cancel not implemented", errors.ErrorNotImplemented)
				return
			}

			params, ok := ctx.MustGet(platformMiddleware.ParamsKey).(*schema.CancelRequestParams)
			if !ok {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Bad request params", nil)
				return
			}

			logger := ctx.MustGet("logger").(*zerolog.Logger)

			response, err := platformWithRatesRequest.CancelBooking(ctx.Request.Context(), *params, logger)
			if err != nil {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Failed requesting canceling", nil)
				return
			}

			ctx.JSON(http.StatusOK, response)
		},
	)

	group.POST("/locations",
		platformMiddleware.PrepareParams(schema.LocationsRequestParams{}),
		func(ctx *gin.Context) {

			platformWithLocationsRequest, ok := ctx.MustGet(platformMiddleware.PlatformKey).(interfaces.WithLocations)
			if !ok {
				middleware.HandleError(ctx, http.StatusBadRequest, "Locations import not implemented", errors.ErrorNotImplemented)
				return
			}

			params, ok := ctx.MustGet(platformMiddleware.ParamsKey).(*schema.LocationsRequestParams)
			if !ok {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Bad request params", nil)
				return
			}

			logger := ctx.MustGet("logger").(*zerolog.Logger)

			response, err := platformWithLocationsRequest.GetLocations(ctx.Request.Context(), *params, logger)
			if err != nil {
				middleware.HandleError(ctx, http.StatusInternalServerError, "Failed requesting locations", nil)
				return
			}

			ctx.JSON(http.StatusOK, response)
		},
	)
}
