package game

import (
	"encoding/json"
	"reflect"
	"sojourn/emulator"
	"testing"
)

// memory is a frame's captured memory, based at SRAM.
type memory []byte

func (m memory) Read(addr uint32, length int) ([]byte, error) {
	start := addr - 0x20000000
	return append([]byte(nil), m[start:int(start)+length]...), nil
}

func sram(at uint32, data ...byte) memory {
	m := make(memory, 0x100)
	copy(m[at:], data)
	return m
}

func rawPredicate(t *testing.T, p any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func frameEvaluation(t *testing.T, objectives ...Objective) *Evaluation {
	t.Helper()
	s := &Scenario{
		Objectives: objectives,
		Symbols: SymbolsFile{
			Symbols: map[string]string{"g_mode": "0x20000010", "g_buf": "0x20000020"},
		},
	}
	e, err := NewEvaluation(s)
	if err != nil {
		t.Fatalf("NewEvaluation: %v", err)
	}
	return &e
}

func readyState(t *testing.T, objectives ...Objective) *State {
	t.Helper()
	e := frameEvaluation(t, objectives...)
	return &State{phase: PhaseReady, Scenario: e.scenario, evaluation: *e}
}

func status(t *testing.T, e *Evaluation, id string) ObjectiveStatus {
	t.Helper()
	state, ok := e.GetObjectiveState(id)
	if !ok {
		t.Fatalf("no state for %q", id)
	}
	return state.Status
}

var modeIs2 = Objective{
	ID:      "mode",
	Success: Predicate{Op: "mem_u8", At: &AddressRef{Sym: "g_mode"}, Cmp: "eq", Value: 2.0},
}

func TestStepEvaluatesFrameMemory(t *testing.T) {
	s := frameEvaluation(t, modeIs2)

	if err := s.Evaluate(Frame{Memory: sram(0x10, 1)}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := status(t, s, "mode"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}

	if err := s.Evaluate(Frame{Memory: sram(0x10, 2)}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := status(t, s, "mode"); got != ObjectiveStatusComplete {
		t.Fatalf("status = %s, want complete", got)
	}
}

func TestStepWithoutRequiredMemoryFails(t *testing.T) {
	s := frameEvaluation(t, modeIs2)

	if err := s.Evaluate(Frame{}); err == nil {
		t.Fatal("expected Step without memory to fail")
	}
	if got := status(t, s, "mode"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}
}

func TestMemChangedComparesAgainstFirstFrame(t *testing.T) {
	s := frameEvaluation(t, Objective{
		ID:      "patched",
		Success: Predicate{Op: "mem_changed", At: &AddressRef{Sym: "g_buf"}, Len: 4},
	})

	if err := s.Evaluate(Frame{Memory: sram(0x20, 0xAA)}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := status(t, s, "patched"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}

	if err := s.Evaluate(Frame{Memory: sram(0x20, 0xAA, 0, 0, 1)}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := status(t, s, "patched"); got != ObjectiveStatusComplete {
		t.Fatalf("status = %s, want complete", got)
	}
}

func TestSustainedSeesEachFramesMemory(t *testing.T) {
	s := frameEvaluation(t, Objective{
		ID: "hold",
		Success: Predicate{
			Op:     "sustained",
			Frames: 3,
			Of:     rawPredicate(t, Predicate{Op: "mem_u8", At: &AddressRef{Sym: "g_mode"}, Cmp: "eq", Value: 1.0}),
		},
	})

	// 1, 0, 1, 1 is never three in a row; one more 1 makes it.
	for i, v := range []byte{1, 0, 1, 1} {
		if err := s.Evaluate(Frame{Memory: sram(0x10, v)}); err != nil {
			t.Fatalf("Evaluate %d: %v", i, err)
		}
		if got := status(t, s, "hold"); got != ObjectiveStatusActive {
			t.Fatalf("after step %d status = %s, want active", i, got)
		}
	}

	if err := s.Evaluate(Frame{Memory: sram(0x10, 1)}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := status(t, s, "hold"); got != ObjectiveStatusComplete {
		t.Fatalf("status = %s, want complete", got)
	}
}

func TestHistoryIsBoundedByTemporalDepth(t *testing.T) {
	s := frameEvaluation(t, Objective{
		ID: "hot",
		Success: Predicate{
			Op:     "within",
			Frames: 2,
			Of:     rawPredicate(t, Predicate{Op: "tlm", Path: "temp", Cmp: "gt", Value: 50.0}),
		},
	})

	for i := 0; i < 10; i++ {
		if err := s.Evaluate(Frame{CRCOK: true, Telemetry: map[string]any{"temp": 20.0}}); err != nil {
			t.Fatalf("Evaluate %d: %v", i, err)
		}
	}

	if len(s.history) > 2 {
		t.Fatalf("history holds %d frames, want at most 2", len(s.history))
	}

	if err := s.Evaluate(Frame{CRCOK: true, Telemetry: map[string]any{"temp": 60.0}}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := status(t, s, "hot"); got != ObjectiveStatusComplete {
		t.Fatalf("status = %s, want complete", got)
	}
}

func TestStepWithoutMemoryPredicatesNeedsNoMemory(t *testing.T) {
	s := frameEvaluation(t, Objective{
		ID:      "safe",
		Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"},
	})

	if ranges := s.MemoryRanges(); len(ranges) != 0 {
		t.Fatalf("MemoryRanges = %v, want none", ranges)
	}

	if err := s.Evaluate(Frame{CRCOK: true, Telemetry: map[string]any{"mode": "SAFE"}}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := status(t, s, "safe"); got != ObjectiveStatusComplete {
		t.Fatalf("status = %s, want complete", got)
	}
}

func TestMemoryRangesCoverEveryPredicateMerged(t *testing.T) {
	fail := Predicate{Op: "mem_u16", At: &AddressRef{Addr: "0x20000011"}, Cmp: "eq", Value: 0.0}
	s := frameEvaluation(t, Objective{
		ID: "all",
		Success: Predicate{Op: "all", Of: rawPredicate(t, []Predicate{
			{Op: "mem_u8", At: &AddressRef{Sym: "g_mode"}, Cmp: "eq", Value: 1.0},
			{Op: "not", Of: rawPredicate(t, Predicate{Op: "mem_changed", At: &AddressRef{Sym: "g_buf"}, Len: 4})},
		})},
		Fail:    &fail,
		Partial: []Partial{{When: Predicate{Op: "mem_u32", At: &AddressRef{Addr: "0x20000022"}, Cmp: "eq", Value: 0.0}, Text: "x"}},
	})

	got := s.MemoryRanges()

	want := []emulator.MemoryRange{{Addr: 0x20000010, Len: 3}, {Addr: 0x20000020, Len: 6}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStateBaselineIsLastSettleFrame(t *testing.T) {
	state := readyState(t, Objective{
		ID:      "patched",
		Success: Predicate{Op: "mem_changed", At: &AddressRef{Sym: "g_buf"}, Len: 1},
	})
	state.Scenario.Setup = &Setup{SettleFrames: 2}
	s := &state.evaluation

	startGame(t, state)

	// Memory still changing while the firmware settles is not a patch.
	for i, v := range []byte{1, 2} {
		if err := state.Evaluate(Frame{Memory: sram(0x20, v)}); err != nil {
			t.Fatalf("settle Step %d: %v", i, err)
		}
	}

	if err := state.Evaluate(Frame{Memory: sram(0x20, 2)}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := status(t, s, "patched"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}

	if err := state.Evaluate(Frame{Memory: sram(0x20, 3)}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := status(t, s, "patched"); got != ObjectiveStatusComplete {
		t.Fatalf("status = %s, want complete", got)
	}
}

func TestStateStepBeforeStartFails(t *testing.T) {
	state := readyState(t)

	if err := state.Evaluate(Frame{}); err == nil {
		t.Fatal("expected Step before Start to fail")
	}
}
