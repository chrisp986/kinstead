//go:build postgres

package postgres

import (
	"errors"
	"fmt"
	"testing"

	"game/backend/internal/port"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeTransactionError(t *testing.T) {
	for _, code := range []string{"40001", "40P01"} {
		err := normalizeTransactionError(fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: code, Message: "concurrent"}))
		if !errors.Is(err, port.ErrConcurrentTransaction) {
			t.Fatalf("code %s: errors.Is(%v, ErrConcurrentTransaction) = false", code, err)
		}
	}
	if err := normalizeTransactionError(&pgconn.PgError{Code: "23505"}); errors.Is(err, port.ErrConcurrentTransaction) {
		t.Fatal("unrelated PostgreSQL errors must not be classified as concurrent transactions")
	}
}
