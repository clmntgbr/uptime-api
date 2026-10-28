package handler

import (
	"errors"
	"time"

	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
)

// bindWorkflowDefinition validates the body and maps it to a domain definition.
// Calls and statics are omitted: they are managed by dedicated endpoints.
func bindWorkflowDefinition(c fiber.Ctx) (domainworkflow.Definition, error) {
	var body dto.WorkflowRequest
	if err := validation.BindBody(c, &body); err != nil {
		return domainworkflow.Definition{}, err
	}

	scheduleType := domainworkflow.ScheduleType(body.ScheduleType)
	if scheduleType == "" {
		scheduleType = domainworkflow.ScheduleInterval
	}

	if err := validateScheduleFields(c, scheduleType, body); err != nil {
		return domainworkflow.Definition{}, err
	}

	def := domainworkflow.Definition{
		Name:         body.Name,
		ScheduleType: scheduleType,
		Enabled:      body.Enabled == nil || *body.Enabled,
		RunAt:        body.RunAt,
	}
	if scheduleType == domainworkflow.ScheduleInterval && body.IntervalSeconds != nil {
		def.Interval = time.Duration(*body.IntervalSeconds) * time.Second
	}
	return def, nil
}

func validateScheduleFields(c fiber.Ctx, scheduleType domainworkflow.ScheduleType, body dto.WorkflowRequest) error {
	errorsMap := map[string]string{}
	switch scheduleType {
	case domainworkflow.ScheduleInterval:
		if body.IntervalSeconds == nil {
			errorsMap["intervalSeconds"] = "required for schedule type interval"
		}
		if body.RunAt != nil {
			errorsMap["runAt"] = "must be empty for schedule type interval"
		}
	case domainworkflow.ScheduleOnce:
		if body.IntervalSeconds != nil {
			errorsMap["intervalSeconds"] = "must be empty for schedule type once"
		}
		if body.RunAt != nil {
			errorsMap["runAt"] = "must be empty for schedule type once"
		}
	case domainworkflow.ScheduleAt:
		if body.IntervalSeconds != nil {
			errorsMap["intervalSeconds"] = "must be empty for schedule type at"
		}
		if body.RunAt == nil {
			errorsMap["runAt"] = "required for schedule type at"
		}
	}
	if len(errorsMap) == 0 {
		return nil
	}
	_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"message": "Validation failed",
		"errors":  errorsMap,
	})
	return validation.ErrValidationFailed
}

// validationErrorBody maps a domain ValidationError to the HTTP envelope.
// ErrVarOutOfOrder adds a structured data payload for the client.
func validationErrorBody(verr *domainworkflow.ValidationError) fiber.Map {
	body := fiber.Map{
		"message": verr.Error(),
		"code":    "INVALID_WORKFLOW",
	}
	if errors.Is(verr, domainworkflow.ErrVarOutOfOrder) {
		body["code"] = "VAR_OUT_OF_ORDER"
		body["data"] = fiber.Map{
			"call":         verr.Call,
			"variable":     verr.Variable,
			"conflictCall": verr.ConflictCall,
			"reason":       verr.Reason,
		}
	}
	return body
}

// writeWorkflowError maps domain errors; validation messages name the call and
// the variable at fault and are part of the contract.
func writeWorkflowError(c fiber.Ctx, err error, fallback string) error {
	var verr *domainworkflow.ValidationError
	switch {
	case errors.As(err, &verr):
		return c.Status(fiber.StatusBadRequest).JSON(validationErrorBody(verr))
	case errors.Is(err, domainworkflow.ErrNotFound), errors.Is(err, domainworkflow.ErrWrongOwner):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Workflow not found",
		})
	case errors.Is(err, domainworkflow.ErrRunInProgress):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message": "A run is already in progress",
			"code":    "RUN_IN_PROGRESS",
		})
	case errors.Is(err, domainworkflow.ErrNoRunInProgress):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"message": "No run is in progress",
			"code":    "NO_RUN_IN_PROGRESS",
		})
	case errors.Is(err, domainworkflow.ErrNoCalls):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Workflow has no calls",
			"code":    "NO_CALLS",
		})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": fallback,
		})
	}
}
