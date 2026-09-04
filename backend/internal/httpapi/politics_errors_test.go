//go:build postgres

package httpapi

import (
	"errors"
	"net/http/httptest"
	"testing"

	"game/backend/internal/application"
	politicsdomain "game/backend/internal/domain/politics"
	"game/backend/internal/port"
)

func TestPoliticalConflictErrorsReturnConflict(t *testing.T) {
	for _, err := range []error{
		port.ErrConcurrentTransaction,
		application.ErrPoliticalDemandResolved,
		politicsdomain.ErrExpired,
	} {
		recorder := httptest.NewRecorder()
		(&Server{}).writeError(recorder, err)
		if recorder.Code != 409 {
			t.Fatalf("%v returned HTTP %d, want 409", err, recorder.Code)
		}
	}
	if errors.Is(politicsdomain.ErrExpired, port.ErrConcurrentTransaction) {
		t.Fatal("politics errors must remain distinct from transaction conflicts")
	}
}
