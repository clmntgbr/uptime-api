package middleware

import (
	"errors"
	"log/slog"
	"strings"

	authcmd "go-api/internal/application/command/auth"
	identitycmd "go-api/internal/application/command/identity"
	usercmd "go-api/internal/application/command/user"
	httpctx "go-api/internal/interfaces/http/context"

	"github.com/gofiber/fiber/v3"
)

type AuthenticateMiddleware struct {
	validateTokenHandler *authcmd.ValidateTokenHandler
	fetchUserHandler     *identitycmd.FetchUserHandler
	createUserHandler    *usercmd.CreateUserHandler
}

func NewAuthenticateMiddleware(
	validateTokenHandler *authcmd.ValidateTokenHandler,
	fetchUserHandler *identitycmd.FetchUserHandler,
	createUserHandler *usercmd.CreateUserHandler,
) *AuthenticateMiddleware {
	return &AuthenticateMiddleware{
		validateTokenHandler: validateTokenHandler,
		fetchUserHandler:     fetchUserHandler,
		createUserHandler:    createUserHandler,
	}
}

func (m *AuthenticateMiddleware) Protected() fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Missing authorization header",
			})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Invalid authorization header format",
			})
		}

		if parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Authorization scheme must be Bearer",
			})
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Token cannot be empty",
			})
		}

		output, err := m.validateTokenHandler.Handle(c.Context(), authcmd.ValidateTokenInput{
			Token: tokenString,
		})
		if err != nil {
			if errors.Is(err, authcmd.ErrInvalidToken) {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"message": "Invalid token",
				})
			}
			return unavailable(c, "validate token", err)
		}

		if output.User == nil {
			clerkID := output.Claims.UserID
			if clerkID == "" {
				clerkID = output.Claims.Subject
			}

			clerkUser, err := m.fetchUserHandler.Handle(c.Context(), clerkID)
			if err != nil {
				return unavailable(c, "fetch clerk user", err)
			}

			user, err := m.createUserHandler.Handle(c.Context(), usercmd.CreateUserCommand{
				ClerkID:   clerkID,
				FirstName: clerkUser.FirstName,
				LastName:  clerkUser.LastName,
				Email:     clerkUser.Email,
				Banned:    clerkUser.Banned,
			})
			if err != nil {
				return unavailable(c, "create user", err)
			}

			output.User = user
		}

		if output.User.Banned {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "User is banned",
			})
		}

		httpctx.SetUser(c, *output.User)

		return c.Next()
	}
}

// unavailable reports an upstream or database failure. It is a 503, not a 401:
// a 401 would make the client drop its session on a transient outage.
func unavailable(c fiber.Ctx, step string, err error) error {
	slog.Error("authentication failed", "step", step, "error", err)
	return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
		"message": "Authentication temporarily unavailable",
	})
}
