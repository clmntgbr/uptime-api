package handler

import (
	schedulecmd "go-api/internal/application/command/schedule"
	workflowcmd "go-api/internal/application/command/workflow"
	queryworkflow "go-api/internal/application/query/workflow"
	"go-api/internal/domain/paginate"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type WorkflowHandler struct {
	createHandler workflowCreateHandler
	updateHandler workflowUpdateHandler
	listHandler   workflowListHandler
	getHandler    workflowGetHandler
	deleteHandler workflowDeleteHandler
	runHandler    workflowRunHandler
	stopHandler   workflowStopHandler
}

func NewWorkflowHandler(
	createHandler workflowCreateHandler,
	updateHandler workflowUpdateHandler,
	listHandler workflowListHandler,
	getHandler workflowGetHandler,
	deleteHandler workflowDeleteHandler,
	runHandler workflowRunHandler,
	stopHandler workflowStopHandler,
) *WorkflowHandler {
	return &WorkflowHandler{
		createHandler: createHandler,
		updateHandler: updateHandler,
		listHandler:   listHandler,
		getHandler:    getHandler,
		deleteHandler: deleteHandler,
		runHandler:    runHandler,
		stopHandler:   stopHandler,
	}
}

func (h *WorkflowHandler) Create(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	def, err := bindWorkflowDefinition(c)
	if err != nil {
		return err
	}

	w, err := h.createHandler.Handle(c.Context(), workflowcmd.CreateWorkflowCommand{
		UserID:     user.ID,
		Definition: def,
	})
	if err != nil {
		return writeWorkflowError(c, err, "Failed to create workflow")
	}
	return c.Status(fiber.StatusCreated).JSON(presenter.NewWorkflowIDResponse(*w))
}

func (h *WorkflowHandler) List(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	var listQuery paginate.PaginateQuery
	if err := validation.BindAndValidateQuery(c, &listQuery); err != nil {
		return err
	}
	sortBy, orderBy := listQuery.SortBy, listQuery.OrderBy
	listQuery.Normalize()
	if sortBy == "" {
		listQuery.SortBy = "createdAt"
	}
	if orderBy == "" {
		listQuery.OrderBy = paginate.OrderByDesc
	}

	views, total, err := h.listHandler.Handle(c.Context(), queryworkflow.ListWorkflowsQuery{
		UserID: user.ID,
		Query:  listQuery,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to list workflows"})
	}
	return c.Status(fiber.StatusOK).JSON(paginate.NewPaginateResponse(
		presenter.NewWorkflowListResponseFromViews(views),
		int(total),
		listQuery,
	))
}

func (h *WorkflowHandler) Update(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}

	def, err := bindWorkflowDefinition(c)
	if err != nil {
		return err
	}

	w, err := h.updateHandler.Handle(c.Context(), workflowcmd.UpdateWorkflowCommand{
		UserID:     user.ID,
		WorkflowID: workflowID,
		Definition: def,
	})
	if err != nil {
		return writeWorkflowError(c, err, "Failed to update workflow")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewWorkflowIDResponse(*w))
}

func (h *WorkflowHandler) GetByID(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}

	view, err := h.getHandler.Handle(c.Context(), queryworkflow.GetWorkflowQuery{
		UserID:     user.ID,
		WorkflowID: workflowID,
	})
	if err != nil {
		return writeWorkflowError(c, err, "Failed to get workflow")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewWorkflowResponseFromView(*view))
}

func (h *WorkflowHandler) Delete(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}

	err = h.deleteHandler.Handle(c.Context(), workflowcmd.DeleteWorkflowCommand{
		UserID:     user.ID,
		WorkflowID: workflowID,
	})
	if err != nil {
		return writeWorkflowError(c, err, "Failed to delete workflow")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *WorkflowHandler) Run(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}

	runID, err := h.runHandler.Handle(c.Context(), schedulecmd.RunManualCommand{
		UserID:     user.ID,
		WorkflowID: workflowID,
	})
	if err != nil {
		return writeWorkflowError(c, err, "Failed to run workflow")
	}
	return c.Status(fiber.StatusAccepted).JSON(presenter.NewWorkflowRunResponse(runID.String()))
}

func (h *WorkflowHandler) Stop(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}

	runID, err := h.stopHandler.Handle(c.Context(), schedulecmd.StopRunCommand{
		UserID:     user.ID,
		WorkflowID: workflowID,
	})
	if err != nil {
		return writeWorkflowError(c, err, "Failed to stop workflow")
	}
	return c.Status(fiber.StatusAccepted).JSON(presenter.NewWorkflowRunResponse(runID.String()))
}
