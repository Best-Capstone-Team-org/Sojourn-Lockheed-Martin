package game

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"slices"
	"sojourn/emulator"
	"strings"
	"testing"
	"time"
)

// gdbStub is a minimal GDB remote stub serving reads of mem, based at SRAM.
// With failReads set every memory read returns an error reply.
type gdbStub struct {
	addr      string
	mem       memory
	failReads bool
}

func newGDBStub(t *testing.T, mem memory, failReads bool) *gdbStub {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	s := &gdbStub{addr: ln.Addr().String(), mem: mem, failReads: failReads}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	return s
}

func (s *gdbStub) reply(conn net.Conn, payload string) {
	var sum byte
	for i := 0; i < len(payload); i++ {
		sum += payload[i]
	}
	fmt.Fprintf(conn, "$%s#%02x", payload, sum)
}

func (s *gdbStub) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case 0x03:
			s.reply(conn, "S05")
			continue
		case '$':
		default:
			continue
		}

		body, err := r.ReadString('#')
		if err != nil {
			return
		}
		if _, err := r.Discard(2); err != nil {
			return
		}
		conn.Write([]byte("+"))

		payload := strings.TrimSuffix(body, "#")
		switch {
		case payload == "qSupported":
			s.reply(conn, "PacketSize=400")
		case payload == "D":
			s.reply(conn, "OK")
			return
		case strings.HasPrefix(payload, "m"):
			var addr uint32
			var n int
			if _, err := fmt.Sscanf(payload, "m%x,%x", &addr, &n); err != nil || s.failReads {
				s.reply(conn, "E01")
				continue
			}
			data, _ := s.mem.Read(addr, n)
			s.reply(conn, hex.EncodeToString(data))
		}
	}
}

func TestGraderCapturesMemory(t *testing.T) {
	stub := newGDBStub(t, sram(0x10, 2), false)

	state := readyState(t, modeIs2)
	startGame(t, state)
	g := NewGrader(state, stub.addr)

	for range 2 {
		if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err != nil {
			t.Fatalf("Grade: %v", err)
		}
	}
	if got := status(t, &state.evaluation, "mode"); got != ObjectiveStatusComplete {
		t.Fatalf("status = %s, want complete", got)
	}

	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if g.introspect != nil {
		t.Fatal("Close kept the introspection connection")
	}
	if err := g.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestGraderCaptureFailureDropsConnection(t *testing.T) {
	stub := newGDBStub(t, sram(0), true)

	state := readyState(t, modeIs2)
	startGame(t, state)
	g := NewGrader(state, stub.addr)
	defer g.Close()

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err == nil {
		t.Fatal("expected Grade to fail when memory reads fail")
	}
	if g.introspect != nil {
		t.Fatal("failed capture kept the introspection connection")
	}
	if got := status(t, &state.evaluation, "mode"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}
}

func TestGraderEvaluationError(t *testing.T) {
	state := readyState(t, Objective{
		ID:      "bits",
		Success: Predicate{Op: "tlm_bits", Path: "mode", Mask: 1, Cmp: "eq", Value: 1.0},
	})
	startGame(t, state)
	g := NewGrader(state, "127.0.0.1:1")

	// mode is a string, which cannot be masked.
	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true, Mode: "SAFE"}); err == nil {
		t.Fatal("expected Grade to report the evaluation error")
	}
}

func TestGraderInProgressWithoutScenario(t *testing.T) {
	g := NewGrader(&State{phase: PhaseInProgress}, "127.0.0.1:1")
	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err == nil {
		t.Fatal("expected Grade to fail with no scenario selected")
	}
}

func TestDownlinkTranslatorPumpClosesGrader(t *testing.T) {
	stub := newGDBStub(t, sram(0x10, 2), false)
	state := readyState(t, modeIs2)
	grader := NewGrader(state, stub.addr)
	if _, err := grader.capture(state.evaluation.MemoryRanges()); err != nil {
		t.Fatalf("capture: %v", err)
	}
	if grader.introspect == nil {
		t.Fatal("capture did not connect")
	}

	in := make(chan []byte)
	out := make(chan DownlinkMessage)
	go state.downlinkTranslatorPump(out, in, grader, nil, nil)
	close(in)

	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("unexpected downlink message")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("translator pump did not exit")
	}
	if grader.introspect != nil {
		t.Error("translator pump left the grader's GDB connection open")
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		raw  string
		want CommandLogEntry
	}{
		{"POKE 0x2001E200 09", CommandLogEntry{Raw: "POKE 0x2001E200 09", Verb: "POKE", Addr: 0x2001E200}},
		{"PEEK 2001e200 16", CommandLogEntry{Raw: "PEEK 2001e200 16", Verb: "PEEK", Addr: 0x2001E200}},
		{"PEEK  0X10   4", CommandLogEntry{Raw: "PEEK  0X10   4", Verb: "PEEK", Addr: 0x10}},
		{"PING", CommandLogEntry{Raw: "PING", Verb: "PING"}},
		{"POKE nothex 1", CommandLogEntry{Raw: "POKE nothex 1", Verb: "POKE"}},
		{"POKE 0x123456789 1", CommandLogEntry{Raw: "POKE 0x123456789 1", Verb: "POKE"}},
		{"", CommandLogEntry{}},
	}

	for _, tc := range tests {
		if got := parseCommand(tc.raw); got != tc.want {
			t.Errorf("parseCommand(%q) = %+v, want %+v", tc.raw, got, tc.want)
		}
	}
}

func TestIsCommandResponse(t *testing.T) {
	for line, want := range map[string]bool{
		"ACK POKE 1":            true,
		"NAK E04":               true,
		"SOJOURN FSW v1.0 boot": false,
		"ACKNOWLEDGED":          false,
		"":                      false,
	} {
		if got := isCommandResponse(line); got != want {
			t.Errorf("isCommandResponse(%q) = %v, want %v", line, got, want)
		}
	}
}

// Commands are logged when the probe answers them and graded with the next
// telemetry frame, so they count toward the log without breaking up the
// frames that sustained and within count.
func TestCommandsAreGradedWithTelemetry(t *testing.T) {
	launches := stubFirmware(t)

	state := readyState(t,
		Objective{
			ID: "poked",
			Success: Predicate{Op: "commanded", Verb: "POKE", At: &AddressRef{Addr: "0x20000010"},
				Result: "ACK"},
		},
		Objective{
			ID: "steady",
			Success: Predicate{Op: "sustained", Frames: 2,
				Of: rawPredicate(t, Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "BOOT"})},
		},
	)
	state.grader = NewGrader(state, "127.0.0.1:1")

	out := make(chan DownlinkMessage)
	up := make(chan []byte)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	sendTLM := func() {
		t.Helper()
		l.downlink <- []byte(tlmLine(0) + "\r\n")
		if _, ok := recvDownlink(t, out).(*StateDownlinkMessage); !ok {
			t.Fatal("expected a state message")
		}
		if _, ok := recvDownlink(t, out).(*TLMDownlinkMessage); !ok {
			t.Fatal("expected a telemetry message")
		}
	}
	sendLine := func(line string) {
		t.Helper()
		l.downlink <- []byte(line + "\r\n")
		if msg, ok := recvDownlink(t, out).(*CommandResponseDownlinkMessage); !ok || msg.Response != line {
			t.Fatalf("downlink = %#v, want command response %q", msg, line)
		}
	}
	statusOf := func(id string) ObjectiveStatus {
		t.Helper()
		return status(t, &state.evaluation, id)
	}
	command := func(raw string) {
		t.Helper()
		up <- []byte(raw)
		if got := recvUplink(t, l.uplink); got != raw {
			t.Fatalf("firmware uplink = %q, want %q", got, raw)
		}
	}

	sendLine("SOJOURN FSW v1.0 boot")
	sendTLM()

	command("POKE 0x20000010 09")
	sendLine("NAK E04")
	command("POKE 0x20000010 09")
	sendLine("ACK POKE 1")

	if got := statusOf("poked"); got != ObjectiveStatusActive {
		t.Fatalf("poked = %s before the next frame, want active", got)
	}

	sendTLM()

	if got := statusOf("poked"); got != ObjectiveStatusComplete {
		t.Errorf("poked = %s, want complete", got)
	}
	if got := statusOf("steady"); got != ObjectiveStatusComplete {
		t.Errorf("steady = %s, want complete: commands between frames broke the streak", got)
	}

	want := []CommandLogEntry{
		{Raw: "POKE 0x20000010 09", Verb: "POKE", Addr: 0x20000010, Result: "NAK E04"},
		{Raw: "POKE 0x20000010 09", Verb: "POKE", Addr: 0x20000010, Result: "ACK POKE 1"},
	}
	if !slices.Equal(state.grader.log, want) {
		t.Errorf("log = %+v, want %+v", state.grader.log, want)
	}

	close(up)
	close(l.downlink)
	for range out {
	}
}

func TestUnansweredCommandsAreBounded(t *testing.T) {
	launches := stubFirmware(t)
	state := readyState(t)
	out := make(chan DownlinkMessage)
	up := make(chan []byte)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	l.downlink <- []byte("SOJOURN FSW v1.0 boot\n")
	recvDownlink(t, out)
	go func() {
		for range l.uplink {
		}
	}()

	sent := make(chan struct{})
	go func() {
		defer close(sent)
		for range pendingCommandLimit + 5 {
			up <- []byte("PING")
		}
	}()
	select {
	case <-sent:
	case <-time.After(5 * time.Second):
		t.Fatal("uplink blocked on unanswered commands")
	}

	close(up)

	close(l.downlink)
	for range out {
	}
}

func TestSetupCommands(t *testing.T) {
	u8, u16, u32 := uint8(0x7F), uint16(600), uint32(20)
	long := strings.Repeat("AB", 40)

	s := frameEvaluation(t).scenario
	s.Symbols.Fields = map[string]map[string]int{"g_buf": {"limit": 4}}
	s.Setup = &Setup{Writes: []SetupWrite{
		{At: AddressRef{Sym: "g_mode"}, U8: &u8},
		{At: AddressRef{Sym: "g_buf", Field: "limit"}, U16: &u16},
		{At: AddressRef{Addr: "0x20000030"}, U32: &u32},
		{At: AddressRef{Addr: "0x20000040"}, Hex: "deadbeef"},
		{At: AddressRef{Addr: "0x20000080"}, Hex: long},
	}}

	got, err := s.setupCommands()
	if err != nil {
		t.Fatalf("setupCommands: %v", err)
	}
	want := []string{
		"POKE 0x20000010 7F",
		"POKE 0x20000024 5802",
		"POKE 0x20000030 14000000",
		"POKE 0x20000040 DEADBEEF",
		"POKE 0x20000080 " + strings.Repeat("AB", 32),
		"POKE 0x200000A0 " + strings.Repeat("AB", 8),
	}
	if !slices.Equal(got, want) {
		t.Errorf("setupCommands =\n%q\nwant\n%q", got, want)
	}

	s.Setup = nil
	if got, err := s.setupCommands(); err != nil || got != nil {
		t.Errorf("setupCommands without setup = %q, %v; want none", got, err)
	}

	s.Setup = &Setup{Writes: []SetupWrite{{At: AddressRef{Sym: "missing"}, U8: &u8}}}
	if _, err := s.setupCommands(); err == nil {
		t.Error("expected an error for an unresolvable setup address")
	}

	s.Setup = &Setup{Writes: []SetupWrite{{At: AddressRef{Addr: "0x20000010"}, Hex: "zz"}}}
	if _, err := s.setupCommands(); err == nil {
		t.Error("expected an error for invalid setup hex")
	}
}

func TestStartFailsOnBadSetup(t *testing.T) {
	launches := stubFirmware(t)
	state := readyState(t)
	u8 := uint8(1)
	state.Scenario.Setup = &Setup{Writes: []SetupWrite{{At: AddressRef{Sym: "missing"}, U8: &u8}}}

	if err := state.Start(make(chan DownlinkMessage), make(chan []byte), make(chan struct{})); err == nil {
		t.Fatal("expected Start to fail when setup cannot be applied")
	}
	if state.Phase() != PhaseReady {
		t.Errorf("phase = %v, want ready", state.Phase())
	}
	select {
	case <-launches:
		t.Error("firmware launched despite a bad setup")
	default:
	}
}

// Setup writes go out once the firmware has booted and before anything the
// player sent, and their replies stay out of the command log and the console.
func TestSetupAppliedBeforePlayerCommands(t *testing.T) {
	launches := stubFirmware(t)
	state := readyState(t)
	u32, u8 := uint32(20), uint8(1)
	state.Scenario.Setup = &Setup{Writes: []SetupWrite{
		{At: AddressRef{Sym: "g_mode"}, U32: &u32},
		{At: AddressRef{Sym: "g_buf"}, U8: &u8},
	}}
	state.grader = NewGrader(state, "127.0.0.1:1")

	out := make(chan DownlinkMessage)
	up := make(chan []byte)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	up <- []byte("  PEEK 0x20000010 4 \n")
	select {
	case msg := <-l.uplink:
		t.Fatalf("firmware got %q before it booted", msg)
	case <-time.After(100 * time.Millisecond):
	}

	l.downlink <- []byte("SOJOURN FSW v1.0 boot  reboots=0 fault=0\r\n")
	recvDownlink(t, out)

	for _, want := range []string{"POKE 0x20000010 14000000", "POKE 0x20000020 01", "PEEK 0x20000010 4"} {
		if got := recvUplink(t, l.uplink); got != want {
			t.Fatalf("firmware uplink = %q, want %q", got, want)
		}
	}

	l.downlink <- []byte("ACK POKE 4\r\n")
	l.downlink <- []byte("NAK E04\r\n")
	l.downlink <- []byte("ACK PEEK 02000000\r\n")
	msg, ok := recvDownlink(t, out).(*CommandResponseDownlinkMessage)
	if !ok || msg.Response != "ACK PEEK 02000000" {
		t.Fatalf("downlink = %#v, want only the player's PEEK reply", msg)
	}

	want := []CommandLogEntry{
		{Raw: "PEEK 0x20000010 4", Verb: "PEEK", Addr: 0x20000010, Result: "ACK PEEK 02000000"},
	}
	if !slices.Equal(state.grader.log, want) {
		t.Errorf("log = %+v, want %+v", state.grader.log, want)
	}

	close(up)
	close(l.downlink)
	for range out {
	}
}

func TestUplinkClosedBeforeBoot(t *testing.T) {
	launches := stubFirmware(t)
	state := readyState(t)
	out := make(chan DownlinkMessage)
	up := make(chan []byte)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	close(up)
	select {
	case _, ok := <-l.uplink:
		if ok {
			t.Fatal("firmware got a command after the player's uplink closed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("firmware uplink not closed when the player left before boot")
	}

	close(l.downlink)
	for range out {
	}
}
