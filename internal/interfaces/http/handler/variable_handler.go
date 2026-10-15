package handler

import (
	cmdvariable "go-api/internal/application/command/variable"
	queryvariable "go-api/internal/application/query/variable"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
)

type VariableHandler struct {
	createGlobal  variableCreateGlobalHandler
	updateGlobal  variableUpdateGlobalHandler
	deleteGlobal  variableDeleteGlobalHandler
	listGlobals   variableListGlobalsHandler
	getGlobal     variableGetGlobalHandler
	createStatic  variableCreateStaticHandler
	updateStatic  variableUpdateStaticHandler
	deleteStatic  variableDeleteStaticHandler
	listStatics   variableListStaticsHandler
	getStatic     variableGetStaticHandler
	createCall    variableCreateCallHandler
	updateCall    variableUpdateCallHandler
	deleteCall    variableDeleteCallHandler
	listCall      variableListCallHandler
	getCall       variableGetCallHandler
	listAvailable     variableListAvailableHandler
	listResponseKeys  variableListResponseKeysHandler
}

func NewVariableHandler(
	createGlobal variableCreateGlobalHandler,
	updateGlobal variableUpdateGlobalHandler,
	deleteGlobal variableDeleteGlobalHandler,
	listGlobals variableListGlobalsHandler,
	getGlobal variableGetGlobalHandler,
	createStatic variableCreateStaticHandler,
	updateStatic variableUpdateStaticHandler,
	deleteStatic variableDeleteStaticHandler,
	listStatics variableListStaticsHandler,
	getStatic variableGetStaticHandler,
	createCall variableCreateCallHandler,
	updateCall variableUpdateCallHandler,
	deleteCall variableDeleteCallHandler,
	listCall variableListCallHandler,
	getCall variableGetCallHandler,
	listAvailable variableListAvailableHandler,
	listResponseKeys variableListResponseKeysHandler,
) *VariableHandler {
	return &VariableHandler{
		createGlobal: createGlobal, updateGlobal: updateGlobal, deleteGlobal: deleteGlobal,
		listGlobals: listGlobals, getGlobal: getGlobal,
		createStatic: createStatic, updateStatic: updateStatic, deleteStatic: deleteStatic,
		listStatics: listStatics, getStatic: getStatic,
		createCall: createCall, updateCall: updateCall, deleteCall: deleteCall,
		listCall: listCall, getCall: getCall, listAvailable: listAvailable,
		listResponseKeys: listResponseKeys,
	}
}

// --- Globals ---

func (h *VariableHandler) ListGlobals(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	views, err := h.listGlobals.Handle(c.Context(), queryvariable.ListGlobalsQuery{UserID: user.ID})
	if err != nil {
		return writeVariableError(c, err, "Failed to list variables")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewGlobalVariableListResponse(views))
}

func (h *VariableHandler) GetGlobal(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	id, err := parseVariableID(c)
	if err != nil {
		return err
	}
	view, err := h.getGlobal.Handle(c.Context(), queryvariable.GetGlobalQuery{UserID: user.ID, ID: id})
	if err != nil {
		return writeVariableError(c, err, "Failed to get variable")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewGlobalVariableResponse(*view))
}

func (h *VariableHandler) CreateGlobal(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	var body dto.GlobalVariableRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}
	g, err := h.createGlobal.Handle(c.Context(), cmdvariable.CreateGlobalCommand{
		UserID: user.ID, Name: body.Name, Value: body.Value, Secret: body.Secret,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to create variable")
	}
	return c.Status(fiber.StatusCreated).JSON(presenter.NewGlobalVariableResponse(*g))
}

func (h *VariableHandler) UpdateGlobal(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	id, err := parseVariableID(c)
	if err != nil {
		return err
	}
	var body dto.GlobalVariableUpdateRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}
	g, err := h.updateGlobal.Handle(c.Context(), cmdvariable.UpdateGlobalCommand{
		UserID: user.ID, ID: id, Value: body.Value, Secret: body.Secret,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to update variable")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewGlobalVariableResponse(*g))
}

func (h *VariableHandler) DeleteGlobal(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	id, err := parseVariableID(c)
	if err != nil {
		return err
	}
	if err := h.deleteGlobal.Handle(c.Context(), cmdvariable.DeleteGlobalVariableCommand{
		UserID: user.ID, ID: id,
	}); err != nil {
		return writeVariableError(c, err, "Failed to delete variable")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Workflow statics ---

func (h *VariableHandler) ListStatics(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := parseWorkflowID(c)
	if err != nil {
		return err
	}
	views, err := h.listStatics.Handle(c.Context(), queryvariable.ListStaticsQuery{
		UserID: user.ID, WorkflowID: workflowID,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to list variables")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewStaticVariableListResponse(views))
}

func (h *VariableHandler) GetStatic(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, name, err := parseWorkflowVariableIDs(c)
	if err != nil {
		return err
	}
	view, err := h.getStatic.Handle(c.Context(), queryvariable.GetStaticQuery{
		UserID: user.ID, WorkflowID: workflowID, Name: name,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to get variable")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewStaticVariableResponse(*view))
}

func (h *VariableHandler) CreateStatic(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := parseWorkflowID(c)
	if err != nil {
		return err
	}
	var body dto.StaticVariableRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}
	s, err := h.createStatic.Handle(c.Context(), cmdvariable.CreateStaticCommand{
		UserID: user.ID, WorkflowID: workflowID, Name: body.Name, Value: body.Value, Secret: body.Secret,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to create variable")
	}
	return c.Status(fiber.StatusCreated).JSON(presenter.NewStaticVariableResponse(*s))
}

func (h *VariableHandler) UpdateStatic(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, name, err := parseWorkflowVariableIDs(c)
	if err != nil {
		return err
	}
	var body dto.StaticVariableUpdateRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}
	s, err := h.updateStatic.Handle(c.Context(), cmdvariable.UpdateStaticCommand{
		UserID: user.ID, WorkflowID: workflowID, Name: name, Value: body.Value, Secret: body.Secret,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to update variable")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewStaticVariableResponse(*s))
}

func (h *VariableHandler) DeleteStatic(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, name, err := parseWorkflowVariableIDs(c)
	if err != nil {
		return err
	}
	if err := h.deleteStatic.Handle(c.Context(), cmdvariable.DeleteStaticCommand{
		UserID: user.ID, WorkflowID: workflowID, Name: name,
	}); err != nil {
		return writeVariableError(c, err, "Failed to delete variable")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Call extractions ---

func (h *VariableHandler) ListCallVariables(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, err := parseWorkflowAndCallIDs(c)
	if err != nil {
		return err
	}
	views, err := h.listCall.Handle(c.Context(), queryvariable.ListCallVariablesQuery{
		UserID: user.ID, WorkflowID: workflowID, CallID: callID,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to list variables")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewCallVariableListResponse(views))
}

func (h *VariableHandler) GetCallVariable(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, name, err := parseCallVariableIDs(c)
	if err != nil {
		return err
	}
	view, err := h.getCall.Handle(c.Context(), queryvariable.GetCallVariableQuery{
		UserID: user.ID, WorkflowID: workflowID, CallID: callID, Name: name,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to get variable")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewCallVariableResponse(*view))
}

func (h *VariableHandler) CreateCallVariable(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, err := parseWorkflowAndCallIDs(c)
	if err != nil {
		return err
	}
	var body dto.CallVariableRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}
	ext, err := h.createCall.Handle(c.Context(), cmdvariable.CreateCallVariableCommand{
		UserID: user.ID, WorkflowID: workflowID, CallID: callID,
		Name: body.Name, Path: body.Path, Source: body.Source, Secret: body.Secret,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to create variable")
	}
	return c.Status(fiber.StatusCreated).JSON(presenter.NewCallVariableResponse(*ext))
}

func (h *VariableHandler) UpdateCallVariable(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, name, err := parseCallVariableIDs(c)
	if err != nil {
		return err
	}
	var body dto.CallVariableUpdateRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}
	ext, err := h.updateCall.Handle(c.Context(), cmdvariable.UpdateCallVariableCommand{
		UserID: user.ID, WorkflowID: workflowID, CallID: callID,
		Name: name, Path: body.Path, Source: body.Source, Secret: body.Secret,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to update variable")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewCallVariableResponse(*ext))
}

func (h *VariableHandler) DeleteCallVariable(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, name, err := parseCallVariableIDs(c)
	if err != nil {
		return err
	}
	if err := h.deleteCall.Handle(c.Context(), cmdvariable.DeleteCallVariableCommand{
		UserID: user.ID, WorkflowID: workflowID, CallID: callID, Name: name,
	}); err != nil {
		return writeVariableError(c, err, "Failed to delete variable")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListAvailable returns globals, workflow statics and previous-call variables
// usable when editing the given call.
func (h *VariableHandler) ListAvailable(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, err := parseWorkflowAndCallIDs(c)
	if err != nil {
		return err
	}
	views, err := h.listAvailable.Handle(c.Context(), queryvariable.ListAvailableQuery{
		UserID: user.ID, WorkflowID: workflowID, CallID: callID,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to list available variables")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewAvailableVariableListResponse(views))
}

// ListResponseKeys returns JSON paths from the latest run of this call, for
// path autocomplete when declaring a call variable.
func (h *VariableHandler) ListResponseKeys(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, callID, err := parseWorkflowAndCallIDs(c)
	if err != nil {
		return err
	}
	keys, err := h.listResponseKeys.Handle(c.Context(), queryvariable.ListResponseKeysQuery{
		UserID: user.ID, WorkflowID: workflowID, CallID: callID,
	})
	if err != nil {
		return writeVariableError(c, err, "Failed to list response keys")
	}
	if keys == nil {
		keys = []string{}
	}
	return c.Status(fiber.StatusOK).JSON(keys)
}
