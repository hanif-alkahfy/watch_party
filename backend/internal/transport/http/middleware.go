package http

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"watchparty/internal/domain"
	"watchparty/internal/service"
)

const (
	localUserID = "user_id"
	localRole   = "role"
	localClaims = "claims"
)

// requireAuth validates the bearer token and exposes the caller identity on the context.
func requireAuth(auth *service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		if header == "" {
			return unauthorized(c, "missing token")
		}
		raw := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		claims, err := auth.ParseToken(raw)
		if err != nil {
			return unauthorized(c, "invalid token")
		}
		c.Locals(localClaims, claims)
		c.Locals(localUserID, claims.UserID)
		c.Locals(localRole, string(claims.Role))
		return c.Next()
	}
}

func requireRole(role domain.Role) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if current, _ := c.Locals(localRole).(string); current != string(role) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden"})
		}
		return c.Next()
	}
}

func callerID(c *fiber.Ctx) string {
	id, _ := c.Locals(localUserID).(string)
	return id
}

// claims returns the identity of the caller, which may be a session-scoped viewer token.
func claims(c *fiber.Ctx) *service.Claims {
	cl, _ := c.Locals(localClaims).(*service.Claims)
	return cl
}

func unauthorized(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": msg})
}

// errorHandler maps domain errors onto HTTP status codes.
func errorHandler(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	msg := "internal error"

	var fe *fiber.Error
	if errors.As(err, &fe) {
		status = fe.Code
		msg = fe.Message
	}

	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		status, msg = fiber.StatusBadRequest, "invalid input"
	case errors.Is(err, domain.ErrInvalidCreds):
		status, msg = fiber.StatusUnauthorized, "invalid credentials"
	case errors.Is(err, domain.ErrEmailTaken):
		status, msg = fiber.StatusConflict, "email already registered"
	case errors.Is(err, domain.ErrForbidden):
		status, msg = fiber.StatusForbidden, "forbidden"
	case errors.Is(err, domain.ErrNotFound):
		status, msg = fiber.StatusNotFound, "not found"
	case errors.Is(err, domain.ErrSessionNotReady):
		status, msg = fiber.StatusConflict, "session not ready"
	case errors.Is(err, domain.ErrInviteExpired):
		status, msg = fiber.StatusGone, "invite link has expired"
	}

	return c.Status(status).JSON(fiber.Map{"error": msg})
}
