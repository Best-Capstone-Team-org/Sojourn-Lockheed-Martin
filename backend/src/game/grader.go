package game

import (
	"fmt"
	"sojourn/emulator"
)

type Grader struct {
	state      *State
	gdbAddr    string
	introspect *emulator.Introspect
}

func NewGrader(state *State, gdbAddr string) *Grader {
	return &Grader{state: state, gdbAddr: gdbAddr}
}

func (g *Grader) Grade(frame *emulator.TelemetryFrame) error {
	if !frame.CRCOK {
		return nil
	}

	if g.state.Phase() == PhaseReady {
		if err := g.state.Start(); err != nil {
			return fmt.Errorf("starting scenario: %w", err)
		}
	}

	if g.state.Phase() != PhaseInProgress {
		return nil
	}

	ranges, err := g.state.MemoryRanges()
	if err != nil {
		return err
	}

	f := Frame{
		Telemetry: frame.AsMap(),
		// TODO: fill in
		// Events
		// Log
		// Budget
	}

	if len(ranges) > 0 {
		mem, err := g.capture(ranges)
		if err != nil {
			return fmt.Errorf("capturing memory for frame %d: %w", frame.Frame, err)
		}
		f.Memory = mem
	}

	if err := g.state.Evaluate(f); err != nil {
		return fmt.Errorf("evaluating frame %d: %w", frame.Frame, err)
	}

	return nil
}

func (g *Grader) Close() error {
	if g.introspect == nil {
		return nil
	}

	err := g.introspect.Close()
	g.introspect = nil
	return err
}

func (g *Grader) capture(ranges []emulator.MemoryRange) (*emulator.MemorySnapshot, error) {
	if g.introspect == nil {
		in, err := emulator.Connect(g.gdbAddr)
		if err != nil {
			return nil, fmt.Errorf("connecting to %s: %w", g.gdbAddr, err)
		}
		g.introspect = in
	}

	mem, err := emulator.NewMemorySnapshot(g.introspect, ranges)
	if err != nil {
		g.Close()
		return nil, err
	}

	return mem, nil
}
