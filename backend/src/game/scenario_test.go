package game
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)
type fakeMemoryReader struct {
	data       []byte
	lastAddr   uint32
	lastLength int
}
type failingMemoryReader struct{}
func (f *failingMemoryReader) Read(addr uint32, length int) ([]byte, error) {
	return nil, fmt.Errorf("simulated memory read failure")
}
func (f *fakeMemoryReader) Read(addr uint32, length int) ([]byte, error) {
	f.lastAddr = addr
	f.lastLength = length
	return f.data[:length], nil
}
func TestResolveAddressDirect(t *testing.T) {
	s := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
			Fields: map[string]map[string]int{
				"g_config": {
					"mode": 4,
				},
			},
		},
	}
	ref := AddressRef{
		Addr: "0x20002000",
	}
	addr, err := s.ResolveAddress(ref)
	if err != nil {
		t.Fatalf("ResolveAddress returned error: %v", err)
	}
	if addr != 0x20002000 {
		t.Errorf("expected 0x20002000, got 0x%x", addr)
	}
}
func TestResolveAddressSymbol(t *testing.T) {
	s := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
		},
	}
	ref := AddressRef{
		Sym: "g_config",
	}
	addr, err := s.ResolveAddress(ref)
	if err != nil {
		t.Fatalf("ResolveAddress returned error: %v", err)
	}
	if addr != 0x20001000 {
		t.Errorf("expected 0x20001000, got 0x%x", addr)
	}
}
func TestResolveAddressOffset(t *testing.T) {
	s := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
		},
	}
	ref := AddressRef{
		Sym:    "g_config",
		Offset: 8,
	}
	addr, err := s.ResolveAddress(ref)
	if err != nil {
		t.Fatalf("ResolveAddress returned error: %v", err)
	}
	if addr != 0x20001008 {
		t.Errorf("expected 0x20001008, got 0x%x", addr)
	}
}
func TestResolveAddressField(t *testing.T) {
	s := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
			Fields: map[string]map[string]int{
				"g_config": {
					"mode": 4,
				},
			},
		},
	}
	ref := AddressRef{
		Sym:   "g_config",
		Field: "mode",
	}
	addr, err := s.ResolveAddress(ref)
	if err != nil {
		t.Fatalf("ResolveAddress returned error: %v", err)
	}
	if addr != 0x20001004 {
		t.Errorf("expected 0x20001004, got 0x%x", addr)
	}
}
func TestResolveAddressUnknownSymbol(t *testing.T) {
	s := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
		},
	}
	ref := AddressRef{
		Sym: "does_not_exist",
	}
	_, err := s.ResolveAddress(ref)
	if err == nil {
		t.Errorf("expected error for unknown symbol, got nil")
	}
}
func TestResolveAddressUnknownField(t *testing.T) {
	s := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
			Fields: map[string]map[string]int{
				"g_config": {
					"mode": 4,
				},
			},
		},
	}
	ref := AddressRef{
		Sym:   "g_config",
		Field: "does_not_exist",
	}
	_, err := s.ResolveAddress(ref)
	if err == nil {
		t.Errorf("expected error for unknown field, got nil")
	}
}
func TestResolveAddressInvalidAddress(t *testing.T) {
	s := &Scenario{}
	ref := AddressRef{
		Addr: "not_an_address",
	}
	_, err := s.ResolveAddress(ref)
	if err == nil {
		t.Errorf("expected error for invalid address, got nil")
	}
}
func TestLoadScenario(t *testing.T) {
	// Create a temporary directory for our fake scenario package.
	dir := t.TempDir()
	// Create the firmware directory expected by the manifest.
	firmwareDir := filepath.Join(dir, "firmware")
	if err := os.MkdirAll(firmwareDir, 0755); err != nil {
		t.Fatalf("failed to create firmware directory: %v", err)
	}
	// Create manifest.json.
	manifest := `{
		"format": 1,
		"id": "sojourn.test.scenario",
		"title": "Test Scenario",
		"revision": 1,
		"author": "Test",
		"summary": "Scenario used for unit testing.",
		"firmware": {
			"rom": "firmware/probe_rom.elf",
			"symbols": "firmware/symbols.json",
			"memmap": "firmware/memmap.json",
			"app_crc32": "0x12345678"
		},
		"link": {
			"uplink_delay_s": 8,
			"downlink_delay_s": 8
		},
		"briefing": "briefing.md",
		"objectives": "objectives.json"
	}`
	// Create objectives.json.
	objectives := `{
		"format": 1,
		"objectives": [
			{
				"id": "test-objective",
				"title": "Test Objective",
				"brief": "Complete the test.",
				"success": {
					"op": "channel_present",
					"id": "HK"
				}
			}
		]
	}`
	// Create symbols.json.
	symbols := `{
		"format": 1,
		"symbols": {
			"g_config": "0x20001000"
		},
		"fields": {
			"g_config": {
				"mode": 4
			}
		}
	}`
	// Create memmap.json.
	memmap := `{
		"format": 1,
		"regions": [
			{
				"name": "RAM",
				"lo": "0x20000000",
				"hi": "0x20010000",
				"poke": "rw"
			}
		]
	}`
	// Create all files referenced by the scenario package.
	files := map[string]string{
		"manifest.json":          manifest,
		"objectives.json":        objectives,
		"briefing.md":            "# Test Briefing\n",
		"firmware/probe_rom.elf": "test firmware",
		"firmware/symbols.json":  symbols,
		"firmware/memmap.json":   memmap,
	}
	for name, contents := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}
	// Actually test LoadScenario.
	scenario, err := LoadScenario(dir)
	if err != nil {
		t.Fatalf("LoadScenario returned error: %v", err)
	}
	// Check that important data was loaded correctly.
	if scenario.Manifest.ID != "sojourn.test.scenario" {
		t.Errorf(
			"expected scenario ID %q, got %q",
			"sojourn.test.scenario",
			scenario.Manifest.ID,
		)
	}
	if scenario.Manifest.Title != "Test Scenario" {
		t.Errorf(
			"expected title %q, got %q",
			"Test Scenario",
			scenario.Manifest.Title,
		)
	}
	if len(scenario.Objectives) != 1 {
		t.Fatalf("expected 1 objective, got %d", len(scenario.Objectives))
	}
	if scenario.Objectives[0].ID != "test-objective" {
		t.Errorf(
			"expected objective ID %q, got %q",
			"test-objective",
			scenario.Objectives[0].ID,
		)
	}
	if scenario.Symbols.Symbols["g_config"] != "0x20001000" {
		t.Errorf("g_config symbol was not loaded correctly")
	}
	if len(scenario.MemMap.Regions) != 1 {
		t.Errorf(
			"expected 1 memory region, got %d",
			len(scenario.MemMap.Regions),
		)
	}
}
func TestLoadScenarioWithSetup(t *testing.T) {
	dir := t.TempDir()
	firmwareDir := filepath.Join(dir, "firmware")
	if err := os.MkdirAll(firmwareDir, 0755); err != nil {
		t.Fatalf("failed to create firmware directory: %v", err)
	}
	manifest := `{
		"format": 1,
		"id": "sojourn.test.setup",
		"title": "Setup Test",
		"revision": 1,
		"author": "Test",
		"summary": "Tests loading setup.json.",
		"firmware": {
			"rom": "firmware/probe_rom.elf",
			"symbols": "firmware/symbols.json",
			"memmap": "firmware/memmap.json",
			"app_crc32": "0x12345678"
		},
		"link": {
			"uplink_delay_s": 8,
			"downlink_delay_s": 8
		},
		"briefing": "briefing.md",
		"setup": "setup.json",
		"objectives": "objectives.json"
	}`
	objectives := `{
		"format": 1,
		"objectives": []
	}`
	symbols := `{
		"format": 1,
		"symbols": {
			"g_config": "0x20001000"
		},
		"fields": {}
	}`
	memmap := `{
		"format": 1,
		"regions": [
			{
				"name": "RAM",
				"lo": "0x20000000",
				"hi": "0x2000FFFF"
			}
		]
	}`
	setup := `{
		"format": 1,
		"writes": [
			{
				"at": {
					"sym": "g_config"
				},
				"u32": 42
			}
		],
		"settle_frames": 3
	}`
	// Create all files referenced by the scenario package.
	files := map[string]string{
		"manifest.json":          manifest,
		"objectives.json":        objectives,
		"setup.json":             setup,
		"briefing.md":            "# Test Briefing\n",
		"firmware/probe_rom.elf": "test firmware",
		"firmware/symbols.json":  symbols,
		"firmware/memmap.json":   memmap,
	}
	for name, contents := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}
	scenario, err := LoadScenario(dir)
	if err != nil {
		t.Fatalf("LoadScenario returned error: %v", err)
	}
	if scenario.Setup == nil {
		t.Fatal("expected setup to be loaded, got nil")
	}
	if scenario.Setup.SettleFrames != 3 {
		t.Errorf(
			"expected settle_frames 3, got %d",
			scenario.Setup.SettleFrames,
		)
	}
	if len(scenario.Setup.Writes) != 1 {
		t.Fatalf(
			"expected 1 setup write, got %d",
			len(scenario.Setup.Writes),
		)
	}
	if scenario.Setup.Writes[0].U32 == nil {
		t.Fatal("expected setup write to contain a u32 value")
	}
	if *scenario.Setup.Writes[0].U32 != 42 {
		t.Errorf(
			"expected setup u32 value 42, got %d",
			*scenario.Setup.Writes[0].U32,
		)
	}
}
func TestLoadScenarioUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	manifest := `{
		"format": 999,
		"id": "sojourn.test.bad-format",
		"title": "Bad Format",
		"revision": 1,
		"author": "Test",
		"summary": "Uses an unsupported format."
	}`
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest.json: %v", err)
	}
	_, err := LoadScenario(dir)
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
}
func TestInitializeObjectiveStatesActive(t *testing.T) {
	objectives := []Objective{
		{
			ID: "objective-1",
		},
	}
	states := initializeObjectiveStates(objectives)
	state := states["objective-1"]
	if state == nil {
		t.Fatal("expected objective state to exist")
	}
	if state.Status != ObjectiveStatusActive {
		t.Errorf(
			"expected objective to be active, got %s",
			state.Status,
		)
	}
}
func TestInitializeObjectiveStatesLocked(t *testing.T) {
	objectives := []Objective{
		{
			ID:       "objective-1",
			Requires: []string{"previous-objective"},
		},
	}
	states := initializeObjectiveStates(objectives)
	state := states["objective-1"]
	if state == nil {
		t.Fatal("expected objective state to exist")
	}
	if state.Status != ObjectiveStatusLocked {
		t.Errorf(
			"expected objective to be locked, got %s",
			state.Status,
		)
	}
}
func TestUpdateObjectiveStatesUnlocksObjective(t *testing.T) {
	s := &Scenario{
		Objectives: []Objective{
			{
				ID: "objective-1",
			},
			{
				ID:       "objective-2",
				Requires: []string{"objective-1"},
			},
		},
		ObjectiveStates: map[string]*ObjectiveState{
			"objective-1": {
				Status: ObjectiveStatusComplete,
			},
			"objective-2": {
				Status: ObjectiveStatusLocked,
			},
		},
	}
	s.updateObjectiveStates()
	if s.ObjectiveStates["objective-2"].Status != ObjectiveStatusActive {
		t.Errorf(
			"expected objective-2 to be active, got %s",
			s.ObjectiveStates["objective-2"].Status,
		)
	}
}
func TestUpdateObjectiveStatesStaysLocked(t *testing.T) {
	s := &Scenario{
		Objectives: []Objective{
			{
				ID: "objective-1",
			},
			{
				ID:       "objective-2",
				Requires: []string{"objective-1"},
			},
		},
		ObjectiveStates: map[string]*ObjectiveState{
			"objective-1": {
				Status: ObjectiveStatusActive,
			},
			"objective-2": {
				Status: ObjectiveStatusLocked,
			},
		},
	}
	s.updateObjectiveStates()
	if s.ObjectiveStates["objective-2"].Status != ObjectiveStatusLocked {
		t.Errorf(
			"expected objective-2 to stay locked, got %s",
			s.ObjectiveStates["objective-2"].Status,
		)
	}
}
func TestUpdateObjectiveStatesMultipleRequirements(t *testing.T) {
	s := &Scenario{
		Objectives: []Objective{
			{
				ID: "objective-1",
			},
			{
				ID: "objective-2",
			},
			{
				ID:       "objective-3",
				Requires: []string{"objective-1", "objective-2"},
			},
		},
		ObjectiveStates: map[string]*ObjectiveState{
			"objective-1": {
				Status: ObjectiveStatusComplete,
			},
			"objective-2": {
				Status: ObjectiveStatusActive,
			},
			"objective-3": {
				Status: ObjectiveStatusLocked,
			},
		},
	}
	s.updateObjectiveStates()
	if s.ObjectiveStates["objective-3"].Status != ObjectiveStatusLocked {
		t.Errorf(
			"expected objective-3 to stay locked, got %s",
			s.ObjectiveStates["objective-3"].Status,
		)
	}
}
func TestUpdateObjectiveStatesDoesNotChangeComplete(t *testing.T) {
	s := &Scenario{
		Objectives: []Objective{
			{
				ID: "objective-1",
			},
		},
		ObjectiveStates: map[string]*ObjectiveState{
			"objective-1": {
				Status: ObjectiveStatusComplete,
			},
		},
	}
	s.updateObjectiveStates()
	if s.ObjectiveStates["objective-1"].Status != ObjectiveStatusComplete {
		t.Errorf(
			"expected objective-1 to stay complete, got %s",
			s.ObjectiveStates["objective-1"].Status,
		)
	}
}
func TestUpdateObjectiveStatesMultipleRequirementsComplete(t *testing.T) {
	s := &Scenario{
		Objectives: []Objective{
			{
				ID: "objective-1",
			},
			{
				ID: "objective-2",
			},
			{
				ID:       "objective-3",
				Requires: []string{"objective-1", "objective-2"},
			},
		},
		ObjectiveStates: map[string]*ObjectiveState{
			"objective-1": {
				Status: ObjectiveStatusComplete,
			},
			"objective-2": {
				Status: ObjectiveStatusComplete,
			},
			"objective-3": {
				Status: ObjectiveStatusLocked,
			},
		},
	}
	s.updateObjectiveStates()
	if s.ObjectiveStates["objective-3"].Status != ObjectiveStatusActive {
		t.Errorf(
			"expected objective-3 to be active, got %s",
			s.ObjectiveStates["objective-3"].Status,
		)
	}
}
func TestGetObjectiveState(t *testing.T) {
	s := &Scenario{
		ObjectiveStates: map[string]*ObjectiveState{
			"objective-1": {
				Status: ObjectiveStatusActive,
			},
		},
	}
	state, ok := s.GetObjectiveState("objective-1")
	if !ok {
		t.Fatal("expected objective state to exist")
	}
	if state.Status != ObjectiveStatusActive {
		t.Errorf(
			"expected objective to be active, got %s",
			state.Status,
		)
	}
}
func TestGetObjectiveStateUnknown(t *testing.T) {
	s := &Scenario{
		ObjectiveStates: map[string]*ObjectiveState{},
	}
	_, ok := s.GetObjectiveState("does-not-exist")
	if ok {
		t.Error("expected unknown objective to not have a state")
	}
}
func TestNumericValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected float64
		ok       bool
	}{
		{"int", int(5), 5, true},
		{"int32", int32(10), 10, true},
		{"uint16", uint16(20), 20, true},
		{"float64", float64(2.5), 2.5, true},
		{"string", "NOMINAL", 0, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := numericValue(test.input)
			if ok != test.ok {
				t.Errorf("expected ok=%v, got %v", test.ok, ok)
			}
			if got != test.expected {
				t.Errorf("expected %v, got %v", test.expected, got)
			}
		})
	}
}
func TestCompareValuesNumeric(t *testing.T) {
	tests := []struct {
		name     string
		actual   any
		expected any
		cmp      string
		result   bool
	}{
		{"equal", uint16(5000), float64(5000), "eq", true},
		{"not equal", 5, float64(10), "ne", true},
		{"less than", int32(5), float64(10), "lt", true},
		{"less than or equal", 10, float64(10), "lte", true},
		{"greater than", uint32(20), float64(10), "gt", true},
		{"greater than or equal", 10, float64(10), "gte", true},
		{"failed comparison", 5, float64(10), "gt", false},
		{"unknown comparison", 5, float64(5), "bad", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := compareValues(test.actual, test.expected, test.cmp)
			if got != test.result {
				t.Errorf("expected %v, got %v", test.result, got)
			}
		})
	}
}
func TestCompareValuesString(t *testing.T) {
	if !compareValues("NOMINAL", "NOMINAL", "eq") {
		t.Error("expected equal strings to match")
	}
	if compareValues("NOMINAL", "SAFE", "eq") {
		t.Error("expected different strings to not match")
	}
	if !compareValues("NOMINAL", "SAFE", "ne") {
		t.Error("expected different strings to satisfy ne")
	}
}
func TestCompareValuesIn(t *testing.T) {
	tests := []struct {
		name     string
		actual   any
		expected any
		result   bool
	}{
		{
			name:     "string in list",
			actual:   "SAFE",
			expected: []any{"NOMINAL", "SAFE"},
			result:   true,
		},
		{
			name:     "string not in list",
			actual:   "BOOT",
			expected: []any{"NOMINAL", "SAFE"},
			result:   false,
		},
		{
			name:     "number in list",
			actual:   uint16(20),
			expected: []any{float64(10), float64(20), float64(30)},
			result:   true,
		},
		{
			name:     "number not in list",
			actual:   uint16(40),
			expected: []any{float64(10), float64(20), float64(30)},
			result:   false,
		},
		{
			name:     "in requires list",
			actual:   "SAFE",
			expected: "SAFE",
			result:   false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := compareValues(test.actual, test.expected, "in")
			if got != test.result {
				t.Errorf("expected %v, got %v", test.result, got)
			}
		})
	}
}
func TestEvaluateTelemetryPredicateUnknownPath(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "NOMINAL",
		},
	}
	predicate := Predicate{
		Op:    "tlm",
		Path:  "temperature",
		Cmp:   "gt",
		Value: 50,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Errorf("expected predicate to be false")
	}
}
func TestEvaluateAndPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode":   "NOMINAL",
			"bus_mv": 5000,
		},
	}
	predicate := Predicate{
		Op: "all",
		Of: json.RawMessage(`[
			{
				"op": "tlm",
				"path": "mode",
				"cmp": "eq",
				"value": "NOMINAL"
			},
			{
				"op": "tlm",
				"path": "bus_mv",
				"cmp": "gte",
				"value": 4000
			}
		]`),
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Errorf("expected and predicate to be true")
	}
}
func TestEvaluateOrPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "SAFE",
		},
	}
	predicate := Predicate{
		Op: "any",
		Of: json.RawMessage(`[
			{
				"op": "tlm",
				"path": "mode",
				"cmp": "eq",
				"value": "NOMINAL"
			},
			{
				"op": "tlm",
				"path": "mode",
				"cmp": "eq",
				"value": "SAFE"
			}
		]`),
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Errorf("expected or predicate to be true")
	}
}
func TestEvaluateNotPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "SAFE",
		},
	}
	predicate := Predicate{
		Op: "not",
		Of: json.RawMessage(`{
			"op": "tlm",
			"path": "mode",
			"cmp": "eq",
			"value": "NOMINAL"
		}`),
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Errorf("expected not predicate to be true")
	}
}
func TestNestedLogicalPredicates(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode":   "NOMINAL",
			"bus_mv": 5000,
		},
	}
	predicate := Predicate{
		Op: "all",
		Of: json.RawMessage(`[
			{
				"op": "tlm",
				"path": "mode",
				"cmp": "eq",
				"value": "NOMINAL"
			},
			{
				"op": "not",
				"of": {
					"op": "tlm",
					"path": "bus_mv",
					"cmp": "lt",
					"value": 4000
				}
			}
		]`),
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Errorf("expected nested predicate to be true")
	}
}
func TestGetIntrospectionValue(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
			Fields: map[string]map[string]int{
				"g_config": {
					"mode": 4,
				},
			},
		},
	}
	reader := &fakeMemoryReader{
		data: []byte{2},
	}
	ctx := &EvaluationContext{
		Introspect: reader,
	}
	predicate := Predicate{
		At: &AddressRef{
			Sym:   "g_config",
			Field: "mode",
		},
		Len: 1,
	}
	data, errMsg := scenario.getIntrospectionValue(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 byte, got %d", len(data))
	}
	if data[0] != 2 {
		t.Errorf("expected value 2, got %d", data[0])
	}
	if reader.lastAddr != 0x20001004 {
		t.Errorf("expected read address 0x20001004, got 0x%x", reader.lastAddr)
	}
	if reader.lastLength != 1 {
		t.Errorf("expected read length 1, got %d", reader.lastLength)
	}
}
func TestIntrospectionBytesToValuesWidth1(t *testing.T) {
	data := []byte{0x2A}
	value, errMsg := introspectionBytesToValues(data, 1)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if value != uint8(42) {
		t.Errorf("expected 42, got %v", value)
	}
}
func TestIntrospectionBytesToValuesWidth2(t *testing.T) {
	data := []byte{0x34, 0x12}
	value, errMsg := introspectionBytesToValues(data, 2)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if value != uint16(0x1234) {
		t.Errorf("expected 0x1234, got %v", value)
	}
}
func TestIntrospectionBytesToValuesWidth4(t *testing.T) {
	data := []byte{0x78, 0x56, 0x34, 0x12}
	value, errMsg := introspectionBytesToValues(data, 4)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if value != uint32(0x12345678) {
		t.Errorf("expected 0x12345678, got %v", value)
	}
}
func TestApplyMask(t *testing.T) {
	value := uint8(0b10110110)
	mask := 0b00000110
	result, errMsg := applyMask(value, mask)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	expected := uint8(0b00000110)
	if result != expected {
		t.Errorf("expected %08b, got %08b", expected, result)
	}
}
func TestApplyMaskZero(t *testing.T) {
	value := uint16(1234)
	result, errMsg := applyMask(value, 0)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result != value {
		t.Errorf("expected %d, got %v", value, result)
	}
}
func TestEvaluateObjectiveSuccess(t *testing.T) {
	scenario := &Scenario{
		ObjectiveStates: map[string]*ObjectiveState{
			"obj1": {
				Status: ObjectiveStatusActive,
			},
		},
	}
	objective := Objective{
		ID: "obj1",
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: 2,
		},
	}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": 2,
		},
	}
	errMsg := scenario.evaluateObjective(ctx, objective)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	state := scenario.ObjectiveStates["obj1"]
	if state.Status != ObjectiveStatusComplete {
		t.Errorf(
			"expected objective status %q, got %q",
			ObjectiveStatusComplete,
			state.Status,
		)
	}
}
func TestEvaluateObjectiveFailure(t *testing.T) {
	scenario := &Scenario{
		ObjectiveStates: map[string]*ObjectiveState{
			"obj1": {
				Status: ObjectiveStatusActive,
			},
		},
	}
	failPredicate := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: 3,
	}
	objective := Objective{
		ID: "obj1",
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: 2,
		},
		Fail: &failPredicate,
	}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": 3,
		},
	}
	errMsg := scenario.evaluateObjective(ctx, objective)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	state := scenario.ObjectiveStates["obj1"]
	if state.Status != ObjectiveStatusFailed {
		t.Errorf(
			"expected objective status %q, got %q",
			ObjectiveStatusFailed,
			state.Status,
		)
	}
}
func TestEvaluateObjectiveStaysActive(t *testing.T) {
	scenario := &Scenario{
		ObjectiveStates: map[string]*ObjectiveState{
			"obj1": {
				Status: ObjectiveStatusActive,
			},
		},
	}
	failPredicate := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: 3,
	}
	objective := Objective{
		ID: "obj1",
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: 2,
		},
		Fail: &failPredicate,
	}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": 1,
		},
	}
	errMsg := scenario.evaluateObjective(ctx, objective)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	state := scenario.ObjectiveStates["obj1"]
	if state.Status != ObjectiveStatusActive {
		t.Errorf(
			"expected objective status %q, got %q",
			ObjectiveStatusActive,
			state.Status,
		)
	}
}
func TestEvaluateObjectivesUnlocksNextObjective(t *testing.T) {
	objective1 := Objective{
		ID: "obj1",
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: 2,
		},
	}
	objective2 := Objective{
		ID:       "obj2",
		Requires: []string{"obj1"},
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: 3,
		},
	}
	scenario := &Scenario{
		Objectives: []Objective{
			objective1,
			objective2,
		},
		ObjectiveStates: initializeObjectiveStates(
			[]Objective{objective1, objective2},
		),
	}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": 2,
		},
	}
	errMsg := scenario.evaluateObjectives(ctx)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if scenario.ObjectiveStates["obj1"].Status != ObjectiveStatusComplete {
		t.Errorf(
			"expected obj1 to be complete, got %q",
			scenario.ObjectiveStates["obj1"].Status,
		)
	}
	if scenario.ObjectiveStates["obj2"].Status != ObjectiveStatusActive {
		t.Errorf(
			"expected obj2 to be active, got %q",
			scenario.ObjectiveStates["obj2"].Status,
		)
	}
}
func TestEvaluateObjectiveRetractable(t *testing.T) {
	scenario := &Scenario{
		ObjectiveStates: map[string]*ObjectiveState{
			"obj1": {
				Status: ObjectiveStatusComplete,
			},
		},
	}
	objective := Objective{
		ID:          "obj1",
		Retractable: true,
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: 2,
		},
	}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": 1,
		},
	}
	errMsg := scenario.evaluateObjective(ctx, objective)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	state := scenario.ObjectiveStates["obj1"]
	if state.Status != ObjectiveStatusActive {
		t.Errorf(
			"expected retractable objective to become %q, got %q",
			ObjectiveStatusActive,
			state.Status,
		)
	}
}
func TestEvaluateObjectiveRetractableStaysComplete(t *testing.T) {
	scenario := &Scenario{
		ObjectiveStates: map[string]*ObjectiveState{
			"obj1": {
				Status: ObjectiveStatusComplete,
			},
		},
	}
	objective := Objective{
		ID:          "obj1",
		Retractable: true,
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: 2,
		},
	}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": 2,
		},
	}
	errMsg := scenario.evaluateObjective(ctx, objective)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if scenario.ObjectiveStates["obj1"].Status != ObjectiveStatusComplete {
		t.Errorf(
			"expected retractable objective to remain %q, got %q",
			ObjectiveStatusComplete,
			scenario.ObjectiveStates["obj1"].Status,
		)
	}
}
func TestEvaluateMemU8Predicate(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
			Fields: map[string]map[string]int{
				"g_config": {
					"mode": 4,
				},
			},
		},
	}
	reader := &fakeMemoryReader{
		data: []byte{2},
	}
	ctx := &EvaluationContext{
		Introspect: reader,
	}
	predicate := Predicate{
		Op: "mem_u8",
		At: &AddressRef{
			Sym:   "g_config",
			Field: "mode",
		},
		Cmp:   "eq",
		Value: 2,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected mem_u8 predicate to be true")
	}
	if reader.lastAddr != 0x20001004 {
		t.Errorf("expected read address 0x20001004, got 0x%x", reader.lastAddr)
	}
	if reader.lastLength != 1 {
		t.Errorf("expected read length 1, got %d", reader.lastLength)
	}
}
func TestEvaluateMemU16Predicate(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
		},
	}
	reader := &fakeMemoryReader{
		data: []byte{0x34, 0x12},
	}
	ctx := &EvaluationContext{
		Introspect: reader,
	}
	predicate := Predicate{
		Op: "mem_u16",
		At: &AddressRef{
			Sym: "g_config",
		},
		Cmp:   "eq",
		Value: 0x1234,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected mem_u16 predicate to be true")
	}
	if reader.lastAddr != 0x20001000 {
		t.Errorf("expected read address 0x20001000, got 0x%x", reader.lastAddr)
	}
	if reader.lastLength != 2 {
		t.Errorf("expected read length 2, got %d", reader.lastLength)
	}
}
func TestEvaluateMemU32Predicate(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
		},
	}
	reader := &fakeMemoryReader{
		data: []byte{0x78, 0x56, 0x34, 0x12},
	}
	ctx := &EvaluationContext{
		Introspect: reader,
	}
	predicate := Predicate{
		Op: "mem_u32",
		At: &AddressRef{
			Sym: "g_config",
		},
		Cmp:   "eq",
		Value: 0x12345678,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected mem_u32 predicate to be true")
	}
	if reader.lastAddr != 0x20001000 {
		t.Errorf("expected read address 0x20001000, got 0x%x", reader.lastAddr)
	}
	if reader.lastLength != 4 {
		t.Errorf("expected read length 4, got %d", reader.lastLength)
	}
}
func TestEvaluateMemBitsPredicate(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"flags": "0x20001000",
			},
		},
	}
	reader := &fakeMemoryReader{
		data: []byte{0xAD},
	}
	ctx := &EvaluationContext{
		Introspect: reader,
	}
	predicate := Predicate{
		Op: "mem_bits",
		At: &AddressRef{
			Sym: "flags",
		},
		Width: 1,
		Mask:  0x0F,
		Cmp:   "eq",
		Value: 0x0D,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected mem_bits predicate to be true")
	}
	if reader.lastAddr != 0x20001000 {
		t.Errorf("expected read address 0x20001000, got 0x%x", reader.lastAddr)
	}
	if reader.lastLength != 1 {
		t.Errorf("expected read length 1, got %d", reader.lastLength)
	}
}
func TestEvaluateMemBitsPredicateDefaultWidth(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"flags": "0x20001000",
			},
		},
	}
	reader := &fakeMemoryReader{
		data: []byte{0x78, 0x56, 0x34, 0x12},
	}
	ctx := &EvaluationContext{
		Introspect: reader,
	}
	predicate := Predicate{
		Op: "mem_bits",
		At: &AddressRef{
			Sym: "flags",
		},
		Mask:  0xFF,
		Cmp:   "eq",
		Value: 0x78,
		// Width intentionally omitted.
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected mem_bits predicate to be true")
	}
	if reader.lastLength != 4 {
		t.Errorf("expected default read length 4, got %d", reader.lastLength)
	}
}
func TestEvaluateMemPredicate(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"config": "0x20001000",
			},
		},
	}
	reader := &fakeMemoryReader{
		data: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	ctx := &EvaluationContext{
		Introspect: reader,
	}
	predicate := Predicate{
		Op: "mem",
		At: &AddressRef{
			Sym: "config",
		},
		Len:   4,
		Value: "deadbeef",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected mem predicate to be true")
	}
	if reader.lastAddr != 0x20001000 {
		t.Errorf("expected read address 0x20001000, got 0x%x", reader.lastAddr)
	}
	if reader.lastLength != 4 {
		t.Errorf("expected read length 4, got %d", reader.lastLength)
	}
}
func TestEvaluateMemPredicateFalse(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"config": "0x20001000",
			},
		},
	}
	reader := &fakeMemoryReader{
		data: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	ctx := &EvaluationContext{
		Introspect: reader,
	}
	predicate := Predicate{
		Op: "mem",
		At: &AddressRef{
			Sym: "config",
		},
		Len:   4,
		Value: "deadbe00",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected mem predicate to be false")
	}
}
func TestEvaluateMemChangedPredicate(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"config": "0x20001000",
			},
		},
	}
	current := &fakeMemoryReader{
		data: []byte{0xDE, 0xAD, 0xBE, 0x00},
	}
	baseline := &fakeMemoryReader{
		data: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	ctx := &EvaluationContext{
		Introspect: current,
		Baseline:   baseline,
	}
	predicate := Predicate{
		Op: "mem_changed",
		At: &AddressRef{
			Sym: "config",
		},
		Len: 4,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected mem_changed predicate to be true")
	}
}
func TestEvaluateMemChangedPredicateFalse(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"config": "0x20001000",
			},
		},
	}
	current := &fakeMemoryReader{
		data: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	baseline := &fakeMemoryReader{
		data: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}
	ctx := &EvaluationContext{
		Introspect: current,
		Baseline:   baseline,
	}
	predicate := Predicate{
		Op: "mem_changed",
		At: &AddressRef{
			Sym: "config",
		},
		Len: 4,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected mem_changed predicate to be false")
	}
}
func TestEvaluateTelemetryPredicateNestedPath(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"channels": map[string]any{
				"COMMS": map[string]any{
					"antenna": 1,
				},
			},
		},
	}
	predicate := Predicate{
		Op:    "tlm",
		Path:  "channels.COMMS.antenna",
		Cmp:   "eq",
		Value: 1,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected nested telemetry predicate to be true")
	}
}
func TestEvaluateTelemetryBitsPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"channels": map[string]any{
				"COMMS": map[string]any{
					"xstat": uint8(0xAD),
				},
			},
		},
	}
	predicate := Predicate{
		Op:    "tlm_bits",
		Path:  "channels.COMMS.xstat",
		Mask:  0x0F,
		Cmp:   "eq",
		Value: 0x0D,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected tlm_bits predicate to be true")
	}
}
func TestEvaluateTelemetryBitsPredicateUnknownPath(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"channels": map[string]any{
				"COMMS": map[string]any{
					"xstat": uint8(0xAD),
				},
			},
		},
	}
	predicate := Predicate{
		Op:    "tlm_bits",
		Path:  "channels.COMMS.missing",
		Mask:  0x0F,
		Cmp:   "eq",
		Value: 0x0D,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected tlm_bits predicate to be false for missing telemetry path")
	}
}
func TestEvaluateChannelPresentPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"channels": map[string]any{
				"COMMS": map[string]any{
					"antenna": 1,
				},
			},
		},
	}
	predicate := Predicate{
		Op: "channel_present",
		ID: "COMMS",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected COMMS channel to be present")
	}
}
func TestEvaluateChannelAbsentPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"channels": map[string]any{
				"COMMS": map[string]any{
					"antenna": 1,
				},
			},
		},
	}
	predicate := Predicate{
		Op: "channel_absent",
		ID: "RAD",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected RAD channel to be absent")
	}
}
func TestEvaluateEventPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Events: []string{
			"MODE_CHANGE:NOMINAL",
			"HGA_DEPLOYED",
		},
	}
	predicate := Predicate{
		Op:    "event",
		Match: "HGA_DEPLOYED",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected event predicate to be true")
	}
}
func TestEvaluateEventPredicateFalse(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Events: []string{
			"MODE_CHANGE:NOMINAL",
		},
	}
	predicate := Predicate{
		Op:    "event",
		Match: "HGA_DEPLOYED",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected event predicate to be false")
	}
}
func TestEvaluateEventPredicateRegex(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Events: []string{
			"MODE_CHANGE:NOMINAL",
			"HGA_DEPLOYED",
		},
	}
	predicate := Predicate{
		Op:    "event",
		Match: "^MODE_CHANGE:",
		Regex: true,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected regex event predicate to be true")
	}
}
func TestEvaluateEventPredicateInvalidRegex(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Events: []string{
			"MODE_CHANGE:NOMINAL",
		},
	}
	predicate := Predicate{
		Op:    "event",
		Match: "[",
		Regex: true,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg == "" {
		t.Fatal("expected invalid regex to return an error")
	}
	if result {
		t.Error("expected invalid regex predicate to be false")
	}
}
func TestEvaluateEverPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		History: []map[string]any{
			{
				"mode": "NOMINAL",
			},
			{
				"mode": "SAFE",
			},
			{
				"mode": "NOMINAL",
			},
		},
	}
	child := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "SAFE",
	}
	childJSON, err := json.Marshal(child)
	if err != nil {
		t.Fatalf("failed to marshal child predicate: %v", err)
	}
	predicate := Predicate{
		Op: "ever",
		Of: childJSON,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected ever predicate to be true")
	}
}
func TestEvaluateEverPredicateFalse(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		History: []map[string]any{
			{
				"mode": "NOMINAL",
			},
			{
				"mode": "BOOT",
			},
			{
				"mode": "NOMINAL",
			},
		},
	}
	child := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "SAFE",
	}
	childJSON, err := json.Marshal(child)
	if err != nil {
		t.Fatalf("failed to marshal child predicate: %v", err)
	}
	predicate := Predicate{
		Op: "ever",
		Of: childJSON,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected ever predicate to be false")
	}
}
func TestEvaluateEverPredicateCurrentFrame(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "SAFE",
		},
		History: []map[string]any{
			{"mode": "NOMINAL"},
			{"mode": "NOMINAL"},
		},
	}
	child := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "SAFE",
	}
	childJSON, err := json.Marshal(child)
	if err != nil {
		t.Fatalf("failed to marshal child predicate: %v", err)
	}
	predicate := Predicate{
		Op: "ever",
		Of: childJSON,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected ever predicate to be true for current frame")
	}
}
func TestEvaluateSustainedPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "NOMINAL",
		},
		History: []map[string]any{
			{"mode": "SAFE"},
			{"mode": "NOMINAL"},
			{"mode": "NOMINAL"},
		},
	}
	child := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "NOMINAL",
	}
	childJSON, err := json.Marshal(child)
	if err != nil {
		t.Fatalf("failed to marshal child predicate: %v", err)
	}
	predicate := Predicate{
		Op:     "sustained",
		Frames: 3,
		Of:     childJSON,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected sustained predicate to be true")
	}
}
func TestEvaluateSustainedPredicateFalse(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "NOMINAL",
		},
		History: []map[string]any{
			{"mode": "NOMINAL"},
			{"mode": "SAFE"},
			{"mode": "NOMINAL"},
		},
	}
	child := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "NOMINAL",
	}
	childJSON, err := json.Marshal(child)
	if err != nil {
		t.Fatalf("failed to marshal child predicate: %v", err)
	}
	predicate := Predicate{
		Op:     "sustained",
		Frames: 3,
		Of:     childJSON,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected sustained predicate to be false")
	}
}
func TestEvaluateWithinPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "NOMINAL",
		},
		History: []map[string]any{
			{"mode": "NOMINAL"},
			{"mode": "SAFE"},
			{"mode": "NOMINAL"},
		},
	}
	child := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "SAFE",
	}
	childJSON, err := json.Marshal(child)
	if err != nil {
		t.Fatalf("failed to marshal child predicate: %v", err)
	}
	predicate := Predicate{
		Op:     "within",
		Frames: 3,
		Of:     childJSON,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected within predicate to be true")
	}
}
func TestEvaluateWithinPredicateFalse(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "NOMINAL",
		},
		History: []map[string]any{
			{"mode": "SAFE"},
			{"mode": "NOMINAL"},
			{"mode": "NOMINAL"},
		},
	}
	child := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "SAFE",
	}
	childJSON, err := json.Marshal(child)
	if err != nil {
		t.Fatalf("failed to marshal child predicate: %v", err)
	}
	predicate := Predicate{
		Op:     "within",
		Frames: 3,
		Of:     childJSON,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected within predicate to be false")
	}
}
func TestEvaluateCommandedPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Log: []CommandLogEntry{
			{
				Verb:   "PEEK",
				Result: "ACK",
			},
			{
				Verb:   "POKE",
				Result: "ACK",
			},
		},
	}
	predicate := Predicate{
		Op:   "commanded",
		Verb: "POKE",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected commanded predicate to be true")
	}
}
func TestEvaluateCommandedPredicateFalse(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Log: []CommandLogEntry{
			{
				Verb:   "PEEK",
				Result: "ACK",
			},
		},
	}
	predicate := Predicate{
		Op:   "commanded",
		Verb: "POKE",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected commanded predicate to be false")
	}
}
func TestEvaluateCommandedPredicateWithResult(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Log: []CommandLogEntry{
			{
				Verb:   "POKE",
				Result: "NAK E04",
			},
			{
				Verb:   "POKE",
				Result: "ACK write accepted",
			},
		},
	}
	predicate := Predicate{
		Op:     "commanded",
		Verb:   "POKE",
		Result: "ACK",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected commanded predicate to match ACK result prefix")
	}
}
func TestEvaluateCommandedPredicateResultFalse(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Log: []CommandLogEntry{
			{
				Verb:   "POKE",
				Result: "NAK E04",
			},
		},
	}
	predicate := Predicate{
		Op:     "commanded",
		Verb:   "POKE",
		Result: "ACK",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected commanded predicate to be false for rejected command")
	}
}
func TestEvaluateCommandedPredicateWithAddress(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
			Fields: map[string]map[string]int{
				"g_config": {
					"mode": 4,
				},
			},
		},
	}
	ctx := &EvaluationContext{
		Log: []CommandLogEntry{
			{
				Verb:   "POKE",
				Addr:   0x20001004,
				Result: "ACK",
			},
		},
	}
	predicate := Predicate{
		Op:   "commanded",
		Verb: "POKE",
		At: &AddressRef{
			Sym:   "g_config",
			Field: "mode",
		},
		Result: "ACK",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected commanded predicate to match resolved address")
	}
}
func TestEvaluateCommandedPredicateAddressFalse(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
			Fields: map[string]map[string]int{
				"g_config": {
					"mode": 4,
				},
			},
		},
	}
	ctx := &EvaluationContext{
		Log: []CommandLogEntry{
			{
				Verb:   "POKE",
				Addr:   0x20002000,
				Result: "ACK",
			},
		},
	}
	predicate := Predicate{
		Op: "commanded",
		At: &AddressRef{
			Sym:   "g_config",
			Field: "mode",
		},
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected commanded predicate to be false for wrong address")
	}
}
func TestEvaluateBudgetPredicate(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Budget: map[string]int{
			"writes": 32,
			"reads":  150,
		},
	}
	predicate := Predicate{
		Op:       "budget",
		Resource: "writes",
		Cmp:      "lte",
		Value:    40,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected budget predicate to be true")
	}
}
func TestEvaluateBudgetPredicateFalse(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Budget: map[string]int{
			"writes": 47,
		},
	}
	predicate := Predicate{
		Op:       "budget",
		Resource: "writes",
		Cmp:      "lte",
		Value:    40,
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if result {
		t.Error("expected budget predicate to be false")
	}
}
func TestEvaluatePartialFirstMatch(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "SAFE",
		},
	}
	objective := Objective{
		Partial: []Partial{
			{
				When: Predicate{
					Op:    "tlm",
					Path:  "mode",
					Cmp:   "eq",
					Value: "NOMINAL",
				},
				Text: "First partial",
			},
			{
				When: Predicate{
					Op:    "tlm",
					Path:  "mode",
					Cmp:   "eq",
					Value: "SAFE",
				},
				Text: "Second partial",
			},
			{
				When: Predicate{
					Op:    "tlm",
					Path:  "mode",
					Cmp:   "eq",
					Value: "SAFE",
				},
				Text: "Third partial",
			},
		},
	}
	diagnostic, errMsg := scenario.evaluatePartial(ctx, objective)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if diagnostic != "Second partial" {
		t.Errorf("expected %q, got %q", "Second partial", diagnostic)
	}
}
func TestEvaluateObjectivePartial(t *testing.T) {
	scenario := &Scenario{
		ObjectiveStates: map[string]*ObjectiveState{
			"test": {
				Status: ObjectiveStatusActive,
			},
		},
	}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "SAFE",
		},
	}
	objective := Objective{
		ID: "test",
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: "NOMINAL",
		},
		Partial: []Partial{
			{
				When: Predicate{
					Op:    "tlm",
					Path:  "mode",
					Cmp:   "eq",
					Value: "SAFE",
				},
				Text: "Probe is still in safe mode.",
			},
		},
	}
	errMsg := scenario.evaluateObjective(ctx, objective)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	state := scenario.ObjectiveStates["test"]
	if state.Status != ObjectiveStatusActive {
		t.Errorf("expected objective to remain active, got %v", state.Status)
	}
	if state.Diagnostic != "Probe is still in safe mode." {
		t.Errorf(
			"expected partial diagnostic %q, got %q",
			"Probe is still in safe mode.",
			state.Diagnostic,
		)
	}
}
func TestEvaluateObjectiveFailTakesPrecedence(t *testing.T) {
	scenario := &Scenario{
		ObjectiveStates: map[string]*ObjectiveState{
			"test": {
				Status: ObjectiveStatusActive,
			},
		},
	}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "SAFE",
		},
	}
	failPredicate := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "SAFE",
	}
	objective := Objective{
		ID:   "test",
		Fail: &failPredicate,
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: "SAFE",
		},
		Partial: []Partial{
			{
				When: Predicate{
					Op:    "tlm",
					Path:  "mode",
					Cmp:   "eq",
					Value: "SAFE",
				},
				Text: "This partial should not be used.",
			},
		},
	}
	errMsg := scenario.evaluateObjective(ctx, objective)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	state := scenario.ObjectiveStates["test"]
	if state.Status != ObjectiveStatusFailed {
		t.Errorf("expected objective to fail, got %v", state.Status)
	}
	if state.Diagnostic != "" {
		t.Errorf("expected no diagnostic after failure, got %q", state.Diagnostic)
	}
}
func TestCopyObjectiveStates(t *testing.T) {
	original := map[string]*ObjectiveState{
		"objective-1": {
			Status:     ObjectiveStatusActive,
			Diagnostic: "original",
		},
	}
	copied := copyObjectiveStates(original)
	copied["objective-1"].Status = ObjectiveStatusComplete
	copied["objective-1"].Diagnostic = "changed"
	if original["objective-1"].Status != ObjectiveStatusActive {
		t.Error("changing copied status modified original state")
	}
	if original["objective-1"].Diagnostic != "original" {
		t.Error("changing copied diagnostic modified original state")
	}
}
func TestEvaluateObjectivesRollsBackOnMemoryError(t *testing.T) {
	objective1 := Objective{
		ID: "obj1",
		Success: Predicate{
			Op:    "tlm",
			Path:  "mode",
			Cmp:   "eq",
			Value: "NOMINAL",
		},
	}
	objective2 := Objective{
		ID: "obj2",
		Success: Predicate{
			Op: "mem_u8",
			At: &AddressRef{
				Sym: "g_config",
			},
			Cmp:   "eq",
			Value: 1,
		},
	}
	scenario := &Scenario{
		Objectives: []Objective{
			objective1,
			objective2,
		},
		ObjectiveStates: map[string]*ObjectiveState{
			"obj1": {
				Status: ObjectiveStatusActive,
			},
			"obj2": {
				Status: ObjectiveStatusActive,
			},
		},
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
		},
	}
	ctx := &EvaluationContext{
		Telemetry: map[string]any{
			"mode": "NOMINAL",
		},
		Introspect: &failingMemoryReader{},
	}
	errMsg := scenario.evaluateObjectives(ctx)
	if errMsg == "" {
		t.Fatal("expected memory read error")
	}
	if scenario.ObjectiveStates["obj1"].Status != ObjectiveStatusActive {
		t.Errorf(
			"expected obj1 to roll back to %q, got %q",
			ObjectiveStatusActive,
			scenario.ObjectiveStates["obj1"].Status,
		)
	}
	if scenario.ObjectiveStates["obj2"].Status != ObjectiveStatusActive {
		t.Errorf(
			"expected obj2 to remain %q, got %q",
			ObjectiveStatusActive,
			scenario.ObjectiveStates["obj2"].Status,
		)
	}
}
func TestEvaluateEventPredicateSubstring(t *testing.T) {
	scenario := &Scenario{}
	ctx := &EvaluationContext{
		Events: []string{
			"ANTENNA HGA -> LGA",
		},
	}
	predicate := Predicate{
		Op:    "event",
		Match: "HGA -> LGA",
	}
	result, errMsg := scenario.evaluatePredicate(ctx, predicate)
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !result {
		t.Error("expected event predicate to match substring")
	}
}
func TestValidateObjectiveIDsDuplicate(t *testing.T) {
	objectives := []Objective{
		{
			ID: "objective-1",
		},
		{
			ID: "objective-2",
		},
		{
			ID: "objective-1",
		},
	}
	err := validateObjectiveIDs(objectives)
	if err == nil {
		t.Fatal("expected duplicate objective ID to return an error")
	}
}
func TestValidateObjectiveIDsUnique(t *testing.T) {
	objectives := []Objective{
		{
			ID: "objective-1",
		},
		{
			ID: "objective-2",
		},
	}
	err := validateObjectiveIDs(objectives)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidateObjectiveRequirementsValid(t *testing.T) {
	objectives := []Objective{
		{
			ID: "objective-1",
		},
		{
			ID:       "objective-2",
			Requires: []string{"objective-1"},
		},
	}
	err := validateObjectiveRequirements(objectives)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidateObjectiveRequirementsUnknown(t *testing.T) {
	objectives := []Objective{
		{
			ID: "objective-1",
		},
		{
			ID:       "objective-2",
			Requires: []string{"does-not-exist"},
		},
	}
	err := validateObjectiveRequirements(objectives)
	if err == nil {
		t.Fatal("expected unknown required objective to return an error")
	}
}
func TestValidateObjectiveCyclesCycle(t *testing.T) {
	objectives := []Objective{
		{
			ID:       "objective-1",
			Requires: []string{"objective-2"},
		},
		{
			ID:       "objective-2",
			Requires: []string{"objective-1"},
		},
	}
	err := validateObjectiveCycles(objectives)
	if err == nil {
		t.Fatal("expected dependency cycle to return an error")
	}
}
func TestValidateObjectiveCyclesValid(t *testing.T) {
	objectives := []Objective{
		{
			ID: "objective-1",
		},
		{
			ID:       "objective-2",
			Requires: []string{"objective-1"},
		},
		{
			ID:       "objective-3",
			Requires: []string{"objective-2"},
		},
	}
	err := validateObjectiveCycles(objectives)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidatePredicateKnownOperation(t *testing.T) {
	predicate := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "NOMINAL",
	}
	err := validatePredicate(predicate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidatePredicateUnknownOperation(t *testing.T) {
	predicate := Predicate{
		Op: "definitely_not_real",
	}
	err := validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected unknown predicate operation to return an error")
	}
}
func TestValidatePredicateNestedUnknownOperation(t *testing.T) {
	children := []Predicate{
		{
			Op: "tlm",
		},
		{
			Op: "not_real",
		},
	}
	of, err := json.Marshal(children)
	if err != nil {
		t.Fatalf("failed to marshal predicates: %v", err)
	}
	predicate := Predicate{
		Op: "all",
		Of: of,
	}
	err = validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected nested unknown predicate operation to return an error")
	}
}
func TestValidatePredicateNestedValid(t *testing.T) {
	child := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "NOMINAL",
	}
	of, err := json.Marshal(child)
	if err != nil {
		t.Fatalf("failed to marshal predicate: %v", err)
	}
	predicate := Predicate{
		Op: "not",
		Of: of,
	}
	err = validatePredicate(predicate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidatePredicateTlmMissingPath(t *testing.T) {
	predicate := Predicate{
		Op:    "tlm",
		Cmp:   "eq",
		Value: 1,
	}
	err := validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected tlm predicate missing path to return an error")
	}
}
func TestValidatePredicateTlmValid(t *testing.T) {
	predicate := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "NOMINAL",
	}
	err := validatePredicate(predicate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidatePredicateMemU8MissingAt(t *testing.T) {
	predicate := Predicate{
		Op:    "mem_u8",
		Cmp:   "eq",
		Value: 1,
	}
	err := validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected mem_u8 predicate missing at to return an error")
	}
}
func TestValidatePredicateMemU8Valid(t *testing.T) {
	predicate := Predicate{
		Op: "mem_u8",
		At: &AddressRef{
			Sym: "g_config",
		},
		Cmp:   "eq",
		Value: 1,
	}
	err := validatePredicate(predicate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidatePredicateMemBitsInvalidWidth(t *testing.T) {
	predicate := Predicate{
		Op: "mem_bits",
		At: &AddressRef{
			Sym: "g_config",
		},
		Width: 3,
		Cmp:   "eq",
		Value: 1,
	}
	err := validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected invalid mem_bits width to return an error")
	}
}
func TestValidatePredicateBudgetMissingResource(t *testing.T) {
	predicate := Predicate{
		Op:    "budget",
		Cmp:   "lte",
		Value: 40,
	}
	err := validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected budget predicate missing resource to return an error")
	}
}
func TestValidatePredicateSustainedMissingFrames(t *testing.T) {
	child := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "eq",
		Value: "NOMINAL",
	}
	of, err := json.Marshal(child)
	if err != nil {
		t.Fatalf("failed to marshal child predicate: %v", err)
	}
	predicate := Predicate{
		Op: "sustained",
		Of: of,
	}
	err = validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected sustained predicate missing frames to return an error")
	}
}
func TestValidateObjectivePredicatesInvalidSuccess(t *testing.T) {
	objectives := []Objective{
		{
			ID: "objective-1",
			Success: Predicate{
				Op: "not_a_real_operation",
			},
		},
	}
	err := validateObjectivePredicates(objectives)
	if err == nil {
		t.Fatal("expected invalid success predicate to return an error")
	}
}
func TestValidateObjectivePredicatesValid(t *testing.T) {
	objectives := []Objective{
		{
			ID: "objective-1",
			Success: Predicate{
				Op:    "tlm",
				Path:  "mode",
				Cmp:   "eq",
				Value: "NOMINAL",
			},
		},
	}
	err := validateObjectivePredicates(objectives)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidatePredicateInvalidComparison(t *testing.T) {
	predicate := Predicate{
		Op:    "tlm",
		Path:  "mode",
		Cmp:   "banana",
		Value: "NOMINAL",
	}
	err := validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected invalid comparison operator to return an error")
	}
}
func TestValidatePredicateMissingOf(t *testing.T) {
	predicate := Predicate{
		Op: "all",
	}
	err := validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected all predicate missing of to return an error")
	}
}
func TestValidatePredicateMemValid(t *testing.T) {
	predicate := Predicate{
		Op: "mem",
		At: &AddressRef{
			Sym: "g_config",
		},
		Len:   4,
		Cmp:   "eq",
		Value: "01020304",
	}
	err := validatePredicate(predicate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidatePredicateMemLengthMismatch(t *testing.T) {
	predicate := Predicate{
		Op: "mem",
		At: &AddressRef{
			Sym: "g_config",
		},
		Len:   4,
		Value: "0102",
	}
	err := validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected mem length mismatch to return an error")
	}
}
func TestValidateMemoryRangeValid(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x20001000",
			},
		},
		MemMap: MemMapFile{
			Regions: []MemoryRegion{
				{
					Name: "RAM",
					Lo:   "0x20000000",
					Hi:   "0x2000FFFF",
				},
			},
		},
	}
	err := scenario.validateMemoryRange(
		AddressRef{Sym: "g_config"},
		4,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestValidateMemoryRangeOutsideMap(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{
				"g_config": "0x2000FFFE",
			},
		},
		MemMap: MemMapFile{
			Regions: []MemoryRegion{
				{
					Name: "RAM",
					Lo:   "0x20000000",
					Hi:   "0x2000FFFF",
				},
			},
		},
	}
	err := scenario.validateMemoryRange(
		AddressRef{Sym: "g_config"},
		4,
	)
	if err == nil {
		t.Fatal("expected memory range outside map to return an error")
	}
}
func TestValidateMemoryRangeUnknownSymbol(t *testing.T) {
	scenario := &Scenario{
		Symbols: SymbolsFile{
			Symbols: map[string]string{},
		},
		MemMap: MemMapFile{
			Regions: []MemoryRegion{
				{
					Name: "RAM",
					Lo:   "0x20000000",
					Hi:   "0x2000FFFF",
				},
			},
		},
	}
	err := scenario.validateMemoryRange(
		AddressRef{Sym: "does_not_exist"},
		4,
	)
	if err == nil {
		t.Fatal("expected unknown symbol to return an error")
	}
}
func TestValidatePredicateEventInvalidRegex(t *testing.T) {
	predicate := Predicate{
		Op:    "event",
		Match: "[invalid",
		Regex: true,
	}
	err := validatePredicate(predicate)
	if err == nil {
		t.Fatal("expected invalid event regex to return an error")
	}
}
