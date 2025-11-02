package routes

import (
	"github.com/labstack/echo/v4"
	"github.com/nrf24l01/tg_group_statistics/backend/handlers"
)

func RegisterRoutes(e *echo.Echo, h *handlers.Handler) {
	RegisterUserMessageRoutes(e, h)
	RegisterConfigRoutes(e, h)
}