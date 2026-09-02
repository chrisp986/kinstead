package character

type ID string
type Activity string
type Intensity string

const (
	ActivityAgriculture  Activity = "agriculture"
	ActivityFishing      Activity = "fishing"
	ActivityWoodcutting  Activity = "woodcutting"
	ActivityCrafting     Activity = "crafting"
	ActivityTraining     Activity = "training"
	ActivityRulerService Activity = "ruler_service"
	ActivityRest         Activity = "rest"

	IntensityLight  Intensity = "light"
	IntensityNormal Intensity = "normal"
	IntensityHigh   Intensity = "high"
)
