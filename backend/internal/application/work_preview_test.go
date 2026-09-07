package application

import (
	"context"
	"encoding/json"
	"testing"

	"game/backend/internal/balance"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

func TestWorkPreviewUsesSharedProductionFatigueAndConflicts(t *testing.T) {
	character := simulation.Character{ID: "worker", Name: "Astrid", LaborPermille: 1000, Fatigue: 68, Specialization: simulation.Fishing}
	reader := reportReaderStub{snapshot: port.HouseholdSnapshot{CurrentTick: 4, CurrentGameDay: 30, CalendarRemainder: 4, GameDaysPerTickNum: 91, GameDaysPerTickDen: 12,
		State:       simulation.HouseholdState{Tick: 4, FarmSpecialization: simulation.Fishing, Characters: []simulation.Character{character}},
		Assignments: []port.AssignmentRecord{{ID: "conflict", CharacterID: "worker", StartsTick: 6, EndsTick: 6, Status: "planned"}}}}
	preview, err := (&WorkPreviewService{Store: reader, Balance: balance.V03()}).Preview(context.Background(), WorkPreviewCommand{HouseholdID: "house", CharacterID: "worker", Activity: simulation.Fishing, Intensity: simulation.Normal, DurationTicks: 3})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ExpectedProvisionsMilli <= 0 || preview.ExpectedWoodMilli != 0 || preview.StartingFatigue != 68 || preview.EndingFatigue != 80 || preview.Season != "spring" || preview.SpecializationModifierPermille != 1150 || preview.FarmModifierPermille != 1150 || len(preview.AssignmentConflicts) != 1 || len(preview.Warnings) != 1 {
		t.Fatalf("preview=%+v", preview)
	}
}

func TestWorkPreviewCrossesSeasonBoundaryAndExposesRequestedContract(t *testing.T) {
	reader := reportReaderStub{snapshot: port.HouseholdSnapshot{CurrentTick: 11, CurrentGameDay: 90, GameDaysPerTickNum: 91, GameDaysPerTickDen: 12,
		State: simulation.HouseholdState{FarmSpecialization: simulation.Fishing, Characters: []simulation.Character{{ID: "worker", LaborPermille: 1000, Specialization: simulation.Fishing}}}}}
	preview, err := NewWorkPreviewService(reader).Preview(context.Background(), WorkPreviewCommand{HouseholdID: "house", CharacterID: "worker", Activity: simulation.Fishing, Intensity: simulation.Normal, DurationTicks: 3})
	if err != nil {
		t.Fatal(err)
	}
	if preview.ExpectedProvisionsMilli != 14547 || preview.EndingFatigue != 12 || preview.Conflict || preview.Warning != nil {
		t.Fatalf("preview=%+v", preview)
	}
	data, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"produced_provisions_milli", "produced_wood_milli", "fatigue_start", "fatigue_end", "specialization_bonus_permille", "farm_bonus_permille", "season", "conflict", "warning"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("missing %s in %s", key, data)
		}
	}
}
