package handler

import (
	querystatistics "go-api/internal/application/query/statistics"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/presenter"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type StatisticsHandler struct {
	dashboardHandler statisticsDashboardHandler
}

func NewStatisticsHandler(dashboardHandler statisticsDashboardHandler) *StatisticsHandler {
	return &StatisticsHandler{dashboardHandler: dashboardHandler}
}

func (h *StatisticsHandler) GetDashboard(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"message": "Unauthorized",
		})
	}

	var query struct {
		WorkflowID string `query:"workflowId"`
	}
	if err := c.Bind().Query(&query); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid query parameters",
		})
	}

	q := querystatistics.GetDashboardQuery{UserID: user.ID}
	if query.WorkflowID != "" {
		id, err := uuid.Parse(query.WorkflowID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid workflow id",
			})
		}
		q.WorkflowID = &id
	}

	result, err := h.dashboardHandler.Handle(c.Context(), q)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to load dashboard statistics",
		})
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewDashboardStatisticsResponse(result))
}
