package game

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sojourn/app"
	"sojourn/emulator"
	"strconv"
	"strings"
	"time"
)

type State struct {
	phase Phase

	Scenario *Scenario

	evaluation Evaluation
	grader     *Grader
	allowance  *allowance

	AvailableScenarios map[string]*Scenario

	// StrictValidation fails packages that have validation warnings.
	StrictValidation bool

	SaveDir string
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
			path := filepath.Join(scenariosDir.Dir, scenarioDir.Name())
			extracted := ""

			// A package is a directory, or a .tar.gz archive whose root
			// is the package root.
			switch {
			case scenarioDir.IsDir():
			case scenarioDir.Type().IsRegular() && strings.HasSuffix(scenarioDir.Name(), ".tar.gz"):
				var err error
				extracted, err = extractScenarioArchive(path)
				if err != nil {
					app.ErrorLogger.Printf("Skipping invalid scenario archive %q: %v", scenarioDir.Name(), err)
					continue
				}
				path = extracted
			default:
				continue
			}

			scenario, err := LoadScenario(path)

			if err != nil {
				app.ErrorLogger.Printf(
					"Skipping invalid scenario %q: %v",
					scenarioDir.Name(),
					err,
				)
				if extracted != "" {
					os.RemoveAll(extracted)
				}
				continue
			}

			if len(scenario.Warnings) > 0 && s.StrictValidation {
				app.ErrorLogger.Printf("Skipping scenario %q in strict mode: %s",
					scenarioDir.Name(), strings.Join(scenario.Warnings, "; "))
				continue
			}
			for _, warning := range scenario.Warnings {
				app.ErrorLogger.Printf("Warning: scenario %q: %s", scenarioDir.Name(), warning)
			}

			s.AvailableScenarios[scenario.Manifest.ID] = scenario
		}
	}

	if len(s.AvailableScenarios) == 0 {
		app.ErrorLogger.Println(
			"Warning: No valid scenarios were loaded",
		)
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

	var session *Session
	if s.SaveDir != "" {
		session, err = NewSession(s.SaveDir, s.Scenario)
		if err != nil {
			return fmt.Errorf("recording session: %w", err)
		}
		app.GameLogger.Printf("Recording session to %s", session.Dir)
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

	link := s.Scenario.Manifest.Link
	s.allowance = newAllowance(link.Budget)

	admittedUplinkChan := make(chan []byte)
	delayedUplinkChan := make(chan []byte)
	translatedDownlinkChan := make(chan DownlinkMessage)
	delayedDownlinkChan := make(chan DownlinkMessage)
	refusals := make(chan DownlinkMessage)
	refusalsDone := make(chan struct{})

	go admitUplink(uplinkChan, admittedUplinkChan, s.allowance, refusals, refusalsDone)
	go delayed(admittedUplinkChan, delayedUplinkChan, time.Duration(link.UplinkDelayS)*linkDelayUnit, done)
	setup := newSetupLink()
	go uplinkForwarder(delayedUplinkChan, emulatorUplinkChan, pendingCommands, booted, setupCommands,
		setup, done, probeUptime(s.grader))

	boot := &bootGate{booted: booted, deadline: time.Now().Add(appImageCheckWindow), stop: done}
	go s.downlinkTranslatorPump(translatedDownlinkChan, emulatorDownlinkChan, s.grader, pendingCommands, boot,
		setup, session)
	go delayed(translatedDownlinkChan, delayedDownlinkChan, time.Duration(link.DownlinkDelayS)*linkDelayUnit, done)
	go mergeDownlink(delayedDownlinkChan, refusals, downlinkChan, refusalsDone)

	return nil
}

var linkDelayUnit = time.Second

const delayQueueSize = 1024

func delayed[T any](in <-chan T, out chan<- T, delay time.Duration, stop <-chan struct{}) {
	type stamped struct {
		message T
		due     time.Time
	}

	queue := make(chan stamped, delayQueueSize)
	go func() {
		defer close(queue)
		for message := range in {
			queue <- stamped{message, time.Now().Add(delay)}
		}
	}()

	defer close(out)
	for s := range queue {
		if wait := time.Until(s.due); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-stop:
				timer.Stop()
			}
		}
		out <- s.message
	}
}

func mergeDownlink(delayedDownlinkChan <-chan DownlinkMessage, immediate <-chan DownlinkMessage,
	out chan<- DownlinkMessage, done chan<- struct{}) {
	defer close(done)
	defer close(out)

	for {
		select {
		case message, ok := <-delayedDownlinkChan:
			if !ok {
				return
			}
			out <- message
		case message := <-immediate:
			out <- message
		}
	}
}

type pendingCommand struct {
	command CommandLogEntry
}

// setupLink carries the probe's replies to setup writes from the downlink to
// the uplink. Until done is closed, every command reply belongs to setup: the
// player's commands are held back, so nothing else can be in flight.
type setupLink struct {
	replies chan string
	done    chan struct{}
	timeout time.Duration
}

func newSetupLink() *setupLink {
	return &setupLink{
		replies: make(chan string, pendingCommandLimit),
		done:    make(chan struct{}),
		timeout: setupReplyTimeout,
	}
}

// active reports whether command replies still belong to setup.
func (l *setupLink) active() bool {
	if l == nil {
		return false
	}
	select {
	case <-l.done:
		return false
	default:
		return true
	}
}

var setupReplyTimeout = 2 * time.Second

const setupAttempts = 3

func applySetup(setupCommands []string, emulatorUplinkChan chan<- []byte, setup *setupLink,
	stop <-chan struct{}) bool {
	ok := true

	for _, raw := range setupCommands {
		app.GameLogger.Printf("Applying setup write: %s", raw)

		reply, answered := "", false
		for attempt := 1; attempt <= setupAttempts && !answered; attempt++ {
			emulatorUplinkChan <- []byte(raw)

			timer := time.NewTimer(setup.timeout)
			select {
			case reply = <-setup.replies:
				answered = true
			case <-timer.C:
				app.ErrorLogger.Printf("Setup write `%s` got no reply (attempt %d of %d)", raw, attempt, setupAttempts)
			case <-stop:
				timer.Stop()
				return false
			}
			timer.Stop()
		}

		switch {
		case !answered:
			app.ErrorLogger.Printf("Setup write `%s` was never acknowledged: the scenario is not set up as designed", raw)
			ok = false
		case !strings.HasPrefix(reply, "ACK "):
			app.ErrorLogger.Printf("Setup write `%s` failed: %s", raw, reply)
			ok = false
		}
	}

	return ok
}

func probeUptime(grader *Grader) func() *uint64 {
	if grader == nil {
		return func() *uint64 { return nil }
	}
	return grader.UptimeMS
}

func uplinkForwarder(playerUplinkChan <-chan []byte, emulatorUplinkChan chan<- []byte,
	pendingCommands chan<- pendingCommand, booted <-chan struct{}, setupCommands []string,
	setup *setupLink, stop <-chan struct{}, uptime func() *uint64) {
	defer close(emulatorUplinkChan)

	send := func(raw string) {
		command := parseCommand(raw)
		command.TMs = uptime()

		select {
		case pendingCommands <- pendingCommand{command: command}:
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

	if len(setupCommands) > 0 && applySetup(setupCommands, emulatorUplinkChan, setup, stop) {
		app.GameLogger.Printf("Applied %d setup writes", len(setupCommands))
	}
	close(setup.done)

	select {
	case <-stop:
		return
	default:
	}

	for _, raw := range held {
		send(raw)
	}

	for uplinkMessage := range playerUplinkChan {
		send(strings.TrimSpace(string(uplinkMessage)))
	}
}

type bootGate struct {
	booted   chan<- struct{}
	deadline time.Time
	stop     <-chan struct{}
}

func (s *State) downlinkTranslatorPump(interfaceDownlinkChan chan DownlinkMessage,
	emulatorDownlinkChan chan []byte, grader *Grader, pendingCommands <-chan pendingCommand,
	boot *bootGate, setup *setupLink, session *Session) {
	app.GameLogger.Println("Starting downlink translator pump")

	var bootTimeout <-chan time.Time
	if boot != nil {
		timer := time.NewTimer(time.Until(boot.deadline))
		defer timer.Stop()
		bootTimeout = timer.C
	}

	refused := false
	refuse := func(mismatch *AppImageMismatch) {
		refused = true
		select {
		case <-boot.stop:
			return
		default:
		}
		app.ErrorLogger.Printf("%s (declared %s, reported %q)", mismatch.Reason, mismatch.Declared, mismatch.Reported)
		interfaceDownlinkChan <- &ScenarioRefusedDownlinkMessage{
			Reason:   mismatch.Reason,
			Declared: mismatch.Declared,
			Reported: mismatch.Reported,
		}
	}

pump:
	for {
		var downlinkMessage []byte
		select {
		case message, ok := <-emulatorDownlinkChan:
			if !ok {
				break pump
			}
			downlinkMessage = message
		case <-bootTimeout:
			bootTimeout = nil
			refuse(appImageUnconfirmed(s.appImageDeclared(), errors.New("no boot banner")))
			continue
		}
		if refused {
			continue
		}

		line := strings.TrimSpace(string(downlinkMessage))

		if boot != nil && boot.booted != nil && strings.HasPrefix(line, bootBanner) {
			bootTimeout = nil
			if mismatch := s.verifyAppImage(boot.deadline, boot.stop); mismatch != nil {
				refuse(mismatch)
				continue
			}
			app.GameLogger.Printf("Probe booted application image %s", s.Scenario.Manifest.Firmware.AppCRC32)
			close(boot.booted)
			boot.booted = nil
		}

		if strings.HasPrefix(line, "TLM ") {
			frame, err := emulator.DecodeTelemetryFrame(line[4:])
			if err != nil {
				app.ErrorLogger.Printf("Failed to decode telemetry frame: %v", err)
				continue
			}

			if err := grader.Grade(frame); err != nil {
				app.ErrorLogger.Printf("Grading: %v", err)
			}

			for _, transition := range s.evaluation.Transitions() {
				app.GameLogger.Printf("Objective %s: %s -> %s", transition.Objective, transition.From, transition.To)
				if err := session.AppendObjective(transition); err != nil {
					app.ErrorLogger.Printf("Recording objective %s: %v", transition.Objective, err)
				}
			}

			s.sendStateDownlink(interfaceDownlinkChan)

			interfaceDownlinkChan <- &TLMDownlinkMessage{
				TLM:       line,
				Telemetry: frame.WithChannels(s.Scenario.Manifest.Console.Decode),
			}
		} else {
			app.ServerLogger.Printf("Command Response: `%s`\n", line)

			if isCommandResponse(line) && setup.active() {
				select {
				case setup.replies <- line:
				default:
					app.ErrorLogger.Printf("Setup reply `%s` with no setup write waiting", line)
				}
				continue
			}

			if isCommandResponse(line) {
				select {
				case pending := <-pendingCommands:
					pending.command.Result = line
					if grader != nil {
						grader.LogCommand(pending.command)
					}
					if err := session.Append(pending.command); err != nil {
						app.ErrorLogger.Printf("Recording `%s`: %v", pending.command.Raw, err)
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

	// Commands the probe never answered, such as one that crashed it, still
	// happened and are evidence.
	for unanswered := true; unanswered; {
		select {
		case pending := <-pendingCommands:
			if err := session.Append(pending.command); err != nil {
				app.ErrorLogger.Printf("Recording `%s`: %v", pending.command.Raw, err)
			}
		default:
			unanswered = false
		}
	}
	if err := session.Close(); err != nil {
		app.ErrorLogger.Printf("Closing command log: %v", err)
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
		hints := []string{}
		if s.evaluation.objectiveStates[objective.ID].Status == ObjectiveStatusActive {
			for _, hint := range objective.Hints {
				if s.evaluation.objectiveStates[objective.ID].FramesActive >= hint.AfterFrames {
					hints = append(hints, hint.Text)
				}
			}
		}
		stateMessage.Objectives = append(stateMessage.Objectives,
			ObjectiveStateDownlinkMessage{
				ID:         objective.ID,
				Status:     s.evaluation.objectiveStates[objective.ID].Status,
				Diagnostic: s.evaluation.objectiveStates[objective.ID].Diagnostic,
				Hints:      hints,
			})
	}
	stateMessage.Constraints = s.allowance.remaining()
	stateMessage.Grading = s.grader.Status()
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

var (
	ErrNoGameInProgress       = errors.New("no game in progress")
	ErrProbeMemoryUnavailable = errors.New("the probe's memory can't be read right now")
)

func (s *State) GoldenImage() ([]byte, error) {
	if s.phase != PhaseInProgress {
		return nil, ErrNoGameInProgress
	}

	return emulator.AppImage(filepath.Join(s.Scenario.Dir, s.Scenario.Manifest.Firmware.ROM))
}

func (s *State) RunningImage() ([]byte, error) {
	golden, err := s.GoldenImage()
	if err != nil {
		return nil, err
	}
	return s.readAppRAM(len(golden))
}

func (s *State) PatchPokes(patched []byte) ([]string, error) {
	golden, err := s.GoldenImage()
	if err != nil {
		return nil, err
	}
	if len(patched) > appRAMEnd-appRAMBase {
		return nil, ErrPatchTooLarge
	}

	running, err := s.readAppRAM(max(len(golden), len(patched)))
	if err != nil {
		return nil, err
	}
	return patchPokes(running, patched)
}

func (s *State) RemainingWrites() (int, bool) {
	remaining, metered := s.allowance.remaining()[ResourceWrites]
	return remaining, metered
}

func (s *State) readAppRAM(length int) ([]byte, error) {
	if s.grader == nil {
		return nil, ErrProbeMemoryUnavailable
	}
	data, err := s.grader.ReadAppRAM(length)
	if err != nil {
		app.ErrorLogger.Printf("Reading APP RAM: %v", err)
		return nil, ErrProbeMemoryUnavailable
	}
	return data, nil
}

func (s *State) Evaluate(frame Frame) error {
	if s.phase != PhaseInProgress {
		return unexpectedPhase("evaluating a frame")
	}

	return s.evaluation.Evaluate(frame)
}

func (s *State) EvaluateTelemetryOnly(frame Frame) error {
	if s.phase != PhaseInProgress {
		return unexpectedPhase("evaluating a frame")
	}

	return s.evaluation.EvaluateTelemetryOnly(frame)
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

// maxScenarioArchiveBytes caps what one archive may unpack to.
const maxScenarioArchiveBytes = 64 << 20

// extractScenarioArchive unpacks a .tar.gz scenario package into a fresh
// temporary directory and returns it. Archives are untrusted: entries may only
// be directories and regular files, and must stay inside the package root.
func extractScenarioArchive(path string) (_ string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tmp, err := os.MkdirTemp("", "sojourn-scenario-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tmp)
		}
	}()

	tr := tar.NewReader(gz)
	var total int64
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return tmp, nil
		}
		if err != nil {
			return "", err
		}

		name := filepath.Clean(strings.TrimPrefix(header.Name, "./"))
		if name == "." {
			continue
		}
		if !filepath.IsLocal(name) {
			return "", fmt.Errorf("entry %q is outside the package root", header.Name)
		}
		target := filepath.Join(tmp, name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			total += header.Size
			if total > maxScenarioArchiveBytes {
				return "", fmt.Errorf("archive unpacks to more than %d bytes", maxScenarioArchiveBytes)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if err != nil {
				return "", err
			}
			_, err = io.Copy(out, io.LimitReader(tr, header.Size))
			if closeErr := out.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("entry %q is not a regular file or directory", header.Name)
		}
	}
}
