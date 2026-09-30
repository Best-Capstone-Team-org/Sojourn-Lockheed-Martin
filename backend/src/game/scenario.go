package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

//DATA STRUCTURES AND LOADING

type ObjectivesFile struct {
	Objectives []Objective `json:"objectives"`
	Format     int         `json:"format"`
}

//Scenarios have manifest.json, objectives.json, setup.json, symbols.json, memmap.json

type Scenario struct {
	Dir             string
	Manifest        Manifest
	Objectives      []Objective
	Setup           *Setup
	Symbols         SymbolsFile
	MemMap          MemMapFile
	ObjectiveStates map[string]*ObjectiveState
}

type Objective struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Brief       string     `json:"brief"`
	Requires    []string   `json:"requires,omitempty"` //objectives that must be completed before this one is unlocked
	Points      int        `json:"points,omitempty"`
	Success     Predicate  `json:"success"`
	Fail        *Predicate `json:"fail,omitempty"` //optional if the objective has a failure condition
	Partial     []Partial  `json:"partial,omitempty"`
	Retractable bool       `json:"retractable,omitempty"` //defaults to false, if true, completion can be undone if success becomes false
	Hints       []Hint     `json:"hints,omitempty"`
}

type Predicate struct {
	Op string `json:"op"` //What question to ask

	Of json.RawMessage `json:"of,omitempty"` //possibility of and, or, and not in the conditions

	Path  string `json:"path,omitempty"`
	Cmp   string `json:"cmp,omitempty"` //comparison operator
	Value any    `json:"value,omitempty"`
	Mask  int    `json:"mask,omitempty"` //Which bits to compare

	Frames int `json:"frames,omitempty"` //predicates that are true for a number of frames

	ID    string `json:"id,omitempty"`
	Match string `json:"match,omitempty"`
	Regex bool   `json:"regex,omitempty"`

	// Next three will connect with introspection channel
	At    *AddressRef `json:"at,omitempty"`
	Len   int         `json:"len,omitempty"`   //length in bytes of data to read
	Width int         `json:"width,omitempty"` //how many bytes make up the integer we are examining

	//Next three have scenario ask question about what player has done
	Verb     string `json:"verb,omitempty"`
	Result   string `json:"result,omitempty"`
	Resource string `json:"resource,omitempty"` // which budget we're checking ("writes", "reads")
}

type AddressRef struct {
	Sym    string `json:"sym,omitempty"`
	Field  string `json:"field,omitempty"`  // address of sym + offset of field
	Offset int    `json:"offset,omitempty"` // address of sym + offset
	Addr   string `json:"addr,omitempty"`   // direct address
}

type Partial struct {
	When Predicate `json:"when"`
	Text string    `json:"text"`
}

type Hint struct {
	AfterFrames int    `json:"after_frames"`
	Text        string `json:"text"`
}

type Manifest struct {
	Format     int    `json:"format"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	Revision   int    `json:"revision"`
	Author     string `json:"author,omitempty"`
	Summary    string `json:"summary"`
	Difficulty string `json:"difficulty,omitempty"`

	Firmware FirmwareConfig `json:"firmware"` //which firmware files belong to this scenario
	Link     LinkConfig     `json:"link"`
	Console  ConsoleConfig  `json:"console,omitempty"`

	Briefing   string   `json:"briefing"`        // briefing.md
	Setup      string   `json:"setup,omitempty"` // setup.json
	Objectives string   `json:"objectives"`      // objectives.json
	Docs       []string `json:"docs,omitempty"`
}

type FirmwareConfig struct {
	ROM      string `json:"rom"`
	Symbols  string `json:"symbols"`
	MemMap   string `json:"memmap"`    //valid memory regions
	AppCRC32 string `json:"app_crc32"` //is the right firmware being used
}

type BudgetConfig struct {
	Writes *int `json:"writes,omitempty"`
	Reads  *int `json:"reads,omitempty"`
}

type LinkConfig struct {
	UplinkDelayS    int          `json:"uplink_delay_s"`
	DownlinkDelayS  int          `json:"downlink_delay_s"`
	RequireChecksum *bool        `json:"require_checksum,omitempty"` //whether player commands require checksum, defaults to true
	Budget          BudgetConfig `json:"budget,omitempty"`
}

type ConsoleConfig struct {
	Decode     []string `json:"decode,omitempty"`      //which telemetry channels the console should decode for the player
	DSNComplex string   `json:"dsn_complex,omitempty"` //which DSN complex has the link
}

type Setup struct {
	Format       int          `json:"format"`                  //Scenario format version
	Writes       []SetupWrite `json:"writes"`                  //things to change before playing the scenario
	SettleFrames int          `json:"settle_frames,omitempty"` //how many frames to wait after writes before starting the scenario
}

type SetupWrite struct {
	At   AddressRef `json:"at"`
	U8   *uint8     `json:"u8,omitempty"`
	U16  *uint16    `json:"u16,omitempty"`
	U32  *uint32    `json:"u32,omitempty"`
	Hex  string     `json:"hex,omitempty"`
	Note string     `json:"note,omitempty"`
}

// bridge between human readable scenario objectives and locations in firmware's memory
type SymbolsFile struct {
	Format  int                       `json:"format"`
	Symbols map[string]string         `json:"symbols"` // symbol name to memory address
	Fields  map[string]map[string]int `json:"fields"`  //symbol name to fields to offset
}

// vlaid memory regions for the firmware
type MemMapFile struct {
	Format  int            `json:"format"`
	Regions []MemoryRegion `json:"regions"`
}

type MemoryRegion struct {
	Name string `json:"name"`
	Lo   string `json:"lo"`             // start address
	Hi   string `json:"hi"`             // end address
	Poke string `json:"poke,omitempty"` //write behavior
}

const SupportedScenarioFormat = 1 //the only scenario format we support right now

func LoadScenario(dir string) (*Scenario, error) {

	//load manifest.json
	manifestPath := filepath.Join(dir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	//make sure the manifest format is supported
	if manifest.Format != SupportedScenarioFormat {
		return nil, fmt.Errorf("unsupported manifest format: %d", manifest.Format)
	}

	//load objectives.json
	objectivesPath := filepath.Join(dir, manifest.Objectives)
	data, err = os.ReadFile(objectivesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read objectives: %w", err)
	}

	var objectivesFile ObjectivesFile
	if err := json.Unmarshal(data, &objectivesFile); err != nil {
		return nil, fmt.Errorf("failed to parse objectives: %w", err)
	}
	//make sure the objectives format is supported
	if objectivesFile.Format != SupportedScenarioFormat {
		return nil, fmt.Errorf("unsupported objectives format: %d", objectivesFile.Format)
	}

	var setup *Setup
	if manifest.Setup != "" {
		setupPath := filepath.Join(dir, manifest.Setup)

		data, err = os.ReadFile(setupPath)
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

	// load symbols.json
	symbolsPath := filepath.Join(dir, manifest.Firmware.Symbols)

	data, err = os.ReadFile(symbolsPath)
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

	// load memmap.json
	memMapPath := filepath.Join(dir, manifest.Firmware.MemMap)

	data, err = os.ReadFile(memMapPath)
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

	//set objective states based on requirements
	objectiveStates := initializeObjectiveStates(objectivesFile.Objectives)
	//assemble the scenario
	return &Scenario{
		Dir:             dir,
		Manifest:        manifest,
		Objectives:      objectivesFile.Objectives,
		Setup:           setup,
		MemMap:          memMapFile,
		Symbols:         symbolsFile,
		ObjectiveStates: objectiveStates,
	}, nil
}

//RESOLVE SCENARIO REFERENCES

// ResolveAddress resolves AddressRef to address in memory
func (s *Scenario) ResolveAddress(ref AddressRef) (uint64, error) {
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

//RUNTIME OBJECTIVE STATE ("complete", "failed", "active", "locked")
//Note: this section only handles "locked" and "active", "complete" and "failed" handled later
type ObjectiveStatus string

const (
	ObjectiveStatusComplete ObjectiveStatus = "complete"
	ObjectiveStatusFailed   ObjectiveStatus = "failed"
	ObjectiveStatusActive   ObjectiveStatus = "active"
	ObjectiveStatusLocked   ObjectiveStatus = "locked"
)

type ObjectiveState struct {
	Status ObjectiveStatus
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
//TODO: OBJECTIVE EVALUATION
