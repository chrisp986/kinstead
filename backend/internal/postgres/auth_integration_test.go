//go:build postgres

package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestDatabaseSessionsAndOwnership(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	world, location, house, character := createChronicleFixture(t, ctx, store)
	t.Cleanup(func() { removeChronicleFixture(t, ctx, store, world, location, house, character) })
	var player string
	if err := store.Pool.QueryRow(ctx, `INSERT INTO players(external_auth_subject) VALUES($1) RETURNING id::text`, "test-"+house).Scan(&player); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = store.Pool.Exec(ctx, `UPDATE households SET owner_player_id=NULL WHERE owner_player_id=$1::uuid`, player)
		_, _ = store.Pool.Exec(ctx, `DELETE FROM players WHERE id=$1::uuid`, player)
	})
	if _, err := store.Pool.Exec(ctx, `UPDATE households SET owner_player_id=$1::uuid WHERE id=$2::uuid`, player, house); err != nil {
		t.Fatal(err)
	}
	token, err := store.IssuePlayerSession(ctx, player, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := store.AuthenticateSession(ctx, token); err != nil || got != player {
		t.Fatalf("player=%s err=%v", got, err)
	}
	if owns, err := store.PlayerOwnsHousehold(ctx, player, house); err != nil || !owns {
		t.Fatalf("owns=%v err=%v", owns, err)
	}
	if owns, err := store.PlayerOwnsHousehold(ctx, player, world); err != nil || owns {
		t.Fatalf("foreign owns=%v err=%v", owns, err)
	}
	hash := sha256.Sum256([]byte(token))
	if _, err := store.Pool.Exec(ctx, `UPDATE player_sessions SET revoked_at=now() WHERE token_hash=$1`, hash[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateSession(ctx, token); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("revoked session err=%v", err)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE player_sessions SET revoked_at=NULL,created_at=now()-interval '2 hours',expires_at=now()-interval '1 hour' WHERE token_hash=$1`, hash[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AuthenticateSession(ctx, token); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expired session err=%v", err)
	}
}
