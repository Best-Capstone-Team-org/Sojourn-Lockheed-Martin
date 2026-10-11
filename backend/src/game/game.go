package game

import (
	"errors"
	"fmt"
	"sojourn/emulator"
)

type State struct {
	phase Phase

	scenario *Scenario

	availableScenarios map[string]*Scenario
}

type Phase int

const (
	PhaseNone Phase = iota
	PhaseLevelSelect
	PhaseReady
	PhaseInProgress
	PhaseComplete
)

func (s *State) Phase() Phase { return s.phase }

func (s *State) LoadScenarios(levelsDir string) error {
	if s.phase != PhaseNone {
		return unexpectedPhase("loading scenarios")
	}

	// TODO: load scenarios into availableScenarios

	s.phase = PhaseLevelSelect

	return nil
}

func (s *State) SelectScenario(scenarioId string) error {
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

	s.scenario.resetEvaluation()
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

	return s.scenario.Evaluate(frame)
}

func (s *State) MemoryRanges() ([]emulator.MemoryRange, error) {
	if s.scenario == nil {
		return nil, errors.New("no scenario selected")
	}
	return s.scenario.MemoryRanges(), nil
}

func unexpectedPhase(phase string) error {
	return errors.New(fmt.Sprintf("Unexpected game state when %s", phase))
}
