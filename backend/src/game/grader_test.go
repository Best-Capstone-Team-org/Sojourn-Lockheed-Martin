package game

import (
	"sojourn/emulator"
	"testing"
)

func TestGraderStartsAndGradesTelemetry(t *testing.T) {
	s := frameScenario(t, Objective{
		ID:      "safe",
		Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"},
	})
	state := &State{phase: PhaseReady, scenario: s}
	g := NewGrader(state, "127.0.0.1:0")
	defer g.Close()

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true, Mode: "NOMINAL"}); err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if state.Phase() != PhaseInProgress {
		t.Fatalf("phase = %v, want in progress", state.Phase())
	}
	if got := status(t, s, "safe"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}

	// A frame that failed CRC is not graded.
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

func TestGraderWithoutIntrospectionFailsMemoryObjectives(t *testing.T) {
	s := frameScenario(t, modeIs2)
	state := &State{phase: PhaseReady, scenario: s}

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

func TestGraderIgnoresFramesWithNoScenario(t *testing.T) {
	g := NewGrader(&State{}, "127.0.0.1:1")

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err != nil {
		t.Fatalf("Grade: %v", err)
	}
}
