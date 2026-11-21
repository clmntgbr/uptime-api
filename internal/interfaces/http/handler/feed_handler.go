package handler

import (
	"errors"
	"strings"
	"time"

	queryfeed "go-api/internal/application/query/feed"
	domainfeed "go-api/internal/domain/feed"
	httpctx "go-api/internal/interfaces/http/context"
	"go-api/internal/interfaces/http/presenter"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type FeedHandler struct {
	list feedListHandler
}

func NewFeedHandler(list feedListHandler) *FeedHandler {
	return &FeedHandler{list: list}
}

func (h *FeedHandler) List(c fiber.Ctx) error {
	user, err := httpctx.GetUser(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthorized"})
	}

	var q struct {
		Cursor     string `query:"cursor"`
		Limit      int    `query:"limit"`
		WorkflowID string `query:"workflowId"`
		Kind       string `query:"kind"`
		Severity   string `query:"severity"`
		From       string `query:"from"`
		To         string `query:"to"`
	}
	if err := c.Bind().Query(&q); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid query parameters",
			"errors":  err.Error(),
		})
	}

	listQ := queryfeed.ListFeedQuery{
		UserID: user.ID,
		Cursor: q.Cursor,
		Limit:  q.Limit,
	}
	if q.WorkflowID != "" {
		wfID, err := uuid.Parse(q.WorkflowID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid workflowId",
				"errors":  map[string]string{"workflowId": "must be a valid UUID"},
			})
		}
		listQ.WorkflowID = &wfID
	}
	if q.Kind != "" {
		for _, raw := range strings.Split(q.Kind, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			listQ.Kinds = append(listQ.Kinds, domainfeed.Kind(raw))
		}
	}
	if q.Severity != "" {
		for _, raw := range strings.Split(q.Severity, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			listQ.Severities = append(listQ.Severities, domainfeed.Severity(raw))
		}
	}
	if q.From != "" {
		from, err := time.Parse(time.RFC3339, q.From)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid from",
				"errors":  map[string]string{"from": "must be RFC3339"},
			})
		}
		from = from.UTC()
		listQ.From = &from
	}
	if q.To != "" {
		to, err := time.Parse(time.RFC3339, q.To)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid to",
				"errors":  map[string]string{"to": "must be RFC3339"},
			})
		}
		to = to.UTC()
		listQ.To = &to
	}

	result, err := h.list.Handle(c.Context(), listQ)
	if err != nil {
		if errors.Is(err, queryfeed.ErrInvalidCursor) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": "Invalid cursor",
				"code":    "INVALID_CURSOR",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to list feed"})
	}
	return c.Status(fiber.StatusOK).JSON(presenter.NewFeedListResponse(result.Events, result.NextCursor))
}
