package game

import (
	"sojourn/emulator"
	"testing"
)

func TestGraderGradesTelemetry(t *testing.T) {
	s := frameScenario(t, Objective{
		ID:      "safe",
		Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"},
	})
	state := &State{phase: PhaseReady, Scenario: s}
	if err := state.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	g := NewGrader(state, "127.0.0.1:0")
	defer g.Close()

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true, Mode: "NOMINAL"}); err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if got := status(t, s, "safe"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}

	// A frame that failed CRC is rejected and not graded.
	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: false, Mode: "SAFE"}); err == nil {
		t.Fatal("expected Grade to fail on a bad CRC")
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

func TestGraderWithoutIntrospectionFailsMemoryObjectives(t *testing.T) {
	s := frameScenario(t, modeIs2)
	state := &State{phase: PhaseReady, Scenario: s}
	if err := state.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Nothing listens on port 1, so the capture cannot connect.
	g := NewGrader(state, "127.0.0.1:1")
	defer g.Close()

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err == nil {
		t.Fatal("expected Grade to fail without an introspection connection")
	}
	if got := status(t, s, "mode"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}
}

func TestGraderRejectsFramesWhenNotInProgress(t *testing.T) {
	s := frameScenario(t, Objective{
		ID:      "safe",
		Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"},
	})
	state := &State{phase: PhaseReady, Scenario: s}
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
