package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	echoKitSchemas "github.com/nrf24l01/go-web-utils/echokit/schemas"
	"github.com/nrf24l01/tg_group_statistics/analyzer/postgres"
	"github.com/nrf24l01/tg_group_statistics/backend/schemas"
)

func (h *Handler) GetAllowedChatsHandler(c echo.Context) error {
	// Check what groups from config is in database
	var groups []postgres.Group
	if err := h.DB.Find(&groups).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echoKitSchemas.DefaultInternalErrorResponse)
	}

	resp := schemas.AllowedGroupsResponse{}
	for _, g := range groups {
		for _, allowedGroupID := range h.Config.TGGroupStatsConfig.AllowedChatIDs {
			if g.TgGroupID == int64(allowedGroupID) {
				resp.Groups = append(resp.Groups, schemas.Group{
					ID:   g.TgGroupID,
					Name: *g.Name,
				})
			}
		}
	}

	return c.JSON(http.StatusOK, resp)
}