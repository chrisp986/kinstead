//go:build postgres

// session provisions an expiring credential for an existing player. It is an
// operator command, never an HTTP endpoint or a way to claim a household.
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
	playerID := flag.String("player-id", "", "existing player UUID")
	lifetime := flag.Duration("lifetime", 24*time.Hour, "session lifetime (maximum 720h)")
	flag.Parse()
	if *playerID == "" || os.Getenv("DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "player-id and DATABASE_URL are required")
		os.Exit(1)
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "database connection failed")
		os.Exit(1)
	}
	defer store.Close()
	token, err := store.IssuePlayerSession(ctx, *playerID, *lifetime)
	if err != nil {
		fmt.Fprintln(os.Stderr, "session provisioning failed:", err)
		os.Exit(1)
	}
	fmt.Println(token)
}
