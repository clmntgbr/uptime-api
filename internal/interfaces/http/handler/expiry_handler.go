package handler

import (
	"errors"

	cmdexpiry "go-api/internal/application/command/expiry"
	queryexpiry "go-api/internal/application/query/expiry"
	domainexpiry "go-api/internal/domain/expiry"
	domainworkflow "go-api/internal/domain/workflow"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type ExpiryHandler struct {
	listPolicies expiryListPoliciesHandler
	upsertPolicy expiryUpsertPolicyHandler
	listItems    expiryListItemsHandler
}

func NewExpiryHandler(
	listPolicies expiryListPoliciesHandler,
	upsertPolicy expiryUpsertPolicyHandler,
	listItems expiryListItemsHandler,
) *ExpiryHandler {
	return &ExpiryHandler{
		listPolicies: listPolicies,
		upsertPolicy: upsertPolicy,
		listItems:    listItems,
	}
}

func (h *ExpiryHandler) ListPolicies(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	policies, err := h.listPolicies.Handle(c.Context(), queryexpiry.ListPoliciesQuery{
		UserID: user.ID, WorkflowID: workflowID,
	})
	if err != nil {
		return writeExpiryError(c, err, "Failed to list expiry policies")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewExpiryPoliciesResponse(policies))
}

func (h *ExpiryHandler) UpsertPolicy(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	expiryType, err := domainexpiry.ParseType(c.Params("type"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Unknown expiry type",
			"code":    "UNKNOWN_EXPIRY_TYPE",
		})
	}
	var body dto.UpsertExpiryPolicyRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}
	if body.Enabled && len(body.ThresholdsDays) == 0 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "thresholdsDays is required when enabled",
			"code":    "INVALID_THRESHOLDS",
		})
	}
	policy, err := h.upsertPolicy.Handle(c.Context(), cmdexpiry.UpsertPolicyCommand{
		UserID: user.ID, WorkflowID: workflowID, Type: expiryType,
		Enabled: body.Enabled, ThresholdsDays: body.ThresholdsDays,
	})
	if err != nil {
		return writeExpiryError(c, err, "Failed to upsert expiry policy")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewExpiryPolicyResponse(policy))
}

func (h *ExpiryHandler) ListItems(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	rawType := c.Query("type", string(domainexpiry.TypeTLSCertificate))
	expiryType, err := domainexpiry.ParseType(rawType)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Unknown expiry type",
			"code":    "UNKNOWN_EXPIRY_TYPE",
		})
	}
	items, err := h.listItems.Handle(c.Context(), queryexpiry.ListItemsQuery{
		UserID: user.ID, WorkflowID: workflowID, Type: expiryType,
	})
	if err != nil {
		return writeExpiryError(c, err, "Failed to list expiry items")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewExpiryItemsResponse(items))
}

func writeExpiryError(c fiber.Ctx, err error, fallback string) error {
	switch {
	case errors.Is(err, domainexpiry.ErrInvalidThresholds):
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": err.Error(),
			"code":    "INVALID_THRESHOLDS",
		})
	case errors.Is(err, domainexpiry.ErrUnknownType):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Unknown expiry type",
			"code":    "UNKNOWN_EXPIRY_TYPE",
		})
	case errors.Is(err, domainworkflow.ErrNotFound), errors.Is(err, domainworkflow.ErrWrongOwner),
		errors.Is(err, domainexpiry.ErrNotFound), errors.Is(err, domainexpiry.ErrWrongOwner):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Workflow not found"})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": fallback})
	}
}
