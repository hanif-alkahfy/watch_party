package ws

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/contrib/websocket"

	"watchparty/internal/domain"
	"watchparty/internal/service"
)

type Handler struct {
	auth     *service.AuthService
	sessions *service.SessionService
	hub      *Hub
}

func NewHandler(auth *service.AuthService, sessions *service.SessionService, hub *Hub) *Handler {
	return &Handler{auth: auth, sessions: sessions, hub: hub}
}

func errorPayload(message string) []byte {
	data, _ := json.Marshal(Message{
		Type:    TypeError,
		Payload: map[string]any{"error": message},
		TS:      time.Now().UnixMilli(),
	})
	return data
}

// fatalPayload marks errors the client cannot fix by retrying (bad token, no access).
func fatalPayload(message string) []byte {
	data, _ := json.Marshal(Message{
		Type:    TypeError,
		Payload: map[string]any{"error": message, "fatal": true},
		TS:      time.Now().UnixMilli(),
	})
	return data
}

func (h *Handler) Handle(c *websocket.Conn) {
	claims, err := h.auth.ParseToken(c.Query("token"))
	if err != nil {
		_ = c.WriteMessage(websocket.TextMessage, fatalPayload("unauthorized"))
		c.Close()
		return
	}

	sessionID := c.Params("id")
	sess, err := h.sessions.Get(context.Background(), sessionID, claims)
	if err != nil {
		_ = c.WriteMessage(websocket.TextMessage, fatalPayload("access denied"))
		c.Close()
		return
	}

	rt := h.hub.GetOrCreate(sessionID)
	wrapped := &conn{c: c}
	if sess.AdminID == claims.UserID {
		rt.AttachAdmin(wrapped)
	} else {
		rt.AttachViewer(wrapped)
	}
	defer rt.Detach(wrapped)
	defer wrapped.Close()

	_ = wrapped.Send(Snapshot(rt))

	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		h.handleMessage(rt, sess, claims, wrapped, data)
	}
}

func (h *Handler) handleMessage(rt *RuntimeSession, sess *domain.Session, claims *service.Claims, sender *conn, data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch msg.Type {
	case TypePlay, TypePause, TypeSeek:
		// Only the session owner may drive playback (PRD 8.5).
		if sess.AdminID != claims.UserID {
			_ = sender.Send(errorPayload("forbidden"))
			return
		}

		playing, position, _ := rt.State()
		if v, ok := msg.Payload["position"].(float64); ok {
			position = v
		}

		switch msg.Type {
		case TypePlay:
			playing = true
		case TypePause:
			playing = false
		}
		rt.SetState(playing, position)

		out, _ := json.Marshal(Message{
			Type:    msg.Type,
			Payload: map[string]any{"playing": playing, "position": position},
			TS:      time.Now().UnixMilli(),
		})
		rt.Broadcast(out)
	}
}
