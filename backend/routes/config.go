package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/nrf24l01/tg_group_statistics/backend/handlers"
	"github.com/nrf24l01/tg_group_statistics/backend/middleware"
)

func RegisterConfigRoutes(e *echo.Echo, h *handlers.Handler) {
	group := e.Group("/config")
	group.Use(middleware.BearerTokenMiddleware(*h.Config.TGGroupStatsConfig))

	group.GET("/groups", h.GetAllowedChatsHandler)
}
