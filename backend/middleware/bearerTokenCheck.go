package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nrf24l01/go-web-utils/echokit/schemas"
	"github.com/nrf24l01/tg_group_statistics/backend/core"
)

func BearerTokenMiddleware(config core.TGGroupStatsConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Извлекаем токен из заголовка Authorization
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, schemas.ErrorResponse{Message: "missing authorization header", Code: http.StatusUnauthorized})
			}

			// Убираем "Bearer " из заголовка
			if len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
				return c.JSON(http.StatusUnauthorized, schemas.ErrorResponse{Message: "invalid token format", Code: http.StatusUnauthorized})
			}
			tokenString := authHeader[7:]
			
			if tokenString != config.AccessTokenSecret {
				return c.JSON(http.StatusUnauthorized, schemas.ErrorResponse{Message: "invalid token", Code: http.StatusUnauthorized})
			}

			return next(c)
		}
	}
}