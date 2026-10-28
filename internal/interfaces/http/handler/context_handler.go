package handler

import (
	"errors"

	querycontext "go-api/internal/application/query/contextpanel"
	domainworkflow "go-api/internal/domain/workflow"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/presenter"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type ContextHandler struct {
	latestRunHandler contextLatestRunHandler
	variablesHandler contextVariablesHandler
}

func NewContextHandler(
	latestRunHandler contextLatestRunHandler,
	variablesHandler contextVariablesHandler,
) *ContextHandler {
	return &ContextHandler{
		latestRunHandler: latestRunHandler,
		variablesHandler: variablesHandler,
	}
}

func (h *ContextHandler) GetLatestRun(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}

	view, err := h.latestRunHandler.Handle(c.Context(), querycontext.GetLatestRunQuery{
		UserID:     user.ID,
		WorkflowID: workflowID,
	})
	if err != nil {
		return writeContextError(c, err, "Failed to get latest run")
	}
	c.Set("Cache-Control", "no-store")
	return c.Status(fiber.StatusOK).JSON(presenter.NewContextLatestRunResponse(view))
}

func (h *ContextHandler) ListVariables(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}

	views, err := h.variablesHandler.Handle(c.Context(), querycontext.ListVariablesQuery{
		UserID:     user.ID,
		WorkflowID: workflowID,
	})
	if err != nil {
		return writeContextError(c, err, "Failed to list context variables")
	}
	c.Set("Cache-Control", "no-store")
	return c.Status(fiber.StatusOK).JSON(presenter.NewContextVariablesResponse(views))
}

func writeContextError(c fiber.Ctx, err error, fallback string) error {
	switch {
	case errors.Is(err, domainworkflow.ErrNotFound), errors.Is(err, domainworkflow.ErrWrongOwner):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Workflow not found",
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": fallback,
		})
	}
}
