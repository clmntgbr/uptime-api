package handler

import (
	querystats "go-api/internal/application/query/stats"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/presenter"

	"github.com/gofiber/fiber/v3"
)

type StatsHandler struct {
	dashboardHandler statsDashboardHandler
}

func NewStatsHandler(dashboardHandler statsDashboardHandler) *StatsHandler {
	return &StatsHandler{dashboardHandler: dashboardHandler}
}

func (h *StatsHandler) GetDashboard(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized",
		})
	}

	stats, err := h.dashboardHandler.Handle(c.Context(), querystats.GetDashboardQuery{UserID: user.ID})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to load dashboard stats",
		})
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewDashboardStatsResponse(stats))
}
