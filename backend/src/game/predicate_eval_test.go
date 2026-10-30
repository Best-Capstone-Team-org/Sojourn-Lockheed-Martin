package game

import (
	"errors"
	"strings"
	"testing"
)

// shortMemory returns fewer bytes than asked for.
type shortMemory struct{}

func (shortMemory) Read(addr uint32, length int) ([]byte, error) { return make([]byte, length-1), nil }

type stubScripts struct {
	result bool
	err    error
}

func (s stubScripts) EvaluateScript(lang, entry string, frame Frame) (bool, error) {
	return s.result, s.err
}

func TestEvaluatePredicateErrors(t *testing.T) {
	e := frameEvaluation(t)
	mem := sram(0)
	bad := &AddressRef{Sym: "missing"}
	at := &AddressRef{Sym: "g_mode"}
	notJSON := []byte("{")
	failing := Predicate{Op: "tlm_bits", Path: "s", Mask: 1}
	tele := map[string]any{"s": "str"}

	tests := []struct {
		name  string
		p     Predicate
		frame Frame
		want  string
	}{
		{"tlm_bits mask", failing, Frame{CRCOK: true, Telemetry: tele}, "mask"},
		{"all of", Predicate{Op: "all", Of: notJSON}, Frame{}, "'all'"},
		{"all child", Predicate{Op: "all", Of: rawPredicate(t, []Predicate{failing})}, Frame{CRCOK: true, Telemetry: tele}, "mask"},
		{"any of", Predicate{Op: "any", Of: notJSON}, Frame{}, "'any'"},
		{"any child", Predicate{Op: "any", Of: rawPredicate(t, []Predicate{failing})}, Frame{CRCOK: true, Telemetry: tele}, "mask"},
		{"not of", Predicate{Op: "not", Of: notJSON}, Frame{}, "'not'"},
		{"not child", Predicate{Op: "not", Of: rawPredicate(t, failing)}, Frame{CRCOK: true, Telemetry: tele}, "mask"},
		{"ever of", Predicate{Op: "ever", Of: notJSON}, Frame{}, "'ever'"},
		{"sustained of", Predicate{Op: "sustained", Of: notJSON}, Frame{}, "'sustained'"},
		{"sustained frames", Predicate{Op: "sustained", Of: rawPredicate(t, failing)}, Frame{}, "frames"},
		{"within of", Predicate{Op: "within", Of: notJSON}, Frame{}, "'within'"},
		{"within frames", Predicate{Op: "within", Of: rawPredicate(t, failing)}, Frame{}, "frames"},
		{"within child", Predicate{Op: "within", Frames: 2, Of: rawPredicate(t, failing)}, Frame{CRCOK: true, Telemetry: tele}, "mask"},
		{"commanded address", Predicate{Op: "commanded", At: bad}, Frame{}, "commanded address"},
		{"script runner", Predicate{Op: "script"}, Frame{}, "runner"},
		{"unknown", Predicate{Op: "nope"}, Frame{}, "Unknown"},
		{"event regex", Predicate{Op: "event", Match: "(", Regex: true}, Frame{}, "regex"},
		{"mem not hex string", Predicate{Op: "mem", At: at, Len: 1, Value: 1.0}, Frame{Memory: mem}, "hex string"},
		{"mem bad hex", Predicate{Op: "mem", At: at, Len: 1, Value: "zz"}, Frame{Memory: mem}, "Invalid hex"},
		{"mem short", Predicate{Op: "mem", At: at, Len: 2, Value: "0000"}, Frame{Memory: shortMemory{}}, "Short"},
		{"mem read", Predicate{Op: "mem", At: at, Len: 1}, Frame{Memory: &failingMemoryReader{}}, "read"},
		{"mem_u8 short", Predicate{Op: "mem_u8", At: at}, Frame{Memory: shortMemory{}}, "less than"},
		{"mem_u16 short", Predicate{Op: "mem_u16", At: at}, Frame{Memory: shortMemory{}}, "less than"},
		{"mem_u32 short", Predicate{Op: "mem_u32", At: at}, Frame{Memory: shortMemory{}}, "less than"},
		{"mem_bits short", Predicate{Op: "mem_bits", At: at, Width: 2}, Frame{Memory: shortMemory{}}, "less than"},
		{"mem_bits width", Predicate{Op: "mem_bits", At: at, Width: 3}, Frame{Memory: mem}, "Unsupported width"},
	}

	for _, op := range []string{"mem_u8", "mem_u16", "mem_u32", "mem_bits", "mem", "mem_changed"} {
		tests = append(tests,
			struct {
				name  string
				p     Predicate
				frame Frame
				want  string
			}{op + " no memory", Predicate{Op: op, At: at, Len: 1}, Frame{}, "not available"},
			struct {
				name  string
				p     Predicate
				frame Frame
				want  string
			}{op + " no at", Predicate{Op: op, Len: 1}, Frame{Memory: mem}, "'at'"},
			struct {
				name  string
				p     Predicate
				frame Frame
				want  string
			}{op + " bad address", Predicate{Op: op, At: bad, Len: 1}, Frame{Memory: mem}, "resolve"},
			struct {
				name  string
				p     Predicate
				frame Frame
				want  string
			}{op + " read error", Predicate{Op: op, At: at, Len: 1}, Frame{Memory: &failingMemoryReader{}}, "read"},
		)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.baseline = mem
			if tt.p.Op == "mem_changed" && tt.frame.Memory == nil {
				e.baseline = nil
			}
			got, msg := e.evaluatePredicate(tt.frame, tt.p)
			if got || !strings.Contains(msg, tt.want) {
				t.Fatalf("got %v, %q; want error containing %q", got, msg, tt.want)
			}
		})
	}
}

func TestEvaluateMemChangedErrors(t *testing.T) {
	e := frameEvaluation(t)
	at := &AddressRef{Sym: "g_buf"}
	p := Predicate{Op: "mem_changed", At: at, Len: 2}

	if _, msg := e.evaluatePredicate(Frame{Memory: sram(0)}, p); !strings.Contains(msg, "Baseline") {
		t.Errorf("no baseline: %q", msg)
	}

	e.baseline = &failingMemoryReader{}
	if _, msg := e.evaluatePredicate(Frame{Memory: sram(0)}, p); !strings.Contains(msg, "baseline memory") {
		t.Errorf("baseline read: %q", msg)
	}

	e.baseline = sram(0)
	if _, msg := e.evaluatePredicate(Frame{Memory: shortMemory{}}, p); !strings.Contains(msg, "current") {
		t.Errorf("short current: %q", msg)
	}

	e.baseline = shortMemory{}
	if _, msg := e.evaluatePredicate(Frame{Memory: sram(0)}, p); !strings.Contains(msg, "baseline") {
		t.Errorf("short baseline: %q", msg)
	}
}

func TestEvaluateMemNotEqual(t *testing.T) {
	e := frameEvaluation(t)
	p := Predicate{Op: "mem", At: &AddressRef{Sym: "g_buf"}, Len: 2, Cmp: "ne", Value: "0102"}

	if got, msg := e.evaluatePredicate(Frame{Memory: sram(0x20, 1, 2)}, p); got || msg != "" {
		t.Errorf("equal memory: %v, %q", got, msg)
	}
	if got, msg := e.evaluatePredicate(Frame{Memory: sram(0x20, 1, 3)}, p); !got || msg != "" {
		t.Errorf("different memory: %v, %q", got, msg)
	}
}

func TestEvaluateTemporalChildErrorsInHistory(t *testing.T) {
	e := frameEvaluation(t)
	child := Predicate{Op: "tlm_bits", Path: "s", Mask: 1, Cmp: "eq", Value: 1.0}
	e.history = []Frame{{Telemetry: map[string]any{"s": "str"}}, {Telemetry: map[string]any{"s": "str"}}}
	now := Frame{CRCOK: true, Telemetry: map[string]any{"s": uint8(0)}}

	for _, p := range []Predicate{
		{Op: "ever", Of: rawPredicate(t, child)},
		{Op: "sustained", Frames: 2, Of: rawPredicate(t, child)},
		{Op: "within", Frames: 3, Of: rawPredicate(t, child)},
	} {
		if got, msg := e.evaluatePredicate(now, p); got || msg == "" {
			t.Errorf("%s: got %v, %q; want history error", p.Op, got, msg)
		}
	}
}

func TestEvaluateScriptPredicate(t *testing.T) {
	e := frameEvaluation(t)
	p := Predicate{Op: "script", Lang: "python", Entry: "checks/a.py:f"}

	e.scriptRunner = stubScripts{result: true}
	if got, msg := e.evaluatePredicate(Frame{}, p); !got || msg != "" {
		t.Errorf("got %v, %q; want true", got, msg)
	}

	e.scriptRunner = stubScripts{err: errors.New("boom")}
	if got, msg := e.evaluatePredicate(Frame{}, p); got || !strings.Contains(msg, "boom") {
		t.Errorf("got %v, %q; want script error", got, msg)
	}
}

func TestEvaluateObjectiveErrors(t *testing.T) {
	failing := Predicate{Op: "nope"}
	ok := Predicate{Op: "channel_absent", ID: "X"}

	tests := []struct {
		name      string
		objective Objective
		want      string
	}{
		{"fail", Objective{ID: "o", Success: ok, Fail: &failing}, "fail"},
		{"partial", Objective{ID: "o", Success: ok, Partial: []Partial{{When: failing}}}, "partial"},
		{"success", Objective{ID: "o", Success: failing}, "success"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := frameEvaluation(t, tt.objective)
			if msg := e.evaluateObjective(Frame{}, tt.objective); !strings.Contains(msg, tt.want) {
				t.Fatalf("msg = %q, want mention of %q", msg, tt.want)
			}
		})
	}

	e := frameEvaluation(t)
	if msg := e.evaluateObjective(Frame{}, Objective{ID: "ghost"}); msg == "" {
		t.Fatal("expected error for an objective without state")
	}
}

func TestEvaluateObjectiveSkipsLockedAndFailed(t *testing.T) {
	o := Objective{ID: "o", Success: Predicate{Op: "nope"}}
	e := frameEvaluation(t, o)

	for _, s := range []ObjectiveStatus{ObjectiveStatusLocked, ObjectiveStatusFailed} {
		e.objectiveStates["o"].Status = s
		if msg := e.evaluateObjective(Frame{}, o); msg != "" {
			t.Errorf("%s objective evaluated: %q", s, msg)
		}
	}
}

func TestCopyObjectiveStatesNil(t *testing.T) {
	copied := copyObjectiveStates(map[string]*ObjectiveState{"x": nil})
	if v, ok := copied["x"]; !ok || v != nil {
		t.Fatalf("copied = %v", copied)
	}
}

func TestNumericValueTypes(t *testing.T) {
	for _, v := range []any{int(1), int8(1), int16(1), int32(1), int64(1), uint(1), uint8(1), uint16(1), uint32(1), uint64(1), float32(1), float64(1)} {
		if n, ok := numericValue(v); !ok || n != 1 {
			t.Errorf("numericValue(%T) = %v, %v", v, n, ok)
		}
	}
	if _, ok := numericValue("1"); ok {
		t.Error("string treated as numeric")
	}
}

func TestCompareValuesUnsupported(t *testing.T) {
	tests := []struct {
		actual, expected any
		cmp              string
	}{
		{1.0, 1.0, "bogus"},
		{"a", "a", "lt"},
		{"a", 1.0, "eq"},
		{1.0, []string{"a"}, "in"},
	}
	for _, tt := range tests {
		if compareValues(tt.actual, tt.expected, tt.cmp) {
			t.Errorf("compareValues(%v, %v, %q) = true", tt.actual, tt.expected, tt.cmp)
		}
	}
	if !compareValues("a", "b", "ne") {
		t.Error("ne on strings")
	}
}

func TestApplyMaskTypes(t *testing.T) {
	tests := []struct {
		value, want any
	}{
		{uint8(0xFF), uint8(0x0F)},
		{uint16(0xFFFF), uint16(0x0F)},
		{uint32(0xFFFFFFFF), uint32(0x0F)},
		{uint64(0xFF), uint64(0x0F)},
	}
	for _, tt := range tests {
		if got, msg := applyMask(tt.value, 0x0F); got != tt.want || msg != "" {
			t.Errorf("applyMask(%T) = %v, %q", tt.value, got, msg)
		}
	}
	if _, msg := applyMask(1.0, 1); msg == "" {
		t.Error("expected float mask to fail")
	}
}

func TestIntrospectionBytesToValuesErrors(t *testing.T) {
	if _, msg := introspectionBytesToValues([]byte{1}, 2); msg == "" {
		t.Error("expected short data to fail")
	}
	if _, msg := introspectionBytesToValues([]byte{1, 2, 3}, 3); msg == "" {
		t.Error("expected width 3 to fail")
	}
}

func TestGetIntrospectionValueErrors(t *testing.T) {
	e := frameEvaluation(t)
	at := &AddressRef{Sym: "g_mode"}

	tests := []struct {
		name  string
		frame Frame
		p     Predicate
	}{
		{"no memory", Frame{}, Predicate{At: at, Len: 1}},
		{"no at", Frame{Memory: sram(0)}, Predicate{Len: 1}},
		{"bad address", Frame{Memory: sram(0)}, Predicate{At: &AddressRef{Sym: "nope"}, Len: 1}},
		{"read error", Frame{Memory: &failingMemoryReader{}}, Predicate{At: at, Len: 1}},
	}
	for _, tt := range tests {
		if data, msg := e.getIntrospectionValue(tt.frame, tt.p); data != nil || msg == "" {
			t.Errorf("%s: got %v, %q", tt.name, data, msg)
		}
	}

	if data, msg := e.getIntrospectionValue(Frame{Memory: sram(0x10, 7)}, Predicate{At: at, Len: 1}); msg != "" || data[0] != 7 {
		t.Errorf("got %v, %q", data, msg)
	}
}

func TestTelemetryLookupMisses(t *testing.T) {
	frame := Frame{CRCOK: true, Telemetry: map[string]any{"mode": "SAFE", "channels": "none"}}
	if _, ok := getTelemetryValue(frame, "mode.sub"); ok {
		t.Error("descended into a scalar")
	}
	if telemetryChannelPresent(frame, "HK") {
		t.Error("channel present without a channels map")
	}
}

func TestCollectMemoryRangesErrors(t *testing.T) {
	tests := []struct {
		name string
		p    Predicate
	}{
		{"unresolvable", Predicate{Op: "mem_u8", At: &AddressRef{Sym: "missing"}}},
		{"bad of", Predicate{Op: "all", Of: []byte("{")}},
		{"bad single of", Predicate{Op: "not", Of: []byte("{")}},
		{"bad child", Predicate{Op: "not", Of: rawPredicate(t, Predicate{Op: "mem_u8", At: &AddressRef{}})}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Scenario{Objectives: []Objective{{ID: "o", Success: tt.p}}}
			if _, err := NewEvaluation(s); err == nil {
				t.Fatal("expected NewEvaluation to fail")
			}
		})
	}
}

func TestMemoryRangesMergeNested(t *testing.T) {
	e := frameEvaluation(t, Objective{ID: "o", Success: Predicate{Op: "all", Of: rawPredicate(t, []Predicate{
		{Op: "mem", At: &AddressRef{Addr: "0x20000000"}, Len: 8},
		{Op: "mem_bits", At: &AddressRef{Addr: "0x20000002"}},
		{Op: "mem_bits", At: &AddressRef{Addr: "0x20000040"}, Width: 2},
	})}})

	got := e.MemoryRanges()
	if len(got) != 2 || got[0].Len != 8 || got[1].Len != 2 {
		t.Fatalf("ranges = %v", got)
	}
}

func TestRequiredHistoryDepth(t *testing.T) {
	tlm := Predicate{Op: "tlm", Path: "x", Cmp: "eq", Value: 1.0}
	tests := []struct {
		name string
		p    Predicate
		want int
	}{
		{"plain", tlm, 0},
		{"ever", Predicate{Op: "ever", Of: rawPredicate(t, tlm)}, -1},
		{"nested ever", Predicate{Op: "not", Of: rawPredicate(t, Predicate{Op: "ever", Of: rawPredicate(t, tlm)})}, -1},
		{"nested within", Predicate{Op: "sustained", Frames: 2, Of: rawPredicate(t, Predicate{Op: "within", Frames: 3, Of: rawPredicate(t, tlm)})}, 5},
		{"bad of", Predicate{Op: "all", Of: []byte("{")}, -1},
	}
	for _, tt := range tests {
		s := &Scenario{Objectives: []Objective{{ID: "o", Success: tt.p}}}
		if got := s.requiredHistoryDepth(); got != tt.want {
			t.Errorf("%s: depth = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestEvaluateUnboundedHistory(t *testing.T) {
	e := frameEvaluation(t, Objective{ID: "o", Success: Predicate{Op: "ever", Of: rawPredicate(t, Predicate{Op: "tlm", Path: "x", Cmp: "eq", Value: 1.0})}})
	for range 5 {
		if err := e.Evaluate(Frame{CRCOK: true, Telemetry: map[string]any{"x": 0.0}}); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.history) != 5 {
		t.Fatalf("history = %d frames, want 5", len(e.history))
	}

	bad := frameEvaluation(t, Objective{ID: "o", Success: Predicate{Op: "nope"}})
	if err := bad.Evaluate(Frame{}); err == nil {
		t.Fatal("expected Evaluate to report predicate errors")
	}
}

func TestEvaluateLogicalShortCircuit(t *testing.T) {
	e := frameEvaluation(t)
	yes := Predicate{Op: "channel_absent", ID: "X"}
	no := Predicate{Op: "channel_present", ID: "X"}

	if got, _ := e.evaluatePredicate(Frame{}, Predicate{Op: "all", Of: rawPredicate(t, []Predicate{yes, no})}); got {
		t.Error("all with a false child was true")
	}
	if got, _ := e.evaluatePredicate(Frame{}, Predicate{Op: "any", Of: rawPredicate(t, []Predicate{no, no})}); got {
		t.Error("any with only false children was true")
	}
	if got, _ := e.evaluatePredicate(Frame{Events: []string{"boot"}}, Predicate{Op: "event", Match: "^fault", Regex: true}); got {
		t.Error("regex event matched nothing but was true")
	}
	if got, _ := e.evaluatePredicate(Frame{}, Predicate{Op: "budget", Resource: "writes", Cmp: "eq", Value: 0.0}); got {
		t.Error("budget without the resource was true")
	}
}

func TestEvaluateEverMatchesHistory(t *testing.T) {
	e := frameEvaluation(t)
	e.history = []Frame{{CRCOK: true, Telemetry: map[string]any{"x": 1.0}}}
	p := Predicate{Op: "ever", Of: rawPredicate(t, Predicate{Op: "tlm", Path: "x", Cmp: "eq", Value: 1.0})}

	if got, msg := e.evaluatePredicate(Frame{CRCOK: true, Telemetry: map[string]any{"x": 0.0}}, p); !got || msg != "" {
		t.Fatalf("got %v, %q; want true from history", got, msg)
	}
}

func TestUpdateObjectiveStatesSkipsUnknown(t *testing.T) {
	e := frameEvaluation(t, Objective{ID: "o"})
	delete(e.objectiveStates, "o")
	e.updateObjectiveStates()
	if _, ok := e.GetObjectiveState("o"); ok {
		t.Fatal("updateObjectiveStates created a state")
	}
}
