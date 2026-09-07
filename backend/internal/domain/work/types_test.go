package work

import (
	"encoding/json"
	"testing"

	"game/backend/internal/calendar"
)

func TestOccupationUsesAPIJSONFieldNames(t *testing.T) {
	activity := Fishing
	day := calendar.GameDay(12)
	occupation := Occupation{
		CharacterID:     "character-1",
		Activity:        Agriculture,
		PendingActivity: &activity,
		EffectiveDay:    &day,
		Revision:        3,
	}

	encoded, err := json.Marshal(occupation)
	if err != nil {
		t.Fatalf("marshal occupation: %v", err)
	}

	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode occupation JSON: %v", err)
	}
	for _, field := range []string{"character_id", "activity", "pending_activity", "effective_game_day", "revision"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("occupation JSON is missing %q: %s", field, encoded)
		}
	}
	for _, field := range []string{"CharacterID", "Activity", "PendingActivity", "EffectiveDay", "Revision"} {
		if _, ok := fields[field]; ok {
			t.Errorf("occupation JSON contains Go field name %q: %s", field, encoded)
		}
	}
}
