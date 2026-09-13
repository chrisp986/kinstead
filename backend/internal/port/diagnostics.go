package port

import "context"

type OperationalErrorRecord struct {
	Source           string
	ErrorCode        string
	Message          string
	RequestID        string
	WorkerInstanceID string
	WorldID          string
	HouseholdID      string
	Tick             *int64
	Stage            string
	Fingerprint      string
}

type OperationalErrorWriter interface {
	RecordOperationalError(context.Context, OperationalErrorRecord) error
}
