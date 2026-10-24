package http

import (
	"strings"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"watchparty/internal/domain"
	"watchparty/internal/service"
	"watchparty/internal/transport/ws"
)

type Deps struct {
	Auth             *service.AuthService
	Sessions         *service.SessionService
	WS               *ws.Handler
	AllowedOrigins   []string
	MaxUploadBytes   int64
	PublicBaseURL    string
	// WebRoot is the built frontend to serve. Empty means API only.
	WebRoot string
}

func New(d Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "watchparty",
		BodyLimit:    int(d.MaxUploadBytes),
		ErrorHandler: errorHandler,
	})

	origins := d.AllowedOrigins
	if d.PublicBaseURL != "" {
		// The tunnel origin sends POST/WS requests with its own Origin header.
		origins = append(origins, d.PublicBaseURL)
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: strings.Join(origins, ","),
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PATCH, DELETE, OPTIONS",
	}))

	// Gzip/brotli the text payloads (the built JS bundle is the big one).
	// The websocket upgrade is skipped so the connection is not wrapped.
	app.Use(compress.New(compress.Config{
		Next: func(c *fiber.Ctx) bool { return strings.HasPrefix(c.Path(), "/ws/") },
	}))

	authHandler := NewAuthHandler(d.Auth)
	sessionHandler := NewSessionHandler(d.Sessions, d.Auth, d.PublicBaseURL)
	hlsHandler := NewHLSHandler(d.Sessions)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api")
	api.Post("/auth/login", authHandler.Login)
	api.Post("/auth/logout", authHandler.Logout)
	// Viewers have no account: an invite link is exchanged for a token.
	api.Post("/invite/exchange", sessionHandler.Exchange)

	secure := api.Group("", requireAuth(d.Auth))
	secure.Get("/me", authHandler.Me)
	secure.Get("/sessions", sessionHandler.List)
	secure.Get("/session/:id", sessionHandler.Get)
	secure.Patch("/session/:id", sessionHandler.UpdateTitle)
	secure.Post("/session/:id/invite", sessionHandler.InviteLink)
	secure.Get("/session/:id/invite", sessionHandler.CurrentInvite)
	secure.Delete("/session/:id", sessionHandler.Delete)
	secure.Post("/upload", requireRole(domain.RoleAdmin), sessionHandler.Upload)

	app.Get("/hls/:id/*", requireAuth(d.Auth), hlsHandler.Serve)
	app.Get("/ws/:id", websocket.New(d.WS.Handle))

	// The built frontend. Registered last so /api, /hls and /ws keep priority.
	if d.WebRoot != "" {
		app.Use(spaHandler(d.WebRoot))
	}

	return app
}
