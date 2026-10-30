package game

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testManifest = `{
	"format": 1,
	"id": "sojourn.test.%s",
	"title": "Test Scenario",
	"revision": 1,
	"summary": "Scenario used for unit testing.",
	"firmware": {
		"rom": "firmware/probe_rom.elf",
		"symbols": "firmware/symbols.json",
		"memmap": "firmware/memmap.json",
		"app_crc32": "0x12345678"
	},
	"link": {"uplink_delay_s": 8, "downlink_delay_s": 8},
	"briefing": "briefing.md",
	"objectives": "objectives.json"
}`

const testObjectives = `{
	"format": 1,
	"objectives": [
		{"id": "safe", "title": "Safe", "brief": "Enter safe mode.",
		 "success": {"op": "tlm", "path": "mode", "cmp": "eq", "value": "SAFE"}},
		{"id": "next", "title": "Next", "brief": "Then this.", "requires": ["safe"],
		 "success": {"op": "channel_present", "id": "HK"}}
	]
}`

const testSymbols = `{"format": 1, "symbols": {"g_mode": "0x20000010"}, "fields": {"g_mode": {"hi": 1}}}`

const testMemMap = `{"format": 1, "regions": [{"name": "RAM", "lo": "0x20000000", "hi": "0x2000FFFF"}]}`

func scenarioFiles(id string) map[string]string {
	return map[string]string{
		"manifest.json":          fmt.Sprintf(testManifest, id),
		"objectives.json":        testObjectives,
		"briefing.md":            "# Briefing\n",
		"firmware/probe_rom.elf": "rom",
		"firmware/symbols.json":  testSymbols,
		"firmware/memmap.json":   testMemMap,
	}
}

// writeScenario writes a scenario package into dir. A file mapped to "" is
// omitted.
func writeScenario(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, contents := range files {
		if contents == "" {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func loadedState(t *testing.T) *State {
	t.Helper()
	root := t.TempDir()
	writeScenario(t, filepath.Join(root, "alpha"), scenarioFiles("alpha"))
	writeScenario(t, filepath.Join(root, "beta"), scenarioFiles("beta"))
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("not a scenario"), 0644); err != nil {
		t.Fatal(err)
	}

	state := InitState()
	if err := state.LoadScenarios(root); err != nil {
		t.Fatalf("LoadScenarios: %v", err)
	}
	return &state
}

func TestInitState(t *testing.T) {
	state := InitState()
	if state.Phase() != PhaseNone || state.Scenario != nil || state.AvailableScenarios == nil {
		t.Fatalf("InitState = %+v", state)
	}
}

func TestLoadScenarios(t *testing.T) {
	state := loadedState(t)

	if state.Phase() != PhaseLevelSelect {
		t.Fatalf("phase = %v, want level select", state.Phase())
	}
	if len(state.AvailableScenarios) != 2 {
		t.Fatalf("loaded %d scenarios, want 2", len(state.AvailableScenarios))
	}
	for _, id := range []string{"sojourn.test.alpha", "sojourn.test.beta"} {
		if state.AvailableScenarios[id] == nil {
			t.Errorf("scenario %q not loaded", id)
		}
	}

	if err := state.LoadScenarios(t.TempDir()); err == nil {
		t.Fatal("expected LoadScenarios to fail outside PhaseNone")
	}
}

func TestLoadScenariosErrors(t *testing.T) {
	state := InitState()
	if err := state.LoadScenarios(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected error for a missing directory")
	}

	root := t.TempDir()
	files := scenarioFiles("bad")
	files["objectives.json"] = "{"
	writeScenario(t, filepath.Join(root, "bad"), files)

	state = InitState()
	if err := state.LoadScenarios(root); err == nil {
		t.Fatal("expected error for an invalid scenario")
	}
	if state.Phase() != PhaseNone {
		t.Fatalf("phase = %v, want none", state.Phase())
	}
}

func TestSelectScenario(t *testing.T) {
	if err := (&State{}).SelectScenario("x"); err == nil {
		t.Fatal("expected SelectScenario to fail outside level select")
	}

	state := loadedState(t)
	if err := state.SelectScenario("nope"); err == nil {
		t.Fatal("expected error for an unknown scenario id")
	}

	if err := state.SelectScenario("sojourn.test.alpha"); err != nil {
		t.Fatalf("SelectScenario: %v", err)
	}
	if state.Phase() != PhaseReady {
		t.Fatalf("phase = %v, want ready", state.Phase())
	}
	if state.Scenario != state.AvailableScenarios["sojourn.test.alpha"] {
		t.Fatal("selected the wrong scenario")
	}
	if state.grader == nil {
		t.Fatal("SelectScenario did not create a grader")
	}
}

func TestSelectScenarioUnresolvableAddress(t *testing.T) {
	state := InitState()
	state.phase = PhaseLevelSelect
	state.AvailableScenarios["broken"] = &Scenario{Objectives: []Objective{{
		ID:      "x",
		Success: Predicate{Op: "mem_u8", At: &AddressRef{Sym: "missing"}},
	}}}

	if err := state.SelectScenario("broken"); err == nil {
		t.Fatal("expected SelectScenario to fail on an unresolvable address")
	}
	if state.Phase() != PhaseLevelSelect {
		t.Fatalf("phase = %v, want level select", state.Phase())
	}
}

func TestObjectiveStatus(t *testing.T) {
	state := loadedState(t)
	alpha := state.AvailableScenarios["sojourn.test.alpha"]
	beta := state.AvailableScenarios["sojourn.test.beta"]

	// Without a selected scenario the status is the initial one.
	if got := state.ObjectiveStatus(alpha, alpha.Objectives[1]); got != ObjectiveStatusLocked {
		t.Fatalf("status = %s, want locked", got)
	}

	if err := state.SelectScenario(alpha.Manifest.ID); err != nil {
		t.Fatal(err)
	}
	startGame(t, state)
	if err := state.Evaluate(Frame{CRCOK: true, Telemetry: map[string]any{"mode": "SAFE"}}); err != nil {
		t.Fatal(err)
	}

	if got := state.ObjectiveStatus(alpha, alpha.Objectives[0]); got != ObjectiveStatusComplete {
		t.Errorf("status = %s, want complete", got)
	}
	if got := state.ObjectiveStatus(alpha, Objective{ID: "unknown"}); got != ObjectiveStatusActive {
		t.Errorf("unknown objective status = %s, want active", got)
	}
	if got := state.ObjectiveStatus(beta, beta.Objectives[0]); got != ObjectiveStatusActive {
		t.Errorf("other scenario status = %s, want active", got)
	}
}

func TestUploadPatchedBinary(t *testing.T) {
	state := readyState(t)
	if err := state.UploadPatchedBinary("patched.elf"); err == nil {
		t.Fatal("expected upload to fail before the game starts")
	}

	startGame(t, state)
	if err := state.UploadPatchedBinary("patched.elf"); err != nil {
		t.Fatalf("UploadPatchedBinary: %v", err)
	}
}

func TestStateMemoryRanges(t *testing.T) {
	if _, err := (&State{}).MemoryRanges(); err == nil {
		t.Fatal("expected error with no scenario selected")
	}

	state := readyState(t, modeIs2)
	ranges, err := state.MemoryRanges()
	if err != nil || len(ranges) != 1 {
		t.Fatalf("MemoryRanges = %v, %v", ranges, err)
	}
}

func TestDownlinkMessageMarkers(t *testing.T) {
	for _, m := range []DownlinkMessage{&TLMDownlinkMessage{}, &CommandResponseDownlinkMessage{}, &StateDownlinkMessage{}} {
		m.downlinkMessage()
	}
}

func crcCCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func tlmLine(mode byte) string {
	payload := []byte{0x00, 0x01, 0, 0, 0, 0x2A, mode, 0, 0, 0x13, 0x88, 0x01, 0xF4}
	body := append([]byte{byte(len(payload))}, payload...)
	frame := append([]byte{0xEB, 0x90}, body...)
	frame = binary.BigEndian.AppendUint16(frame, crcCCITT(body))
	return "TLM " + hex.EncodeToString(frame)
}

func recvDownlink(t *testing.T, ch chan DownlinkMessage) DownlinkMessage {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for downlink")
		return nil
	}
}

func TestDownlinkTranslatorPumpTelemetry(t *testing.T) {
	launches := stubFirmware(t)

	state := loadedState(t)
	if err := state.SelectScenario("sojourn.test.alpha"); err != nil {
		t.Fatal(err)
	}

	downlink := make(chan DownlinkMessage)
	if err := state.Start(downlink, make(chan []byte), make(chan struct{})); err != nil {
		t.Fatal(err)
	}
	l := <-launches

	// An undecodable frame is logged and skipped.
	l.downlink <- []byte("TLM zz\n")

	// Mode 0x02 decodes as SAFE.
	line := tlmLine(0x02)
	l.downlink <- []byte(line + "\n")

	msg := recvDownlink(t, downlink)
	stateMsg, ok := msg.(*StateDownlinkMessage)
	if !ok {
		t.Fatalf("first message = %#v, want state", msg)
	}
	want := map[string]ObjectiveStatus{"safe": ObjectiveStatusComplete, "next": ObjectiveStatusActive}
	if len(stateMsg.Objectives) != len(want) {
		t.Fatalf("objectives = %v", stateMsg.Objectives)
	}
	for _, o := range stateMsg.Objectives {
		if o.Status != want[o.ID] {
			t.Errorf("objective %q = %s, want %s", o.ID, o.Status, want[o.ID])
		}
	}
	if stateMsg.Constraints == nil {
		t.Error("constraints is nil")
	}

	msg = recvDownlink(t, downlink)
	tlm, ok := msg.(*TLMDownlinkMessage)
	if !ok || tlm.TLM != line || tlm.Telemetry == nil || tlm.Telemetry.Mode != "SAFE" {
		t.Fatalf("second message = %#v, want telemetry for %q", msg, line)
	}

	// A frame that fails grading is still forwarded.
	bad := []byte(tlmLine(0x01))
	if bad[len(bad)-1] == '0' {
		bad[len(bad)-1] = '1'
	} else {
		bad[len(bad)-1] = '0'
	}
	l.downlink <- bad
	if _, ok := recvDownlink(t, downlink).(*StateDownlinkMessage); !ok {
		t.Fatal("expected state message after a bad-CRC frame")
	}
	if _, ok := recvDownlink(t, downlink).(*TLMDownlinkMessage); !ok {
		t.Fatal("expected telemetry message after a bad-CRC frame")
	}

	close(l.downlink)
	if _, ok := <-downlink; ok {
		t.Fatal("downlink not closed")
	}
}

func TestDeselectScenario(t *testing.T) {
	state := loadedState(t)
	if err := state.DeselectScenario(); err == nil {
		t.Fatal("expected DeselectScenario to fail with no scenario selected")
	}

	if err := state.SelectScenario("sojourn.test.alpha"); err != nil {
		t.Fatalf("SelectScenario: %v", err)
	}
	if err := state.DeselectScenario(); err != nil {
		t.Fatalf("DeselectScenario: %v", err)
	}
	if state.Phase() != PhaseLevelSelect || state.Scenario != nil || state.grader != nil {
		t.Fatalf("phase = %v, scenario = %v, grader = %v; want level select with nothing selected",
			state.Phase(), state.Scenario, state.grader)
	}

	if err := state.SelectScenario("sojourn.test.beta"); err != nil {
		t.Fatalf("SelectScenario after deselect: %v", err)
	}
}
