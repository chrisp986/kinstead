package simulation

import "fmt"

type Season string

const (
	Spring Season = "spring"
	Summer Season = "summer"
	Autumn Season = "autumn"
	Winter Season = "winter"
)

type Activity string

const (
	Agriculture  Activity = "agriculture"
	Fishing      Activity = "fishing"
	Woodcutting  Activity = "woodcutting"
	Building     Activity = "building"
	Rest         Activity = "rest"
	RulerService Activity = "ruler_service"
)

type Intensity string

const (
	Light  Intensity = "light"
	Normal Intensity = "normal"
	High   Intensity = "high"
)

type Character struct {
	ID             string
	Name           string
	LaborPermille  int64
	Fatigue        int
	Specialization Activity
}

type Assignment struct {
	Character string
	Activity  Activity
	Intensity Intensity
}

type BuildingState struct {
	Name               string
	WoodCostMilli      int64
	WorkerDaysPermille int64
	ProgressPermille   int64
	Started            bool
	Completed          bool
}

type HouseholdState struct {
	Tick                     int64
	FarmSpecialization       Activity
	ProvisionsMilli          int64
	WoodMilli                int64
	TradeGoodsMilli          int64
	SilverMilli              int64
	JarlStanding             int
	Characters               []Character
	Buildings                []BuildingState
	PoliticalServiceDays     int
	PoliticalWoodPaidMilli   int64
	PoliticalSilverPaidMilli int64
	CriticalDays             int
	StrainedDays             int
	TradeVolumeMilli         int64
}

type TickContext struct {
	Season                      Season
	AgricultureModifierPermille int64
	FishingModifierPermille     int64
}

func SeasonForTick(tick int64) Season {
	if tick <= 0 {
		return Spring
	}
	day := ((tick - 1) % 48) + 1
	switch {
	case day <= 12:
		return Spring
	case day <= 24:
		return Summer
	case day <= 36:
		return Autumn
	default:
		return Winter
	}
}

func (s HouseholdState) CharacterIndex(name string) (int, error) {
	for i := range s.Characters {
		if s.Characters[i].Name == name {
			return i, nil
		}
	}
	return -1, fmt.Errorf("unknown character %q", name)
}

func (s HouseholdState) SupplyDays(cfg BalanceConfig) float64 {
	if cfg.DailyConsumptionMilli <= 0 {
		return 0
	}
	return float64(s.ProvisionsMilli) / float64(cfg.DailyConsumptionMilli)
}
