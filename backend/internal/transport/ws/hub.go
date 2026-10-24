package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

const (
	TypePlay     = "play"
	TypePause    = "pause"
	TypeSeek     = "seek"
	TypeSnapshot = "snapshot"
	TypeError    = "error"
)

// Message is the wire format shared with the frontend (PRD 7.3).
type Message struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
	TS      int64          `json:"ts"`
}

type conn struct {
	mu sync.Mutex
	c  *websocket.Conn
}

// Send serialises writes: gorilla/websocket allows only one writer per connection.
func (c *conn) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.c.WriteMessage(websocket.TextMessage, data)
}

func (c *conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.c.Close()
}

// RuntimeSession is the in-memory playback state of one room.
type RuntimeSession struct {
	ID        string
	mu        sync.Mutex
	admin     *conn
	viewers   map[*conn]struct{}
	playing   bool
	position  float64
	updatedAt int64
}

func (s *RuntimeSession) SetState(playing bool, position float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playing = playing
	s.position = position
	s.updatedAt = time.Now().UnixMilli()
}

func (s *RuntimeSession) State() (bool, float64, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.playing, s.position, s.updatedAt
}

func (s *RuntimeSession) AttachAdmin(c *conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.admin != nil {
		s.admin.Close()
	}
	s.admin = c
}

// AttachViewer adds a viewer to the room. Many viewers may watch the same
// session, so existing connections are kept.
func (s *RuntimeSession) AttachViewer(c *conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.viewers == nil {
		s.viewers = map[*conn]struct{}{}
	}
	s.viewers[c] = struct{}{}
}

func (s *RuntimeSession) Detach(c *conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.admin == c {
		s.admin = nil
	}
	delete(s.viewers, c)
}

func (s *RuntimeSession) Broadcast(data []byte) {
	s.mu.Lock()
	admin := s.admin
	viewers := make([]*conn, 0, len(s.viewers))
	for v := range s.viewers {
		viewers = append(viewers, v)
	}
	s.mu.Unlock()

	if admin != nil {
		_ = admin.Send(data)
	}
	for _, v := range viewers {
		_ = v.Send(data)
	}
}

// Hub stores runtime state for every live room.
type Hub struct {
	mu       sync.RWMutex
	sessions map[string]*RuntimeSession
}

func NewHub() *Hub {
	return &Hub{sessions: map[string]*RuntimeSession{}}
}

func (h *Hub) GetOrCreate(id string) *RuntimeSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s, ok := h.sessions[id]; ok {
		return s
	}
	s := &RuntimeSession{ID: id, updatedAt: time.Now().UnixMilli()}
	h.sessions[id] = s
	return s
}

// Snapshot builds the state a freshly connected client needs to sync to.
func Snapshot(s *RuntimeSession) []byte {
	playing, position, updatedAt := s.State()
	msg := Message{
		Type: TypeSnapshot,
		Payload: map[string]any{
			"playing":    playing,
			"position":   position,
			"updated_at": updatedAt,
		},
		TS: time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(msg)
	return data
}
