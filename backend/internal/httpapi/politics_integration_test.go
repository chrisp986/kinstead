//go:build postgres

package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"

	"game/backend/internal/postgres"
)

func TestPoliticsProjectionJSONShapeAndMissingHousehold(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := postgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	var worldID, locationID, householdID string
	if err := store.Pool.QueryRow(ctx, `INSERT INTO worlds(name, historical_start_date, tick_duration_seconds, next_tick_at) VALUES ('politics http test', DATE '0980-01-01', 3600, now()) RETURNING id::text`).Scan(&worldID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = store.Pool.Exec(ctx, `DELETE FROM worlds WHERE id = $1::uuid`, worldID) })
	if err := store.Pool.QueryRow(ctx, `INSERT INTO locations(world_id, name, location_type) VALUES ($1::uuid, 'Politics HTTP place', 'farm') RETURNING id::text`, worldID).Scan(&locationID); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `INSERT INTO households(world_id, location_id, name, created_tick) VALUES ($1::uuid, $2::uuid, 'Politics HTTP household', 0) RETURNING id::text`, worldID, locationID).Scan(&householdID); err != nil {
		t.Fatal(err)
	}
	handler := New(store, slog.Default())
	req := httptest.NewRequest("GET", "/api/households/"+householdID+"/politics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"relationships", "decisions"} {
		raw, ok := body[key]
		if !ok {
			t.Fatalf("missing %q in %s", key, rec.Body.String())
		}
		if string(raw) == "null" {
			t.Fatalf("%q must be an array", key)
		}
	}
	if string(body["relationships"]) != "[]" || string(body["decisions"]) != "[]" {
		t.Fatalf("empty politics projection = %s", rec.Body.String())
	}
	if _, ok := body["Relationships"]; ok {
		t.Fatal("projection must use lower-case JSON keys")
	}
	unknown := httptest.NewRequest("GET", "/api/households/00000000-0000-0000-0000-000000000099/politics", nil)
	unknownRec := httptest.NewRecorder()
	handler.ServeHTTP(unknownRec, unknown)
	if unknownRec.Code != 404 {
		t.Fatalf("unknown household status = %d", unknownRec.Code)
	}
}
