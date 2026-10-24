package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every runtime setting. Credentials come from .env only, never from code.
type Config struct {
	Port           string
	AdminEmail     string
	AdminPassword  string
	JWTSecret      string
	JWTTTL         time.Duration
	DBPath         string
	MediaRoot      string
	AllowedOrigins []string
	FFmpegPath     string
	MaxUploadBytes int64
	// PublicBaseURL is the origin used in invite links (e.g. the Cloudflare
	// tunnel URL). Empty means "use the origin of the incoming request".
	PublicBaseURL string
	// WebRoot is the built frontend to serve. Empty means "API only, the
	// frontend is served by the Vite dev server".
	WebRoot string
	// ListenAddr is the address the server binds to. ":3001" listens on every
	// interface, which lets other devices on the local network connect.
	ListenAddr string
}

func Load() (*Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	cfg := &Config{
		Port:           env("PORT", "3001"),
		AdminEmail:     env("ADMIN_EMAIL", "admin@mail.com"),
		AdminPassword:  env("ADMIN_PASSWORD", "admin123"),
		JWTSecret:      strings.TrimSpace(os.Getenv("JWT_SECRET")),
		DBPath:         env("DB_PATH", "./data/watchparty.db"),
		MediaRoot:      env("MEDIA_ROOT", "./media"),
		AllowedOrigins: splitCSV(env("ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000")),
		FFmpegPath:     env("FFMPEG_PATH", "ffmpeg"),
		MaxUploadBytes: int64(envInt("MAX_UPLOAD_MB", 512)) << 20,
		PublicBaseURL:  strings.TrimRight(env("PUBLIC_BASE_URL", ""), "/"),
		WebRoot:        env("WEB_ROOT", ""),
		ListenAddr:     env("LISTEN_ADDR", ":"+env("PORT", "3001")),
	}

	// Absolute paths: ffmpeg runs with its own working directory.
	var err error
	if cfg.DBPath, err = filepath.Abs(cfg.DBPath); err != nil {
		return nil, fmt.Errorf("DB_PATH: %w", err)
	}
	if cfg.MediaRoot, err = filepath.Abs(cfg.MediaRoot); err != nil {
		return nil, fmt.Errorf("MEDIA_ROOT: %w", err)
	}

	ttl, err := time.ParseDuration(env("JWT_TTL", "24h"))
	if err != nil {
		return nil, fmt.Errorf("JWT_TTL: %w", err)
	}
	cfg.JWTTTL = ttl

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required: copy .env.example to .env and set a random value")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	return cfg, nil
}

// loadDotEnv parses .env, tolerating a UTF-8 BOM written by Windows editors.
func loadDotEnv(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	values, err := godotenv.Unmarshal(strings.TrimPrefix(string(raw), "\ufeff"))
	if err != nil {
		return err
	}
	for k, v := range values {
		if _, ok := os.LookupEnv(k); !ok {
			if err := os.Setenv(k, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return fallback
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
