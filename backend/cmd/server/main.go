package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"watchparty/internal/config"
	"watchparty/internal/repository"
	"watchparty/internal/service"
	httptransport "watchparty/internal/transport/http"
	"watchparty/internal/transport/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := repository.OpenSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	authService := service.NewAuthService(db.Users(), cfg.JWTSecret, cfg.JWTTTL, cfg.AdminEmail)
	if err := authService.SeedAdmin(context.Background(), cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	sessionService := service.NewSessionService(db.Sessions(), db.Users(), cfg.MediaRoot, service.NewTranscoder(cfg.FFmpegPath))
	hub := ws.NewHub()

	app := httptransport.New(httptransport.Deps{
		Auth:           authService,
		Sessions:       sessionService,
		WS:             ws.NewHandler(authService, sessionService, hub),
		AllowedOrigins: cfg.AllowedOrigins,
		MaxUploadBytes: cfg.MaxUploadBytes,
		PublicBaseURL:  cfg.PublicBaseURL,
		WebRoot:        cfg.WebRoot,
	})

	errCh := make(chan error, 1)
	go func() {
		addr := cfg.ListenAddr
	log.Printf("watchparty listening on %s", addr)
	if err := app.Listen(addr); err != nil {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Fatalf("listen: %v", err)
	case <-quit:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
