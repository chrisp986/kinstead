//go:build postgres

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"game/backend/internal/application"
	"game/backend/internal/port"
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
	instanceID, err := workerInstanceID()
	if err != nil {
		log.Error("worker instance id failed", "error", err)
		os.Exit(1)
	}
	startedAt := time.Now().UTC()
	heartbeatCtx, stopHeartbeat := context.WithCancel(ctx)
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		runHeartbeat(heartbeatCtx, store, log, instanceID, startedAt)
	}()
	defer func() {
		stopHeartbeat()
		<-heartbeatDone
	}()
	processor := application.NewTickProcessor(store)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	log.Info("worker started", "instance_id", instanceID)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Drain due worlds without sleeping between successful claims.
			for i := 0; i < 100; i++ {
				ok, err := processor.ProcessOneDueWorld(ctx)
				if err != nil {
					recordWorkerFailure(ctx, store, log, instanceID, err)
					break
				}
				if !ok {
					break
				}
			}
		}
	}
}

func recordWorkerFailure(ctx context.Context, store *postgres.Store, log *slog.Logger, instanceID string, err error) {
	code, stage, worldID, householdID, tick := "tick_failed", "unknown", "", "", (*int64)(nil)
	var failure *application.TickFailure
	if errors.As(err, &failure) {
		stage, worldID, householdID = failure.Stage, failure.WorldID, failure.HouseholdID
		value := failure.Tick
		tick = &value
	}
	fingerprintInput := fmt.Sprintf("worker:%s:%s:%s:%s:%d", code, stage, worldID, householdID, dereferenceTick(tick))
	digest := sha256.Sum256([]byte(fingerprintInput))
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if writeErr := store.RecordOperationalError(writeCtx, port.OperationalErrorRecord{Source: "worker", ErrorCode: code, Message: "world tick failed", WorkerInstanceID: instanceID, WorldID: worldID, HouseholdID: householdID, Tick: tick, Stage: stage, Fingerprint: hex.EncodeToString(digest[:])}); writeErr != nil {
		log.Error("tick failure record failed", "instance_id", instanceID, "error", writeErr)
	}
	log.Error("tick failed", "error", err, "stage", stage, "world_id", worldID, "household_id", householdID, "tick", dereferenceTick(tick))
}

func dereferenceTick(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func runHeartbeat(ctx context.Context, store *postgres.Store, log *slog.Logger, instanceID string, startedAt time.Time) {
	write := func() {
		writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := store.UpsertWorkerHeartbeat(writeCtx, instanceID, startedAt, time.Now().UTC()); err != nil && ctx.Err() == nil {
			log.Error("worker heartbeat failed", "instance_id", instanceID, "error", err)
		}
	}
	write()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			write()
		}
	}
}

func workerInstanceID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}
