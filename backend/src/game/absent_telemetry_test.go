package game

import (
	"testing"
)

func absentTelemetryFrame(crcOK bool) Frame {
	return Frame{CRCOK: crcOK, Telemetry: map[string]any{
		"mode": "NOMINAL",
		"channels": map[string]any{
			"TEMP":  int32(-1),
			"AUX":   "0x0004",
			"COMMS": map[string]any{"xstat": uint8(0x04)},
		},
	}}
}

func TestAbsentTelemetryIsFalse(t *testing.T) {
	e := frameEvaluation(t)
	good, bad := absentTelemetryFrame(true), absentTelemetryFrame(false)

	tests := []struct {
		name  string
		p     Predicate
		frame Frame
	}{
		{"tlm absent channel", Predicate{Op: "tlm", Path: "channels.MAG", Cmp: "eq", Value: 0.0}, good},
		{"tlm undefined path", Predicate{Op: "tlm", Path: "channels.COMMS.nope", Cmp: "eq", Value: 0.0}, good},
		{"tlm path through a value", Predicate{Op: "tlm", Path: "mode.x", Cmp: "eq", Value: 0.0}, good},
		{"tlm failed CRC", Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "NOMINAL"}, bad},
		{"tlm_bits absent channel", Predicate{Op: "tlm_bits", Path: "channels.MAG", Mask: 4, Cmp: "eq", Value: 4.0}, good},
		{"tlm_bits undefined path", Predicate{Op: "tlm_bits", Path: "channels.COMMS.nope", Mask: 4, Cmp: "eq", Value: 4.0}, good},
		{"tlm_bits on a channel object", Predicate{Op: "tlm_bits", Path: "channels.COMMS", Mask: 4, Cmp: "eq", Value: 4.0}, good},
		{"tlm_bits on a string", Predicate{Op: "tlm_bits", Path: "mode", Mask: 4, Cmp: "eq", Value: 4.0}, good},
		{"tlm_bits failed CRC", Predicate{Op: "tlm_bits", Path: "channels.COMMS.xstat", Mask: 4, Cmp: "eq", Value: 4.0}, bad},
		{"channel_present absent channel", Predicate{Op: "channel_present", ID: "MAG"}, good},
		{"channel_present failed CRC", Predicate{Op: "channel_present", ID: "TEMP"}, bad},
		{"channel_absent failed CRC", Predicate{Op: "channel_absent", ID: "MAG"}, bad},
		{"any of absent", Predicate{Op: "any", Of: rawPredicate(t, []Predicate{
			{Op: "tlm", Path: "channels.MAG", Cmp: "eq", Value: 0.0},
			{Op: "tlm_bits", Path: "channels.AUX", Mask: 4, Cmp: "eq", Value: 4.0},
		})}, good},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, msg := e.evaluatePredicate(tt.frame, tt.p); got || msg != "" {
				t.Fatalf("got %v, %q; want false with no error", got, msg)
			}
		})
	}
}

// The same predicates are true over a good frame that carries the data, so
// the false results above come from the data being absent.
func TestPresentTelemetryIsEvaluated(t *testing.T) {
	e := frameEvaluation(t)
	good := absentTelemetryFrame(true)

	for _, p := range []Predicate{
		{Op: "tlm", Path: "mode", Cmp: "eq", Value: "NOMINAL"},
		{Op: "tlm_bits", Path: "channels.COMMS.xstat", Mask: 4, Cmp: "eq", Value: 4.0},
		{Op: "tlm_bits", Path: "channels.TEMP", Mask: 4, Cmp: "eq", Value: 4.0},
		{Op: "channel_present", ID: "TEMP"},
		{Op: "channel_absent", ID: "MAG"},
	} {
		if got, msg := e.evaluatePredicate(good, p); !got || msg != "" {
			t.Errorf("%s %s%s: got %v, %q; want true", p.Op, p.Path, p.ID, got, msg)
		}
	}
}

func TestAbsentTelemetryDoesNotAbortPass(t *testing.T) {
	e := frameEvaluation(t,
		Objective{ID: "absent", Success: Predicate{Op: "tlm_bits", Path: "channels.COMMS", Mask: 4, Cmp: "eq", Value: 4.0}},
		Objective{ID: "missing", Success: Predicate{Op: "tlm", Path: "channels.MAG", Cmp: "eq", Value: 0.0}},
		Objective{ID: "present", Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "NOMINAL"}},
	)

	if err := e.Evaluate(absentTelemetryFrame(true)); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	want := map[string]ObjectiveStatus{
		"absent":  ObjectiveStatusActive,
		"missing": ObjectiveStatusActive,
		"present": ObjectiveStatusComplete,
	}
	for id, wantStatus := range want {
		if got := status(t, e, id); got != wantStatus {
			t.Errorf("objective %q = %s, want %s", id, got, wantStatus)
		}
	}
}

func TestFailedCRCFrameDoesNotAbortPass(t *testing.T) {
	e := frameEvaluation(t,
		Objective{ID: "gone", Success: Predicate{Op: "channel_absent", ID: "MAG"}},
		Objective{ID: "bits", Success: Predicate{Op: "tlm_bits", Path: "channels.COMMS.xstat", Mask: 4, Cmp: "eq", Value: 4.0}},
	)

	if err := e.Evaluate(absentTelemetryFrame(false)); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for _, id := range []string{"gone", "bits"} {
		if got := status(t, e, id); got != ObjectiveStatusActive {
			t.Errorf("objective %q = %s after a failed-CRC frame, want active", id, got)
		}
	}

	if err := e.Evaluate(absentTelemetryFrame(true)); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	for _, id := range []string{"gone", "bits"} {
		if got := status(t, e, id); got != ObjectiveStatusComplete {
			t.Errorf("objective %q = %s after a good frame, want complete", id, got)
		}
	}
}
