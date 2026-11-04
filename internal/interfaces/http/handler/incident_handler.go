package handler

import (
	"errors"
	"strings"

	cmdincident "go-api/internal/application/command/incident"
	queryincident "go-api/internal/application/query/incident"
	domainincident "go-api/internal/domain/incident"
	"go-api/internal/domain/paginate"
	domainworkflow "go-api/internal/domain/workflow"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/dto"
	"go-api/internal/interfaces/http/presenter"
	"go-api/internal/interfaces/http/validation"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type IncidentHandler struct {
	listPolicies incidentListPoliciesHandler
	upsertPolicy incidentUpsertPolicyHandler
	list         incidentListHandler
	get          incidentGetHandler
	acknowledge  incidentAcknowledgeHandler
}

func NewIncidentHandler(
	listPolicies incidentListPoliciesHandler,
	upsertPolicy incidentUpsertPolicyHandler,
	list incidentListHandler,
	get incidentGetHandler,
	acknowledge incidentAcknowledgeHandler,
) *IncidentHandler {
	return &IncidentHandler{
		listPolicies: listPolicies,
		upsertPolicy: upsertPolicy,
		list:         list,
		get:          get,
		acknowledge:  acknowledge,
	}
}

func (h *IncidentHandler) ListPolicies(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	policies, err := h.listPolicies.Handle(c.Context(), queryincident.ListPoliciesQuery{
		UserID: user.ID, WorkflowID: workflowID,
	})
	if err != nil {
		return writeIncidentError(c, err, "Failed to list incident policies")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewIncidentPoliciesResponse(policies))
}

func (h *IncidentHandler) UpsertPolicy(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	kind, err := domainincident.ParseKind(c.Params("kind"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Unknown incident kind",
			"code":    "UNKNOWN_INCIDENT_KIND",
		})
	}
	var body dto.UpsertIncidentPolicyRequest
	if err := validation.BindBody(c, &body); err != nil {
		return err
	}
	severity, err := domainincident.ParseSeverity(body.Severity)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Invalid severity",
			"code":    "INVALID_INCIDENT_POLICY",
		})
	}
	policy, err := h.upsertPolicy.Handle(c.Context(), cmdincident.UpsertPolicyCommand{
		UserID: user.ID, WorkflowID: workflowID, Kind: kind,
		Enabled: body.Enabled, Severity: severity,
		OpenAfter: body.OpenAfter, CloseAfter: body.CloseAfter, Params: body.Params,
	})
	if err != nil {
		return writeIncidentError(c, err, "Failed to upsert incident policy")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewIncidentPolicyResponse(policy))
}

func (h *IncidentHandler) List(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	var listQuery struct {
		paginate.PaginateQuery
		State    string `query:"state"`
		Severity string `query:"severity"`
	}
	if err := validation.BindAndValidateQuery(c, &listQuery); err != nil {
		return err
	}
	listQuery.Normalize()

	states, err := parseStatesCSV(listQuery.State)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid state filter",
			"code":    "INVALID_INCIDENT_FILTER",
		})
	}
	severities, err := parseSeveritiesCSV(listQuery.Severity)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid severity filter",
			"code":    "INVALID_INCIDENT_FILTER",
		})
	}

	items, total, err := h.list.Handle(c.Context(), queryincident.ListIncidentsQuery{
		UserID: user.ID, WorkflowID: workflowID,
		States: states, Severities: severities, Query: listQuery.PaginateQuery,
	})
	if err != nil {
		return writeIncidentError(c, err, "Failed to list incidents")
	}
	return c.Status(fiber.StatusOK).JSON(paginate.NewPaginateResponse(
		presenter.NewIncidentListResponse(items),
		int(total),
		listQuery.PaginateQuery,
	))
}

func (h *IncidentHandler) GetByID(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	incidentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid incident id"})
	}
	detail, err := h.get.Handle(c.Context(), queryincident.GetIncidentQuery{
		UserID: user.ID, IncidentID: incidentID,
	})
	if err != nil {
		return writeIncidentError(c, err, "Failed to get incident")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewIncidentDetailResponse(detail.Incident, detail.Events))
}

func (h *IncidentHandler) Acknowledge(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	incidentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid incident id"})
	}
	by := user.ID.String()
	inc, err := h.acknowledge.Handle(c.Context(), cmdincident.AcknowledgeCommand{
		UserID: user.ID, IncidentID: incidentID, By: by,
	})
	if err != nil {
		return writeIncidentError(c, err, "Failed to acknowledge incident")
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewIncidentDetailResponse(inc, nil))
}

func parseStatesCSV(raw string) ([]domainincident.State, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]domainincident.State, 0, len(parts))
	for _, p := range parts {
		s := domainincident.State(strings.TrimSpace(p))
		switch s {
		case domainincident.StateOpen, domainincident.StateAcknowledged, domainincident.StateResolved:
			out = append(out, s)
		default:
			return nil, domainincident.ErrInvalidPolicy
		}
	}
	return out, nil
}

func parseSeveritiesCSV(raw string) ([]domainincident.Severity, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]domainincident.Severity, 0, len(parts))
	for _, p := range parts {
		s, err := domainincident.ParseSeverity(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func writeIncidentError(c fiber.Ctx, err error, fallback string) error {
	switch {
	case errors.Is(err, domainincident.ErrUnknownKind):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Unknown incident kind",
			"code":    "UNKNOWN_INCIDENT_KIND",
		})
	case errors.Is(err, domainincident.ErrInvalidPolicy), errors.Is(err, domainincident.ErrInvalidSeverity):
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": err.Error(),
			"code":    "INVALID_INCIDENT_POLICY",
		})
	case errors.Is(err, domainincident.ErrNotFound),
		errors.Is(err, domainworkflow.ErrNotFound),
		errors.Is(err, domainworkflow.ErrWrongOwner),
		errors.Is(err, domainincident.ErrWrongOwner):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Not found"})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": fallback})
	}
}
