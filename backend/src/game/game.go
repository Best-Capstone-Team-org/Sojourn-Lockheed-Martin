package game

import (
	"errors"
	"fmt"
	"io/fs"
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

type ScenariosDir struct {
	Dir      string
	Required bool
}

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

func (s *State) LoadScenarios(scenariosDirs []ScenariosDir) error {
	if s.phase != PhaseNone {
		return unexpectedPhase("loading scenarios")
	}

	for _, scenariosDir := range scenariosDirs {
		files, err := os.ReadDir(scenariosDir.Dir)
		if err != nil {
			if !scenariosDir.Required && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}

		for _, scenarioDir := range files {
			if !scenarioDir.IsDir() {
				continue
			}

			scenario, err := LoadScenario(filepath.Join(scenariosDir.Dir, scenarioDir.Name()))

			if err != nil {
				return err
			}

			s.AvailableScenarios[scenario.Manifest.ID] = scenario
		}
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

	setupCommands, err := s.Scenario.setupCommands()
	if err != nil {
		return err
	}

	s.evaluation.reset()
	s.phase = PhaseInProgress

	emulatorDownlinkChan := make(chan []byte)
	emulatorUplinkChan := make(chan []byte)
	pendingCommands := make(chan pendingCommand, pendingCommandLimit)
	booted := make(chan struct{})

	go launchFirmware(
		filepath.Join(s.Scenario.Dir, s.Scenario.Manifest.Firmware.ROM),
		emulatorDownlinkChan,
		emulatorUplinkChan,
		done,
	)

	go uplinkForwarder(uplinkChan, emulatorUplinkChan, pendingCommands, booted, setupCommands)
	go s.downlinkTranslatorPump(downlinkChan, emulatorDownlinkChan, s.grader, pendingCommands, booted)

	return nil
}

type pendingCommand struct {
	command CommandLogEntry
	setup   bool
}

func uplinkForwarder(playerUplinkChan <-chan []byte, emulatorUplinkChan chan<- []byte,
	pendingCommands chan<- pendingCommand, booted <-chan struct{}, setupCommands []string) {
	defer close(emulatorUplinkChan)

	send := func(raw string, setup bool) {
		select {
		case pendingCommands <- pendingCommand{command: parseCommand(raw), setup: setup}:
		default:
			app.ErrorLogger.Printf("Too many unanswered commands, not logging `%s`", raw)
		}
		emulatorUplinkChan <- []byte(raw)
	}

	var held []string

waitForBoot:
	for {
		select {
		case <-booted:
			break waitForBoot
		case uplinkMessage, ok := <-playerUplinkChan:
			if !ok {
				return
			}
			held = append(held, strings.TrimSpace(string(uplinkMessage)))
		}
	}

	for _, raw := range setupCommands {
		app.GameLogger.Printf("Applying setup write: %s", raw)
		send(raw, true)
	}

	for _, raw := range held {
		send(raw, false)
	}

	for uplinkMessage := range playerUplinkChan {
		send(strings.TrimSpace(string(uplinkMessage)), false)
	}
}

func (s *State) downlinkTranslatorPump(interfaceDownlinkChan chan DownlinkMessage,
	emulatorDownlinkChan chan []byte, grader *Grader, pendingCommands <-chan pendingCommand,
	booted chan<- struct{}) {
	app.GameLogger.Println("Starting downlink translator pump")

	for downlinkMessage := range emulatorDownlinkChan {
		line := strings.TrimSpace(string(downlinkMessage))

		if booted != nil && strings.HasPrefix(line, bootBanner) {
			close(booted)
			booted = nil
		}

		if strings.HasPrefix(line, "TLM ") {
			frame, err := emulator.DecodeTelemetryFrame(line[4:])
			if err != nil {
				app.ErrorLogger.Printf("Failed to decode telemetry frame: %v", err)
				continue
			}

			emulator.PrintTelemetryFrame(frame)

			if err := grader.Grade(frame); err != nil {
				app.ErrorLogger.Printf("Grading: %v", err)
			}

			s.sendStateDownlink(interfaceDownlinkChan)

			interfaceDownlinkChan <- &TLMDownlinkMessage{
				TLM:       line,
				Telemetry: frame,
			}
		} else {
			app.ServerLogger.Printf("Command Response: `%s`\n", line)

			if isCommandResponse(line) {
				select {
				case pending := <-pendingCommands:
					if pending.setup {
						if !strings.HasPrefix(line, "ACK ") {
							app.ErrorLogger.Printf("Setup write `%s` failed: %s", pending.command.Raw, line)
						}
						continue
					}

					pending.command.Result = line
					if grader != nil {
						grader.LogCommand(pending.command)
					}
				default:
					app.ErrorLogger.Printf("Command response `%s` with no pending command", line)
				}
			}

			interfaceDownlinkChan <- &CommandResponseDownlinkMessage{
				Response: line,
			}
		}
	}

	if grader != nil {
		grader.Close()
	}

	close(interfaceDownlinkChan)

	app.GameLogger.Println("Exiting downlink translator pump")
}

func (s *State) DeselectScenario() error {
	if s.phase != PhaseReady {
		return unexpectedPhase("deselecting a scenario")
	}

	s.Scenario = nil
	s.grader = nil
	s.phase = PhaseLevelSelect

	return nil
}

func (s *State) sendStateDownlink(interfaceDownlinkChan chan DownlinkMessage) {
	stateMessage := &StateDownlinkMessage{}
	for _, objective := range s.Scenario.Objectives {
		stateMessage.Objectives = append(stateMessage.Objectives,
			ObjectiveStateDownlinkMessage{
				ID:     objective.ID,
				Status: s.evaluation.objectiveStates[objective.ID].Status,
			})
	}
	stateMessage.Constraints = map[string]int{}
	interfaceDownlinkChan <- stateMessage
}

func (s *State) Stop() error {
	if s.phase != PhaseInProgress {
		return unexpectedPhase("stopping game")
	}

	s.phase = PhaseLevelSelect
	s.evaluation.reset()

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
