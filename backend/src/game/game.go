package game

import (
	"errors"
	"fmt"
)

type State struct {
	phase Phase

	scenario *Scenario

	availableScenarios map[ScenarioId]*Scenario
}

type Phase int

const (
	PhaseNone Phase = iota
	PhaseLevelSelect
	PhaseReady
	PhaseInProgress
	PhaseComplete
)

func (s *State) LoadScenarios(levelsDir string) error {
	if s.phase != PhaseNone {
		return unexpectedPhase("loading scenarios")
	}

	// TODO: load scenarios into availableScenarios

	s.phase = PhaseLevelSelect

	return nil
}

func (s *State) SelectScenario(scenarioId int) error {
	if s.phase != PhaseLevelSelect {
		return unexpectedPhase("selecting a scenario")
	}

	// TODO:

	s.phase = PhaseReady

	return nil
}

func (s *State) Start() error {
	if s.phase != PhaseReady {
		return unexpectedPhase("starting game")
	}
	
	// TODO:

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

func unexpectedPhase(phase string) error {
	return errors.New(fmt.Sprintf("Unexpected game state when %s", phase))
}
