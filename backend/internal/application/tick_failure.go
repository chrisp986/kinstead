package application

import "fmt"

// TickFailure preserves the authoritative identifiers known at the point of
// failure. The worker records only these allowlisted fields, not the wrapped
// database error text.
type TickFailure struct {
	WorldID     string
	HouseholdID string
	Tick        int64
	Stage       string
	Err         error
}

func (e *TickFailure) Error() string {
	if e == nil {
		return "tick failure"
	}
	return fmt.Sprintf("world %s tick %d failed at %s: %v", e.WorldID, e.Tick, e.Stage, e.Err)
}

func (e *TickFailure) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
