package handler

import (
	"errors"

	queryrun "go-api/internal/application/query/run"
	"go-api/internal/domain/paginate"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type RunHandler struct {
	listHandler runListHandler
	getHandler  runGetHandler
}

func NewRunHandler(listHandler runListHandler, getHandler runGetHandler) *RunHandler {
	return &RunHandler{listHandler: listHandler, getHandler: getHandler}
}

func (h *RunHandler) List(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}

	var listQuery paginate.PaginateQuery
	if err := validation.BindAndValidateQuery(c, &listQuery); err != nil {
		return err
	}
	sortBy, orderBy := listQuery.SortBy, listQuery.OrderBy
	listQuery.Normalize()
	if sortBy == "" {
		listQuery.SortBy = "startedAt"
	}
	if orderBy == "" {
		listQuery.OrderBy = paginate.OrderByDesc
	}

	views, total, err := h.listHandler.Handle(c.Context(), queryrun.ListRunsQuery{
		UserID:     user.ID,
		WorkflowID: workflowID,
		Query:      listQuery,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to list runs",
		})
	}
	return c.Status(fiber.StatusOK).JSON(paginate.NewPaginateResponse(
		presenter.NewRunListResponseFromViews(views),
		int(total),
		listQuery,
	))
}

func (h *RunHandler) GetByID(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	runID, err := uuid.Parse(c.Params("runId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid run id"})
	}

	view, err := h.getHandler.Handle(c.Context(), queryrun.GetRunQuery{
		UserID:     user.ID,
		WorkflowID: workflowID,
		RunID:      runID,
	})
	if err != nil {
		if errors.Is(err, queryrun.ErrRunNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Run not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to get run",
		})
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewRunDetailResponseFromView(*view))
}
