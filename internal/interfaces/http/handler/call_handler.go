package handler

import (
	callcmd "go-api/internal/application/command/call"
	querycall "go-api/internal/application/query/call"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/presenter"

	"github.com/gofiber/fiber/v3"
)

type CallHandler struct {
	createHandler callCreateHandler
	updateHandler callUpdateHandler
	deleteHandler callDeleteHandler
	listHandler   callListHandler
	getHandler    callGetHandler
}

func NewCallHandler(
	createHandler callCreateHandler,
	updateHandler callUpdateHandler,
	deleteHandler callDeleteHandler,
	listHandler callListHandler,
	getHandler callGetHandler,
) *CallHandler {
	return &CallHandler{
		createHandler: createHandler,
		updateHandler: updateHandler,
		deleteHandler: deleteHandler,
		listHandler:   listHandler,
		getHandler:    getHandler,
	}
}

func (h *CallHandler) List(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := parseWorkflowID(c)
	if err != nil {
		return err
	}

	views, err := h.listHandler.Handle(c.Context(), querycall.ListCallsQuery{
		UserID:     user.ID,
		WorkflowID: workflowID,
	})
	if err != nil {
		return writeCallError(c, err, "Failed to list calls")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewCallListResponseFromViews(views))
}

func (h *CallHandler) GetByID(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, err := parseWorkflowAndCallIDs(c)
	if err != nil {
		return err
	}

	view, err := h.getHandler.Handle(c.Context(), querycall.GetCallQuery{
		UserID:     user.ID,
		WorkflowID: workflowID,
		CallID:     callID,
	})
	if err != nil {
		return writeCallError(c, err, "Failed to get call")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewCallDetailResponseFromView(*view))
}

func (h *CallHandler) Create(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := parseWorkflowID(c)
	if err != nil {
		return err
	}
	def, err := bindCallDefinition(c)
	if err != nil {
		return err
	}

	call, err := h.createHandler.Handle(c.Context(), callcmd.CreateCallCommand{
		UserID:     user.ID,
		WorkflowID: workflowID,
		Definition: def,
	})
	if err != nil {
		return writeCallError(c, err, "Failed to create call")
	}
	return c.Status(fiber.StatusCreated).JSON(presenter.NewCallResponseFromCall(*call))
}

func (h *CallHandler) Update(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, err := parseWorkflowAndCallIDs(c)
	if err != nil {
		return err
	}
	def, err := bindCallDefinition(c)
	if err != nil {
		return err
	}

	call, err := h.updateHandler.Handle(c.Context(), callcmd.UpdateCallCommand{
		UserID:     user.ID,
		WorkflowID: workflowID,
		CallID:     callID,
		Definition: def,
	})
	if err != nil {
		return writeCallError(c, err, "Failed to update call")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewCallResponseFromCall(*call))
}

func (h *CallHandler) Delete(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, err := parseWorkflowAndCallIDs(c)
	if err != nil {
		return err
	}

	err = h.deleteHandler.Handle(c.Context(), callcmd.DeleteCallCommand{
		UserID:     user.ID,
		WorkflowID: workflowID,
		CallID:     callID,
	})
	if err != nil {
		return writeCallError(c, err, "Failed to delete call")
	}
	return c.SendStatus(fiber.StatusNoContent)
}
