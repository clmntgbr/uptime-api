package handler

import (
	"errors"
	"strconv"
	"time"

	workflowcmd "go-api/internal/application/command/workflow"
	domainexpiry "go-api/internal/domain/expiry"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
)

type boundWorkflowRequest struct {
	Definition     domainworkflow.Definition
	ExpiryPolicies []workflowcmd.ExpiryPolicyInput
}

// bindWorkflowRequest validates the body and maps it to a domain definition
// (and optional create-time expiry policies). Calls/statics use dedicated endpoints.
func bindWorkflowRequest(c fiber.Ctx) (boundWorkflowRequest, error) {
	var body dto.WorkflowRequest
	if err := validation.BindBody(c, &body); err != nil {
		return boundWorkflowRequest{}, err
	}

	scheduleType := domainworkflow.ScheduleType(body.ScheduleType)
	if scheduleType == "" {
		scheduleType = domainworkflow.ScheduleInterval
	}

	if err := validateScheduleFields(c, scheduleType, body); err != nil {
		return boundWorkflowRequest{}, err
	}

	policies, err := mapExpiryPolicies(c, body.ExpiryPolicies)
	if err != nil {
		return boundWorkflowRequest{}, err
	}

	def := domainworkflow.Definition{
		Name:         body.Name,
		ScheduleType: scheduleType,
		// Omitted enabled defaults to false (workflows start paused until they have calls).
		Enabled: body.Enabled != nil && *body.Enabled,
		RunAt:   body.RunAt,
	}
	if scheduleType == domainworkflow.ScheduleInterval && body.IntervalSeconds != nil {
		def.Interval = time.Duration(*body.IntervalSeconds) * time.Second
	}
	return boundWorkflowRequest{Definition: def, ExpiryPolicies: policies}, nil
}

func mapExpiryPolicies(c fiber.Ctx, in []dto.WorkflowExpiryPolicyInput) ([]workflowcmd.ExpiryPolicyInput, error) {
	if len(in) == 0 {
		return nil, nil
	}
	seen := map[domainexpiry.Type]struct{}{}
	out := make([]workflowcmd.ExpiryPolicyInput, 0, len(in))
	for i, p := range in {
		// Type already constrained by DTO oneof=tls_certificate.
		t := domainexpiry.Type(p.Type)
		if _, ok := seen[t]; ok {
			_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Validation failed",
				"errors":  map[string]string{"expiryPolicies": "duplicate type"},
			})
			return nil, validation.ErrValidationFailed
		}
		seen[t] = struct{}{}
		if p.Enabled && len(p.ThresholdsDays) == 0 {
			_ = c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"message": "thresholdsDays is required when enabled",
				"code":    "INVALID_THRESHOLDS",
				"errors": map[string]string{
					"expiryPolicies." + strconv.Itoa(i) + ".thresholdsDays": "required when enabled",
				},
			})
			return nil, validation.ErrValidationFailed
		}
		out = append(out, workflowcmd.ExpiryPolicyInput{
			Type: t, Enabled: p.Enabled, ThresholdsDays: p.ThresholdsDays,
		})
	}
	return out, nil
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
	case errors.Is(err, domainexpiry.ErrInvalidThresholds):
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": err.Error(),
			"code":    "INVALID_THRESHOLDS",
		})
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
