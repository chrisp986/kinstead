//go:build postgres

// admin provisions read-only developer-console membership. It is an operator
// command, never an HTTP endpoint and never creates credentials.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"game/backend/internal/postgres"
)

func main() {
	playerID := flag.String("player-id", "", "existing player UUID")
	flag.Parse()
	if *playerID == "" || os.Getenv("DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "player-id and DATABASE_URL are required")
		os.Exit(1)
	}
	store, err := postgres.Open(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "database connection failed")
		os.Exit(1)
	}
	defer store.Close()
	created, err := store.GrantAdminMembership(context.Background(), *playerID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "admin membership grant failed:", err)
		os.Exit(1)
	}
	outcome := "already_member"
	if created {
		outcome = "granted"
	}
	fmt.Printf("player_id=%s outcome=%s\n", *playerID, outcome)
}
