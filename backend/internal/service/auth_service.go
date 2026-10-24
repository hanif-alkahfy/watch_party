package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"watchparty/internal/domain"
	"watchparty/internal/repository"
)

const minPasswordLen = 8

// Token kinds. A user token belongs to an account, a viewer token to a single
// watch session and is minted from a multi-use invite link.
const (
	TokenKindUser   = "user"
	TokenKindViewer = "viewer"
)

type Claims struct {
	UserID    string
	Email     string
	Role      domain.Role
	Kind      string
	SessionID string
}

func (c *Claims) IsViewer() bool { return c.Kind == TokenKindViewer }

type AuthService struct {
	users       repository.UserRepository
	secret      []byte
	ttl         time.Duration
	adminEmail  string
}

func NewAuthService(users repository.UserRepository, secret string, ttl time.Duration, adminEmail string) *AuthService {
	return &AuthService{users: users, secret: []byte(secret), ttl: ttl, adminEmail: strings.ToLower(strings.TrimSpace(adminEmail))}
}

// SeedAdmin ensures the default admin user configured in .env exists with the latest password.
func (s *AuthService) SeedAdmin(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(email); err != nil {
		return domain.ErrInvalidInput
	}
	if len(password) < 6 {
		return domain.ErrInvalidInput
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	u := &domain.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         domain.RoleAdmin,
		CreatedAt:    time.Now().UTC(),
	}
	return s.users.UpsertAdmin(ctx, u)
}

// Login authenticates the admin by password only: the account is the one
// configured in .env, so no email field is needed.
func (s *AuthService) Login(ctx context.Context, password string) (string, *domain.User, error) {
	if password == "" {
		return "", nil, domain.ErrInvalidCreds
	}
	u, err := s.users.GetByEmail(ctx, s.adminEmail)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", nil, domain.ErrInvalidCreds
		}
		return "", nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", nil, domain.ErrInvalidCreds
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": u.ID,
		"email":   u.Email,
		"role":    string(u.Role),
		"kind":    TokenKindUser,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(s.ttl).Unix(),
	})
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", nil, err
	}
	return signed, u, nil
}

// ParseToken validates the signature and expiry, then returns the caller identity.
func (s *AuthService) ParseToken(tokenStr string) (*Claims, error) {
	claims := jwt.MapClaims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrInvalidCreds
		}
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, domain.ErrInvalidCreds
	}

	kind, _ := claims["kind"].(string)
	if kind == "" {
		kind = TokenKindUser
	}
	sessionID, _ := claims["session_id"].(string)
	if kind == TokenKindViewer {
		if sessionID == "" {
			return nil, domain.ErrInvalidCreds
		}
		return &Claims{Kind: kind, SessionID: sessionID}, nil
	}

	userID, _ := claims["user_id"].(string)
	email, _ := claims["email"].(string)
	role, _ := claims["role"].(string)
	if userID == "" {
		return nil, domain.ErrInvalidCreds
	}
	return &Claims{UserID: userID, Email: email, Role: domain.Role(role), Kind: kind}, nil
}

// IssueViewerToken mints a token that grants access to one session only.
func (s *AuthService) IssueViewerToken(sessionID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"kind":       TokenKindViewer,
		"session_id": sessionID,
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(s.ttl).Unix(),
	})
	return token.SignedString(s.secret)
}
