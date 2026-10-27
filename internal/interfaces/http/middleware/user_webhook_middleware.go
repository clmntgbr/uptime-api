package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go-api/internal/interfaces/http/dto"

	"github.com/gofiber/fiber/v3"
	svix "github.com/svix/svix-webhooks/go"
)

type UserWebhookMiddleware struct {
	webhook *svix.Webhook
}

// NewUserWebhookMiddleware fails on an invalid secret so the API refuses to start
// instead of failing every webhook request.
func NewUserWebhookMiddleware(secret string) (*UserWebhookMiddleware, error) {
	wh, err := svix.NewWebhook(secret)
	if err != nil {
		return nil, fmt.Errorf("invalid clerk webhook secret: %w", err)
	}
	return &UserWebhookMiddleware{webhook: wh}, nil
}

func (m *UserWebhookMiddleware) Protected() fiber.Handler {
	return func(c fiber.Ctx) error {
		payload := c.Body()

		headers := http.Header{}
		headers.Set("svix-id", c.Get("svix-id"))
		headers.Set("svix-timestamp", c.Get("svix-timestamp"))
		headers.Set("svix-signature", c.Get("svix-signature"))

		if err := m.webhook.Verify(payload, headers); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "invalid signature",
			})
		}

		var clerkEvent dto.ClerkEvent
		if err := json.Unmarshal(payload, &clerkEvent); err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "invalid payload",
			})
		}

		c.Locals("payload", clerkEvent)

		return c.Next()
	}
}
