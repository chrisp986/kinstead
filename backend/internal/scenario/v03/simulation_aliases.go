package v03

import "game/backend/internal/simulation"

type (
	Activity       = simulation.Activity
	Assignment     = simulation.Assignment
	BalanceConfig  = simulation.BalanceConfig
	BuildingState  = simulation.BuildingState
	Character      = simulation.Character
	HouseholdState = simulation.HouseholdState
	Intensity      = simulation.Intensity
	IntensityRule  = simulation.IntensityRule
	Season         = simulation.Season
	TickContext    = simulation.TickContext
)

const (
	Spring = simulation.Spring
	Summer = simulation.Summer
	Autumn = simulation.Autumn
	Winter = simulation.Winter

	Agriculture  = simulation.Agriculture
	Fishing      = simulation.Fishing
	Woodcutting  = simulation.Woodcutting
	Building     = simulation.Building
	Rest         = simulation.Rest
	RulerService = simulation.RulerService

	Light  = simulation.Light
	Normal = simulation.Normal
	High   = simulation.High
)

var (
	EstimateProduction = simulation.EstimateProduction
	ProcessTick        = simulation.ProcessTick
)
