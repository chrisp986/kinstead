//go:build postgres

// admin-cleanup removes only bounded, retained developer-console observations.
// It is an operator command and is never invoked by an HTTP request.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"game/backend/internal/postgres"
)

func main() {
	days := flag.Int("days", 30, "retain tick diagnostics and operational errors for this many days")
	heartbeatDays := flag.Int("heartbeat-days", 7, "retain stale heartbeat rows for this many days")
	batch := flag.Int("batch", 500, "maximum rows deleted per table operation")
	flag.Parse()
	if *days <= 0 || *heartbeatDays <= 0 || *batch <= 0 || os.Getenv("DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL and positive retention/batch values are required")
		os.Exit(1)
	}
	store, err := postgres.Open(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "database connection failed")
		os.Exit(1)
	}
	defer store.Close()
	result, err := store.CleanupDeveloperConsole(context.Background(), time.Duration(*days)*24*time.Hour, time.Duration(*heartbeatDays)*24*time.Hour, *batch)
	if err != nil {
		fmt.Fprintln(os.Stderr, "developer-console cleanup failed:", err)
		os.Exit(1)
	}
	fmt.Printf("tick_diagnostics=%d operational_errors=%d stale_heartbeats=%d\n", result.TickDiagnostics, result.OperationalErrors, result.StaleHeartbeats)
}
