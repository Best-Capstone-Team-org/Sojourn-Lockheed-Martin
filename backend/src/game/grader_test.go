package game

import (
	"sojourn/emulator"
	"testing"
)

func TestGraderGradesTelemetry(t *testing.T) {
	state := readyState(t, Objective{
		ID:      "safe",
		Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"},
	})
	s := &state.evaluation
	startGame(t, state)
	g := NewGrader(state, "127.0.0.1:0")
	defer g.Close()

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true, Mode: "NOMINAL"}); err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if got := status(t, s, "safe"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}

	// A frame that failed CRC is graded, but its telemetry reads as false.
	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: false, Mode: "SAFE"}); err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if got := status(t, s, "safe"); got != ObjectiveStatusActive {
		t.Fatalf("status after bad CRC = %s, want active", got)
	}

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true, Mode: "SAFE"}); err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if got := status(t, s, "safe"); got != ObjectiveStatusComplete {
		t.Fatalf("status = %s, want complete", got)
	}
}

func TestGraderWithoutIntrospectionGradesTelemetryOnly(t *testing.T) {
	state := readyState(t, modeIs2, Objective{
		ID:      "safe",
		Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"},
	})
	s := &state.evaluation
	startGame(t, state)

	// Nothing listens on port 1, so the capture cannot connect.
	g := NewGrader(state, "127.0.0.1:1")
	defer g.Close()

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true, Mode: "SAFE"}); err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if got := status(t, s, "safe"); got != ObjectiveStatusComplete {
		t.Errorf("telemetry objective = %s, want complete", got)
	}
	if got := status(t, s, "mode"); got != ObjectiveStatusActive {
		t.Errorf("memory objective = %s, want unchanged", got)
	}
	if got := g.Status(); !got.Degraded || got.Reason != reasonMemoryUnreachable {
		t.Errorf("status = %+v, want degraded: memory unreachable", got)
	}
}

func TestGraderRejectsFramesWhenNotInProgress(t *testing.T) {
	state := readyState(t, Objective{
		ID:      "safe",
		Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"},
	})
	s := &state.evaluation
	g := NewGrader(state, "127.0.0.1:1")

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true, Mode: "SAFE"}); err == nil {
		t.Fatal("expected Grade to fail before the scenario has started")
	}
	if state.Phase() != PhaseReady {
		t.Fatalf("phase = %v, want ready", state.Phase())
	}
	if got := status(t, s, "safe"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}

	if err := NewGrader(&State{}, "127.0.0.1:1").Grade(&emulator.TelemetryFrame{CRCOK: true}); err == nil {
		t.Fatal("expected Grade to fail with no scenario")
	}
}

// Event predicates see what changed since the last good frame (R23.12).
func TestGraderComputesEvents(t *testing.T) {
	state := readyState(t,
		Objective{ID: "fallback", Success: Predicate{Op: "event", Match: "ANTENNA HGA -> LGA"}},
		Objective{ID: "rebooted", Success: Predicate{Op: "event", Match: `^PROBE REBOOTED \(\d+ -> \d+`, Regex: true}},
	)
	startGame(t, state)
	g := NewGrader(state, "127.0.0.1:1")
	defer g.Close()

	frame := func(antenna, reboots byte, crcOK bool) *emulator.TelemetryFrame {
		return &emulator.TelemetryFrame{CRCOK: crcOK, Mode: "NOMINAL", LastFault: "-", Reboots: reboots,
			Comms: &emulator.CommsData{Antenna: antenna, Budget: 100, XStat: 1, HGADeployPct: 100}}
	}
	grade := func(f *emulator.TelemetryFrame) {
		t.Helper()
		if err := g.Grade(f); err != nil {
			t.Fatalf("Grade: %v", err)
		}
	}

	grade(frame(0, 0, true)) // first frame: nothing to compare against
	if got := status(t, &state.evaluation, "fallback"); got != ObjectiveStatusActive {
		t.Fatalf("fallback = %s after the first frame, want active", got)
	}

	// A corrupt frame claiming LGA is no evidence, and isn't what the next
	// frame is compared against.
	grade(frame(1, 0, false))
	if got := status(t, &state.evaluation, "fallback"); got != ObjectiveStatusActive {
		t.Fatalf("fallback = %s after a bad-CRC frame, want active", got)
	}

	grade(frame(1, 1, true))
	for _, id := range []string{"fallback", "rebooted"} {
		if got := status(t, &state.evaluation, id); got != ObjectiveStatusComplete {
			t.Errorf("%s = %s, want complete", id, got)
		}
	}
}
