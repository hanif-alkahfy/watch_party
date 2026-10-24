package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"watchparty/internal/domain"
	"watchparty/internal/repository"
)

func nowUTC() time.Time { return time.Now().UTC() }

type SessionService struct {
	sessions   repository.SessionRepository
	users      repository.UserRepository
	mediaRoot  string
	transcoder *Transcoder
}

func NewSessionService(sessions repository.SessionRepository, users repository.UserRepository, mediaRoot string, t *Transcoder) *SessionService {
	return &SessionService{sessions: sessions, users: users, mediaRoot: mediaRoot, transcoder: t}
}

// CreateUpload stores the uploaded file, registers the session and starts transcoding.
func (s *SessionService) CreateUpload(ctx context.Context, adminID, title string, src io.Reader) (*domain.Session, error) {
	id := uuid.NewString()

	relSource := filepath.Join("source", id+".mp4")
	absSource := filepath.Join(s.mediaRoot, relSource)
	if err := os.MkdirAll(filepath.Dir(absSource), 0o755); err != nil {
		return nil, err
	}

	dst, err := os.Create(absSource)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(absSource)
		return nil, err
	}
	if err := dst.Close(); err != nil {
		return nil, err
	}

	relPlaylist := filepath.Join("hls", id, "master.m3u8")
	sess := &domain.Session{
		ID:          id,
		AdminID:     adminID,
		VideoTitle:  title,
		SourcePath:  filepath.ToSlash(relSource),
		VideoPath:   filepath.ToSlash(relPlaylist),
		VideoStatus: domain.StatusUploading,
		CreatedAt:   nowUTC(),
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		os.Remove(absSource)
		return nil, err
	}

	go s.runTranscode(id, absSource, filepath.Join(s.mediaRoot, "hls", id))
	return sess, nil
}

func (s *SessionService) runTranscode(id, sourcePath, outDir string) {
	ctx := context.Background()
	_ = s.sessions.UpdateStatus(ctx, id, domain.StatusProcessing, "")

	// Probe the film duration so the session list can show it.
	if seconds, err := s.transcoder.ProbeDuration(ctx, sourcePath); err == nil {
		_ = s.sessions.UpdateDuration(ctx, id, seconds)
	}

	if err := s.transcoder.Transcode(ctx, sourcePath, outDir); err != nil {
		_ = s.sessions.UpdateStatus(ctx, id, domain.StatusFailed, err.Error())
		return
	}
	_ = s.sessions.UpdateStatus(ctx, id, domain.StatusReady, "")
}

// UpdateTitle renames a session owned by the caller.
func (s *SessionService) UpdateTitle(ctx context.Context, id, title string, claims *Claims) (*domain.Session, error) {
	sess, err := s.sessions.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if claims == nil || sess.AdminID != claims.UserID {
		return nil, domain.ErrForbidden
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, domain.ErrInvalidInput
	}
	if len(title) > 200 {
		return nil, domain.ErrInvalidInput
	}
	if err := s.sessions.UpdateTitle(ctx, id, title); err != nil {
		return nil, err
	}
	return s.sessions.Get(ctx, id)
}

func (s *SessionService) Get(ctx context.Context, id string, claims *Claims) (*domain.Session, error) {
	sess, err := s.sessions.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canView(sess, claims) {
		return nil, domain.ErrForbidden
	}
	return sess, nil
}

// canView grants access to the session owner and to viewers holding a token
// minted for that exact session.
func canView(sess *domain.Session, claims *Claims) bool {
	if claims == nil {
		return false
	}
	if claims.IsViewer() {
		return claims.SessionID == sess.ID
	}
	return sess.AdminID == claims.UserID
}

func (s *SessionService) List(ctx context.Context, claims *Claims) ([]*domain.Session, error) {
	if claims == nil || claims.IsViewer() {
		return nil, nil
	}
	return s.sessions.ListForAdmin(ctx, claims.UserID)
}

// InviteTTL is how long an invite link stays valid. The link is multi-use:
// every viewer can redeem it until it expires.
const InviteTTL = 30 * 24 * time.Hour

// CreateInvite mints a new invite link for a session.
func (s *SessionService) CreateInvite(ctx context.Context, id string, claims *Claims) (string, error) {
	sess, err := s.sessions.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if claims == nil || sess.AdminID != claims.UserID {
		return "", domain.ErrForbidden
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	expiresAt := time.Now().UTC().Add(InviteTTL)
	if err := s.sessions.CreateInvite(ctx, token, id, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

// CurrentInvite returns the session's invite link without changing it, so a
// reload keeps the same link. A new link is minted when the session has none.
func (s *SessionService) CurrentInvite(ctx context.Context, id string, claims *Claims) (string, error) {
	sess, err := s.sessions.Get(ctx, id)
	if err != nil {
		return "", err
	}
	if claims == nil || sess.AdminID != claims.UserID {
		return "", domain.ErrForbidden
	}
	token, err := s.sessions.LatestInvite(ctx, id)
	if err == nil {
		// An expired leftover link is replaced by a fresh one.
		if _, err := s.sessions.ConsumeInvite(ctx, token); err == nil {
			return token, nil
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return "", err
	}
	return s.CreateInvite(ctx, id, claims)
}

// ConsumeInvite validates a multi-use invite link and returns the session it unlocks if not expired.
func (s *SessionService) ConsumeInvite(ctx context.Context, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", domain.ErrInvalidInput
	}
	return s.sessions.ConsumeInvite(ctx, token)
}

func (s *SessionService) Delete(ctx context.Context, id, callerID string) error {
	sess, err := s.sessions.Get(ctx, id)
	if err != nil {
		return err
	}
	if sess.AdminID != callerID {
		return domain.ErrForbidden
	}
	if err := s.sessions.Delete(ctx, id); err != nil {
		return err
	}
	os.Remove(filepath.Join(s.mediaRoot, sess.SourcePath))
	os.RemoveAll(filepath.Join(s.mediaRoot, "hls", id))
	return nil
}

// PlaylistPath resolves the on-disk HLS playlist for a session id.
func (s *SessionService) PlaylistPath(id string) string {
	return filepath.Join(s.mediaRoot, "hls", id, "master.m3u8")
}

// HLSRoot resolves the directory holding the playlist and its segments.
func (s *SessionService) HLSRoot(id string) string {
	return filepath.Join(s.mediaRoot, "hls", id)
}
