package game

import (
	"fmt"
	"slices"
	"sojourn/emulator"
	"strconv"
	"strings"
)

const (
	pendingCommandLimit = 64
	bootBanner          = "SOJOURN FSW"
)

type Grader struct {
	state      *State
	gdbAddr    string
	introspect *emulator.Introspect
	log        []CommandLogEntry
}

func NewGrader(state *State, gdbAddr string) *Grader {
	return &Grader{state: state, gdbAddr: gdbAddr}
}

func (g *Grader) LogCommand(command CommandLogEntry) {
	g.log = append(g.log, command)
}

func (g *Grader) Grade(frame *emulator.TelemetryFrame) error {
	if g.state.Phase() != PhaseInProgress {
		return fmt.Errorf("simulation not in progress")
	}

	ranges, err := g.state.MemoryRanges()
	if err != nil {
		return err
	}

	f := Frame{
		CRCOK:     frame.CRCOK,
		Telemetry: frame.AsMap(),
		Log:       slices.Clone(g.log),
		// TODO: fill in
		// Events
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

func parseCommand(raw string) CommandLogEntry {
	command := CommandLogEntry{Raw: raw}

	fields := strings.Fields(raw)
	if len(fields) > 0 {
		command.Verb = fields[0]
	}

	if len(fields) > 1 {
		hex := strings.TrimPrefix(strings.TrimPrefix(fields[1], "0x"), "0X")
		if addr, err := strconv.ParseUint(hex, 16, 32); err == nil {
			command.Addr = uint32(addr)
		}
	}

	return command
}

func isCommandResponse(line string) bool {
	return strings.HasPrefix(line, "ACK ") || strings.HasPrefix(line, "NAK ")
}
