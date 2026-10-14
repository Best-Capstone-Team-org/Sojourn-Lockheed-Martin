package emulator

import (
	"bytes"
	"errors"
	"fmt"
)

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
	if err := in.Halt(); err != nil {
		return nil, fmt.Errorf("halting: %w", err)
	}

	defer func() {
		if resumeErr := in.Resume(); resumeErr != nil {
			snap = nil
			err = errors.Join(err, fmt.Errorf("resuming: %w", resumeErr))
		}
	}()

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
