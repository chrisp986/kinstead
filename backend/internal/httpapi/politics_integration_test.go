//go:build postgres

package httpapi

import (
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
	store, err := postgres.Open(t.Context(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(store, slog.Default())
	req := httptest.NewRequest("GET", "/api/households/00000000-0000-0000-0000-000000000020/politics", nil)
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
