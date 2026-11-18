package emulator

import (
	"bytes"
	"errors"
	"fmt"
	"time"
)

// SnapshotWindow bounds how long the guest may stay halted for one capture
// (charter R24.13). A full pass is a handful of packets.
const SnapshotWindow = 250 * time.Millisecond

// ErrSnapshotWindow reports a capture that would have held the guest halted
// longer than SnapshotWindow. The guest is resumed regardless.
var ErrSnapshotWindow = errors.New("snapshot window exceeded")

type MemoryRange struct {
	Addr uint32
	Len  int
}

func (r MemoryRange) End() uint64 { return uint64(r.Addr) + uint64(r.Len) }

type MemorySnapshot struct {
	regions []snapshotRegion
}

type snapshotRegion struct {
	addr uint32
	data []byte
}

func NewMemorySnapshot(in *Introspect, ranges []MemoryRange) (snap *MemorySnapshot, err error) {
	start := time.Now()
	in.deadline = start.Add(SnapshotWindow)

	defer func() {
		in.deadline = time.Time{}
		if elapsed := time.Since(start); elapsed > SnapshotWindow {
			snap = nil
			if err == nil {
				err = errors.New("reads finished too late")
			}
			err = fmt.Errorf("%w after %s: %w", ErrSnapshotWindow, elapsed.Round(time.Millisecond), err)
		}
	}()

	// However the capture ends, the guest must not be left halted.
	defer func() {
		if resumeErr := in.Resume(); resumeErr != nil {
			snap = nil
			err = errors.Join(err, fmt.Errorf("resuming: %w", resumeErr))
		}
	}()

	if err := in.Halt(); err != nil {
		return nil, fmt.Errorf("halting: %w", err)
	}

	snap = &MemorySnapshot{regions: make([]snapshotRegion, 0, len(ranges))}
	for _, r := range ranges {
		data, err := in.Read(r.Addr, r.Len)
		if err != nil {
			return nil, err
		}

		snap.regions = append(snap.regions, snapshotRegion{r.Addr, data})
	}

	return snap, nil
}

func (m *MemorySnapshot) Read(addr uint32, length int) ([]byte, error) {
	end := MemoryRange{addr, length}.End()

	for _, region := range m.regions {
		if addr >= region.addr && end <= (MemoryRange{region.addr, len(region.data)}).End() {
			start := addr - region.addr
			return bytes.Clone(region.data[start : int(start)+length]), nil
		}
	}

	return nil, fmt.Errorf("0x%X+%d was not captured", addr, length)
}
