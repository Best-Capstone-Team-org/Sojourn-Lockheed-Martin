package game

import (
	"slices"
	"testing"
)

func pinged(uptime uint32) Frame {
	return Frame{UptimeS: uptime, Log: []CommandLogEntry{{Verb: "PING"}}}
}

func quiet(uptime uint32) Frame {
	return Frame{UptimeS: uptime}
}

func TestFirstCompletionIsKept(t *testing.T) {
	e := frameEvaluation(t, Objective{
		ID:          "ping",
		Retractable: true,
		Success:     Predicate{Op: "commanded", Verb: "PING"},
	})
	transition := func(frame int, uptime uint32, from, to ObjectiveStatus) []ObjectiveTransition {
		return []ObjectiveTransition{{Frame: frame, UptimeS: uptime, Objective: "ping", From: from, To: to}}
	}

	steps := []struct {
		frame Frame
		want  []ObjectiveTransition
	}{
		{quiet(5), nil},
		{pinged(10), transition(2, 10, ObjectiveStatusActive, ObjectiveStatusComplete)},
		{pinged(15), nil},
		{quiet(20), transition(4, 20, ObjectiveStatusComplete, ObjectiveStatusActive)},
		{pinged(25), transition(5, 25, ObjectiveStatusActive, ObjectiveStatusComplete)},
	}
	for i, step := range steps {
		if err := e.Evaluate(step.frame); err != nil {
			t.Fatalf("frame %d: %v", i+1, err)
		}
		if got := e.Transitions(); !slices.Equal(got, step.want) {
			t.Errorf("frame %d transitions = %+v, want %+v", i+1, got, step.want)
		}
	}

	state, _ := e.GetObjectiveState("ping")
	if state.FirstFrame != 2 || state.FirstUptimeS != 10 {
		t.Errorf("first completion = frame %d at %d s, want frame 2 at 10 s", state.FirstFrame, state.FirstUptimeS)
	}

	e.reset()
	if state, _ := e.GetObjectiveState("ping"); state.FirstFrame != 0 || len(e.Transitions()) != 0 {
		t.Errorf("after reset: first frame %d, transitions %v; want none", state.FirstFrame, e.Transitions())
	}
}

func TestTransitionsCountSettleFrames(t *testing.T) {
	e := frameEvaluation(t, Objective{ID: "ping", Success: Predicate{Op: "commanded", Verb: "PING"}})
	e.scenario.Setup = &Setup{SettleFrames: 2}

	for _, frame := range []Frame{pinged(5), pinged(10)} {
		if err := e.Evaluate(frame); err != nil {
			t.Fatal(err)
		}
		if got := e.Transitions(); got != nil {
			t.Errorf("settle frame produced transitions %+v", got)
		}
	}
	if err := e.Evaluate(pinged(15)); err != nil {
		t.Fatal(err)
	}

	want := []ObjectiveTransition{{Frame: 3, UptimeS: 15, Objective: "ping",
		From: ObjectiveStatusActive, To: ObjectiveStatusComplete}}
	if got := e.Transitions(); !slices.Equal(got, want) {
		t.Errorf("transitions = %+v, want %+v", got, want)
	}
}

func TestTransitionsRecordFailure(t *testing.T) {
	e := frameEvaluation(t, Objective{
		ID:      "careful",
		Success: Predicate{Op: "commanded", Verb: "NEVER"},
		Fail:    &Predicate{Op: "commanded", Verb: "SAFE"},
	})
	if err := e.Evaluate(Frame{UptimeS: 5, Log: []CommandLogEntry{{Verb: "SAFE"}}}); err != nil {
		t.Fatal(err)
	}

	want := []ObjectiveTransition{{Frame: 1, UptimeS: 5, Objective: "careful",
		From: ObjectiveStatusActive, To: ObjectiveStatusFailed}}
	if got := e.Transitions(); !slices.Equal(got, want) {
		t.Errorf("transitions = %+v, want %+v", got, want)
	}
	if state, _ := e.GetObjectiveState("careful"); state.FirstFrame != 0 {
		t.Errorf("failed objective has first completion frame %d", state.FirstFrame)
	}
}

func TestFailedPassRecordsNothing(t *testing.T) {
	e := frameEvaluation(t, Objective{ID: "ping", Success: Predicate{Op: "commanded", Verb: "PING"}}, modeIs2)
	if err := e.Evaluate(Frame{UptimeS: 5, Log: []CommandLogEntry{{Verb: "PING"}}, Memory: sram(0x10, 0)}); err != nil {
		t.Fatal(err)
	}
	if len(e.Transitions()) != 1 {
		t.Fatalf("transitions = %+v, want the completion", e.Transitions())
	}

	if err := e.Evaluate(Frame{UptimeS: 10, Memory: &failingMemoryReader{}}); err == nil {
		t.Fatal("expected the pass to fail")
	}
	if got := e.Transitions(); got != nil {
		t.Errorf("failed pass left transitions %+v", got)
	}
}
