package game

import (
	"os"
	"path/filepath"
	"testing"
)

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

	// Write all required JSON files.
	files := map[string]string{
		"manifest.json":         manifest,
		"objectives.json":       objectives,
		"firmware/symbols.json": symbols,
		"firmware/memmap.json":  memmap,
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
		t.Errorf("expected scenario ID %q, got %q",
			"sojourn.test.scenario",
			scenario.Manifest.ID)
	}

	if scenario.Manifest.Title != "Test Scenario" {
		t.Errorf("expected title %q, got %q",
			"Test Scenario",
			scenario.Manifest.Title)
	}

	if len(scenario.Objectives) != 1 {
		t.Fatalf("expected 1 objective, got %d", len(scenario.Objectives))
	}

	if scenario.Objectives[0].ID != "test-objective" {
		t.Errorf("expected objective ID %q, got %q",
			"test-objective",
			scenario.Objectives[0].ID)
	}

	if scenario.Symbols.Symbols["g_config"] != "0x20001000" {
		t.Errorf("g_config symbol was not loaded correctly")
	}

	if len(scenario.MemMap.Regions) != 1 {
		t.Errorf("expected 1 memory region, got %d",
			len(scenario.MemMap.Regions))
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
		"regions": []
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

	files := map[string]string{
		"manifest.json":         manifest,
		"objectives.json":       objectives,
		"setup.json":            setup,
		"firmware/symbols.json": symbols,
		"firmware/memmap.json":  memmap,
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