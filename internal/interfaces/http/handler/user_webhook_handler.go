package handler

import (
	"encoding/json"
	"log/slog"

	usercmd "go-api/internal/application/command/user"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
)

type UserWebhookHandler struct {
	getUserByExternalIDHandler    userWebhookGetByExternalIDHandler
	createUserHandler             userWebhookCreateUserHandler
	updateUserHandler             userWebhookUpdateUserHandler
	deleteUserByExternalIDHandler userWebhookDeleteByExternalIDHandler
}

func NewUserWebhookHandler(
	getUserByExternalIDHandler userWebhookGetByExternalIDHandler,
	createUserHandler userWebhookCreateUserHandler,
	updateUserHandler userWebhookUpdateUserHandler,
	deleteUserByExternalIDHandler userWebhookDeleteByExternalIDHandler,
) *UserWebhookHandler {
	return &UserWebhookHandler{
		getUserByExternalIDHandler:    getUserByExternalIDHandler,
		createUserHandler:             createUserHandler,
		updateUserHandler:             updateUserHandler,
		deleteUserByExternalIDHandler: deleteUserByExternalIDHandler,
	}
}

func (h *UserWebhookHandler) Execute(c fiber.Ctx) error {
	clerkEvent, ok := c.Locals("payload").(dto.ClerkEvent)
	if !ok {
		// The signature middleware always sets it: missing means a wiring bug.
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	switch clerkEvent.Type {
	case "user.created":
		var data dto.ClerkUserCreated
		if err := json.Unmarshal(clerkEvent.Data, &data); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid request body",
			})
		}
		if err := validation.Struct(c, &data); err != nil {
			return err
		}
		if err := h.createUser(c, data); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to create user",
			})
		}
		return c.SendStatus(fiber.StatusCreated)

	case "user.updated":
		var data dto.ClerkUserUpdated
		if err := json.Unmarshal(clerkEvent.Data, &data); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid request body",
			})
		}
		if err := validation.Struct(c, &data); err != nil {
			return err
		}
		if err := h.updateUser(c, data); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to update user",
			})
		}
		return c.SendStatus(fiber.StatusNoContent)

	case "user.deleted":
		var data dto.ClerkUserDeleted
		if err := json.Unmarshal(clerkEvent.Data, &data); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid request body",
			})
		}
		if err := validation.Struct(c, &data); err != nil {
			return err
		}
		if err := h.deleteUser(c, data); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to delete user",
			})
		}
		return c.SendStatus(fiber.StatusNoContent)

	default:
		return c.SendStatus(fiber.StatusOK)
	}
}

func (h *UserWebhookHandler) createUser(c fiber.Ctx, data dto.ClerkUserCreated) error {
	existing, err := h.getUserByExternalIDHandler.Handle(c.Context(), data.ID)
	if err != nil {
		slog.Error("find user failed", "clerk_id", data.ID, "error", err)
		return err
	}
	if existing != nil {
		return nil
	}

	_, err = h.createUserHandler.Handle(c.Context(), usercmd.CreateUserCommand{
		ClerkID:   data.ID,
		FirstName: data.FirstName,
		LastName:  data.LastName,
		Email:     data.Email,
		Banned:    *data.Banned,
	})
	if err != nil {
		slog.Error("create user failed", "clerk_id", data.ID, "error", err)
		return err
	}
	return nil
}

func (h *UserWebhookHandler) updateUser(c fiber.Ctx, data dto.ClerkUserUpdated) error {
	user, err := h.getUserByExternalIDHandler.Handle(c.Context(), data.ID)
	if err != nil {
		slog.Error("find user failed", "clerk_id", data.ID, "error", err)
		return err
	}
	if user == nil {
		// user.created was missed or never processed. Provision the user from this
		// payload and answer 2xx: a 404 would make Svix retry the same event forever.
		_, err := h.createUserHandler.Handle(c.Context(), usercmd.CreateUserCommand{
			ClerkID:   data.ID,
			FirstName: data.FirstName,
			LastName:  data.LastName,
			Email:     data.Email,
			Banned:    *data.Banned,
		})
		if err != nil {
			slog.Error("provision user failed", "clerk_id", data.ID, "error", err)
		}
		return err
	}

	if err := h.updateUserHandler.Handle(c.Context(), usercmd.UpdateUserCommand{
		User:      user,
		FirstName: data.FirstName,
		LastName:  data.LastName,
		Email:     data.Email,
		Banned:    *data.Banned,
	}); err != nil {
		slog.Error("update user failed", "clerk_id", data.ID, "error", err)
		return err
	}
	return nil
}

func (h *UserWebhookHandler) deleteUser(c fiber.Ctx, data dto.ClerkUserDeleted) error {
	if err := h.deleteUserByExternalIDHandler.Handle(c.Context(), data.ID); err != nil {
		slog.Error("delete user failed", "clerk_id", data.ID, "error", err)
		return err
	}
	return nil
}
