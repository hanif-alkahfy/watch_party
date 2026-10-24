package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"watchparty/internal/domain"
)

type sessionRepo struct{ db *sql.DB }

func (r *sessionRepo) Create(ctx context.Context, s *domain.Session) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (id, admin_id, video_title, duration_seconds, source_path, video_path, video_status, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, '', ?)`,
		s.ID, s.AdminID, s.VideoTitle, s.DurationSeconds, s.SourcePath, s.VideoPath, string(s.VideoStatus), now())
	return err
}

func (r *sessionRepo) Get(ctx context.Context, id string) (*domain.Session, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, admin_id, video_title, duration_seconds, source_path, video_path, video_status, error, created_at
		 FROM sessions WHERE id = ?`, id)

	var s domain.Session
	var status, created string
	err := row.Scan(&s.ID, &s.AdminID, &s.VideoTitle, &s.DurationSeconds, &s.SourcePath, &s.VideoPath, &status, &s.Error, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.VideoStatus = domain.VideoStatus(status)
	s.CreatedAt = toTime(created)
	return &s, nil
}

// ListForAdmin returns the sessions a user owns.
func (r *sessionRepo) ListForAdmin(ctx context.Context, adminID string) ([]*domain.Session, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, admin_id, video_title, duration_seconds, source_path, video_path, video_status, error, created_at
		 FROM sessions
		 WHERE admin_id = ?
		 ORDER BY created_at DESC`, adminID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*domain.Session{}
	for rows.Next() {
		var s domain.Session
		var status, created string
		if err := rows.Scan(&s.ID, &s.AdminID, &s.VideoTitle, &s.DurationSeconds, &s.SourcePath, &s.VideoPath, &status, &s.Error, &created); err != nil {
			return nil, err
		}
		s.VideoStatus = domain.VideoStatus(status)
		s.CreatedAt = toTime(created)
		out = append(out, &s)
	}
	return out, rows.Err()
}

// UpdateTitle renames a session.
func (r *sessionRepo) UpdateTitle(ctx context.Context, id, title string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE sessions SET video_title = ? WHERE id = ?`, title, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdateDuration stores the probed film duration.
func (r *sessionRepo) UpdateDuration(ctx context.Context, id string, seconds float64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE sessions SET duration_seconds = ? WHERE id = ?`, seconds, id)
	return err
}

func (r *sessionRepo) UpdateStatus(ctx context.Context, id string, status domain.VideoStatus, errMsg string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sessions SET video_status = ?, error = ? WHERE id = ?`, string(status), errMsg, id)
	return err
}

func (r *sessionRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CreateInvite stores an invite token for a session.
func (r *sessionRepo) CreateInvite(ctx context.Context, token, sessionID string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO session_invites (token, session_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		token, sessionID, now(), expiresAt.UTC().Format(timeLayout))
	return err
}

// LatestInvite returns the invite token of a session, if it has one.
func (r *sessionRepo) LatestInvite(ctx context.Context, sessionID string) (string, error) {
	var token string
	err := r.db.QueryRowContext(ctx,
		`SELECT token FROM session_invites WHERE session_id = ? ORDER BY created_at DESC LIMIT 1`, sessionID).Scan(&token)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return token, err
}

// ConsumeInvite validates a multi-use invite token against its expiration time and returns the session it unlocks.
func (r *sessionRepo) ConsumeInvite(ctx context.Context, token string) (string, error) {
	var sessionID string
	var expiresStr sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT session_id, expires_at FROM session_invites WHERE token = ?`, token).Scan(&sessionID, &expiresStr)
	if errors.Is(err, sql.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if expiresStr.Valid {
		exp := toTime(expiresStr.String)
		if !exp.IsZero() && time.Now().UTC().After(exp) {
			return "", domain.ErrInviteExpired
		}
	}
	// Multi-use: the token stays valid for any viewer until it expires.
	return sessionID, nil
}
