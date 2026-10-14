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
	"github.com/google/uuid"
)

func parseWorkflowAndCallIDs(c fiber.Ctx) (workflowID, callID uuid.UUID, err error) {
	workflowID, err = uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
		return uuid.Nil, uuid.Nil, validation.ErrValidationFailed
	}
	callID, err = uuid.Parse(c.Params("callId"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid call id"})
		return uuid.Nil, uuid.Nil, validation.ErrValidationFailed
	}
	return workflowID, callID, nil
}

func parseWorkflowID(c fiber.Ctx) (uuid.UUID, error) {
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
		return uuid.Nil, validation.ErrValidationFailed
	}
	return workflowID, nil
}

func bindCallDefinition(c fiber.Ctx) (domaincall.Definition, error) {
	var body dto.CallRequest
	if err := validation.BindBody(c, &body); err != nil {
		return domaincall.Definition{}, err
	}

	def := domaincall.Definition{
		Name:        body.Name,
		Method:      body.Method,
		URL:         body.URL,
		Headers:     body.Headers,
		Query:       body.Query,
		Body:        body.Body,
		Timeout:     time.Duration(body.TimeoutMS) * time.Millisecond,
		VerifyTLS:   body.VerifyTLS,
		IPVersion:   domaincall.IPVersion(body.IPVersion),
		ProxyURL:    body.ProxyURL,
		Extractions: make([]domaincall.Extraction, 0, len(body.Extractions)),
		Rules:       make([]domainrule.Rule, 0, len(body.Rules)),
	}
	for _, e := range body.Extractions {
		def.Extractions = append(def.Extractions, domaincall.Extraction{
			Name:       e.Name,
			Source:     domaincall.ExtractionSource(e.Source),
			Expression: e.Expression,
		})
	}

	ruleErrors := map[string]string{}
	for i, r := range body.Rules {
		built, err := domainrule.New(domainrule.Subject(r.Subject), domainrule.Operator(r.Operator), r.Target, r.Value)
		if err != nil {
			ruleErrors[fmt.Sprintf("rules[%d]", i)] = err.Error()
			continue
		}
		def.Rules = append(def.Rules, built)
	}
	if len(ruleErrors) > 0 {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation failed",
			"errors":  ruleErrors,
		})
		return domaincall.Definition{}, validation.ErrValidationFailed
	}
	return def, nil
}

func writeCallError(c fiber.Ctx, err error, fallback string) error {
	var verr *domainworkflow.ValidationError
	switch {
	case errors.As(err, &verr):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": verr.Error(),
			"code":    "INVALID_WORKFLOW",
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
