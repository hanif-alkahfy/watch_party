package repository

import (
	"context"
	"time"

	"watchparty/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	UpsertAdmin(ctx context.Context, u *domain.User) error
}

func (s *SQLite) Users() UserRepository { return &userRepo{db: s.db} }

func (s *SQLite) Sessions() SessionRepository { return &sessionRepo{db: s.db} }

type SessionRepository interface {
	Create(ctx context.Context, s *domain.Session) error
	Get(ctx context.Context, id string) (*domain.Session, error)
	ListForAdmin(ctx context.Context, adminID string) ([]*domain.Session, error)
	UpdateStatus(ctx context.Context, id string, status domain.VideoStatus, errMsg string) error
	Delete(ctx context.Context, id string) error
	CreateInvite(ctx context.Context, token, sessionID string, expiresAt time.Time) error
	LatestInvite(ctx context.Context, sessionID string) (string, error)
	ConsumeInvite(ctx context.Context, token string) (string, error)
	UpdateTitle(ctx context.Context, id, title string) error
	UpdateDuration(ctx context.Context, id string, seconds float64) error
}
