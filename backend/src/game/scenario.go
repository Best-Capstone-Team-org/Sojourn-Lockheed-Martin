package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Scenarios have manifest.json, objectives.json, setup.json, symbols.json, memmap.json
type Scenario struct {
	Dir             string
	Manifest        Manifest
	Objectives      []Objective
	Setup           *Setup
	Symbols         SymbolsFile
	MemMap          MemMapFile
	ObjectiveStates map[string]*ObjectiveState
}

func validateObjectiveRequirements(objectives []Objective) error {
	ids := make(map[string]bool, len(objectives))
	for _, objective := range objectives {
		ids[objective.ID] = true
	}
	for _, objective := range objectives {
		for _, requiredID := range objective.Requires {
			if !ids[requiredID] {
				return fmt.Errorf("objective %q requires unknown objective %q", objective.ID, requiredID)
			}
		}
	}
	return nil
}

func validateObjectiveCycles(objectives []Objective) error {
	requires := make(map[string][]string, len(objectives))
	for _, objective := range objectives {
		requires[objective.ID] = objective.Requires
	}
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("objective dependency cycle detected at %q", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, requiredID := range requires[id] {
			if err := visit(requiredID); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for _, objective := range objectives {
		if err := visit(objective.ID); err != nil {
			return err
		}
	}
	return nil
}

func validateObjectiveIDs(objectives []Objective) error {
	seen := make(map[string]bool, len(objectives))
	for _, objective := range objectives {
		if strings.TrimSpace(objective.ID) == "" {
			return fmt.Errorf("objective is missing required 'id'")
		}
		if seen[objective.ID] {
			return fmt.Errorf("duplicate objective ID %q", objective.ID)
		}
		seen[objective.ID] = true
	}
	return nil
}

func validateObjectiveShape(objectives []Objective) error {
	for _, objective := range objectives {
		if strings.TrimSpace(objective.Title) == "" {
			return fmt.Errorf("objective %q is missing required 'title'", objective.ID)
		}
		if strings.TrimSpace(objective.Brief) == "" {
			return fmt.Errorf("objective %q is missing required 'brief'", objective.ID)
		}
		if strings.TrimSpace(objective.Success.Op) == "" {
			return fmt.Errorf("objective %q is missing required 'success' predicate", objective.ID)
		}
		for i, partial := range objective.Partial {
			if strings.TrimSpace(partial.When.Op) == "" {
				return fmt.Errorf("objective %q partial %d is missing required 'when' predicate", objective.ID, i)
			}
			if partial.Text == "" {
				return fmt.Errorf("objective %q partial %d is missing required 'text'", objective.ID, i)
			}
		}
	}
	return nil
}
func validateObjectivePredicates(objectives []Objective) error {
	for _, objective := range objectives {
		if err := validatePredicate(objective.Success); err != nil {
			return fmt.Errorf("invalid success predicate for objective %q: %w", objective.ID, err)
		}
		if objective.Fail != nil {
			if err := validatePredicate(*objective.Fail); err != nil {
				return fmt.Errorf("invalid fail predicate for objective %q: %w", objective.ID, err)
			}
		}
		for _, partial := range objective.Partial {
			if err := validatePredicate(partial.When); err != nil {
				return fmt.Errorf("invalid partial predicate for objective %q: %w", objective.ID, err)
			}
		}
	}
	return nil
}

func hasJSONKey(raw map[string]json.RawMessage, key string) bool { _, ok := raw[key]; return ok }
func validateManifestJSON(data []byte, manifest Manifest) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, key := range []string{"format", "id", "title", "revision", "summary", "firmware", "link", "briefing", "objectives"} {
		if !hasJSONKey(raw, key) {
			return fmt.Errorf("manifest missing required key %q", key)
		}
	}
	if strings.TrimSpace(manifest.ID) == "" || strings.TrimSpace(manifest.Title) == "" || strings.TrimSpace(manifest.Summary) == "" || strings.TrimSpace(manifest.Briefing) == "" || strings.TrimSpace(manifest.Objectives) == "" {
		return fmt.Errorf("manifest contains an empty required string field")
	}
	var fw map[string]json.RawMessage
	if err := json.Unmarshal(raw["firmware"], &fw); err != nil {
		return fmt.Errorf("manifest firmware must be an object: %w", err)
	}
	for _, key := range []string{"rom", "symbols", "memmap", "app_crc32"} {
		if !hasJSONKey(fw, key) {
			return fmt.Errorf("manifest firmware missing required key %q", key)
		}
	}
	if manifest.Firmware.ROM == "" || manifest.Firmware.Symbols == "" || manifest.Firmware.MemMap == "" || manifest.Firmware.AppCRC32 == "" {
		return fmt.Errorf("manifest firmware contains an empty required string field")
	}
	var link map[string]json.RawMessage
	if err := json.Unmarshal(raw["link"], &link); err != nil {
		return fmt.Errorf("manifest link must be an object: %w", err)
	}
	for _, key := range []string{"uplink_delay_s", "downlink_delay_s"} {
		if !hasJSONKey(link, key) {
			return fmt.Errorf("manifest link missing required key %q", key)
		}
	}
	return nil
}

func validateReferencedFiles(dir string, manifest Manifest) error {
	paths := []string{manifest.Briefing, manifest.Objectives, manifest.Firmware.ROM, manifest.Firmware.Symbols, manifest.Firmware.MemMap}
	if manifest.Setup != "" {
		paths = append(paths, manifest.Setup)
	}
	paths = append(paths, manifest.Docs...)
	for _, rel := range paths {
		if rel == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(dir, rel))
		if err != nil {
			return fmt.Errorf("referenced file %q is unavailable: %w", rel, err)
		}
		if info.IsDir() {
			return fmt.Errorf("referenced file %q is a directory", rel)
		}
	}
	return nil
}

func validatePurity(dir string, manifest Manifest, pureMode bool) error {
	info, err := os.Stat(filepath.Join(dir, "checks"))
	hasChecks := err == nil && info.IsDir()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect checks directory: %w", err)
	}
	if hasChecks != manifest.Impure {
		if hasChecks {
			return fmt.Errorf("checks/ is present but manifest does not declare impure=true")
		}
		return fmt.Errorf("manifest declares impure=true but checks/ is absent")
	}
	if pureMode && manifest.Impure {
		return fmt.Errorf("impure scenario refused in pure mode")
	}
	return nil
}

func LoadScenario(dir string) (*Scenario, error) { return LoadScenarioWithOptions(dir, true) }

// LoadScenarioWithOptions loads a package. pureMode=true is the normative packaged default.
func LoadScenarioWithOptions(dir string, pureMode bool) (*Scenario, error) {
	manifestPath := filepath.Join(dir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}
	if err := validateManifestJSON(data, manifest); err != nil {
		return nil, err
	}
	if manifest.Format != SupportedScenarioFormat {
		return nil, fmt.Errorf("unsupported manifest format: %d", manifest.Format)
	}
	if err := validatePurity(dir, manifest, pureMode); err != nil {
		return nil, err
	}
	if err := validateReferencedFiles(dir, manifest); err != nil {
		return nil, err
	}
	data, err = os.ReadFile(filepath.Join(dir, manifest.Objectives))
	if err != nil {
		return nil, fmt.Errorf("failed to read objectives: %w", err)
	}
	var objectivesFile ObjectivesFile
	if err := json.Unmarshal(data, &objectivesFile); err != nil {
		return nil, fmt.Errorf("failed to parse objectives: %w", err)
	}
	if objectivesFile.Format != SupportedScenarioFormat {
		return nil, fmt.Errorf("unsupported objectives format: %d", objectivesFile.Format)
	}
	if err := validateObjectiveIDs(objectivesFile.Objectives); err != nil {
		return nil, err
	}
	if err := validateObjectiveShape(objectivesFile.Objectives); err != nil {
		return nil, err
	}
	if err := validateObjectiveRequirements(objectivesFile.Objectives); err != nil {
		return nil, err
	}
	if err := validateObjectiveCycles(objectivesFile.Objectives); err != nil {
		return nil, err
	}
	if err := validateObjectivePredicates(objectivesFile.Objectives); err != nil {
		return nil, err
	}
	var setup *Setup
	if manifest.Setup != "" {
		data, err = os.ReadFile(filepath.Join(dir, manifest.Setup))
		if err != nil {
			return nil, fmt.Errorf("failed to read setup: %w", err)
		}
		var setupFile Setup
		if err := json.Unmarshal(data, &setupFile); err != nil {
			return nil, fmt.Errorf("failed to parse setup: %w", err)
		}
		if setupFile.Format != SupportedScenarioFormat {
			return nil, fmt.Errorf("unsupported setup format: %d", setupFile.Format)
		}
		setup = &setupFile
	}
	data, err = os.ReadFile(filepath.Join(dir, manifest.Firmware.Symbols))
	if err != nil {
		return nil, fmt.Errorf("failed to read symbols: %w", err)
	}
	var symbolsFile SymbolsFile
	if err := json.Unmarshal(data, &symbolsFile); err != nil {
		return nil, fmt.Errorf("failed to parse symbols: %w", err)
	}
	if symbolsFile.Format != SupportedScenarioFormat {
		return nil, fmt.Errorf("unsupported symbols format: %d", symbolsFile.Format)
	}
	data, err = os.ReadFile(filepath.Join(dir, manifest.Firmware.MemMap))
	if err != nil {
		return nil, fmt.Errorf("failed to read memmap: %w", err)
	}
	var memMapFile MemMapFile
	if err := json.Unmarshal(data, &memMapFile); err != nil {
		return nil, fmt.Errorf("failed to parse memmap: %w", err)
	}
	if memMapFile.Format != SupportedScenarioFormat {
		return nil, fmt.Errorf("unsupported memmap format: %d", memMapFile.Format)
	}
	scenario := &Scenario{Dir: dir, Manifest: manifest, Objectives: objectivesFile.Objectives, Setup: setup, Symbols: symbolsFile, MemMap: memMapFile, ObjectiveStates: initializeObjectiveStates(objectivesFile.Objectives)}
	if err := scenario.validateObjectiveAddresses(); err != nil {
		return nil, err
	}
	if err := scenario.validateSetupAddresses(); err != nil {
		return nil, err
	}
	return scenario, nil
}

// RESOLVE SCENARIO REFERENCES
// ResolveAddress resolves AddressRef to address in memory
// RUNTIME OBJECTIVE STATE ("complete", "failed", "active", "locked")
// Note: this section only handles "locked" and "active", "complete" and "failed" handled later
type ObjectiveStatus string

const (
	ObjectiveStatusComplete ObjectiveStatus = "complete"
	ObjectiveStatusFailed   ObjectiveStatus = "failed"
	ObjectiveStatusActive   ObjectiveStatus = "active"
	ObjectiveStatusLocked   ObjectiveStatus = "locked"
)

type ObjectiveState struct {
	Status     ObjectiveStatus
	Diagnostic string
}

// create the starting runtime state for every objective in the scenario
func initializeObjectiveStates(objectives []Objective) map[string]*ObjectiveState {
	states := make(map[string]*ObjectiveState)
	for _, objective := range objectives {
		status := ObjectiveStatusActive
		if len(objective.Requires) > 0 {
			status = ObjectiveStatusLocked
		}
		states[objective.ID] = &ObjectiveState{
			Status: status,
		}
	}
	return states
}

// check if all requirements for an objective are complete
func (s *Scenario) requirementsComplete(objective Objective) bool {
	for _, requiredID := range objective.Requires {
		state, ok := s.ObjectiveStates[requiredID]
		if !ok || state.Status != ObjectiveStatusComplete {
			return false
		}
	}
	return true
}

// update which states are now unlocked
func (s *Scenario) updateObjectiveStates() {
	for _, objective := range s.Objectives {
		state, ok := s.ObjectiveStates[objective.ID]
		if !ok {
			continue
		}
		if state.Status != ObjectiveStatusLocked {
			continue //we don't have to unlock objectives that are already active or complete
		}
		if s.requirementsComplete(objective) {
			state.Status = ObjectiveStatusActive
		}
	}
}

// return the current objective state
func (s *Scenario) GetObjectiveState(id string) (*ObjectiveState, bool) {
	state, ok := s.ObjectiveStates[id]
	return state, ok
}
