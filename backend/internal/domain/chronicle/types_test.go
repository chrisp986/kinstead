package chronicle

import "testing"

func TestProductionEntryTypesAreStable(t *testing.T) {
	if AssignmentScheduled != "assignment_scheduled" || ContractObligationBroken != "contract_obligation_broken" || EmergencyWorkOverridden != "emergency_work_overridden" {
		t.Fatal("chronicle production entry type constants changed unexpectedly")
	}
}
