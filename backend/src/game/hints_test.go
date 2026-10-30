package game

import (
	"slices"
	"sojourn/emulator"
	"testing"
)

// revealedHints returns the hints the player is shown for objective id.
func revealedHints(t *testing.T, state *State, id string) []string {
	t.Helper()
	out := make(chan DownlinkMessage, 1)
	state.sendStateDownlink(out)
	for _, objective := range (<-out).(*StateDownlinkMessage).Objectives {
		if objective.ID == id {
			return objective.Hints
		}
	}
	t.Fatalf("no objective %q in the state message", id)
	return nil
}

func TestHintsRevealCumulativelyWhileActive(t *testing.T) {
	state := readyState(t,
		Objective{
			ID:      "first",
			Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"},
			Hints:   []Hint{{AfterFrames: 1, Text: "one"}, {AfterFrames: 3, Text: "three"}},
		},
		Objective{
			ID:       "second",
			Requires: []string{"first"},
			Success:  Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "NOMINAL"},
			Hints:    []Hint{{AfterFrames: 0, Text: "now"}, {AfterFrames: 2, Text: "later"}},
		},
	)
	startGame(t, state)
	grader := NewGrader(state, "127.0.0.1:1")

	expect := func(step string, first, second []string) {
		t.Helper()
		if got := revealedHints(t, state, "first"); !slices.Equal(got, first) {
			t.Errorf("%s: first hints = %q, want %q", step, got, first)
		}
		if got := revealedHints(t, state, "second"); !slices.Equal(got, second) {
			t.Errorf("%s: second hints = %q, want %q", step, got, second)
		}
	}
	frame := func(mode string) {
		t.Helper()
		if err := grader.Grade(&emulator.TelemetryFrame{CRCOK: true, Mode: mode}); err != nil {
			t.Fatalf("Grade: %v", err)
		}
	}

	expect("before any frame", []string{}, []string{})

	frame("BOOT")
	expect("frame 1", []string{"one"}, []string{})

	frame("BOOT")
	expect("frame 2", []string{"one"}, []string{})

	frame("BOOT")
	expect("frame 3", []string{"one", "three"}, []string{})

	// first completes and second unlocks in the same pass, so second's
	// zero-frame hint shows at once and first's hints are withdrawn.
	frame("SAFE")
	expect("frame 4", []string{}, []string{"now"})

	frame("BOOT")
	expect("frame 5", []string{}, []string{"now", "later"})

	frame("NOMINAL")
	expect("frame 6", []string{}, []string{})
}

// framesActive counts only frames an objective spent active.
func TestFramesActiveCounting(t *testing.T) {
	never := Predicate{Op: "commanded", Verb: "NEVER"}
	objective := func(id string) Objective { return Objective{ID: id, Success: never} }
	count := func(e *Evaluation, id string) int {
		t.Helper()
		state, ok := e.GetObjectiveState(id)
		if !ok {
			t.Fatalf("no state for %q", id)
		}
		return state.FramesActive
	}

	t.Run("counts each evaluated frame and resets", func(t *testing.T) {
		e := frameEvaluation(t, objective("o"))
		for range 3 {
			if err := e.Evaluate(Frame{}); err != nil {
				t.Fatal(err)
			}
		}
		if got := count(e, "o"); got != 3 {
			t.Fatalf("FramesActive = %d, want 3", got)
		}
		e.reset()
		if got := count(e, "o"); got != 0 {
			t.Errorf("FramesActive after reset = %d, want 0", got)
		}
	})

	t.Run("skips settle frames", func(t *testing.T) {
		e := frameEvaluation(t, objective("o"))
		e.scenario.Setup = &Setup{SettleFrames: 2}
		for range 3 {
			if err := e.Evaluate(Frame{}); err != nil {
				t.Fatal(err)
			}
		}
		if got := count(e, "o"); got != 1 {
			t.Errorf("FramesActive = %d, want 1 after 2 settle frames", got)
		}
	})

	t.Run("not while locked", func(t *testing.T) {
		locked := objective("locked")
		locked.Requires = []string{"o"}
		e := frameEvaluation(t, objective("o"), locked)
		for range 2 {
			if err := e.Evaluate(Frame{}); err != nil {
				t.Fatal(err)
			}
		}
		if got := count(e, "locked"); got != 0 {
			t.Errorf("locked FramesActive = %d, want 0", got)
		}
	})

	t.Run("stops once failed", func(t *testing.T) {
		failing := objective("o")
		failing.Fail = &Predicate{Op: "commanded", Verb: "SAFE"}
		failing.Hints = []Hint{{AfterFrames: 0, Text: "too late"}}
		state := readyState(t, failing)
		startGame(t, state)

		if err := state.Evaluate(Frame{}); err != nil {
			t.Fatal(err)
		}
		if err := state.Evaluate(Frame{Log: []CommandLogEntry{{Verb: "SAFE"}}}); err != nil {
			t.Fatal(err)
		}
		if err := state.Evaluate(Frame{}); err != nil {
			t.Fatal(err)
		}
		if got := status(t, &state.evaluation, "o"); got != ObjectiveStatusFailed {
			t.Fatalf("status = %s, want failed", got)
		}
		if got := count(&state.evaluation, "o"); got != 1 {
			t.Errorf("FramesActive = %d, want 1: counting continued after failure", got)
		}
		if got := revealedHints(t, state, "o"); len(got) != 0 {
			t.Errorf("failed objective revealed hints %q", got)
		}
	})

	t.Run("rolled back with a failed pass", func(t *testing.T) {
		e := frameEvaluation(t, modeIs2)
		if err := e.Evaluate(Frame{Memory: sram(0x10, 0)}); err != nil {
			t.Fatal(err)
		}
		if err := e.Evaluate(Frame{Memory: &failingMemoryReader{}}); err == nil {
			t.Fatal("expected the pass to fail on a memory read error")
		}
		if got := count(e, "mode"); got != 1 {
			t.Errorf("FramesActive = %d, want 1: the failed pass was counted", got)
		}
	})
}
