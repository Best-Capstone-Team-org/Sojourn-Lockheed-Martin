package game

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sojourn/emulator"
)

type Frame struct {
	CRCOK     bool
	Telemetry map[string]any
	Events    []string
	Log       []CommandLogEntry
	Budget    map[string]int
	Memory    MemoryReader
}

type Evaluation struct {
	scenario        *Scenario
	objectiveStates map[string]*ObjectiveState
	memoryRanges    []emulator.MemoryRange
	historyDepth    int
	framesSeen      int
	baseline        MemoryReader
	history         []Frame
	scriptRunner    ScriptRunner
}

func NewEvaluation(s *Scenario) (Evaluation, error) {
	ranges, err := s.collectMemoryRanges()
	if err != nil {
		return Evaluation{}, err
	}

	return Evaluation{
		scenario:        s,
		objectiveStates: initializeObjectiveStates(s.Objectives),
		memoryRanges:    ranges,
		historyDepth:    s.requiredHistoryDepth(),
	}, nil
}

func (s *Scenario) collectMemoryRanges() ([]emulator.MemoryRange, error) {
	var ranges []emulator.MemoryRange

	var walk func(Predicate) error
	walk = func(p Predicate) error {
		length := 0
		switch p.Op {
		case "mem_u8":
			length = 1
		case "mem_u16":
			length = 2
		case "mem_u32":
			length = 4
		case "mem_bits":
			length = p.Width
			if length == 0 {
				length = 4
			}
		case "mem", "mem_changed":
			length = p.Len
		}

		if length > 0 && p.At != nil {
			addr, err := s.ResolveAddress(*p.At)
			if err != nil {
				return err
			}
			ranges = append(ranges, emulator.MemoryRange{Addr: uint32(addr), Len: length})
		}

		children, err := predicateChildren(p)
		if err != nil {
			return err
		}
		for _, child := range children {
			if err := walk(child); err != nil {
				return err
			}
		}

		return nil
	}

	for _, objective := range s.Objectives {
		for _, p := range objectivePredicates(objective) {
			if err := walk(p); err != nil {
				return nil, fmt.Errorf("objective %q: %w", objective.ID, err)
			}
		}
	}

	return mergeRanges(ranges), nil
}

func mergeRanges(ranges []emulator.MemoryRange) []emulator.MemoryRange {
	if len(ranges) == 0 {
		return nil
	}

	slices.SortFunc(ranges, func(a, b emulator.MemoryRange) int { return cmp.Compare(a.Addr, b.Addr) })

	merged := []emulator.MemoryRange{ranges[0]}
	for _, r := range ranges[1:] {
		last := &merged[len(merged)-1]
		if uint64(r.Addr) <= last.End() {
			if r.End() > last.End() {
				last.Len = int(r.End() - uint64(last.Addr))
			}
			continue
		}
		merged = append(merged, r)
	}

	return merged
}

func (s *Scenario) requiredHistoryDepth() int {
	var depth func(Predicate) int
	depth = func(p Predicate) int {
		children, err := predicateChildren(p)
		if err != nil {
			return -1
		}

		deepest := 0
		for _, child := range children {
			d := depth(child)
			if d < 0 {
				return -1
			}
			deepest = max(deepest, d)
		}

		switch p.Op {
		case "ever":
			return -1
		case "sustained", "within":
			return deepest + p.Frames
		default:
			return deepest
		}
	}

	deepest := 0
	for _, objective := range s.Objectives {
		for _, p := range objectivePredicates(objective) {
			d := depth(p)
			if d < 0 {
				return -1
			}
			deepest = max(deepest, d)
		}
	}

	return deepest
}

func objectivePredicates(objective Objective) []Predicate {
	predicates := []Predicate{objective.Success}
	if objective.Fail != nil {
		predicates = append(predicates, *objective.Fail)
	}
	for _, partial := range objective.Partial {
		predicates = append(predicates, partial.When)
	}
	return predicates
}

func predicateChildren(p Predicate) ([]Predicate, error) {
	switch p.Op {
	case "all", "any":
		var children []Predicate
		if err := json.Unmarshal(p.Of, &children); err != nil {
			return nil, err
		}
		return children, nil
	case "not", "ever", "sustained", "within":
		var child Predicate
		if err := json.Unmarshal(p.Of, &child); err != nil {
			return nil, err
		}
		return []Predicate{child}, nil
	default:
		return nil, nil
	}
}

func (e *Evaluation) MemoryRanges() []emulator.MemoryRange {
	return slices.Clone(e.memoryRanges)
}

func (e *Evaluation) Evaluate(frame Frame) error {
	if len(e.memoryRanges) > 0 && frame.Memory == nil {
		return errors.New("scenario reads memory but the frame carries none")
	}

	e.framesSeen++

	if e.scenario.Setup != nil && e.framesSeen <= e.scenario.Setup.SettleFrames {
		e.baseline = frame.Memory
		return nil
	}

	if e.baseline == nil {
		e.baseline = frame.Memory
	}

	if msg := e.evaluateObjectives(frame); msg != "" {
		return errors.New(msg)
	}

	e.history = append(e.history, frame)
	if e.historyDepth >= 0 && len(e.history) > e.historyDepth {
		e.history = slices.Clone(e.history[len(e.history)-e.historyDepth:])
	}

	return nil
}

func (e *Evaluation) reset() {
	e.objectiveStates = initializeObjectiveStates(e.scenario.Objectives)
	e.framesSeen = 0
	e.baseline = nil
	e.history = nil
}

func (e *Evaluation) past(index int) *Evaluation {
	past := *e
	past.history = e.history[:index]
	return &past
}
