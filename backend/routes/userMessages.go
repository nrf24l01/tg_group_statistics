package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/nrf24l01/tg_group_statistics/backend/handlers"
	"github.com/nrf24l01/tg_group_statistics/backend/middleware"
	"github.com/nrf24l01/tg_group_statistics/backend/schemas"

	echokitMW "github.com/nrf24l01/go-web-utils/echokit/middleware"
)

func RegisterUserMessageRoutes(e *echo.Echo, h *handlers.Handler) {
	group := e.Group("/metrics/user-messages")
	group.Use(middleware.BearerTokenMiddleware(*h.Config.TGGroupStatsConfig), echokitMW.QueryValidationMiddleware(func() interface{} {
		return &schemas.TimeRangeQuery{}
	}))

	group.GET("/per-user-per-day", h.GetMessagesPerUserPerDay)
	group.GET("/per-day-user-column", h.GetPerDayUserColumnHandler)
	group.GET("/per-day", h.GetMessagesPerDay)
	group.GET("/per-user-total", h.GetMessagesPerUserTotal)
}
