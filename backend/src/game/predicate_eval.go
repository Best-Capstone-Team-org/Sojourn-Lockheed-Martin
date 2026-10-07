package game

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func (s *Scenario) evaluatePredicate(ctx *EvaluationContext, predicate Predicate) (bool, string) {
	switch predicate.Op {
	case "tlm":
		actual, found := getTelemetryValue(ctx, predicate.Path)
		if !found {
			return false, ""
		}

		return compareValues(actual, predicate.Value, predicate.Cmp), ""
	case "tlm_bits":
		actual, found := getTelemetryValue(ctx, predicate.Path)
		if !found {
			return false, ""
		}

		masked, errMsg := applyMask(actual, predicate.Mask)
		if errMsg != "" {
			return false, errMsg
		}

		return compareValues(masked, predicate.Value, predicate.Cmp), ""
	case "channel_present":
		return telemetryChannelPresent(ctx, predicate.ID), ""
	case "channel_absent":
		return !telemetryChannelPresent(ctx, predicate.ID), ""
	case "all":
		var predicates []Predicate
		if err := json.Unmarshal(predicate.Of, &predicates); err != nil {
			return false, fmt.Sprintf("Failed to parse 'of' field for 'all' operation: %v", err)
		}

		for _, child := range predicates {
			result, err := s.evaluatePredicate(ctx, child)
			if err != "" {
				return false, err
			}

			if !result {
				return false, ""
			}
		}

		return true, ""
	case "any":
		var predicates []Predicate
		if err := json.Unmarshal(predicate.Of, &predicates); err != nil {
			return false, fmt.Sprintf("Failed to parse 'of' field for 'any' operation: %v", err)
		}

		for _, child := range predicates {
			result, err := s.evaluatePredicate(ctx, child)
			if err != "" {
				return false, err
			}

			if result {
				return true, ""
			}
		}

		return false, ""
	case "not":
		var child Predicate
		if err := json.Unmarshal(predicate.Of, &child); err != nil {
			return false, fmt.Sprintf("Failed to parse 'of' field for 'not' operation: %v", err)
		}

		result, err := s.evaluatePredicate(ctx, child)
		if err != "" {
			return false, err
		}

		return !result, ""
	case "mem_u8":
		if ctx.Introspect == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_u8 operation"
		}

		addr, err := s.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := ctx.Introspect.Read(uint32(addr), 1)
		if err != nil {
			return false, fmt.Sprintf("Failed to read memory: %v", err)
		}

		actual, errMsg := introspectionBytesToValues(data, 1)
		if errMsg != "" {
			return false, errMsg
		}

		return compareValues(actual, predicate.Value, predicate.Cmp), ""
	case "mem_u16":
		if ctx.Introspect == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_u16 operation"
		}

		addr, err := s.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := ctx.Introspect.Read(uint32(addr), 2)
		if err != nil {
			return false, fmt.Sprintf("Failed to read memory: %v", err)
		}

		actual, errMsg := introspectionBytesToValues(data, 2)
		if errMsg != "" {
			return false, errMsg
		}

		return compareValues(actual, predicate.Value, predicate.Cmp), ""
	case "mem_u32":
		if ctx.Introspect == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_u32 operation"
		}

		addr, err := s.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := ctx.Introspect.Read(uint32(addr), 4)
		if err != nil {
			return false, fmt.Sprintf("Failed to read memory: %v", err)
		}

		actual, errMsg := introspectionBytesToValues(data, 4)
		if errMsg != "" {
			return false, errMsg
		}

		return compareValues(actual, predicate.Value, predicate.Cmp), ""
	case "mem_bits":
		if ctx.Introspect == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_bits operation"
		}

		width := predicate.Width
		if width == 0 {
			width = 4
		}

		addr, err := s.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := ctx.Introspect.Read(uint32(addr), width)
		if err != nil {
			return false, fmt.Sprintf("Failed to read memory: %v", err)
		}

		actual, errMsg := introspectionBytesToValues(data, width)
		if errMsg != "" {
			return false, errMsg
		}

		actual, errMsg = applyMask(actual, predicate.Mask)
		if errMsg != "" {
			return false, errMsg
		}

		return compareValues(actual, predicate.Value, predicate.Cmp), ""
	case "mem":
		if ctx.Introspect == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem operation"
		}

		addr, err := s.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := ctx.Introspect.Read(uint32(addr), predicate.Len)
		if err != nil {
			return false, fmt.Sprintf("Failed to read memory: %v", err)
		}

		if len(data) != predicate.Len {
			return false, fmt.Sprintf("Short memory read: got %d bytes, expected %d", len(data), predicate.Len)
		}

		expectedHex, ok := predicate.Value.(string)
		if !ok {
			return false, "mem predicate value is not a hex string"
		}

		expected, err := hex.DecodeString(expectedHex)
		if err != nil {
			return false, fmt.Sprintf("Invalid hex value %q: %v", expectedHex, err)
		}

		equal := bytes.Equal(data, expected)
		if predicate.Cmp == "ne" {
			return !equal, ""
		}

		return equal, ""
	case "mem_changed":
		if ctx.Introspect == nil {
			return false, "Introspect not available"
		}

		if ctx.Baseline == nil {
			return false, "Baseline memory not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_changed operation"
		}

		addr, err := s.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		current, err := ctx.Introspect.Read(uint32(addr), predicate.Len)
		if err != nil {
			return false, fmt.Sprintf("Failed to read current memory: %v", err)
		}

		baseline, err := ctx.Baseline.Read(uint32(addr), predicate.Len)
		if err != nil {
			return false, fmt.Sprintf("Failed to read baseline memory: %v", err)
		}

		if len(current) != predicate.Len {
			return false, fmt.Sprintf("Short current memory read: got %d bytes, expected %d", len(current), predicate.Len)
		}

		if len(baseline) != predicate.Len {
			return false, fmt.Sprintf("Short baseline memory read: got %d bytes, expected %d", len(baseline), predicate.Len)
		}

		for i := range current {
			if current[i] != baseline[i] {
				return true, ""
			}
		}

		return false, ""
	case "event":
		if predicate.Regex {
			re, err := regexp.Compile(predicate.Match)
			if err != nil {
				return false, fmt.Sprintf("Invalid event regex %q: %v", predicate.Match, err)
			}

			for _, event := range ctx.Events {
				if re.MatchString(event) {
					return true, ""
				}
			}

			return false, ""
		}

		for _, event := range ctx.Events {
			if strings.Contains(event, predicate.Match) {
				return true, ""
			}
		}

		return false, ""
	case "ever":
		var child Predicate
		if err := json.Unmarshal(predicate.Of, &child); err != nil {
			return false, fmt.Sprintf("Failed to parse 'of' field for 'ever' operation: %v", err)
		}

		for i := 0; i < temporalHistoryLen(ctx); i++ {
			h, msg := historicalContext(ctx, i, child)
			if msg != "" {
				return false, msg
			}

			result, msg := s.evaluatePredicate(h, child)
			if msg != "" {
				return false, msg
			}

			if result {
				return true, ""
			}
		}

		return s.evaluatePredicate(ctx, child)
	case "sustained":
		var child Predicate
		if err := json.Unmarshal(predicate.Of, &child); err != nil {
			return false, fmt.Sprintf("Failed to parse 'of' field for 'sustained' operation: %v", err)
		}

		if predicate.Frames <= 0 {
			return false, "sustained operation requires frames greater than 0"
		}

		if temporalHistoryLen(ctx)+1 < predicate.Frames {
			return false, ""
		}

		start := temporalHistoryLen(ctx) - (predicate.Frames - 1)
		for i := start; i < temporalHistoryLen(ctx); i++ {
			h, msg := historicalContext(ctx, i, child)
			if msg != "" {
				return false, msg
			}

			result, msg := s.evaluatePredicate(h, child)
			if msg != "" {
				return false, msg
			}

			if !result {
				return false, ""
			}
		}

		return s.evaluatePredicate(ctx, child)
	case "within":
		var child Predicate
		if err := json.Unmarshal(predicate.Of, &child); err != nil {
			return false, fmt.Sprintf("Failed to parse 'of' field for 'within' operation: %v", err)
		}

		if predicate.Frames <= 0 {
			return false, "within operation requires frames greater than 0"
		}

		result, msg := s.evaluatePredicate(ctx, child)
		if msg != "" || result {
			return result, msg
		}

		start := temporalHistoryLen(ctx) - (predicate.Frames - 1)
		if start < 0 {
			start = 0
		}

		for i := start; i < temporalHistoryLen(ctx); i++ {
			h, msg := historicalContext(ctx, i, child)
			if msg != "" {
				return false, msg
			}

			result, msg := s.evaluatePredicate(h, child)
			if msg != "" {
				return false, msg
			}

			if result {
				return true, ""
			}
		}

		return false, ""
	case "commanded":
		var expectedAddr uint32
		if predicate.At != nil {
			addr, err := s.ResolveAddress(*predicate.At)
			if err != nil {
				return false, fmt.Sprintf("Failed to resolve commanded address: %v", err)
			}

			expectedAddr = uint32(addr)
		}

		for _, entry := range ctx.Log {
			if predicate.Verb != "" && entry.Verb != predicate.Verb {
				continue
			}

			if predicate.At != nil && entry.Addr != expectedAddr {
				continue
			}

			if predicate.Result != "" && !strings.HasPrefix(entry.Result, predicate.Result) {
				continue
			}

			return true, ""
		}

		return false, ""
	case "budget":
		used, found := ctx.Budget[predicate.Resource]
		if !found {
			return false, ""
		}

		return compareValues(used, predicate.Value, predicate.Cmp), ""
	case "script":
		if ctx.ScriptRunner == nil {
			return false, "Script runner not available"
		}

		result, err := ctx.ScriptRunner.EvaluateScript(predicate.Lang, predicate.Entry, ctx)
		if err != nil {
			return false, fmt.Sprintf("Script predicate failed: %v", err)
		}

		return result, ""
	default:
		return false, fmt.Sprintf("Unknown predicate operation %q", predicate.Op)
	}
}

// TODO: OBJECTIVE EVALUATION
func (s *Scenario) evaluateObjective(ctx *EvaluationContext, objective Objective) string {
	state, ok := s.ObjectiveStates[objective.ID]
	if !ok {
		return fmt.Sprintf("Objective state for %q not found", objective.ID)
	}

	if state.Status == ObjectiveStatusFailed {
		return ""
	}

	if state.Status == ObjectiveStatusComplete && !objective.Retractable {
		return ""
	}

	if state.Status == ObjectiveStatusLocked {
		return ""
	}

	// Retractable completed objectives re-enter the normal fail -> partial -> success order.
	if state.Status == ObjectiveStatusComplete && objective.Retractable {
		state.Status = ObjectiveStatusActive
	}

	if objective.Fail != nil {
		failed, msg := s.evaluatePredicate(ctx, *objective.Fail)
		if msg != "" {
			return fmt.Sprintf("Error evaluating fail predicate for objective %q: %s", objective.ID, msg)
		}
		if failed {
			state.Status = ObjectiveStatusFailed
			state.Diagnostic = ""
			return ""
		}
	}

	diagnostic, msg := s.evaluatePartial(ctx, objective)
	if msg != "" {
		return fmt.Sprintf("Error evaluating partial predicate for objective %q: %s", objective.ID, msg)
	}
	state.Diagnostic = diagnostic

	success, msg := s.evaluatePredicate(ctx, objective.Success)
	if msg != "" {
		return fmt.Sprintf("Error evaluating success predicate for objective %q: %s", objective.ID, msg)
	}

	if success {
		state.Status = ObjectiveStatusComplete
		state.Diagnostic = ""
	} else {
		state.Status = ObjectiveStatusActive
	}

	return ""
}

func (s *Scenario) evaluateObjectives(ctx *EvaluationContext) string {
	originalStates := s.ObjectiveStates
	s.ObjectiveStates = copyObjectiveStates(originalStates)

	for _, objective := range s.Objectives {
		state := s.ObjectiveStates[objective.ID]
		// Declaration order is meaningful: a later objective may unlock after an earlier
		// requirement completes during this same evaluation pass.
		if state != nil && state.Status == ObjectiveStatusLocked && s.requirementsComplete(objective) {
			state.Status = ObjectiveStatusActive
		}

		if msg := s.evaluateObjective(ctx, objective); msg != "" {
			s.ObjectiveStates = originalStates
			return msg
		}
	}

	return ""
}

func copyObjectiveStates(states map[string]*ObjectiveState) map[string]*ObjectiveState {
	copied := make(map[string]*ObjectiveState, len(states))

	for id, state := range states {
		if state == nil {
			copied[id] = nil
			continue
		}

		stateCopy := *state
		copied[id] = &stateCopy
	}

	return copied
}

func (s *Scenario) evaluatePartial(ctx *EvaluationContext, objective Objective) (string, string) {
	for _, partial := range objective.Partial {
		result, errMsg := s.evaluatePredicate(ctx, partial.When)
		if errMsg != "" {
			return "", errMsg
		}

		if result {
			return partial.Text, ""
		}
	}

	return "", ""
}
