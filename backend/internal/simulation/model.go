package simulation

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
)

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
	BirthGameDay   int64
	Status         string
}

// DailyLaborState is the authoritative household state for hourly models.
// Spendable stocks are deliberately separate from earned pending output so
// production cannot be consumed, sold, dispatched, or levied before the
// model's calculated settlement boundary.
type DailyLaborState struct {
	Tick                   int64
	CurrentGameDay         calendar.GameDay
	CalendarRemainder      int64
	FarmSpecialization     Activity
	ProvisionsMilli        int64
	WoodMilli              int64
	TradeGoodsMilli        int64
	SilverMilli            int64
	PendingProvisionsMilli int64
	PendingWoodMilli       int64
	ProductionRemainders   map[string]int64
	ConsumptionRemainder   int64
	WoodUpkeepRemainder    int64
	FatigueRemainders      map[string]int64
	LastSettlementDay      *calendar.GameDay
	PolicyReason           string
	Characters             []DailyCharacter
}

type DailyCharacter struct {
	ID              string
	Name            string
	BirthGameDay    int64
	LaborPermille   int64
	Fatigue         int
	Health          int
	Status          string
	Specialization  Activity
	Occupation      workdomain.Occupation
	TemporaryDuties []workdomain.TemporaryDuty
}

func (s DailyLaborState) CharacterIndexByID(id string) (int, error) {
	for i := range s.Characters {
		if s.Characters[i].ID == id {
			return i, nil
		}
	}
	return -1, fmt.Errorf("unknown character ID %q", id)
}

type Assignment struct {
	CharacterID string
	Activity    Activity
	Intensity   Intensity
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
	GameDaysPerTickNum          int64
	GameDaysPerTickDen          int64
}

// SeasonForDate is the production calendar convention for the northern
// hemisphere. Synthetic balancing calendars provide their own TickContext.
func SeasonForDate(date time.Time) Season {
	switch date.Month() {
	case time.March, time.April, time.May:
		return Spring
	case time.June, time.July, time.August:
		return Summer
	case time.September, time.October, time.November:
		return Autumn
	default:
		return Winter
	}
}

func NeutralTickContext(season Season) TickContext {
	return TickContext{Season: season, AgricultureModifierPermille: 1000, FishingModifierPermille: 1000, GameDaysPerTickNum: 91, GameDaysPerTickDen: 12}
}

func (s HouseholdState) CharacterIndexByID(id string) (int, error) {
	for i := range s.Characters {
		if s.Characters[i].ID == id {
			return i, nil
		}
	}
	return -1, fmt.Errorf("unknown character ID %q", id)
}

func (s HouseholdState) SupplyTicks(cfg BalanceConfig) int64 {
	if cfg.ConsumptionPerTickMilli <= 0 || s.ProvisionsMilli <= 0 {
		return 0
	}
	return s.ProvisionsMilli / cfg.ConsumptionPerTickMilli
}

// SupplyGameDays converts the remaining provisions directly through the
// world's deterministic pacing ratio, flooring only the final game-day value.
// Wall-clock tick duration is deliberately absent.
func (s HouseholdState) SupplyGameDays(cfg BalanceConfig, gameDaysPerTickNum, gameDaysPerTickDen int64) int64 {
	if s.ProvisionsMilli <= 0 || cfg.ConsumptionPerTickMilli <= 0 || gameDaysPerTickNum <= 0 || gameDaysPerTickDen <= 0 {
		return 0
	}
	value := new(big.Int).Mul(big.NewInt(s.ProvisionsMilli), big.NewInt(gameDaysPerTickNum))
	divisor := new(big.Int).Mul(big.NewInt(cfg.ConsumptionPerTickMilli), big.NewInt(gameDaysPerTickDen))
	value.Quo(value, divisor)
	if !value.IsInt64() {
		return math.MaxInt64
	}
	return value.Int64()
}

func SupplyStatus(gameDays int64, cfg BalanceConfig) string {
	switch {
	case gameDays < cfg.EmergencySupplyDays:
		return "emergency"
	case gameDays < cfg.CriticalSupplyDays:
		return "critical"
	case gameDays <= cfg.StrainedSupplyDays:
		return "strained"
	default:
		return "safe"
	}
}

func (s HouseholdState) SupplyBelowGameDays(cfg BalanceConfig, threshold, gameDaysPerTickNum, gameDaysPerTickDen int64) bool {
	if cfg.ConsumptionPerTickMilli <= 0 || gameDaysPerTickNum <= 0 || gameDaysPerTickDen <= 0 {
		return true
	}
	left := new(big.Int).Mul(big.NewInt(s.ProvisionsMilli), big.NewInt(gameDaysPerTickNum))
	right := new(big.Int).Mul(big.NewInt(threshold), big.NewInt(cfg.ConsumptionPerTickMilli))
	right.Mul(right, big.NewInt(gameDaysPerTickDen))
	return left.Cmp(right) < 0
}

func (s HouseholdState) SupplyAtMostGameDays(cfg BalanceConfig, threshold, gameDaysPerTickNum, gameDaysPerTickDen int64) bool {
	if cfg.ConsumptionPerTickMilli <= 0 || gameDaysPerTickNum <= 0 || gameDaysPerTickDen <= 0 {
		return true
	}
	left := new(big.Int).Mul(big.NewInt(s.ProvisionsMilli), big.NewInt(gameDaysPerTickNum))
	right := new(big.Int).Mul(big.NewInt(threshold), big.NewInt(cfg.ConsumptionPerTickMilli))
	right.Mul(right, big.NewInt(gameDaysPerTickDen))
	return left.Cmp(right) <= 0
}
