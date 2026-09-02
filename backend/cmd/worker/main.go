//go:build postgres

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"game/backend/internal/application"
	"game/backend/internal/postgres"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://game:game@localhost:5432/game?sslmode=disable"
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	store, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		log.Error("database connect failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	processor := application.NewTickProcessor(store)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	log.Info("worker started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Drain due worlds without sleeping between successful claims.
			for i := 0; i < 100; i++ {
				ok, err := processor.ProcessOneDueWorld(ctx)
				if err != nil {
					log.Error("tick failed", "error", err)
					break
				}
				if !ok {
					break
				}
			}
		}
	}
}
