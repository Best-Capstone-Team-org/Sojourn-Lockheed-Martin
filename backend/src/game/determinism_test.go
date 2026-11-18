package game

import (
	"encoding/hex"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Charter R24.21: objective evaluation is a pure function of setup state, the
// command log, and probe behavior. These tests hold the probe's behavior fixed
// (the frames it produces in response to each command) and check that the
// evaluator itself adds nothing: no clock, no random source, no dependence on
// map order or on how fast the host runs.

// determinismLog is the command log being replayed: 24 commands, delivered
// one per frame.
var determinismLog = []struct{ raw, result string }{
	{"PING", "ACK PING"},
	{"STAT", "ACK STAT mode=1 up=5s reboots=0 fault=0 load=775mW"},
	{"PEEK 0x20000010 1", "ACK PEEK 00"},
	{"PEEK 0x20000020 4", "ACK PEEK 00000000"},
	{"SAFE", "ACK SAFE"},
	{"PING", "ACK PING"},
	{"POKE 0x20000010 01", "ACK POKE 1"},
	{"PEEK 0x20000010 1", "ACK PEEK 01"},
	{"POKE 0x00000000 FF", "NAK E04"},
	{"POKE 0x20000020 AABBCCDD", "ACK POKE 4"},
	{"STAT", "ACK STAT mode=2 up=50s reboots=0 fault=0 load=640mW"},
	{"POKE 0x20000010 02", "ACK POKE 1"},
	{"PING", "ACK PING"},
	{"PEEK 0x20000010 1", "ACK PEEK 02"},
	{"PING", "ACK PING"},
	{"STAT", "ACK STAT mode=2 up=80s reboots=0 fault=0 load=640mW"},
	{"CALL 0x20001000", "ACK CALL 0"},
	{"TRIM", "ACK TRIM"},
	{"PEEK 0x20000020 4", "ACK PEEK AABBCCDD"},
	{"PING", "ACK PING"},
	{"POKE 0x20000010 03", "ACK POKE 1"},
	{"PING", "ACK PING"},
	{"STAT", "ACK STAT mode=2 up=110s reboots=0 fault=0 load=640mW"},
	{"PING", "ACK PING"},
}

// determinismObjectives exercise every kind of predicate state can depend on.
func determinismObjectives(t *testing.T) []Objective {
	modeIs := func(v float64) Predicate {
		return Predicate{Op: "mem_u8", At: &AddressRef{Sym: "g_mode"}, Cmp: "eq", Value: v}
	}
	return []Objective{
		{ID: "contact", Success: Predicate{Op: "ever", Of: rawPredicate(t,
			Predicate{Op: "commanded", Verb: "PING", Result: "ACK"})}},
		{ID: "safe-mode", Requires: []string{"contact"},
			Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"}},
		{ID: "patched", Requires: []string{"safe-mode"},
			Success: Predicate{Op: "mem_changed", At: &AddressRef{Sym: "g_buf"}, Len: 4},
			Partial: []Partial{{When: modeIs(1), Text: "The mode changed, but the buffer has not."}}},
		{ID: "hold", Success: Predicate{Op: "sustained", Frames: 3, Of: rawPredicate(t, modeIs(2))}},
		{ID: "budget", Requires: []string{"hold"},
			Success: Predicate{Op: "budget", Resource: ResourceWrites, Cmp: "lte", Value: 6.0}},
		{ID: "careful",
			Success: Predicate{Op: "commanded", Verb: "TRIM", Result: "ACK"},
			Fail:    &Predicate{Op: "commanded", Verb: "CALL", Result: "ACK"}},
		{ID: "hot", Retractable: true,
			Success: Predicate{Op: "within", Frames: 2, Of: rawPredicate(t,
				Predicate{Op: "tlm", Path: "temp", Cmp: "gt", Value: 70.0})}},
	}
}

// determinismFrames is the probe's fixed behavior: frame i follows command i,
// with memory and telemetry reflecting what the commands so far did.
func determinismFrames(t *testing.T) []Frame {
	t.Helper()
	mem := sram(0)
	mode := "NOMINAL"
	var log []CommandLogEntry
	var frames []Frame

	for i, c := range determinismLog {
		entry := parseCommand(c.raw)
		entry.Result = c.result
		log = append(log, entry)

		if strings.HasPrefix(c.result, "ACK") {
			switch entry.Verb {
			case "SAFE":
				mode = "SAFE"
			case "POKE":
				data, err := hex.DecodeString(strings.Fields(c.raw)[2])
				if err != nil {
					t.Fatal(err)
				}
				copy(mem[entry.Addr-0x20000000:], data)
			}
		}

		frames = append(frames, Frame{
			CRCOK:   true,
			UptimeS: uint32(5 * (i + 1)),
			Telemetry: map[string]any{
				"mode": mode,
				"temp": float64((i * 37) % 100),
			},
			Log:    slices.Clone(log),
			Budget: budgetUsed(log),
			Memory: slices.Clone(mem),
		})
	}
	return frames
}

type replayResult struct {
	Transitions [][]ObjectiveTransition
	Errors      []string
	Final       map[string]ObjectiveState
}

// replay evaluates the frames through a fresh evaluation, as a replayed
// session does.
func replay(t *testing.T, frames []Frame) replayResult {
	t.Helper()
	e := frameEvaluation(t, determinismObjectives(t)...)
	e.scenario.Setup = &Setup{SettleFrames: 1}

	var r replayResult
	for _, frame := range frames {
		err := e.Evaluate(frame)
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		r.Errors = append(r.Errors, msg)
		r.Transitions = append(r.Transitions, e.Transitions())
	}

	r.Final = map[string]ObjectiveState{}
	for id, state := range e.objectiveStates {
		r.Final[id] = *state
	}
	return r
}

func TestEvaluationIsDeterministic(t *testing.T) {
	if len(determinismLog) < 20 {
		t.Fatalf("the log has %d commands; R24.21 asks for at least 20", len(determinismLog))
	}
	frames := determinismFrames(t)

	first := replay(t, frames)

	// Twice is the requirement; more replays also catch anything that only
	// varies sometimes, such as map iteration order.
	for i := 2; i <= 10; i++ {
		again := replay(t, frames)
		if !reflect.DeepEqual(again.Final, first.Final) {
			t.Fatalf("replay %d reached different objective states:\n%+v\nwant\n%+v", i, again.Final, first.Final)
		}
		if !reflect.DeepEqual(again.Transitions, first.Transitions) {
			t.Fatalf("replay %d changed objectives on different frames", i)
		}
		if !slices.Equal(again.Errors, first.Errors) {
			t.Fatalf("replay %d failed differently: %q vs %q", i, again.Errors, first.Errors)
		}
	}

	// Guard against a vacuous pass: the replay must actually move every kind
	// of objective, so identical results mean something.
	for id, want := range map[string]ObjectiveStatus{
		"contact":   ObjectiveStatusComplete,
		"safe-mode": ObjectiveStatusComplete,
		"patched":   ObjectiveStatusComplete,
		"hold":      ObjectiveStatusComplete,
		"budget":    ObjectiveStatusComplete,
		"careful":   ObjectiveStatusFailed,
	} {
		state := first.Final[id]
		if state.Status != want {
			t.Errorf("%s = %s, want %s", id, state.Status, want)
		}
		if want == ObjectiveStatusComplete && state.FirstFrame == 0 {
			t.Errorf("%s completed without a first-completion frame", id)
		}
	}
	if hot := first.Final["hot"]; hot.FirstFrame == 0 {
		t.Error("the retractable objective never completed")
	}
	retractions := 0
	for _, frame := range first.Transitions {
		for _, transition := range frame {
			if transition.Objective == "hot" && transition.From == ObjectiveStatusComplete {
				retractions++
			}
		}
	}
	if retractions == 0 {
		t.Error("the retractable objective never retracted")
	}
	for i, msg := range first.Errors {
		if msg != "" {
			t.Errorf("frame %d failed to evaluate: %s", i+1, msg)
		}
	}
}

// The evaluator must not be able to consult a clock or a random source at all.
func TestEvaluationImportsNoClockOrRandomness(t *testing.T) {
	forbidden := []string{"time", "math/rand", "math/rand/v2", "crypto/rand"}
	for _, file := range []string{"frame.go", "predicate.go", "predicate_eval.go", "address.go", "scenario.go"} {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, imp := range parsed.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if slices.Contains(forbidden, path) {
				t.Errorf("%s imports %q; objective evaluation must not depend on it (R24.21)", file, path)
			}
		}
	}
}

// determinismBursts is the command log played on the firmware: 24 commands
// in two bursts, the first after frame 1 and the second after frame 2. The
// first only observes; the second sets a marker byte (the camera easter egg's
// odds, read only when the camera captures, every 60 s) and powers the IMU
// down.
var determinismBursts = [][]string{
	{
		"PING", "STAT", "PEEK 0x00000000 4", "PING", "STAT", "PEEK 0x200191C8 4",
		"PING", "STAT", "PEEK 0x2001E010 4", "PING", "STAT", "PEEK 0x200191C8 1",
	},
	{
		"POKE 0x200191C8 2A", "PEEK 0x200191C8 1", "POKE 0x2001E010 00", "PING",
		"STAT", "PEEK 0x2001E010 4", "PING", "STAT", "PEEK 0x200191C8 4", "PING",
		"STAT", "PING",
	},
}

// determinismObjectivesJSON resolve within four frames of boot.
const determinismObjectivesJSON = `{"format": 1, "objectives": [
	{"id": "contact", "title": "c", "brief": "c", "hints": [{"after_frames": 9, "text": "h"}],
	 "success": {"op": "ever", "of": {"op": "commanded", "verb": "PING", "result": "ACK"}}},
	{"id": "marked", "title": "m", "brief": "m", "requires": ["contact"], "hints": [{"after_frames": 9, "text": "h"}],
	 "success": {"op": "mem_u8", "at": {"sym": "g_cam_egg_pct"}, "cmp": "eq", "value": 42}},
	{"id": "changed", "title": "c", "brief": "c", "hints": [{"after_frames": 9, "text": "h"}],
	 "success": {"op": "mem_changed", "at": {"sym": "g_cam_egg_pct"}, "len": 4}},
	{"id": "steady", "title": "s", "brief": "s", "requires": ["marked"], "hints": [{"after_frames": 9, "text": "h"}],
	 "success": {"op": "sustained", "frames": 2, "of": {"op": "all", "of": [
		{"op": "channel_present", "id": "MAG"},
		{"op": "mem_u8", "at": {"sym": "g_cam_egg_pct"}, "cmp": "eq", "value": 42}]}}},
	{"id": "imu-off", "title": "i", "brief": "i", "hints": [{"after_frames": 9, "text": "h"}],
	 "success": {"op": "sustained", "frames": 2, "of": {"op": "channel_absent", "id": "IMU"}}}
]}`

// determinismScenario writes a package around the real firmware whose
// objectives cover the command log, memory, baselines, requires and
// sustained, and all resolve within a few frames.
func determinismScenario(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(commsTriageDir); err != nil {
		t.Skipf("scenario firmware not checked out: %v", err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "determinism")

	files := map[string]string{
		"manifest.json": `{"format": 1, "id": "sojourn.test.determinism", "title": "Determinism",
			"revision": 1, "summary": "R24.21", "briefing": "briefing.md", "objectives": "objectives.json",
			"firmware": {"rom": "firmware/probe_rom.elf", "symbols": "firmware/symbols.json",
				"memmap": "firmware/memmap.json", "app_crc32": "0xf72478e0"},
			"link": {"uplink_delay_s": 0, "downlink_delay_s": 0}}`,
		"briefing.md":     "# Determinism\n",
		"objectives.json": determinismObjectivesJSON,
	}
	for _, name := range []string{"probe_rom.elf", "symbols.json", "memmap.json"} {
		data, err := os.ReadFile(filepath.Join(commsTriageDir, "firmware", name))
		if err != nil {
			t.Fatal(err)
		}
		files["firmware/"+name] = string(data)
	}
	writeScenario(t, dir, files)
	return root
}

// playthroughRecord is what a playthrough leaves on disk: every objective
// transition with the frame and probe uptime it happened at.
type playthroughRecord struct {
	transitions []string
	final       map[string]ObjectiveStatus
	replies     []string
	telemetry   []string
}

// playOnFirmware plays the bursts against real firmware through the game
// pipeline. Each burst starts the moment a frame has been graded, so it
// reaches the probe at a frame boundary, and each command waits for the
// previous one's reply: the firmware drops a line that arrives while another
// is still pending.
func playOnFirmware(t *testing.T, scenarios string, bursts [][]string, frames int) playthroughRecord {
	t.Helper()

	state := InitState()
	state.SaveDir = t.TempDir()
	if err := state.LoadScenarios([]ScenariosDir{{Dir: scenarios, Required: true}}); err != nil {
		t.Fatalf("LoadScenarios: %v", err)
	}
	if err := state.SelectScenario("sojourn.test.determinism"); err != nil {
		t.Fatalf("SelectScenario: %v", err)
	}

	out := make(chan DownlinkMessage)
	up := make(chan []byte, 1)
	done := make(chan struct{})
	if err := state.Start(out, up, done); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var record playthroughRecord
	var queue []string
	awaiting := false
	sendNext := func() {
		if !awaiting && len(queue) > 0 {
			up <- []byte(queue[0])
			queue = queue[1:]
			awaiting = true
		}
	}

	graded := 0
	deadline := time.After(time.Duration(frames+4) * 5 * time.Second)
loop:
	for {
		select {
		case msg := <-out:
			switch m := msg.(type) {
			case *CommandResponseDownlinkMessage:
				if isCommandResponse(m.Response) {
					record.replies = append(record.replies, m.Response)
					awaiting = false
					sendNext()
				}
			case *TLMDownlinkMessage:
				record.telemetry = append(record.telemetry, m.TLM)
			case *StateDownlinkMessage:
				graded++
				record.final = map[string]ObjectiveStatus{}
				for _, o := range m.Objectives {
					record.final[o.ID] = o.Status
				}
				if graded >= frames {
					break loop
				}
				if graded <= len(bursts) {
					queue = append(queue, bursts[graded-1]...)
					sendNext()
				}
			}
		case <-deadline:
			t.Fatalf("playthrough timed out after %d frames", graded)
		}
	}

	close(done)
	close(up)
	for range out {
	}

	sessions, _ := filepath.Glob(filepath.Join(state.SaveDir, "sessions", "*"))
	if len(sessions) != 1 {
		t.Fatalf("sessions = %v, want one", sessions)
	}
	for _, r := range readLog(t, filepath.Join(sessions[0], "objectives.jsonl")) {
		record.transitions = append(record.transitions,
			fmt.Sprintf("frame %v (%vs): %v %v -> %v", r["frame"], r["uptime_s"], r["objective"], r["from"], r["state"]))
	}
	return record
}

// R24.21 end to end: the same 20+ command log played twice against the real
// probe reaches identical objective states on identical frames.
func TestPlaythroughIsDeterministicOnFirmware(t *testing.T) {
	if os.Getenv("SOJOURN_FIRMWARE_TESTS") == "" {
		t.Skip("set SOJOURN_FIRMWARE_TESTS=1 to boot the real firmware")
	}
	if _, err := exec.LookPath("qemu-system-arm"); err != nil {
		t.Skip("qemu-system-arm not installed")
	}
	commands := 0
	for _, burst := range determinismBursts {
		commands += len(burst)
	}
	if commands < 20 {
		t.Fatalf("the log has %d commands; R24.21 asks for at least 20", commands)
	}

	scenarios := determinismScenario(t)
	const frames = 4

	first := playOnFirmware(t, scenarios, determinismBursts, frames)
	second := playOnFirmware(t, scenarios, determinismBursts, frames)

	t.Logf("transitions:\n  %s", strings.Join(first.transitions, "\n  "))
	if !slices.Equal(first.transitions, second.transitions) {
		t.Errorf("objectives changed differently:\nfirst\n  %s\nsecond\n  %s",
			strings.Join(first.transitions, "\n  "), strings.Join(second.transitions, "\n  "))
	}
	if !reflect.DeepEqual(first.final, second.final) {
		t.Errorf("final states differ: %v vs %v", first.final, second.final)
	}

	// Not vacuous: every command was carried out and every objective moved.
	for i, record := range []playthroughRecord{first, second} {
		if len(record.replies) != commands {
			t.Errorf("playthrough %d: %d replies for %d commands", i+1, len(record.replies), commands)
		}
		for _, reply := range record.replies {
			if !strings.HasPrefix(reply, "ACK ") {
				t.Errorf("playthrough %d: command refused: %s", i+1, reply)
			}
		}
	}
	for _, id := range []string{"contact", "marked", "changed", "steady", "imu-off"} {
		if first.final[id] != ObjectiveStatusComplete {
			t.Errorf("%s = %s, want complete", id, first.final[id])
		}
	}

	// Stronger than R24.21 asks, so reported rather than required: did the
	// probe itself behave identically, frame for frame?
	if slices.Equal(first.telemetry, second.telemetry) {
		t.Logf("telemetry identical across both playthroughs (%d frames)", len(first.telemetry))
	} else {
		t.Logf("telemetry differed between playthroughs")
	}
}
