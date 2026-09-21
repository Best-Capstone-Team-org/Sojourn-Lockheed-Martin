package game

type Scenario struct {
	objectives []Objective
	// TODO:
}

type ScenarioId = int

type Objective struct {
	status ObjectiveStatus
	// TODO:
}

type ObjectiveStatus string

const (
	ObjectiveStatusComplete ObjectiveStatus = "complete"
	ObjectiveStatusFailed   ObjectiveStatus = "failed"
	ObjectiveStatusActive   ObjectiveStatus = "active"
	ObjectiveStatusLocked   ObjectiveStatus = "locked"
)

// TODO: parse and load scenario files
