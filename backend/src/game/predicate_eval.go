package game

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func (e *Evaluation) evaluatePredicate(frame Frame, predicate Predicate) (bool, string) {
	switch predicate.Op {
	case "tlm":
		actual, found := getTelemetryValue(frame, predicate.Path)
		if !found {
			return false, ""
		}

		return compareValues(actual, predicate.Value, predicate.Cmp), ""
	case "tlm_bits":
		actual, found := getTelemetryValue(frame, predicate.Path)
		if !found {
			return false, ""
		}

		masked, errMsg := applyMask(actual, predicate.Mask)
		if errMsg != "" {
			return false, errMsg
		}

		return compareValues(masked, predicate.Value, predicate.Cmp), ""
	case "channel_present":
		return telemetryChannelPresent(frame, predicate.ID), ""
	case "channel_absent":
		return !telemetryChannelPresent(frame, predicate.ID), ""
	case "all":
		var predicates []Predicate
		if err := json.Unmarshal(predicate.Of, &predicates); err != nil {
			return false, fmt.Sprintf("Failed to parse 'of' field for 'all' operation: %v", err)
		}

		for _, child := range predicates {
			result, err := e.evaluatePredicate(frame, child)
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
			result, err := e.evaluatePredicate(frame, child)
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

		result, err := e.evaluatePredicate(frame, child)
		if err != "" {
			return false, err
		}

		return !result, ""
	case "mem_u8":
		if frame.Memory == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_u8 operation"
		}

		addr, err := e.scenario.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := frame.Memory.Read(uint32(addr), 1)
		if err != nil {
			return false, fmt.Sprintf("Failed to read memory: %v", err)
		}

		actual, errMsg := introspectionBytesToValues(data, 1)
		if errMsg != "" {
			return false, errMsg
		}

		return compareValues(actual, predicate.Value, predicate.Cmp), ""
	case "mem_u16":
		if frame.Memory == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_u16 operation"
		}

		addr, err := e.scenario.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := frame.Memory.Read(uint32(addr), 2)
		if err != nil {
			return false, fmt.Sprintf("Failed to read memory: %v", err)
		}

		actual, errMsg := introspectionBytesToValues(data, 2)
		if errMsg != "" {
			return false, errMsg
		}

		return compareValues(actual, predicate.Value, predicate.Cmp), ""
	case "mem_u32":
		if frame.Memory == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_u32 operation"
		}

		addr, err := e.scenario.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := frame.Memory.Read(uint32(addr), 4)
		if err != nil {
			return false, fmt.Sprintf("Failed to read memory: %v", err)
		}

		actual, errMsg := introspectionBytesToValues(data, 4)
		if errMsg != "" {
			return false, errMsg
		}

		return compareValues(actual, predicate.Value, predicate.Cmp), ""
	case "mem_bits":
		if frame.Memory == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_bits operation"
		}

		width := predicate.Width
		if width == 0 {
			width = 4
		}

		addr, err := e.scenario.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := frame.Memory.Read(uint32(addr), width)
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
		if frame.Memory == nil {
			return false, "Introspect not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem operation"
		}

		addr, err := e.scenario.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		data, err := frame.Memory.Read(uint32(addr), predicate.Len)
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
		if frame.Memory == nil {
			return false, "Introspect not available"
		}

		if e.baseline == nil {
			return false, "Baseline memory not available"
		}

		if predicate.At == nil {
			return false, "Predicate missing 'at' field for mem_changed operation"
		}

		addr, err := e.scenario.ResolveAddress(*predicate.At)
		if err != nil {
			return false, fmt.Sprintf("Failed to resolve address: %v", err)
		}

		current, err := frame.Memory.Read(uint32(addr), predicate.Len)
		if err != nil {
			return false, fmt.Sprintf("Failed to read current memory: %v", err)
		}

		baseline, err := e.baseline.Read(uint32(addr), predicate.Len)
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

			for _, event := range frame.Events {
				if re.MatchString(event) {
					return true, ""
				}
			}

			return false, ""
		}

		for _, event := range frame.Events {
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

		for i, past := range e.history {
			result, msg := e.past(i).evaluatePredicate(past, child)
			if msg != "" {
				return false, msg
			}

			if result {
				return true, ""
			}
		}

		return e.evaluatePredicate(frame, child)
	case "sustained":
		var child Predicate
		if err := json.Unmarshal(predicate.Of, &child); err != nil {
			return false, fmt.Sprintf("Failed to parse 'of' field for 'sustained' operation: %v", err)
		}

		if predicate.Frames <= 0 {
			return false, "sustained operation requires frames greater than 0"
		}

		if len(e.history)+1 < predicate.Frames {
			return false, ""
		}

		start := len(e.history) - (predicate.Frames - 1)
		for i := start; i < len(e.history); i++ {
			result, msg := e.past(i).evaluatePredicate(e.history[i], child)
			if msg != "" {
				return false, msg
			}

			if !result {
				return false, ""
			}
		}

		return e.evaluatePredicate(frame, child)
	case "within":
		var child Predicate
		if err := json.Unmarshal(predicate.Of, &child); err != nil {
			return false, fmt.Sprintf("Failed to parse 'of' field for 'within' operation: %v", err)
		}

		if predicate.Frames <= 0 {
			return false, "within operation requires frames greater than 0"
		}

		result, msg := e.evaluatePredicate(frame, child)
		if msg != "" || result {
			return result, msg
		}

		start := max(len(e.history)-(predicate.Frames-1), 0)

		for i := start; i < len(e.history); i++ {
			result, msg := e.past(i).evaluatePredicate(e.history[i], child)
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
			addr, err := e.scenario.ResolveAddress(*predicate.At)
			if err != nil {
				return false, fmt.Sprintf("Failed to resolve commanded address: %v", err)
			}

			expectedAddr = uint32(addr)
		}

		for _, entry := range frame.Log {
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
		used, found := frame.Budget[predicate.Resource]
		if !found {
			return false, ""
		}

		return compareValues(used, predicate.Value, predicate.Cmp), ""
	case "script":
		if e.scriptRunner == nil {
			return false, "Script runner not available"
		}

		result, err := e.scriptRunner.EvaluateScript(predicate.Lang, predicate.Entry, frame)
		if err != nil {
			return false, fmt.Sprintf("Script predicate failed: %v", err)
		}

		return result, ""
	default:
		return false, fmt.Sprintf("Unknown predicate operation %q", predicate.Op)
	}
}

// TODO: OBJECTIVE EVALUATION
func (e *Evaluation) evaluateObjective(frame Frame, objective Objective) string {
	state, ok := e.objectiveStates[objective.ID]
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
		failed, msg := e.evaluatePredicate(frame, *objective.Fail)
		if msg != "" {
			return fmt.Sprintf("Error evaluating fail predicate for objective %q: %s", objective.ID, msg)
		}
		if failed {
			state.Status = ObjectiveStatusFailed
			state.Diagnostic = ""
			return ""
		}
	}

	diagnostic, msg := e.evaluatePartial(frame, objective)
	if msg != "" {
		return fmt.Sprintf("Error evaluating partial predicate for objective %q: %s", objective.ID, msg)
	}
	state.Diagnostic = diagnostic

	success, msg := e.evaluatePredicate(frame, objective.Success)
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

func (e *Evaluation) evaluateObjectives(frame Frame) string {
	originalStates := e.objectiveStates
	e.objectiveStates = copyObjectiveStates(originalStates)

	for _, objective := range e.scenario.Objectives {
		state := e.objectiveStates[objective.ID]
		// Declaration order is meaningful: a later objective may unlock after an earlier
		// requirement completes during this same evaluation pass.
		if state != nil && state.Status == ObjectiveStatusLocked && e.requirementsComplete(objective) {
			state.Status = ObjectiveStatusActive
		}

		if msg := e.evaluateObjective(frame, objective); msg != "" {
			e.objectiveStates = originalStates
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

func (e *Evaluation) evaluatePartial(frame Frame, objective Objective) (string, string) {
	for _, partial := range objective.Partial {
		result, errMsg := e.evaluatePredicate(frame, partial.When)
		if errMsg != "" {
			return "", errMsg
		}

		if result {
			return partial.Text, ""
		}
	}

	return "", ""
}
