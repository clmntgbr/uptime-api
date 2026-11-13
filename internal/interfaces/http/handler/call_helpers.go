package handler

import (
	"errors"
	"fmt"
	"time"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
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

func bindReorderCallIDs(c fiber.Ctx) ([]uuid.UUID, error) {
	var body dto.ReorderCallsRequest
	if err := validation.BindBody(c, &body); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(body.CallIDs))
	for _, raw := range body.CallIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Validation failed",
				"errors":  map[string]string{"callIds": "must be valid UUIDs"},
			})
			return nil, validation.ErrValidationFailed
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func bindCallDefinition(c fiber.Ctx) (domaincall.Definition, error) {
	var body dto.CallRequest
	if err := validation.BindBody(c, &body); err != nil {
		return domaincall.Definition{}, err
	}

	def, ruleErrors := callDefinitionFromRequest(body)
	if len(ruleErrors) > 0 {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Validation failed",
			"errors":  ruleErrors,
		})
		return domaincall.Definition{}, validation.ErrValidationFailed
	}
	return def, nil
}

// callDefinitionFromRequest maps a validated CallRequest to a call definition.
// It returns the rule errors keyed by rules[i] when a rule cannot be built.
func callDefinitionFromRequest(body dto.CallRequest) (domaincall.Definition, map[string]string) {
	def := domaincall.Definition{
		Name:        body.Name,
		Description: body.Description,
		Method:      body.Method,
		URL:         body.URL,
		Headers:     body.Headers,
		Query:       body.Query,
		Body:        body.Body,
		Timeout:     time.Duration(body.TimeoutMS) * time.Millisecond,
		VerifyTLS:   body.VerifyTLS,
		IPVersion:   domaincall.IPVersion(body.IPVersion),
		ProxyURL:    body.ProxyURL,
		Rules:       make([]domainrule.Rule, 0, len(body.Rules)),
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
		return domaincall.Definition{}, ruleErrors
	}
	return def, nil
}

func parseWorkflowAndLibraryItemIDs(c fiber.Ctx) (workflowID, itemID uuid.UUID, err error) {
	workflowID, err = uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
		return uuid.Nil, uuid.Nil, validation.ErrValidationFailed
	}
	itemID, err = uuid.Parse(c.Params("itemId"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid library item id"})
		return uuid.Nil, uuid.Nil, validation.ErrValidationFailed
	}
	return workflowID, itemID, nil
}

// bindInsertPosition reads the optional body of the from-library insert.
// An empty body means "append at the end" and returns 0.
func bindInsertPosition(c fiber.Ctx) (int, error) {
	if len(c.Body()) == 0 {
		return 0, nil
	}
	var body dto.InsertFromLibraryRequest
	if err := validation.BindBody(c, &body); err != nil {
		return 0, err
	}
	if body.Position == nil {
		return 0, nil
	}
	return *body.Position, nil
}

func writeCallError(c fiber.Ctx, err error, fallback string) error {
	var verr *domainworkflow.ValidationError
	var cverr *domaincall.ValidationError
	switch {
	case errors.Is(err, domainlibrary.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Library item not found",
		})
	case errors.As(err, &cverr):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": cverr.Error(),
			"code":    "INVALID_CALL",
		})
	case errors.Is(err, domainworkflow.ErrInvalidCallOrder):
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "callIds must list every call exactly once",
			"code":    "INVALID_CALL_ORDER",
		})
	case errors.As(err, &verr):
		return c.Status(fiber.StatusBadRequest).JSON(validationErrorBody(verr))
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
