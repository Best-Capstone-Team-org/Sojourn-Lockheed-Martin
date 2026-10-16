package game

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sojourn/app"
	"sojourn/emulator"
	"strconv"
	"strings"
)

type State struct {
	phase Phase

	Scenario *Scenario

	evaluation Evaluation
	grader     *Grader

	AvailableScenarios map[string]*Scenario
}

type Phase int

const (
	PhaseNone Phase = iota
	PhaseLevelSelect
	PhaseReady
	PhaseInProgress
)

// launchFirmware starts the scenario's firmware. Tests replace it so Start
// doesn't boot QEMU.
var launchFirmware = emulator.LoadAndStartFirmware

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

		scenario, err := LoadScenario(filepath.Join(scenariosDir, scenarioDir.Name()))

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
		return fmt.Errorf("Invalid scenario id %s", scenarioId)
	}

	s.Scenario = scenario
	s.grader = NewGrader(s, net.JoinHostPort(emulator.GDBTCPAddr, strconv.Itoa(emulator.GDBTCPPort)))

	evaluation, err := NewEvaluation(scenario)
	if err != nil {
		return err
	}
	s.evaluation = evaluation

	s.phase = PhaseReady

	return nil
}

func (s *State) Start(downlinkChan chan DownlinkMessage, uplinkChan chan []byte, done chan struct{}) error {
	if s.phase != PhaseReady {
		return unexpectedPhase("starting game")
	}

	s.evaluation.reset()
	s.phase = PhaseInProgress

	emulatorDownlinkChan := make(chan []byte)

	go launchFirmware(
		filepath.Join(s.Scenario.Dir, s.Scenario.Manifest.Firmware.ROM),
		emulatorDownlinkChan,
		uplinkChan,
		done,
	)

	go s.downlinkTranslatorPump(downlinkChan, emulatorDownlinkChan)

	return nil
}

func (s *State) downlinkTranslatorPump(interfaceDownlinkChan chan DownlinkMessage,
	emulatorDownlinkChan chan []byte) {
	app.GameLogger.Println("Starting downlink translator pump")

	for downlinkMessage := range emulatorDownlinkChan {
		line := strings.TrimSpace(string(downlinkMessage))

		var err error

		if strings.HasPrefix(line, "TLM ") {
			frame, err := emulator.DecodeTelemetryFrame(line[4:])
			if err != nil {
				app.ErrorLogger.Printf("Failed to decode telemetry frame: %v", err)
				continue
			}

			emulator.PrintTelemetryFrame(frame)

			if err := s.grader.Grade(frame); err != nil {
				app.ErrorLogger.Printf("Grading: %v", err)
			}

			// TODO: send state downlink

			interfaceDownlinkChan <- &TLMDownlinkMessage{
				TLM:       line,
				Telemetry: frame,
			}
		} else {
			app.ServerLogger.Printf("Command Response: `%s`\n", line)

			interfaceDownlinkChan <- &CommandResponseDownlinkMessage{
				Response: line,
			}
		}

		if err != nil {
			app.ServerLogger.Printf("Error writing JSON, probably WS connection closed: %+v\n", err)
			break
		}
	}

	close(interfaceDownlinkChan)

	app.GameLogger.Println("Exiting downlink translator pump")
}

func (s *State) Stop() error {
	if s.phase != PhaseInProgress {
		return unexpectedPhase("stopping game")
	}

	s.phase = PhaseLevelSelect

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

	return s.evaluation.Evaluate(frame)
}

func (s *State) ObjectiveStatus(scenario *Scenario, objective Objective) ObjectiveStatus {
	if s.Scenario == scenario {
		if state, ok := s.evaluation.GetObjectiveState(objective.ID); ok {
			return state.Status
		}
	}

	return initialObjectiveStatus(objective)
}

func (s *State) MemoryRanges() ([]emulator.MemoryRange, error) {
	if s.Scenario == nil {
		return nil, errors.New("no scenario selected")
	}
	return s.evaluation.MemoryRanges(), nil
}

func unexpectedPhase(phase string) error {
	return errors.New(fmt.Sprintf("Unexpected game state when %s", phase))
}
