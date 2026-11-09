package handler

import (
	"errors"
	"strconv"
	"strings"

	querycall "go-api/internal/application/query/call"
	domaincall "go-api/internal/domain/call"
	domainworkflow "go-api/internal/domain/workflow"
	httpctx "go-api/internal/interfaces/http/context"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type SuggestionHandler struct {
	suggest suggestCallFieldsHandler
}

func NewSuggestionHandler(suggest suggestCallFieldsHandler) *SuggestionHandler {
	return &SuggestionHandler{suggest: suggest}
}

func (h *SuggestionHandler) HeaderKeys(c fiber.Ctx) error {
	return h.suggestKind(c, querycall.SuggestHeaderKeys)
}

func (h *SuggestionHandler) HeaderValues(c fiber.Ctx) error {
	return h.suggestKind(c, querycall.SuggestHeaderValues)
}

func (h *SuggestionHandler) QueryKeys(c fiber.Ctx) error {
	return h.suggestKind(c, querycall.SuggestQueryKeys)
}

func (h *SuggestionHandler) QueryValues(c fiber.Ctx) error {
	return h.suggestKind(c, querycall.SuggestQueryValues)
}

func (h *SuggestionHandler) suggestKind(c fiber.Ctx, kind querycall.SuggestKind) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}
	workflowID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid workflow id"})
	}
	limit := domaincall.DefaultSuggestionLimit
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "Invalid limit"})
		}
		limit = n
	}
	items, err := h.suggest.Handle(c.Context(), querycall.SuggestFieldsQuery{
		UserID: user.ID, WorkflowID: workflowID, Kind: kind,
		Search: c.Query("q"), Limit: limit,
	})
	if err != nil {
		return writeSuggestionError(c, err)
	}
	if items == nil {
		items = []string{}
	}
	return c.Status(fiber.StatusOK).JSON(items)
}

func writeSuggestionError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domainworkflow.ErrNotFound), errors.Is(err, domainworkflow.ErrWrongOwner):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"message": "Not found"})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Failed to load suggestions",
		})
	}
}
