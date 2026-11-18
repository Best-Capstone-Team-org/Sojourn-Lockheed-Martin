package game

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

// editJSON rewrites one JSON file of a test package in place.
func editJSON(t *testing.T, files map[string]string, name string, edit func(map[string]any)) {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(files[name]), &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	files[name] = string(data)
}

// cleanScenarioFiles is a package with no validation warnings.
func cleanScenarioFiles(t *testing.T, id string) map[string]string {
	t.Helper()
	files := scenarioFiles(id)
	editJSON(t, files, "objectives.json", func(doc map[string]any) {
		for _, o := range doc["objectives"].([]any) {
			o.(map[string]any)["hints"] = []any{map[string]any{"after_frames": 3.0, "text": "Look harder."}}
		}
	})
	return files
}

func TestValidationWarnings(t *testing.T) {
	setup := &Setup{Writes: []SetupWrite{
		{At: AddressRef{Sym: "g_mode"}, Note: "documented"},
		{At: AddressRef{Sym: "g_mode"}},
		{At: AddressRef{Sym: "g_mode"}, Note: "  "},
	}}
	objectives := []Objective{
		{ID: "hinted", Hints: []Hint{{AfterFrames: 1, Text: "x"}}},
		{ID: "bare"},
	}
	manifest := []byte(`{
		"format": 1, "id": "x", "title": "x", "revision": 1, "summary": "x",
		"briefing": "b.md", "objectives": "o.json", "difficulty": "easy", "docs": [],
		"firmware": {"rom": "r", "symbols": "s", "memmap": "m", "app_crc32": "0x0", "signed_by": "nobody"},
		"link": {"uplink_delay_s": 0, "downlink_delay_s": 0, "budget": {"writes": 1, "bribes": 3}},
		"console": {"decode": [], "theme": "dark"},
		"zz_custom": true
	}`)

	got := validationWarnings(manifest, Manifest{Impure: true}, objectives, setup)
	want := []string{
		`manifest has unknown key "console.theme"`,
		`manifest has unknown key "firmware.signed_by"`,
		`manifest has unknown key "link.budget.bribes"`,
		`manifest has unknown key "zz_custom"`,
		`objective "bare" has no hints`,
		`setup write 1 has no note`,
		`setup write 2 has no note`,
		`package is declared impure`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("warnings =\n%q\nwant\n%q", got, want)
	}

	if got := validationWarnings([]byte(`{"format": 1, "id": "x"}`), Manifest{}, objectives[:1], nil); len(got) != 0 {
		t.Errorf("clean package warned: %q", got)
	}
}

func TestLoadScenarioRecordsWarnings(t *testing.T) {
	dir := t.TempDir()
	files := cleanScenarioFiles(t, "clean")
	writeScenario(t, dir, files)

	scenario, err := LoadScenario(dir)
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	if len(scenario.Warnings) != 0 {
		t.Errorf("clean package warned: %q", scenario.Warnings)
	}

	dir = t.TempDir()
	editJSON(t, files, "manifest.json", func(doc map[string]any) { doc["mystery"] = 1.0 })
	writeScenario(t, dir, files)
	if scenario, err = LoadScenario(dir); err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	if want := []string{`manifest has unknown key "mystery"`}; !slices.Equal(scenario.Warnings, want) {
		t.Errorf("warnings = %q, want %q", scenario.Warnings, want)
	}
}

// Warnings never fail a package unless the daemon runs in strict mode.
func TestStrictValidation(t *testing.T) {
	root := t.TempDir()
	writeScenario(t, filepath.Join(root, "clean"), cleanScenarioFiles(t, "clean"))

	unknownKey := cleanScenarioFiles(t, "unknown-key")
	editJSON(t, unknownKey, "manifest.json", func(doc map[string]any) { doc["mystery"] = 1.0 })
	writeScenario(t, filepath.Join(root, "unknown-key"), unknownKey)

	writeScenario(t, filepath.Join(root, "no-hints"), scenarioFiles("no-hints"))

	for _, tc := range []struct {
		strict bool
		want   []string
	}{
		{false, []string{"sojourn.test.clean", "sojourn.test.no-hints", "sojourn.test.unknown-key"}},
		{true, []string{"sojourn.test.clean"}},
	} {
		state := InitState()
		state.StrictValidation = tc.strict
		if err := state.LoadScenarios([]ScenariosDir{{Dir: root, Required: true}}); err != nil {
			t.Fatalf("LoadScenarios: %v", err)
		}
		got := slices.Sorted(func(yield func(string) bool) {
			for id := range state.AvailableScenarios {
				if !yield(id) {
					return
				}
			}
		})
		if !slices.Equal(got, tc.want) {
			t.Errorf("strict=%v: loaded %v, want %v", tc.strict, got, tc.want)
		}
	}
}
