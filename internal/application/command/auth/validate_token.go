package auth

import (
	"context"
	"errors"
	"fmt"

	"go-api/internal/domain/port"
	domainuser "go-api/internal/domain/user"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken means the caller's token is bad (client error, 401).
// Any other error from ValidateTokenHandler is a server-side failure (503).
var ErrInvalidToken = errors.New("invalid token")

type ValidateTokenHandler struct {
	tokenKeys port.TokenKeyProvider
	userRepo  domainuser.UserWriteRepository
}

func NewValidateTokenHandler(
	tokenKeys port.TokenKeyProvider,
	userRepo domainuser.UserWriteRepository,
) *ValidateTokenHandler {
	return &ValidateTokenHandler{
		tokenKeys: tokenKeys,
		userRepo:  userRepo,
	}
}

func (uc *ValidateTokenHandler) Handle(ctx context.Context, input ValidateTokenInput) (*ValidateTokenOutput, error) {
	token, err := jwt.ParseWithClaims(
		input.Token,
		&JWTClaims{},
		uc.tokenKeys.GetKeyfunc(),
		jwt.WithIssuer(uc.tokenKeys.GetIssuer()),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	clerkID := claims.UserID
	if clerkID == "" {
		clerkID = claims.Subject
	}

	// A nil user without error means "not created yet": the caller provisions it.
	user, err := uc.userRepo.GetByClerkID(ctx, clerkID)
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}

	return &ValidateTokenOutput{
		User:   user,
		Claims: claims,
	}, nil
}
