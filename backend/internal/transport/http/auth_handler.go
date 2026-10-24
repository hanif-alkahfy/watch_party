package http

import (
	"github.com/gofiber/fiber/v2"

	"watchparty/internal/service"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

type loginRequest struct {
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid body"})
	}

	token, user, err := h.auth.Login(c.Context(), req.Password)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"token": token,
		"user": fiber.Map{
			"user_id": user.ID,
			"email":   user.Email,
			"role":    user.Role,
		},
	})
}

// Logout is a no-op on the server: tokens are stateless and cleared client side.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"message": "ok"})
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"user_id": callerID(c),
		"role":    c.Locals(localRole),
	})
}
