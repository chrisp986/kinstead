package port

import "context"

// CalendarContext is a range-bounded read model. It contains source records,
// not a second calendar-events table: the calendar is a projection of
// authoritative contracts, shipments, politics, and work.
type CalendarContext struct {
	Snapshot    HouseholdSnapshot
	Obligations []CalendarObligationRecord
	Shipments   []CalendarShipmentRecord
	Deadlines   []CalendarDeadlineRecord
	Assignments []CalendarAssignmentRecord
}

type CalendarObligationRecord struct {
	ID                    string
	ContractID            string
	DebtorHouseholdID     string
	CreditorHouseholdID   string
	CounterpartyName      string
	ResourceType          string
	QuantityMilli         int64
	DueGameDay            int64
	LatestDispatchGameDay int64
	ShipmentID            string
	Status                string
}

type CalendarShipmentRecord struct {
	ID                     string
	SenderHouseholdID      string
	ReceiverHouseholdID    string
	CounterpartyName       string
	ResourceType           string
	QuantityMilli          int64
	DepartureGameDay       int64
	ExpectedArrivalGameDay int64
	ActualArrivalGameDay   *int64
	Status                 string
}

type CalendarDeadlineRecord struct {
	ID              string
	Kind            string
	Title           string
	DeadlineGameDay int64
	Category        string
	Importance      string
}

type CalendarAssignmentRecord struct {
	ID         string
	Activity   string
	EndsTick   int64
	Importance string
}

type CalendarReader interface {
	LoadCalendarContext(context.Context, string, int64, int64) (CalendarContext, error)
}
