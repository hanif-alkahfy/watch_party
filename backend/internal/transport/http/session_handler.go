package http

import (
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"

	"watchparty/internal/domain"
	"watchparty/internal/service"
)

type SessionHandler struct {
	sessions *service.SessionService
	auth     *service.AuthService
	// publicBaseURL is the externally reachable origin (tunnel) used in invite links.
	publicBaseURL string
}

func NewSessionHandler(sessions *service.SessionService, auth *service.AuthService, publicBaseURL string) *SessionHandler {
	return &SessionHandler{sessions: sessions, auth: auth, publicBaseURL: publicBaseURL}
}

// publicBase returns the origin viewers should use to open an invite link.
// A request that came in on a private LAN address keeps that address, so a
// phone on the same network can open the link; anything else uses the tunnel.
func (h *SessionHandler) publicBase(c *fiber.Ctx) string {
	if host := c.Hostname(); isPrivateHost(host) {
		return strings.TrimRight(c.BaseURL(), "/")
	}
	if h.publicBaseURL != "" {
		return h.publicBaseURL
	}
	// No tunnel: fall back to this machine's LAN address so a phone on the
	// same network can still open the link.
	if lan := lanOrigin(c); lan != "" {
		return lan
	}
	if origin := c.Get("Origin"); origin != "" {
		return strings.TrimRight(origin, "/")
	}
	return strings.TrimRight(c.BaseURL(), "/")
}

// isPrivateHost reports whether host is an address on a private network. The
// host may carry a port ("192.168.1.6:3001").
func isPrivateHost(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsPrivate()
}

// lanOrigin returns this machine's LAN origin, keeping the request port.
func lanOrigin(c *fiber.Ctx) string {
	ip := firstPrivateIPv4()
	if ip == "" {
		return ""
	}
	u, err := url.Parse(c.BaseURL())
	if err != nil {
		return ""
	}
	u.Host = net.JoinHostPort(ip, u.Port())
	return strings.TrimRight(u.String(), "/")
}

// firstPrivateIPv4 returns this machine's address on the local network. The
// UDP dial picks the interface of the default route, so virtual adapters do
// not win; nothing is actually sent.
func firstPrivateIPv4() string {
	if conn, err := net.Dial("udp4", "8.8.8.8:80"); err == nil {
		defer conn.Close()
		if ua, ok := conn.LocalAddr().(*net.UDPAddr); ok && ua.IP.To4() != nil {
			return ua.IP.String()
		}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.To4() == nil || !ip.IsPrivate() {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

func sessionJSON(s *domain.Session) fiber.Map {
	return fiber.Map{
		"session_id":       s.ID,
		"admin_id":         s.AdminID,
		"video_title":      s.VideoTitle,
		"duration_seconds": s.DurationSeconds,
		"video_status":     s.VideoStatus,
		"error":            s.Error,
		"created_at":       s.CreatedAt,
		"hls_url":          "/hls/" + s.ID + "/master.m3u8",
	}
}

func (h *SessionHandler) Upload(c *fiber.Ctx) error {
	file, err := c.FormFile("video")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing video file"})
	}
	if strings.ToLower(filepath.Ext(file.Filename)) != ".mp4" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "only .mp4 files are supported"})
	}

	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot read upload"})
	}
	defer src.Close()

	sess, err := h.sessions.CreateUpload(c.Context(), callerID(c), file.Filename, src)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(sessionJSON(sess))
}

func (h *SessionHandler) List(c *fiber.Ctx) error {
	list, err := h.sessions.List(c.Context(), claims(c))
	if err != nil {
		return err
	}
	out := make([]fiber.Map, 0, len(list))
	for _, s := range list {
		out = append(out, sessionJSON(s))
	}
	return c.JSON(fiber.Map{"sessions": out})
}

func (h *SessionHandler) Get(c *fiber.Ctx) error {
	sess, err := h.sessions.Get(c.Context(), c.Params("id"), claims(c))
	if err != nil {
		return err
	}
	return c.JSON(sessionJSON(sess))
}

func (h *SessionHandler) Delete(c *fiber.Ctx) error {
	if err := h.sessions.Delete(c.Context(), c.Params("id"), callerID(c)); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"message": "deleted"})
}

type updateTitleRequest struct {
	Title string `json:"title"`
}

// UpdateTitle renames a session.
func (h *SessionHandler) UpdateTitle(c *fiber.Ctx) error {
	var req updateTitleRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.ErrInvalidInput
	}
	sess, err := h.sessions.UpdateTitle(c.Context(), c.Params("id"), req.Title, claims(c))
	if err != nil {
		return err
	}
	return c.JSON(sessionJSON(sess))
}

// InviteLink mints the invite link of a session. The link is multi-use and
// stays valid until a new one is generated.
func (h *SessionHandler) InviteLink(c *fiber.Ctx) error {
	token, err := h.sessions.CreateInvite(c.Context(), c.Params("id"), claims(c))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"token": token,
		"url":   h.publicBase(c) + "/watch?invite=" + token,
	})
}

// CurrentInvite returns the session's invite link without changing it, so a
// page reload keeps the same link.
func (h *SessionHandler) CurrentInvite(c *fiber.Ctx) error {
	token, err := h.sessions.CurrentInvite(c.Context(), c.Params("id"), claims(c))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"token": token,
		"url":   h.publicBase(c) + "/watch?invite=" + token,
	})
}

type exchangeRequest struct {
	Token string `json:"token"`
}

// Exchange redeems a multi-use invite link for a session-scoped viewer token.
// Viewers need no account, so this endpoint is unauthenticated.
func (h *SessionHandler) Exchange(c *fiber.Ctx) error {
	var req exchangeRequest
	if err := c.BodyParser(&req); err != nil {
		return domain.ErrInvalidInput
	}
	sessionID, err := h.sessions.ConsumeInvite(c.Context(), req.Token)
	if err != nil {
		return err
	}
	token, err := h.auth.IssueViewerToken(sessionID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"token": token, "session_id": sessionID})
}
