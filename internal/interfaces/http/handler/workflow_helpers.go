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
// Calls are omitted: they are managed by dedicated endpoints.
func bindWorkflowDefinition(c fiber.Ctx) (domainworkflow.Definition, error) {
	var body dto.WorkflowRequest
	if err := validation.BindBody(c, &body); err != nil {
		return domainworkflow.Definition{}, err
	}

	def := domainworkflow.Definition{
		Name:     body.Name,
		Interval: time.Duration(body.IntervalSeconds) * time.Second,
		Enabled:  body.Enabled == nil || *body.Enabled,
		Statics:  make([]domainworkflow.StaticVariable, 0, len(body.Statics)),
	}
	for _, s := range body.Statics {
		def.Statics = append(def.Statics, domainworkflow.StaticVariable{Name: s.Name, Value: s.Value})
	}
	return def, nil
}

// writeWorkflowError maps domain errors; validation messages name the call and
// the variable at fault and are part of the contract.
func writeWorkflowError(c fiber.Ctx, err error, fallback string) error {
	var verr *domainworkflow.ValidationError
	switch {
	case errors.As(err, &verr):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": verr.Error(),
			"code":    "INVALID_WORKFLOW",
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
