package http

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"

	"watchparty/internal/domain"
	"watchparty/internal/service"
)

var safeSegment = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

type HLSHandler struct {
	sessions *service.SessionService
}

func NewHLSHandler(sessions *service.SessionService) *HLSHandler {
	return &HLSHandler{sessions: sessions}
}

// Serve streams the playlist or a segment to authenticated users only (PRD 18.5).
func (h *HLSHandler) Serve(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	name := c.Params("*")
	if !safeSegment.MatchString(name) || strings.Contains(name, "..") {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid segment"})
	}

	sess, err := h.sessions.Get(c.Context(), sessionID, claims(c))
	if err != nil {
		return err
	}
	if sess.VideoStatus != domain.StatusReady {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "session not ready"})
	}

	path := filepath.Join(h.sessions.HLSRoot(sessionID), name)
	if _, err := os.Stat(path); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
	}

	// The playlist may change, the segments never do.
	if filepath.Ext(name) == ".m3u8" {
		c.Set("Cache-Control", "no-cache")
	} else {
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	if err := c.SendFile(path); err != nil {
		return err
	}
	// fasthttp's file handler guesses the content type; override it for HLS.
	switch filepath.Ext(name) {
	case ".m3u8":
		c.Context().Response.Header.SetContentType("application/vnd.apple.mpegurl")
	case ".m4s":
		c.Context().Response.Header.SetContentType("video/iso.segment")
	case ".mp4":
		c.Context().Response.Header.SetContentType("video/mp4")
	}
	return nil
}
