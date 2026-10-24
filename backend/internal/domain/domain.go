package domain

import (
	"errors"
	"time"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

func (r Role) Valid() bool { return r == RoleAdmin || r == RoleUser }

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
}

type VideoStatus string

const (
	StatusUploading  VideoStatus = "uploading"
	StatusProcessing VideoStatus = "processing"
	StatusReady      VideoStatus = "ready"
	StatusFailed     VideoStatus = "failed"
)

// Session is a watch party room. Exactly one admin owns it; viewers join
// through a multi-use invite link that is exchanged for a session-scoped token.
type Session struct {
	ID              string
	AdminID         string
	VideoTitle      string
	DurationSeconds float64
	SourcePath      string
	VideoPath       string
	VideoStatus     VideoStatus
	Error           string
	CreatedAt       time.Time
}

var (
	ErrNotFound        = errors.New("not found")
	ErrEmailTaken      = errors.New("email already registered")
	ErrInvalidCreds    = errors.New("invalid credentials")
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidInput    = errors.New("invalid input")
	ErrSessionNotReady = errors.New("session not ready")
	ErrInviteExpired   = errors.New("invite link has expired")
)
