package game

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func fixClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = prev })
}

func sessionScenario() *Scenario {
	return &Scenario{Manifest: Manifest{
		ID:       "sojourn.test.alpha",
		Revision: 3,
		Firmware: FirmwareConfig{AppCRC32: "0xf72478e0"},
	}}
}

// readLog returns each line of a command log decoded loosely, so tests see
// exactly which keys were written and which were null.
func readLog(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var records []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("line %q is not JSON: %v", scanner.Text(), err)
		}
		records = append(records, record)
	}
	return records
}

func TestNewSession(t *testing.T) {
	fixClock(t, time.Date(2026, 11, 2, 15, 4, 5, 6e6, time.UTC))
	saveDir := t.TempDir()

	session, err := NewSession(saveDir, sessionScenario())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer session.Close()

	wantDir := filepath.Join(saveDir, "sessions", "20261102T150405.006Z-sojourn.test.alpha")
	if session.Dir != wantDir {
		t.Errorf("Dir = %q, want %q", session.Dir, wantDir)
	}

	data, err := os.ReadFile(filepath.Join(session.Dir, "session.json"))
	if err != nil {
		t.Fatal(err)
	}
	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatal(err)
	}
	want := SessionInfo{Format: SaveFormat, Scenario: "sojourn.test.alpha", Revision: 3,
		AppCRC32: "0xf72478e0", Started: "2026-11-02T15:04:05.006Z"}
	if info != want {
		t.Errorf("session.json = %+v, want %+v", info, want)
	}

	for _, name := range []string{"commands.jsonl", "objectives.jsonl"} {
		if stat, err := os.Stat(filepath.Join(session.Dir, name)); err != nil || stat.Size() != 0 {
			t.Errorf("%s = %v, %v; want an empty file", name, stat, err)
		}
	}
	if leftovers, _ := filepath.Glob(filepath.Join(session.Dir, "*.tmp")); len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestNewSessionErrors(t *testing.T) {
	fixClock(t, time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC))

	notADir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notADir, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSession(notADir, sessionScenario()); err == nil {
		t.Error("expected an error when the save directory is a file")
	}

	saveDir := t.TempDir()
	first, err := NewSession(saveDir, sessionScenario())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := NewSession(saveDir, sessionScenario()); err == nil {
		t.Error("expected an error rather than reusing an existing session")
	}
}

func TestSessionAppend(t *testing.T) {
	fixClock(t, time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC))
	session, err := NewSession(t.TempDir(), sessionScenario())
	if err != nil {
		t.Fatal(err)
	}

	tMs := uint64(12480)
	for _, command := range []CommandLogEntry{
		{Raw: "PEEK 0x2001E200 4", Verb: "PEEK", Charged: ResourceReads, Result: "ACK PEEK 01000000", TMs: &tMs},
		{Raw: "POKE 0x00000000 FF", Verb: "POKE", Charged: ResourceWrites, Result: "NAK E04"},
		{Raw: "NOOP", Verb: "NOOP"},
	} {
		if err := session.Append(command); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	got := readLog(t, filepath.Join(session.Dir, "commands.jsonl"))
	wall := "2026-11-02T15:00:00Z"
	want := []map[string]any{
		{"seq": 1.0, "t_ms": 12480.0, "raw": "PEEK 0x2001E200 4", "verb": "PEEK", "charged": "reads",
			"result": "ACK PEEK 01000000", "wall": wall},
		{"seq": 2.0, "t_ms": nil, "raw": "POKE 0x00000000 FF", "verb": "POKE", "charged": "writes",
			"result": "NAK E04", "wall": wall},
		{"seq": 3.0, "t_ms": nil, "raw": "NOOP", "verb": "NOOP", "charged": nil, "result": nil, "wall": wall},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("commands.jsonl =\n%v\nwant\n%v", got, want)
	}
}

func TestNilSession(t *testing.T) {
	var session *Session
	if err := session.Append(CommandLogEntry{Raw: "PING"}); err != nil {
		t.Error(err)
	}
	if err := session.Close(); err != nil {
		t.Error(err)
	}
}

// probeMemoryWithUptime is SRAM from 0x20000000 with SYSBLK->ticks set.
func probeMemoryWithUptime(ticks uint32) memory {
	m := make(memory, 0x200)
	binary.LittleEndian.PutUint32(m[uptimeTicksAddr-0x20000000:], ticks)
	return m
}

func TestGraderUptimeMS(t *testing.T) {
	stub := newGDBStub(t, probeMemoryWithUptime(2496), false)
	g := NewGrader(&State{}, stub.addr)
	defer g.Close()

	got := g.UptimeMS()
	if got == nil || *got != 24960 {
		t.Fatalf("UptimeMS = %v, want 24960 (2496 ticks at 100 Hz)", got)
	}

	if got := NewGrader(&State{}, "127.0.0.1:1").UptimeMS(); got != nil {
		t.Errorf("UptimeMS without introspection = %d, want nil", *got)
	}
	if got := probeUptime(nil)(); got != nil {
		t.Errorf("uptime without a grader = %d, want nil", *got)
	}
}

// Playing records each player command once the probe answers, stamped with the
// probe's uptime; setup writes and refused commands never happened as far as
// the log is concerned.
func TestStartRecordsSession(t *testing.T) {
	stub := newGDBStub(t, probeMemoryWithUptime(1500), false)
	launches := stubFirmware(t)

	state := readyState(t)
	state.SaveDir = t.TempDir()
	u8, writes := uint8(1), 1
	state.Scenario.Setup = &Setup{Writes: []SetupWrite{{At: AddressRef{Sym: "g_mode"}, U8: &u8}}}
	state.Scenario.Manifest.Link.Budget = BudgetConfig{Writes: &writes}
	state.grader = NewGrader(state, stub.addr)

	out := make(chan DownlinkMessage)
	up := make(chan []byte)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	l.downlink <- []byte("SOJOURN FSW v1.0 boot\r\n")
	recvDownlink(t, out)
	recvUplink(t, l.uplink)
	l.downlink <- []byte("ACK POKE 1\r\n")

	command := func(raw string) {
		t.Helper()
		up <- []byte(raw)
		recvUplink(t, l.uplink)
	}
	command("PEEK 0x20000010 4")
	l.downlink <- []byte("ACK PEEK 00000000\r\n")
	recvDownlink(t, out)

	command("POKE 0x20000010 02")
	l.downlink <- []byte("NAK E04\r\n")
	recvDownlink(t, out)

	up <- []byte("POKE 0x20000010 03")
	if _, ok := recvDownlink(t, out).(*CommandRefusedDownlinkMessage); !ok {
		t.Fatal("expected the second write to be refused")
	}

	command("NOOP")

	// The probe goes away without answering NOOP.
	close(up)
	close(l.downlink)
	for range out {
	}

	sessions, err := filepath.Glob(filepath.Join(state.SaveDir, "sessions", "*"))
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions = %v, %v; want one", sessions, err)
	}
	got := readLog(t, filepath.Join(sessions[0], "commands.jsonl"))

	type line struct {
		seq         float64
		raw, result any
		charged     any
		tMs         any
	}
	want := []line{
		{1, "PEEK 0x20000010 4", "ACK PEEK 00000000", "reads", 15000.0},
		{2, "POKE 0x20000010 02", "NAK E04", "writes", 15000.0},
		{3, "NOOP", nil, nil, 15000.0},
	}
	if len(got) != len(want) {
		t.Fatalf("logged %d commands, want %d: %v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g["seq"] != w.seq || g["raw"] != w.raw || g["result"] != w.result || g["charged"] != w.charged ||
			g["t_ms"] != w.tMs {
			t.Errorf("record %d = %v, want seq %v raw %v result %v charged %v t_ms %v",
				i, g, w.seq, w.raw, w.result, w.charged, w.tMs)
		}
	}
}

func TestStartFailsWithoutSaveDirectory(t *testing.T) {
	launches := stubFirmware(t)
	state := readyState(t)
	state.SaveDir = filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(state.SaveDir, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := state.Start(make(chan DownlinkMessage), make(chan []byte), make(chan struct{})); err == nil {
		t.Fatal("expected Start to fail when the session cannot be recorded")
	}
	if state.Phase() != PhaseReady {
		t.Errorf("phase = %v, want ready", state.Phase())
	}
	select {
	case <-launches:
		t.Error("firmware launched without a session to record into")
	default:
	}
}

func TestSessionAppendObjective(t *testing.T) {
	fixClock(t, time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC))
	session, err := NewSession(t.TempDir(), sessionScenario())
	if err != nil {
		t.Fatal(err)
	}

	for _, transition := range []ObjectiveTransition{
		{Frame: 7, UptimeS: 35, Objective: "diagnose", From: ObjectiveStatusActive, To: ObjectiveStatusComplete},
		{Frame: 7, UptimeS: 35, Objective: "attempt-redeploy", From: ObjectiveStatusLocked, To: ObjectiveStatusActive},
	} {
		if err := session.AppendObjective(transition); err != nil {
			t.Fatalf("AppendObjective: %v", err)
		}
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	got := readLog(t, filepath.Join(session.Dir, "objectives.jsonl"))
	wall := "2026-11-02T15:00:00Z"
	want := []map[string]any{
		{"frame": 7.0, "uptime_s": 35.0, "objective": "diagnose", "from": "active", "state": "complete", "wall": wall},
		{"frame": 7.0, "uptime_s": 35.0, "objective": "attempt-redeploy", "from": "locked", "state": "active", "wall": wall},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("objectives.jsonl =\n%v\nwant\n%v", got, want)
	}

	var nilSession *Session
	if err := nilSession.AppendObjective(ObjectiveTransition{}); err != nil {
		t.Error(err)
	}
}

func TestStartRecordsObjectives(t *testing.T) {
	launches := stubFirmware(t)
	state := readyState(t,
		Objective{ID: "boot", Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "BOOT"}},
		Objective{ID: "safe", Requires: []string{"boot"},
			Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"}},
	)
	state.SaveDir = t.TempDir()
	state.grader = NewGrader(state, "127.0.0.1:1")

	out := make(chan DownlinkMessage)
	up := make(chan []byte)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	for _, mode := range []byte{0, 2} {
		l.downlink <- []byte(tlmLine(mode) + "\r\n")
		recvDownlink(t, out)
		recvDownlink(t, out)
	}
	close(up)
	close(l.downlink)
	for range out {
	}

	sessions, _ := filepath.Glob(filepath.Join(state.SaveDir, "sessions", "*"))
	if len(sessions) != 1 {
		t.Fatalf("sessions = %v, want one", sessions)
	}
	got := readLog(t, filepath.Join(sessions[0], "objectives.jsonl"))

	// tlmLine's frames report 42 s of uptime.
	want := [][4]any{
		{1.0, "boot", "active", "complete"},
		{1.0, "safe", "locked", "active"},
		{2.0, "safe", "active", "complete"},
	}
	if len(got) != len(want) {
		t.Fatalf("objectives.jsonl = %v, want %d records", got, len(want))
	}
	for i, w := range want {
		g := got[i]
		if g["frame"] != w[0] || g["objective"] != w[1] || g["from"] != w[2] || g["state"] != w[3] ||
			g["uptime_s"] != 42.0 {
			t.Errorf("record %d = %v, want frame %v %v %v -> %v at 42 s", i, g, w[0], w[1], w[2], w[3])
		}
	}

	boot, _ := state.evaluation.GetObjectiveState("boot")
	safe, _ := state.evaluation.GetObjectiveState("safe")
	if boot.FirstFrame != 1 || safe.FirstFrame != 2 || boot.FirstUptimeS != 42 {
		t.Errorf("first completions: boot frame %d at %d s, safe frame %d; want 1 at 42 s and 2",
			boot.FirstFrame, boot.FirstUptimeS, safe.FirstFrame)
	}
}
