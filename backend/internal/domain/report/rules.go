package report

import (
	"sort"
)

type scoredItem struct {
	priority int
	item     Item
}

// BuildAttention returns at most three deterministic awareness items.
func BuildAttention(in Input) []Item {
	items := make([]scoredItem, 0)
	if in.SupplyDays < 7 {
		items = append(items, scoredItem{100, Item{Code: "supply_emergency", Severity: "critical", Target: "trade", Data: map[string]any{"supply_days": in.SupplyDays}}})
	} else if in.SupplyDays < 15 {
		items = append(items, scoredItem{85, Item{Code: "supply_critical", Severity: "critical", Target: "trade", Data: map[string]any{"supply_days": in.SupplyDays}}})
	} else if in.SupplyDays <= 30 {
		items = append(items, scoredItem{60, Item{Code: "supply_strained", Severity: "warning", Target: "trade", Data: map[string]any{"supply_days": in.SupplyDays}}})
	}
	for _, c := range in.Characters {
		switch {
		case c.Fatigue >= 85:
			items = append(items, scoredItem{90, Item{Code: "character_fatigue_critical", Severity: "critical", Target: "work", RelatedID: c.ID, Data: map[string]any{"character_name": c.Name, "fatigue": c.Fatigue}}})
		case c.Fatigue >= 70:
			items = append(items, scoredItem{55, Item{Code: "character_fatigue_high", Severity: "warning", Target: "work", RelatedID: c.ID, Data: map[string]any{"character_name": c.Name, "fatigue": c.Fatigue}}})
		}
	}
	for _, d := range in.PoliticalDemands {
		due := d.ExpiresTick - in.CurrentTick
		var dueDay *int64
		if d.ExpiresGameDay > 0 {
			due = d.ExpiresGameDay - in.CurrentGameDay
			value := d.ExpiresGameDay
			dueDay = &value
		}
		if due <= 3 {
			severity := "warning"
			priority := 75
			if due <= 1 {
				severity, priority = "critical", 95
			}
			tick := d.ExpiresTick
			items = append(items, scoredItem{priority, Item{Code: "political_demand_due", Severity: severity, Target: "politics", RelatedID: d.ID, DueTick: &tick, DueGameDay: dueDay, Data: map[string]any{"actor_name": d.ActorName}}})
		}
	}
	for _, o := range in.ContractObligations {
		if o.DueGameDay > 0 {
			due := o.DueGameDay - in.CurrentGameDay
			if o.ExpectedArrivalGameDay != nil && *o.ExpectedArrivalGameDay > o.DueGameDay {
				items = append(items, scoredItem{70, Item{Code: "contract_delivery_expected_late", Severity: "warning", Target: "contracts", RelatedID: o.ID, DueGameDay: &o.DueGameDay, Data: map[string]any{"resource_type": o.ResourceType, "quantity_milli": o.QuantityMilli, "expected_arrival_game_day": *o.ExpectedArrivalGameDay}}})
				continue
			}
			if due <= 3 {
				severity, priority := "warning", 70
				if due <= 1 {
					severity, priority = "critical", 90
				}
				items = append(items, scoredItem{priority, Item{Code: "contract_obligation_due", Severity: severity, Target: "contracts", RelatedID: o.ID, DueGameDay: &o.DueGameDay, Data: map[string]any{"resource_type": o.ResourceType, "quantity_milli": o.QuantityMilli}}})
			}
			continue
		}
		if o.ExpectedArrivalTick != nil && *o.ExpectedArrivalTick > o.DueArrivalTick {
			items = append(items, scoredItem{70, Item{Code: "contract_delivery_expected_late", Severity: "warning", Target: "contracts", RelatedID: o.ID, DueTick: &o.DueArrivalTick, Data: map[string]any{"resource_type": o.ResourceType, "quantity_milli": o.QuantityMilli, "expected_arrival_tick": *o.ExpectedArrivalTick}}})
			continue
		}
		due := o.DueArrivalTick - in.CurrentTick
		if due <= 3 {
			severity := "warning"
			priority := 70
			if due <= 1 {
				severity, priority = "critical", 90
			}
			tick := o.DueArrivalTick
			items = append(items, scoredItem{priority, Item{Code: "contract_obligation_due", Severity: severity, Target: "contracts", RelatedID: o.ID, DueTick: &tick, Data: map[string]any{"resource_type": o.ResourceType, "quantity_milli": o.QuantityMilli}}})
		}
	}
	return selectItems(items, 3)
}

// BuildDecisions returns at most three actionable pointers into existing
// gameplay surfaces. It deliberately does not execute any command.
func BuildDecisions(in Input) []Item {
	items := make([]scoredItem, 0)
	if in.SupplyDays < 7 {
		items = append(items, scoredItem{100, Item{Code: "secure_provisions", Severity: "critical", Target: "trade", Data: map[string]any{"supply_days": in.SupplyDays}}})
	} else if in.SupplyDays < 15 {
		items = append(items, scoredItem{85, Item{Code: "secure_provisions", Severity: "critical", Target: "trade", Data: map[string]any{"supply_days": in.SupplyDays}}})
	} else if in.SupplyDays <= 30 {
		items = append(items, scoredItem{60, Item{Code: "secure_provisions", Severity: "warning", Target: "trade", Data: map[string]any{"supply_days": in.SupplyDays}}})
	}
	for _, d := range in.PoliticalDemands {
		due := d.ExpiresTick - in.CurrentTick
		var dueDay *int64
		if d.ExpiresGameDay > 0 {
			due = d.ExpiresGameDay - in.CurrentGameDay
			value := d.ExpiresGameDay
			dueDay = &value
		}
		if due <= 3 {
			priority := 75
			severity := "warning"
			if due <= 1 {
				priority, severity = 95, "critical"
			}
			tick := d.ExpiresTick
			items = append(items, scoredItem{priority, Item{Code: "respond_political_demand", Severity: severity, Target: "politics", RelatedID: d.ID, DueTick: &tick, DueGameDay: dueDay, Data: map[string]any{"actor_name": d.ActorName}}})
		}
	}
	for _, o := range in.ContractObligations {
		if o.DueGameDay > 0 {
			due := o.DueGameDay - in.CurrentGameDay
			if due <= 3 {
				priority, severity := 70, "warning"
				if due <= 1 {
					priority, severity = 90, "critical"
				}
				items = append(items, scoredItem{priority, Item{Code: "dispatch_contract_obligation", Severity: severity, Target: "contracts", RelatedID: o.ID, DueGameDay: &o.DueGameDay, Data: map[string]any{"resource_type": o.ResourceType, "quantity_milli": o.QuantityMilli}}})
			}
			continue
		}
		due := o.DueArrivalTick - in.CurrentTick
		if due <= 3 {
			priority := 70
			severity := "warning"
			if due <= 1 {
				priority, severity = 90, "critical"
			}
			tick := o.DueArrivalTick
			items = append(items, scoredItem{priority, Item{Code: "dispatch_contract_obligation", Severity: severity, Target: "contracts", RelatedID: o.ID, DueTick: &tick, Data: map[string]any{"resource_type": o.ResourceType, "quantity_milli": o.QuantityMilli}}})
		}
	}
	for _, c := range in.Characters {
		if c.Fatigue >= 85 {
			items = append(items, scoredItem{80, Item{Code: "rest_fatigued_character", Severity: "critical", Target: "work", RelatedID: c.ID, Data: map[string]any{"character_name": c.Name, "fatigue": c.Fatigue}}})
		} else if c.Fatigue >= 70 {
			items = append(items, scoredItem{55, Item{Code: "rest_fatigued_character", Severity: "warning", Target: "work", RelatedID: c.ID, Data: map[string]any{"character_name": c.Name, "fatigue": c.Fatigue}}})
		}
	}
	return selectItems(items, 3)
}

func selectItems(items []scoredItem, limit int) []Item {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].priority != items[j].priority {
			return items[i].priority > items[j].priority
		}
		a, b := items[i].item, items[j].item
		ad, bd := int64(1<<62), int64(1<<62)
		if a.DueTick != nil {
			ad = *a.DueTick
		}
		if b.DueTick != nil {
			bd = *b.DueTick
		}
		if ad != bd {
			return ad < bd
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.RelatedID < b.RelatedID
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]Item, 0, len(items))
	for _, item := range items {
		out = append(out, item.item)
	}
	return out
}
