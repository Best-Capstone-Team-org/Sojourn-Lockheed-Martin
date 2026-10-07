package game

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

//DATA STRUCTURES AND LOADING

type ObjectivesFile struct {
	Objectives []Objective `json:"objectives"`

	Format int `json:"format"`
}

//Scenarios have manifest.json, objectives.json, setup.json, symbols.json, memmap.json

type Scenario struct {
	Dir string

	Manifest Manifest

	Objectives []Objective

	Setup *Setup

	Symbols SymbolsFile

	MemMap MemMapFile

	ObjectiveStates map[string]*ObjectiveState
}

type Objective struct {
	ID string `json:"id"`

	Title string `json:"title"`

	Brief string `json:"brief"`

	Requires []string `json:"requires,omitempty"` //objectives that must be completed before this one is unlocked

	Points int `json:"points,omitempty"`

	Success Predicate `json:"success"`

	Fail *Predicate `json:"fail,omitempty"` //optional if the objective has a failure condition

	Partial []Partial `json:"partial,omitempty"`

	Retractable bool `json:"retractable,omitempty"` //defaults to false, if true, completion can be undone if success becomes false

	Hints []Hint `json:"hints,omitempty"`
}

type Predicate struct {
	Op string `json:"op"` //What question to ask

	Of json.RawMessage `json:"of,omitempty"` //possibility of and, or, and not in the conditions

	Path string `json:"path,omitempty"`

	Cmp string `json:"cmp,omitempty"` //comparison operator

	Value any `json:"value,omitempty"`

	Mask int `json:"mask,omitempty"` //Which bits to compare

	Frames int `json:"frames,omitempty"` //predicates that are true for a number of frames

	ID string `json:"id,omitempty"`

	Match string `json:"match,omitempty"`

	Regex bool `json:"regex,omitempty"`

	// Next three will connect with introspection channel

	At *AddressRef `json:"at,omitempty"`

	Len int `json:"len,omitempty"` //length in bytes of data to read

	Width int `json:"width,omitempty"` //how many bytes make up the integer we are examining

	//Next three have scenario ask question about what player has done

	Verb string `json:"verb,omitempty"`

	Result string `json:"result,omitempty"`

	Resource string `json:"resource,omitempty"` // which budget we're checking ("writes", "reads")

	Lang  string `json:"lang,omitempty"`
	Entry string `json:"entry,omitempty"`
}

type AddressRef struct {
	Sym string `json:"sym,omitempty"`

	Field string `json:"field,omitempty"` // address of sym + offset of field

	Offset int `json:"offset,omitempty"` // address of sym + offset

	Addr string `json:"addr,omitempty"` // direct address

}

type Partial struct {
	When Predicate `json:"when"`

	Text string `json:"text"`
}

type CommandLogEntry struct {
	Verb string

	Result string

	Addr uint32
}

type Hint struct {
	AfterFrames int `json:"after_frames"`

	Text string `json:"text"`
}

type Manifest struct {
	Format int `json:"format"`

	ID string `json:"id"`

	Title string `json:"title"`

	Revision int `json:"revision"`

	Author string `json:"author,omitempty"`

	Summary string `json:"summary"`

	Difficulty string `json:"difficulty,omitempty"`

	Impure bool `json:"impure,omitempty"`

	Firmware FirmwareConfig `json:"firmware"` //which firmware files belong to this scenario

	Link LinkConfig `json:"link"`

	Console ConsoleConfig `json:"console,omitempty"`

	Briefing string `json:"briefing"` // briefing.md

	Setup string `json:"setup,omitempty"` // setup.json

	Objectives string `json:"objectives"` // objectives.json

	Docs []string `json:"docs,omitempty"`
}

type FirmwareConfig struct {
	ROM string `json:"rom"`

	Symbols string `json:"symbols"`

	MemMap string `json:"memmap"` //valid memory regions

	AppCRC32 string `json:"app_crc32"` //is the right firmware being used

}

type BudgetConfig struct {
	Writes *int `json:"writes,omitempty"`

	Reads *int `json:"reads,omitempty"`
}

type LinkConfig struct {
	UplinkDelayS int `json:"uplink_delay_s"`

	DownlinkDelayS int `json:"downlink_delay_s"`

	RequireChecksum *bool `json:"require_checksum,omitempty"` //whether player commands require checksum, defaults to true

	Budget BudgetConfig `json:"budget,omitempty"`
}

type ConsoleConfig struct {
	Decode []string `json:"decode,omitempty"` //which telemetry channels the console should decode for the player

	DSNComplex string `json:"dsn_complex,omitempty"` //which DSN complex has the link

}

type Setup struct {
	Format int `json:"format"` //Scenario format version

	Writes []SetupWrite `json:"writes"` //things to change before playing the scenario

	SettleFrames int `json:"settle_frames,omitempty"` //how many frames to wait after writes before starting the scenario

}

type SetupWrite struct {
	At AddressRef `json:"at"`

	U8 *uint8 `json:"u8,omitempty"`

	U16 *uint16 `json:"u16,omitempty"`

	U32 *uint32 `json:"u32,omitempty"`

	Hex string `json:"hex,omitempty"`

	Note string `json:"note,omitempty"`
}

// bridge between human readable scenario objectives and locations in firmware's memory

type SymbolsFile struct {
	Format int `json:"format"`

	Symbols map[string]string `json:"symbols"` // symbol name to memory address

	Fields map[string]map[string]int `json:"fields"` //symbol name to fields to offset

}

// vlaid memory regions for the firmware

type MemMapFile struct {
	Format int `json:"format"`

	Regions []MemoryRegion `json:"regions"`
}

type MemoryRegion struct {
	Name string `json:"name"`

	Lo string `json:"lo"` // start address

	Hi string `json:"hi"` // end address

	Poke string `json:"poke,omitempty"` //write behavior

}

const SupportedScenarioFormat = 1 //the only scenario format we support right now

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

func validateComparison(cmp string) error {
	switch cmp {
	case "eq", "ne", "lt", "lte", "gt", "gte", "in":
		return nil
	default:
		return fmt.Errorf("unknown comparison operator %q", cmp)
	}
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

//RESOLVE SCENARIO REFERENCES

// ResolveAddress resolves AddressRef to address in memory

func validateAddressRef(ref AddressRef) error {
	if ref.Addr != "" {
		if ref.Sym != "" || ref.Field != "" || ref.Offset != 0 {
			return fmt.Errorf("address reference using 'addr' cannot also use sym, field, or offset")
		}
		return nil
	}
	if ref.Sym == "" {
		return fmt.Errorf("address reference requires either 'addr' or 'sym'")
	}
	if ref.Field != "" && ref.Offset != 0 {
		return fmt.Errorf("symbol address reference cannot use both 'field' and 'offset'")
	}
	return nil
}

func (s *Scenario) ResolveAddress(ref AddressRef) (uint64, error) {
	if err := validateAddressRef(ref); err != nil {
		return 0, err
	}

	// If we are given a direct address

	if ref.Addr != "" {

		addr, err := strconv.ParseUint(ref.Addr, 0, 64)

		if err != nil {

			return 0, fmt.Errorf("invalid address %q: %w", ref.Addr, err)

		}

		return addr, nil

	}

	// Otherwise, we need a symbol

	if ref.Sym == "" {

		return 0, fmt.Errorf("address reference has no addr or sym")

	}

	symbolAddr, ok := s.Symbols.Symbols[ref.Sym]

	if !ok {

		return 0, fmt.Errorf("unknown symbol %q", ref.Sym)

	}

	base, err := strconv.ParseUint(symbolAddr, 0, 64)

	if err != nil {

		return 0, fmt.Errorf("invalid address for symbol %q: %w", ref.Sym, err)

	}

	// Symbol + field case

	if ref.Field != "" {

		fields, ok := s.Symbols.Fields[ref.Sym]

		if !ok {

			return 0, fmt.Errorf("no field information for symbol %q", ref.Sym)

		}

		offset, ok := fields[ref.Field]

		if !ok {

			return 0, fmt.Errorf(

				"unknown field %q for symbol %q",

				ref.Field,

				ref.Sym,
			)

		}

		return base + uint64(offset), nil

	}

	// Symbol + offset case

	if ref.Offset != 0 {

		return base + uint64(ref.Offset), nil

	}

	// Just the symbol itself

	return base, nil

}

func (s *Scenario) validateMemoryRange(ref AddressRef, length int) error {

	if length <= 0 {

		return fmt.Errorf("memory range length must be greater than 0")

	}

	addr, err := s.ResolveAddress(ref)

	if err != nil {

		return err

	}

	if addr > math.MaxUint32 {
		return fmt.Errorf("address 0x%X exceeds 32-bit introspection address space", addr)
	}

	if uint64(length-1) > ^uint64(0)-addr {

		return fmt.Errorf(

			"memory range starting at 0x%X with length %d overflows address space",

			addr,

			length,
		)

	}

	end := addr + uint64(length) - 1

	for _, region := range s.MemMap.Regions {

		lo, err := strconv.ParseUint(region.Lo, 0, 64)

		if err != nil {

			return fmt.Errorf(

				"invalid lower address %q for memory region %q",

				region.Lo,

				region.Name,
			)

		}

		hi, err := strconv.ParseUint(region.Hi, 0, 64)

		if err != nil {

			return fmt.Errorf(

				"invalid upper address %q for memory region %q",

				region.Hi,

				region.Name,
			)

		}

		if addr >= lo && end <= hi {

			return nil

		}

	}

	return fmt.Errorf(

		"memory range 0x%X-0x%X is outside the declared memory map",

		addr,

		end,
	)

}

func (s *Scenario) validatePredicateAddresses(predicate Predicate) error {

	var length int

	switch predicate.Op {

	case "mem_u8":

		length = 1

	case "mem_u16":

		length = 2

	case "mem_u32":

		length = 4

	case "mem_bits":

		length = predicate.Width

		if length == 0 {

			length = 4

		}

	case "mem", "mem_changed":

		length = predicate.Len

	}

	if length > 0 && predicate.At != nil {

		if err := s.validateMemoryRange(*predicate.At, length); err != nil {

			return fmt.Errorf(

				"invalid address for %q predicate: %w",

				predicate.Op,

				err,
			)

		}

	}

	if predicate.Op == "script" {
		parts := strings.Split(predicate.Entry, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid script entry %q", predicate.Entry)
		}
		if _, err := os.Stat(filepath.Join(s.Dir, parts[0])); err != nil {
			return fmt.Errorf("script file %q is unavailable: %w", parts[0], err)
		}
	}

	if predicate.Op == "commanded" && predicate.At != nil {

		if _, err := s.ResolveAddress(*predicate.At); err != nil {

			return fmt.Errorf(

				"invalid address for commanded predicate: %w",

				err,
			)

		}

	}

	switch predicate.Op {

	case "all", "any":

		var children []Predicate

		if err := json.Unmarshal(predicate.Of, &children); err != nil {

			return err

		}

		for _, child := range children {

			if err := s.validatePredicateAddresses(child); err != nil {

				return err

			}

		}

	case "not", "ever", "sustained", "within":

		var child Predicate

		if err := json.Unmarshal(predicate.Of, &child); err != nil {

			return err

		}

		if err := s.validatePredicateAddresses(child); err != nil {

			return err

		}

	}

	return nil

}

func (s *Scenario) validateObjectiveAddresses() error {

	for _, objective := range s.Objectives {

		if err := s.validatePredicateAddresses(objective.Success); err != nil {

			return fmt.Errorf(

				"invalid success predicate address for objective %q: %w",

				objective.ID,

				err,
			)

		}

		if objective.Fail != nil {

			if err := s.validatePredicateAddresses(*objective.Fail); err != nil {

				return fmt.Errorf(

					"invalid fail predicate address for objective %q: %w",

					objective.ID,

					err,
				)

			}

		}

		for _, partial := range objective.Partial {

			if err := s.validatePredicateAddresses(partial.When); err != nil {

				return fmt.Errorf(

					"invalid partial predicate address for objective %q: %w",

					objective.ID,

					err,
				)

			}

		}

	}

	return nil

}

func (s *Scenario) validateSetupAddresses() error {
	if s.Setup == nil {
		return nil
	}
	for i, write := range s.Setup.Writes {
		if err := validateAddressRef(write.At); err != nil {
			return fmt.Errorf("setup write %d has invalid address reference: %w", i, err)
		}
		count := 0
		length := 0
		if write.U8 != nil {
			count++
			length = 1
		}
		if write.U16 != nil {
			count++
			length = 2
		}
		if write.U32 != nil {
			count++
			length = 4
		}
		if write.Hex != "" {
			count++
			data, err := hex.DecodeString(write.Hex)
			if err != nil {
				return fmt.Errorf("setup write %d has invalid hex value %q: %w", i, write.Hex, err)
			}
			if len(data) == 0 {
				return fmt.Errorf("setup write %d has empty hex value", i)
			}
			length = len(data)
		}
		if count != 1 {
			return fmt.Errorf("setup write %d must contain exactly one of u8, u16, u32, or hex", i)
		}
		if err := s.validateMemoryRange(write.At, length); err != nil {
			return fmt.Errorf("invalid setup write %d address: %w", i, err)
		}
	}
	return nil
}

//RUNTIME OBJECTIVE STATE ("complete", "failed", "active", "locked")

//Note: this section only handles "locked" and "active", "complete" and "failed" handled later

type ObjectiveStatus string

const (
	ObjectiveStatusComplete ObjectiveStatus = "complete"

	ObjectiveStatusFailed ObjectiveStatus = "failed"

	ObjectiveStatusActive ObjectiveStatus = "active"

	ObjectiveStatusLocked ObjectiveStatus = "locked"
)

type ObjectiveState struct {
	Status ObjectiveStatus

	Diagnostic string
}

//create the starting runtime state for every objective in the scenario

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

//check if all requirements for an objective are complete

func (s *Scenario) requirementsComplete(objective Objective) bool {

	for _, requiredID := range objective.Requires {

		state, ok := s.ObjectiveStates[requiredID]

		if !ok || state.Status != ObjectiveStatusComplete {

			return false

		}

	}

	return true

}

//update which states are now unlocked

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

//return the current objective state

func (s *Scenario) GetObjectiveState(id string) (*ObjectiveState, bool) {

	state, ok := s.ObjectiveStates[id]

	return state, ok

}

//TODO: PREDICATE EVALUATION

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

//normalize types due to json unmarshalling returning everything as float64

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

//TODO: OBJECTIVE EVALUATION

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
