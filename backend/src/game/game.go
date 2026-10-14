package game

import (
	"errors"
	"fmt"
	"os"
	"sojourn/emulator"
)

type State struct {
	phase Phase

	Scenario *Scenario

	AvailableScenarios map[string]*Scenario
}

type Phase int

const (
	PhaseNone Phase = iota
	PhaseLevelSelect
	PhaseReady
	PhaseInProgress
	PhaseComplete
)

func InitState() State {
	return State{
		phase:              PhaseNone,
		Scenario:           nil,
		AvailableScenarios: make(map[string]*Scenario),
	}
}

func (s *State) Phase() Phase { return s.phase }

func (s *State) LoadScenarios(scenariosDir string) error {
	if s.phase != PhaseNone {
		return unexpectedPhase("loading scenarios")
	}

	files, err := os.ReadDir(scenariosDir)
	if err != nil {
		return err
	}

	for _, scenarioDir := range files {
		if !scenarioDir.IsDir() {
			continue
		}

		scenario, err := LoadScenario(scenarioDir.Name())

		if err != nil {
			return err
		}

		s.AvailableScenarios[scenario.Manifest.ID] = scenario
	}

	s.phase = PhaseLevelSelect

	return nil
}

func (s *State) SelectScenario(scenarioId string) error {
	if s.phase != PhaseLevelSelect {
		return unexpectedPhase("selecting a scenario")
	}

	scenario, ok := s.AvailableScenarios[scenarioId]

	if !ok {
		return errors.New(fmt.Sprintf("Invalid scenario id %s", scenarioId))
	}

	s.Scenario = scenario
	s.phase = PhaseReady

	return nil
}

func (s *State) Start() error {
	if s.phase != PhaseReady {
		return unexpectedPhase("starting game")
	}

	// TODO:

	s.Scenario.resetEvaluation()
	s.phase = PhaseInProgress

	return nil
}

func (s *State) UploadPatchedBinary(patchedBinaryPath string) error {
	if s.phase != PhaseInProgress {
		return unexpectedPhase("uploading patched binary")
	}

	// TODO:

	return nil
}

func (s *State) Evaluate(frame Frame) error {
	if s.phase != PhaseInProgress {
		return unexpectedPhase("evaluating a frame")
	}

	return s.Scenario.Evaluate(frame)
}

func (s *State) MemoryRanges() ([]emulator.MemoryRange, error) {
	if s.Scenario == nil {
		return nil, errors.New("no scenario selected")
	}
	return s.Scenario.MemoryRanges(), nil
}

func unexpectedPhase(phase string) error {
	return errors.New(fmt.Sprintf("Unexpected game state when %s", phase))
}
