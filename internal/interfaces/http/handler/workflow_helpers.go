package handler

import (
	"errors"
	"fmt"
	"time"

	domaincall "go-api/internal/domain/call"
	domainrule "go-api/internal/domain/rule"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
)

// bindWorkflowDefinition validates the body and maps it to a domain definition.
// Rules are built here because their sentence is validated per field.
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
		Calls:    make([]domaincall.Definition, 0, len(body.Calls)),
	}
	for _, s := range body.Statics {
		def.Statics = append(def.Statics, domainworkflow.StaticVariable{Name: s.Name, Value: s.Value})
	}

	ruleErrors := map[string]string{}
	for i, call := range body.Calls {
		callDef := domaincall.Definition{
			Name:        call.Name,
			Method:      call.Method,
			URL:         call.URL,
			Headers:     call.Headers,
			Query:       call.Query,
			Body:        call.Body,
			Timeout:     time.Duration(call.TimeoutMS) * time.Millisecond,
			VerifyTLS:   call.VerifyTLS,
			IPVersion:   domaincall.IPVersion(call.IPVersion),
			ProxyURL:    call.ProxyURL,
			Extractions: make([]domaincall.Extraction, 0, len(call.Extractions)),
			Rules:       make([]domainrule.Rule, 0, len(call.Rules)),
		}
		for _, e := range call.Extractions {
			callDef.Extractions = append(callDef.Extractions, domaincall.Extraction{
				Name:       e.Name,
				Source:     domaincall.ExtractionSource(e.Source),
				Expression: e.Expression,
			})
		}
		for j, r := range call.Rules {
			built, err := domainrule.New(domainrule.Subject(r.Subject), domainrule.Operator(r.Operator), r.Target, r.Value)
			if err != nil {
				ruleErrors[fmt.Sprintf("calls[%d].rules[%d]", i, j)] = err.Error()
				continue
			}
			callDef.Rules = append(callDef.Rules, built)
		}
		def.Calls = append(def.Calls, callDef)
	}
	if len(ruleErrors) > 0 {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation failed",
			"errors":  ruleErrors,
		})
		return domainworkflow.Definition{}, validation.ErrValidationFailed
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
