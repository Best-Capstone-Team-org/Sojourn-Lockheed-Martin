package game

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

func validateComparison(cmp string) error {
	switch cmp {
	case "eq", "ne", "lt", "lte", "gt", "gte", "in":
		return nil
	default:
		return fmt.Errorf("unknown comparison operator %q", cmp)
	}
}

func isValidPredicateOp(op string) bool {
	switch op {
	case "tlm", "tlm_bits", "channel_present", "channel_absent", "event", "mem_u8", "mem_u16", "mem_u32", "mem_bits", "mem", "mem_changed", "commanded", "budget", "all", "any", "not", "ever", "sustained", "within", "script":
		return true
	default:
		return false
	}
}

func isJSONNumber(v any) bool { _, ok := numericValue(v); return ok }
func validateComparisonValue(cmp string, value any, numericOnly bool) error {
	if err := validateComparison(cmp); err != nil {
		return err
	}
	if value == nil {
		return fmt.Errorf("comparison requires 'value'")
	}
	if cmp == "in" {
		values, ok := value.([]any)
		if !ok {
			return fmt.Errorf("comparison operator 'in' requires an array value")
		}
		if numericOnly {
			for _, v := range values {
				if !isJSONNumber(v) {
					return fmt.Errorf("numeric comparison requires numeric values")
				}
			}
		}
		return nil
	}
	if _, ok := value.([]any); ok {
		return fmt.Errorf("comparison operator %q requires a scalar value", cmp)
	}
	if numericOnly && !isJSONNumber(value) {
		return fmt.Errorf("numeric comparison requires a numeric value")
	}
	return nil
}

func validatePredicate(predicate Predicate) error {
	if !isValidPredicateOp(predicate.Op) {
		return fmt.Errorf("unknown predicate operation %q", predicate.Op)
	}
	switch predicate.Op {
	case "all", "any":
		if len(predicate.Of) == 0 {
			return fmt.Errorf("%s predicate requires 'of'", predicate.Op)
		}
		var children []Predicate
		if err := json.Unmarshal(predicate.Of, &children); err != nil {
			return fmt.Errorf("invalid 'of' field for %q predicate: %w", predicate.Op, err)
		}
		for _, child := range children {
			if err := validatePredicate(child); err != nil {
				return err
			}
		}
	case "not", "ever", "sustained", "within":
		if len(predicate.Of) == 0 {
			return fmt.Errorf("%s predicate requires 'of'", predicate.Op)
		}
		if (predicate.Op == "sustained" || predicate.Op == "within") && predicate.Frames <= 0 {
			return fmt.Errorf("%s predicate requires 'frames' greater than 0", predicate.Op)
		}
		var child Predicate
		if err := json.Unmarshal(predicate.Of, &child); err != nil {
			return fmt.Errorf("invalid 'of' field for %q predicate: %w", predicate.Op, err)
		}
		if err := validatePredicate(child); err != nil {
			return err
		}
	case "tlm":
		if predicate.Path == "" {
			return fmt.Errorf("tlm predicate requires 'path'")
		}
		if err := validateComparisonValue(predicate.Cmp, predicate.Value, false); err != nil {
			return fmt.Errorf("tlm predicate: %w", err)
		}
	case "tlm_bits":
		if predicate.Path == "" {
			return fmt.Errorf("tlm_bits predicate requires 'path'")
		}
		if err := validateComparisonValue(predicate.Cmp, predicate.Value, true); err != nil {
			return fmt.Errorf("tlm_bits predicate: %w", err)
		}
	case "channel_present", "channel_absent":
		if predicate.ID == "" {
			return fmt.Errorf("%s predicate requires 'id'", predicate.Op)
		}
	case "event":
		if predicate.Match == "" {
			return fmt.Errorf("event predicate requires 'match'")
		}
		if predicate.Regex {
			if _, err := regexp.Compile(predicate.Match); err != nil {
				return fmt.Errorf("event predicate has invalid regex %q: %w", predicate.Match, err)
			}
		}
	case "mem_u8", "mem_u16", "mem_u32":
		if predicate.At == nil {
			return fmt.Errorf("%s predicate requires 'at'", predicate.Op)
		}
		if err := validateAddressRef(*predicate.At); err != nil {
			return err
		}
		if err := validateComparisonValue(predicate.Cmp, predicate.Value, true); err != nil {
			return fmt.Errorf("%s predicate: %w", predicate.Op, err)
		}
	case "mem_bits":
		if predicate.At == nil {
			return fmt.Errorf("mem_bits predicate requires 'at'")
		}
		if err := validateAddressRef(*predicate.At); err != nil {
			return err
		}
		if predicate.Width != 0 && predicate.Width != 1 && predicate.Width != 2 && predicate.Width != 4 {
			return fmt.Errorf("mem_bits predicate has unsupported width %d", predicate.Width)
		}
		if err := validateComparisonValue(predicate.Cmp, predicate.Value, true); err != nil {
			return fmt.Errorf("mem_bits predicate: %w", err)
		}
	case "mem":
		if predicate.At == nil {
			return fmt.Errorf("mem predicate requires 'at'")
		}
		if err := validateAddressRef(*predicate.At); err != nil {
			return err
		}
		if predicate.Len <= 0 {
			return fmt.Errorf("mem predicate requires 'len' greater than 0")
		}
		if predicate.Cmp != "eq" && predicate.Cmp != "ne" {
			return fmt.Errorf("mem predicate supports only 'eq' and 'ne' comparisons")
		}
		expectedHex, ok := predicate.Value.(string)
		if !ok || expectedHex == "" {
			return fmt.Errorf("mem predicate requires a hex string 'value'")
		}
		expected, err := hex.DecodeString(expectedHex)
		if err != nil {
			return fmt.Errorf("mem predicate has invalid hex value %q", expectedHex)
		}
		if len(expected) != predicate.Len {
			return fmt.Errorf("mem predicate length %d does not match value length %d", predicate.Len, len(expected))
		}
	case "mem_changed":
		if predicate.At == nil {
			return fmt.Errorf("mem_changed predicate requires 'at'")
		}
		if err := validateAddressRef(*predicate.At); err != nil {
			return err
		}
		if predicate.Len <= 0 {
			return fmt.Errorf("mem_changed predicate requires 'len' greater than 0")
		}
	case "commanded":
		if predicate.At != nil {
			if err := validateAddressRef(*predicate.At); err != nil {
				return err
			}
		}
	case "budget":
		if predicate.Resource == "" {
			return fmt.Errorf("budget predicate requires 'resource'")
		}
		if err := validateComparisonValue(predicate.Cmp, predicate.Value, true); err != nil {
			return fmt.Errorf("budget predicate: %w", err)
		}
	case "script":
		if predicate.Lang != "python" {
			return fmt.Errorf("script predicate requires lang 'python'")
		}
		if predicate.Entry == "" {
			return fmt.Errorf("script predicate requires 'entry'")
		}
		parts := strings.Split(predicate.Entry, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("script predicate entry must have form path.py:callable")
		}
		clean := filepath.Clean(parts[0])
		if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			return fmt.Errorf("script predicate entry must be a relative package path")
		}
		if clean != "checks" && !strings.HasPrefix(clean, "checks"+string(filepath.Separator)) {
			return fmt.Errorf("script predicate entry must point inside checks/")
		}
	}
	return nil
}

type MemoryReader interface {
	Read(addr uint32, length int) ([]byte, error)
}

type EvaluationSnapshot struct {
	Telemetry  map[string]any
	Events     []string
	Log        []CommandLogEntry
	Budget     map[string]int
	Introspect MemoryReader
	Baseline   MemoryReader
}

type ScriptRunner interface {
	EvaluateScript(lang, entry string, ctx *EvaluationContext) (bool, error)
}

type EvaluationContext struct {
	Telemetry  map[string]any
	Introspect MemoryReader
	Baseline   MemoryReader
	Events     []string
	History    []map[string]any
	// HistoryContexts is the full per-frame history needed when temporal combinators
	// wrap predicates that depend on events, command log, budget, or memory.
	// If omitted, History remains valid for telemetry-only temporal predicates.
	HistoryContexts []EvaluationSnapshot
	Log             []CommandLogEntry
	Budget          map[string]int
	ScriptRunner    ScriptRunner
}

// normalize types due to json unmarshalling returning everything as float64
func numericValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	default:
		return 0, false
	}
}

func sameValueType(a, b any) bool {
	_, an := numericValue(a)
	_, bn := numericValue(b)
	if an || bn {
		return an && bn
	}
	return reflect.TypeOf(a) == reflect.TypeOf(b)
}

func compareValues(actual any, expected any, cmp string) bool {
	if cmp == "in" {
		values, ok := expected.([]any)
		if !ok {
			return false
		}
		for _, v := range values {
			if compareValues(actual, v, "eq") {
				return true
			}
		}
		return false
	}
	if !sameValueType(actual, expected) {
		return false
	}
	a, an := numericValue(actual)
	e, en := numericValue(expected)
	if an && en {
		switch cmp {
		case "eq":
			return a == e
		case "ne":
			return a != e
		case "lt":
			return a < e
		case "lte":
			return a <= e
		case "gt":
			return a > e
		case "gte":
			return a >= e
		default:
			return false
		}
	}
	switch cmp {
	case "eq":
		return reflect.DeepEqual(actual, expected)
	case "ne":
		return !reflect.DeepEqual(actual, expected)
	default:
		return false
	}
}

func getTelemetryValue(ctx *EvaluationContext, path string) (any, bool) {
	current := any(ctx.Telemetry)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func telemetryChannelPresent(ctx *EvaluationContext, id string) bool {
	channels, ok := ctx.Telemetry["channels"].(map[string]any)
	if !ok {
		return false
	}
	_, exists := channels[id]
	return exists
}

func (s *Scenario) getIntrospectionValue(ctx *EvaluationContext, predicate Predicate) ([]byte, string) {
	if ctx.Introspect == nil {
		return nil, "Introspect not available"
	}
	if predicate.At == nil {
		return nil, "Predicate missing 'at' field for introspect operation"
	}
	addr, err := s.ResolveAddress(*predicate.At)
	if err != nil {
		return nil, fmt.Sprintf("Failed to resolve address: %v", err)
	}
	data, err := ctx.Introspect.Read(uint32(addr), predicate.Len)
	if err != nil {
		return nil, fmt.Sprintf("Failed to read memory: %v", err)
	}
	return data, ""
}

func introspectionBytesToValues(data []byte, width int) (any, string) {
	if len(data) < width {
		return nil, fmt.Sprintf("Data length %d is less than expected width %d", len(data), width)
	}
	switch width {
	case 1:
		return uint8(data[0]), ""
	case 2:
		return uint16(data[0]) | uint16(data[1])<<8, ""
	case 4:
		return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24, ""
	default:
		return nil, fmt.Sprintf("Unsupported width %d for introspection", width)
	}
}

func applyMask(value any, mask int) (any, string) {
	if mask == 0 {
		return value, ""
	}
	switch v := value.(type) {
	case uint8:
		return v & uint8(mask), ""
	case uint16:
		return v & uint16(mask), ""
	case uint32:
		return v & uint32(mask), ""
	case uint64:
		return v & uint64(mask), ""
	default:
		return nil, fmt.Sprintf("Unsupported type %T for mask application", value)
	}
}

func predicateNeedsFullHistory(p Predicate) bool {
	switch p.Op {
	case "event", "commanded", "budget", "mem_u8", "mem_u16", "mem_u32", "mem_bits", "mem", "mem_changed", "script":
		return true
	case "all", "any":
		var children []Predicate
		if json.Unmarshal(p.Of, &children) != nil {
			return true
		}
		for _, c := range children {
			if predicateNeedsFullHistory(c) {
				return true
			}
		}
	case "not", "ever", "sustained", "within":
		var child Predicate
		if json.Unmarshal(p.Of, &child) != nil {
			return true
		}
		return predicateNeedsFullHistory(child)
	}
	return false
}

func temporalHistoryLen(ctx *EvaluationContext) int {
	if len(ctx.HistoryContexts) > 0 {
		return len(ctx.HistoryContexts)
	}
	return len(ctx.History)
}

func historicalContext(ctx *EvaluationContext, index int, child Predicate) (*EvaluationContext, string) {
	if len(ctx.HistoryContexts) > 0 {
		if index < 0 || index >= len(ctx.HistoryContexts) {
			return nil, "historical context index out of range"
		}
		snap := ctx.HistoryContexts[index]
		h := *ctx
		h.Telemetry = snap.Telemetry
		h.Events = snap.Events
		h.Log = snap.Log
		h.Budget = snap.Budget
		if snap.Introspect != nil {
			h.Introspect = snap.Introspect
		}
		if snap.Baseline != nil {
			h.Baseline = snap.Baseline
		}
		if index <= len(ctx.History) {
			h.History = ctx.History[:index]
		} else {
			h.History = nil
		}
		h.HistoryContexts = ctx.HistoryContexts[:index]
		return &h, ""
	}
	if predicateNeedsFullHistory(child) {
		return nil, "temporal predicate requires HistoryContexts for non-telemetry child"
	}
	if index < 0 || index >= len(ctx.History) {
		return nil, "historical telemetry index out of range"
	}
	h := *ctx
	h.Telemetry = ctx.History[index]
	h.History = ctx.History[:index]
	return &h, ""
}
