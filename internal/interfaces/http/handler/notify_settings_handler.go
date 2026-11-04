package handler

import (
	"errors"

	cmdnotify "go-api/internal/application/command/notifysettings"
	querynotify "go-api/internal/application/query/notifysettings"
	domainnotify "go-api/internal/domain/notifysettings"
	domainworkflow "go-api/internal/domain/workflow"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type NotifySettingsHandler struct {
	get    getNotifySettingsHandler
	upsert upsertNotifySettingsHandler
}

func NewNotifySettingsHandler(
	get getNotifySettingsHandler,
	upsert upsertNotifySettingsHandler,
) *NotifySettingsHandler {
	return &NotifySettingsHandler{get: get, upsert: upsert}
}

func (h *NotifySettingsHandler) Get(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	s, err := h.get.Handle(c.Context(), querynotify.GetQuery{
		UserID: user.ID, WorkflowID: workflowID,
	})
	if err != nil {
		return writeNotifySettingsError(c, err, "Failed to load notification settings")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewNotificationSettingsResponse(s))
}

func (h *NotifySettingsHandler) Upsert(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	var body dto.UpsertNotificationSettingsRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}
	s, err := h.upsert.Handle(c.Context(), cmdnotify.UpsertCommand{
		UserID: user.ID, WorkflowID: workflowID,
		Emails: body.Emails, IncidentsEnabled: body.IncidentsEnabled,
		TLSEnabled: body.TLSEnabled,
	})
	if err != nil {
		return writeNotifySettingsError(c, err, "Failed to save notification settings")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewNotificationSettingsResponse(s))
}

func writeNotifySettingsError(c fiber.Ctx, err error, fallback string) error {
	switch {
	case errors.Is(err, domainnotify.ErrInvalid):
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": err.Error(),
			"code":    "INVALID_NOTIFICATION_SETTINGS",
		})
	case errors.Is(err, domainworkflow.ErrNotFound), errors.Is(err, domainworkflow.ErrWrongOwner):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Not found"})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": fallback})
	}
}
