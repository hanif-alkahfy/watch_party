package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"watchparty/internal/domain"
)

type userRepo struct{ db *sql.DB }

func (r *userRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.PasswordHash, string(u.Role), now())
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return domain.ErrEmailTaken
	}
	return err
}

func (r *userRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return r.get(ctx, `SELECT id, email, password_hash, role, created_at FROM users WHERE id = ?`, id)
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return r.get(ctx, `SELECT id, email, password_hash, role, created_at FROM users WHERE email = ?`, strings.ToLower(email))
}

func (r *userRepo) UpsertAdmin(ctx context.Context, u *domain.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, role, created_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(email) DO UPDATE SET password_hash = excluded.password_hash, role = excluded.role`,
		u.ID, strings.ToLower(strings.TrimSpace(u.Email)), u.PasswordHash, string(domain.RoleAdmin), now())
	return err
}

func (r *userRepo) get(ctx context.Context, query string, arg string) (*domain.User, error) {
	var u domain.User
	var role, created string
	err := r.db.QueryRowContext(ctx, query, arg).Scan(&u.ID, &u.Email, &u.PasswordHash, &role, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Role = domain.Role(role)
	u.CreatedAt = toTime(created)
	return &u, nil
}
