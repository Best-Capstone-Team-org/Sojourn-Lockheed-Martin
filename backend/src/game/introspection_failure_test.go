package game

import (
	"net"
	"strings"
	"testing"

	"sojourn/emulator"
)

// freeAddr returns a loopback address nothing is listening on yet.
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestGradingRecoversWhenIntrospectionReturns(t *testing.T) {
	state := readyState(t, modeIs2)
	startGame(t, state)
	addr := freeAddr(t)
	g := NewGrader(state, addr)
	defer g.Close()

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err != nil {
		t.Fatalf("Grade while unreachable: %v", err)
	}
	if !g.Status().Degraded {
		t.Fatal("expected degraded grading while memory is unreachable")
	}

	// The emulator's stub comes back; the next pass reconnects before reading.
	newGDBStubOn(t, addr, sram(0x10, 2), false)

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err != nil {
		t.Fatalf("Grade after recovery: %v", err)
	}
	if got := g.Status(); got.Degraded {
		t.Errorf("status = %+v after recovery, want healthy", got)
	}
	if got := status(t, &state.evaluation, "mode"); got != ObjectiveStatusComplete {
		t.Errorf("memory objective = %s after recovery, want complete", got)
	}
}

func TestEvaluateTelemetryOnly(t *testing.T) {
	patched := Objective{ID: "patched", Success: Predicate{Op: "mem_changed", At: &AddressRef{Sym: "g_buf"}, Len: 1}}
	pinged := Objective{ID: "pinged", Success: Predicate{Op: "commanded", Verb: "PING"}}
	e := frameEvaluation(t, patched, pinged)

	if err := e.Evaluate(Frame{Memory: sram(0x20, 1)}); err != nil {
		t.Fatal(err)
	}
	baseline := e.baseline

	if err := e.EvaluateTelemetryOnly(Frame{Log: []CommandLogEntry{{Verb: "PING"}}, Memory: sram(0x20, 9)}); err != nil {
		t.Fatalf("EvaluateTelemetryOnly: %v", err)
	}
	if got := status(t, e, "pinged"); got != ObjectiveStatusComplete {
		t.Errorf("pinged = %s, want complete", got)
	}
	if got := status(t, e, "patched"); got != ObjectiveStatusActive {
		t.Errorf("patched = %s, want unchanged without memory", got)
	}
	if state, _ := e.GetObjectiveState("patched"); state.FramesActive != 2 {
		t.Errorf("patched FramesActive = %d, want 2: hints keep pacing while grading is degraded", state.FramesActive)
	}
	if e.baseline == nil || !sameMemory(t, e.baseline, baseline) {
		t.Error("telemetry-only frame replaced the mem_changed baseline")
	}

	// Memory is back: the patch made meanwhile is seen against the old baseline.
	if err := e.Evaluate(Frame{Memory: sram(0x20, 9)}); err != nil {
		t.Fatal(err)
	}
	if got := status(t, e, "patched"); got != ObjectiveStatusComplete {
		t.Errorf("patched = %s once memory is readable, want complete", got)
	}
}

func sameMemory(t *testing.T, a, b MemoryReader) bool {
	t.Helper()
	x, err1 := a.Read(0x20000020, 1)
	y, err2 := b.Read(0x20000020, 1)
	return err1 == nil && err2 == nil && x[0] == y[0]
}

func TestPredicateReadsMemory(t *testing.T) {
	mem := Predicate{Op: "mem_u8", At: &AddressRef{Sym: "g_mode"}, Cmp: "eq", Value: 1.0}
	tlm := Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"}

	for name, tc := range map[string]struct {
		p    Predicate
		want bool
	}{
		"memory":             {mem, true},
		"telemetry":          {tlm, false},
		"nested in all":      {Predicate{Op: "all", Of: rawPredicate(t, []Predicate{tlm, mem})}, true},
		"telemetry only all": {Predicate{Op: "all", Of: rawPredicate(t, []Predicate{tlm, tlm})}, false},
		"under sustained":    {Predicate{Op: "sustained", Frames: 2, Of: rawPredicate(t, mem)}, true},
		"unparseable":        {Predicate{Op: "not", Of: []byte("{")}, true},
	} {
		if got := predicateReadsMemory(tc.p); got != tc.want {
			t.Errorf("%s: predicateReadsMemory = %v, want %v", name, got, tc.want)
		}
	}
}

// The player is told within the frame, and never about the channel itself
// (charter R24.19).
func TestStateMessageReportsDegradedGrading(t *testing.T) {
	state := readyState(t, modeIs2)
	startGame(t, state)
	state.grader = NewGrader(state, "127.0.0.1:1")

	if err := state.grader.Grade(&emulator.TelemetryFrame{CRCOK: true}); err != nil {
		t.Fatal(err)
	}
	out := make(chan DownlinkMessage, 1)
	state.sendStateDownlink(out)
	grading := (<-out).(*StateDownlinkMessage).Grading
	if !grading.Degraded || grading.Reason == "" {
		t.Fatalf("grading = %+v, want degraded with a reason", grading)
	}

	for _, reason := range []string{reasonMemoryUnreachable, reasonFrameNotGraded} {
		lower := strings.ToLower(reason)
		for _, word := range []string{"introspect", "gdb", "debug", "3333"} {
			if strings.Contains(lower, word) {
				t.Errorf("player-facing reason %q mentions %q", reason, word)
			}
		}
	}

	if got := (*Grader)(nil).Status(); got.Degraded {
		t.Error("a game without a grader reports degraded grading")
	}
}
