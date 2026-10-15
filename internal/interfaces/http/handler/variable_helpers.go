package handler

import (
	"errors"

	cmdvariable "go-api/internal/application/command/variable"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func parseVariableID(c fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid variable id"})
		return uuid.Nil, validation.ErrValidationFailed
	}
	return id, nil
}

func parseVariableName(c fiber.Ctx) (string, error) {
	name := c.Params("name")
	if err := variable.ValidateName(name); err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": err.Error()})
		return "", validation.ErrValidationFailed
	}
	return name, nil
}

func parseWorkflowVariableIDs(c fiber.Ctx) (workflowID uuid.UUID, name string, err error) {
	workflowID, err = parseWorkflowID(c)
	if err != nil {
		return uuid.Nil, "", err
	}
	name, err = parseVariableName(c)
	if err != nil {
		return uuid.Nil, "", err
	}
	return workflowID, name, nil
}

func parseCallVariableIDs(c fiber.Ctx) (workflowID, callID uuid.UUID, name string, err error) {
	workflowID, callID, err = parseWorkflowAndCallIDs(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}
	name, err = parseVariableName(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}
	return workflowID, callID, name, nil
}

func writeVariableError(c fiber.Ctx, err error, fallback string) error {
	var verr *domainworkflow.ValidationError
	switch {
	case errors.As(err, &verr):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": verr.Error(),
			"code":    "INVALID_WORKFLOW",
		})
	case errors.Is(err, variable.ErrAlreadyExists):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message": "Variable already exists",
			"code":    "VARIABLE_EXISTS",
		})
	case errors.Is(err, variable.ErrNotFound), errors.Is(err, domainworkflow.ErrVariableNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Variable not found",
		})
	case errors.Is(err, cmdvariable.ErrVariableInUse), errors.Is(err, domainworkflow.ErrVariableInUse):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message": "Variable is in use",
			"code":    "VARIABLE_IN_USE",
		})
	case errors.Is(err, variable.ErrInvalidName), errors.Is(err, variable.ErrReservedName), errors.Is(err, variable.ErrValueTooLarge):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": err.Error(),
		})
	case errors.Is(err, domainworkflow.ErrCallNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Call not found",
		})
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
