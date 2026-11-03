package game

import (
	"encoding/binary"
	"fmt"
	"slices"
	"sojourn/app"
	"sojourn/emulator"
	"strconv"
	"strings"
	"sync"
)

const (
	pendingCommandLimit = 64
	bootBanner          = "SOJOURN FSW"
)

// The probe's uptime is SYSBLK->ticks, a 100 Hz counter the bootloader keeps
// at a fixed address (firmware spec §10).
const (
	uptimeTicksAddr = 0x20000100
	msPerTick       = 10
)

type Grader struct {
	state   *State
	gdbAddr string
	log     []CommandLogEntry

	// mu guards introspect, which the uplink stamps commands through while
	// the downlink grades frames.
	mu         sync.Mutex
	introspect *emulator.Introspect
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
		UptimeS:   frame.UptimeS,
		Telemetry: frame.AsMap(),
		Log:       slices.Clone(g.log),
		Budget:    budgetUsed(g.log),
		// TODO: fill in
		// Events
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
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closeLocked()
}

func (g *Grader) closeLocked() error {
	if g.introspect == nil {
		return nil
	}

	err := g.introspect.Close()
	g.introspect = nil
	return err
}

// UptimeMS reads the probe's uptime in milliseconds, or nil if it can't be read.
func (g *Grader) UptimeMS() *uint64 {
	snapshot, err := g.capture([]emulator.MemoryRange{{Addr: uptimeTicksAddr, Len: 4}})
	if err != nil {
		app.ErrorLogger.Printf("Reading probe uptime: %v", err)
		return nil
	}

	data, err := snapshot.Read(uptimeTicksAddr, 4)
	if err != nil {
		app.ErrorLogger.Printf("Reading probe uptime: %v", err)
		return nil
	}

	ms := uint64(binary.LittleEndian.Uint32(data)) * msPerTick
	return &ms
}

func (g *Grader) capture(ranges []emulator.MemoryRange) (*emulator.MemorySnapshot, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.introspect == nil {
		in, err := emulator.Connect(g.gdbAddr)
		if err != nil {
			return nil, fmt.Errorf("connecting to %s: %w", g.gdbAddr, err)
		}
		g.introspect = in
	}

	mem, err := emulator.NewMemorySnapshot(g.introspect, ranges)
	if err != nil {
		g.closeLocked()
		return nil, err
	}

	return mem, nil
}

func parseCommand(raw string) CommandLogEntry {
	command := CommandLogEntry{Raw: raw}

	fields := strings.Fields(raw)
	if len(fields) > 0 {
		command.Verb = fields[0]
		command.Charged = chargedResource(command.Verb)
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
