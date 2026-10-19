package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func of(t *testing.T, p any) json.RawMessage { return rawPredicate(t, p) }

func TestValidateComparisonValue(t *testing.T) {
	tests := []struct {
		name    string
		cmp     string
		value   any
		numeric bool
		ok      bool
	}{
		{"unknown cmp", "approx", 1.0, false, false},
		{"nil value", "eq", nil, false, false},
		{"in array", "in", []any{"a", "b"}, false, true},
		{"in scalar", "in", 1.0, false, false},
		{"in numeric", "in", []any{1.0, 2.0}, true, true},
		{"in non-numeric", "in", []any{1.0, "x"}, true, false},
		{"array for scalar cmp", "eq", []any{1.0}, false, false},
		{"numeric scalar", "gt", 3.0, true, true},
		{"string for numeric", "eq", "x", true, false},
		{"string", "ne", "x", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateComparisonValue(tt.cmp, tt.value, tt.numeric)
			if (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestValidatePredicateTable(t *testing.T) {
	sym := &AddressRef{Sym: "g_mode"}
	badRef := &AddressRef{}
	tlm := Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"}

	tests := []struct {
		name string
		p    Predicate
		ok   bool
	}{
		{"all valid", Predicate{Op: "any", Of: of(t, []Predicate{tlm})}, true},
		{"all bad of", Predicate{Op: "all", Of: of(t, tlm)}, false},
		{"all bad child", Predicate{Op: "all", Of: of(t, []Predicate{{Op: "nope"}})}, false},
		{"not missing of", Predicate{Op: "not"}, false},
		{"not bad of", Predicate{Op: "not", Of: of(t, []Predicate{tlm})}, false},
		{"not bad child", Predicate{Op: "ever", Of: of(t, Predicate{Op: "tlm"})}, false},
		{"within no frames", Predicate{Op: "within", Of: of(t, tlm)}, false},
		{"within valid", Predicate{Op: "within", Frames: 2, Of: of(t, tlm)}, true},
		{"tlm bad cmp", Predicate{Op: "tlm", Path: "mode", Cmp: "x", Value: 1.0}, false},
		{"tlm_bits missing path", Predicate{Op: "tlm_bits"}, false},
		{"tlm_bits non-numeric", Predicate{Op: "tlm_bits", Path: "x", Cmp: "eq", Value: "a"}, false},
		{"tlm_bits valid", Predicate{Op: "tlm_bits", Path: "x", Cmp: "eq", Value: 1.0}, true},
		{"channel missing id", Predicate{Op: "channel_absent"}, false},
		{"channel valid", Predicate{Op: "channel_present", ID: "HK"}, true},
		{"event missing match", Predicate{Op: "event"}, false},
		{"event regex", Predicate{Op: "event", Match: "a+", Regex: true}, true},
		{"event plain", Predicate{Op: "event", Match: "(", Regex: false}, true},
		{"mem_u16 bad ref", Predicate{Op: "mem_u16", At: badRef, Cmp: "eq", Value: 1.0}, false},
		{"mem_u32 string value", Predicate{Op: "mem_u32", At: sym, Cmp: "eq", Value: "a"}, false},
		{"mem_bits missing at", Predicate{Op: "mem_bits"}, false},
		{"mem_bits bad ref", Predicate{Op: "mem_bits", At: badRef}, false},
		{"mem_bits bad value", Predicate{Op: "mem_bits", At: sym, Width: 2, Cmp: "eq"}, false},
		{"mem_bits valid", Predicate{Op: "mem_bits", At: sym, Width: 1, Cmp: "eq", Value: 1.0}, true},
		{"mem missing at", Predicate{Op: "mem"}, false},
		{"mem bad ref", Predicate{Op: "mem", At: badRef}, false},
		{"mem no len", Predicate{Op: "mem", At: sym, Cmp: "eq", Value: "00"}, false},
		{"mem bad cmp", Predicate{Op: "mem", At: sym, Len: 1, Cmp: "gt", Value: "00"}, false},
		{"mem no value", Predicate{Op: "mem", At: sym, Len: 1, Cmp: "eq"}, false},
		{"mem bad hex", Predicate{Op: "mem", At: sym, Len: 1, Cmp: "eq", Value: "zz"}, false},
		{"mem length mismatch", Predicate{Op: "mem", At: sym, Len: 2, Cmp: "eq", Value: "00"}, false},
		{"mem valid", Predicate{Op: "mem", At: sym, Len: 1, Cmp: "ne", Value: "00"}, true},
		{"mem_changed missing at", Predicate{Op: "mem_changed"}, false},
		{"mem_changed bad ref", Predicate{Op: "mem_changed", At: badRef}, false},
		{"mem_changed no len", Predicate{Op: "mem_changed", At: sym}, false},
		{"mem_changed valid", Predicate{Op: "mem_changed", At: sym, Len: 4}, true},
		{"commanded", Predicate{Op: "commanded"}, true},
		{"commanded at", Predicate{Op: "commanded", At: sym}, true},
		{"commanded bad ref", Predicate{Op: "commanded", At: badRef}, false},
		{"budget bad value", Predicate{Op: "budget", Resource: "writes", Cmp: "lt", Value: "x"}, false},
		{"budget valid", Predicate{Op: "budget", Resource: "writes", Cmp: "lt", Value: 3.0}, true},
		{"script lang", Predicate{Op: "script", Lang: "lua", Entry: "checks/a.py:f"}, false},
		{"script no entry", Predicate{Op: "script", Lang: "python"}, false},
		{"script no callable", Predicate{Op: "script", Lang: "python", Entry: "checks/a.py"}, false},
		{"script empty callable", Predicate{Op: "script", Lang: "python", Entry: "checks/a.py:"}, false},
		{"script dot", Predicate{Op: "script", Lang: "python", Entry: ".:f"}, false},
		{"script absolute", Predicate{Op: "script", Lang: "python", Entry: "/checks/a.py:f"}, false},
		{"script parent", Predicate{Op: "script", Lang: "python", Entry: "../a.py:f"}, false},
		{"script dotdot", Predicate{Op: "script", Lang: "python", Entry: "..:f"}, false},
		{"script outside checks", Predicate{Op: "script", Lang: "python", Entry: "lib/a.py:f"}, false},
		{"script valid", Predicate{Op: "script", Lang: "python", Entry: "checks/a.py:f"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePredicate(tt.p)
			if (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestValidateAddressRef(t *testing.T) {
	tests := []struct {
		ref AddressRef
		ok  bool
	}{
		{AddressRef{Addr: "0x1"}, true},
		{AddressRef{Addr: "0x1", Sym: "a"}, false},
		{AddressRef{Addr: "0x1", Offset: 2}, false},
		{AddressRef{}, false},
		{AddressRef{Sym: "a", Field: "f", Offset: 1}, false},
		{AddressRef{Sym: "a", Field: "f"}, true},
	}
	for _, tt := range tests {
		if err := validateAddressRef(tt.ref); (err == nil) != tt.ok {
			t.Errorf("validateAddressRef(%+v) = %v, want ok=%v", tt.ref, err, tt.ok)
		}
	}

	s := &Scenario{}
	if _, err := s.ResolveAddress(AddressRef{}); err == nil {
		t.Error("expected ResolveAddress to reject an empty reference")
	}
	s.Symbols.Symbols = map[string]string{"bad": "nope", "a": "0x10"}
	if _, err := s.ResolveAddress(AddressRef{Sym: "bad"}); err == nil {
		t.Error("expected ResolveAddress to reject an invalid symbol address")
	}
	if _, err := s.ResolveAddress(AddressRef{Sym: "a", Field: "f"}); err == nil {
		t.Error("expected ResolveAddress to reject a symbol without fields")
	}
}

func addressScenario(t *testing.T, regions ...MemoryRegion) *Scenario {
	t.Helper()
	if regions == nil {
		regions = []MemoryRegion{{Name: "RAM", Lo: "0x20000000", Hi: "0x2000FFFF"}}
	}
	return &Scenario{
		Dir: t.TempDir(),
		Symbols: SymbolsFile{
			Symbols: map[string]string{"g_mode": "0x20000010", "g_top": "0x2000FFFE"},
		},
		MemMap: MemMapFile{Regions: regions},
	}
}

func TestValidateMemoryRangeErrors(t *testing.T) {
	s := addressScenario(t)
	tests := []struct {
		name   string
		s      *Scenario
		ref    AddressRef
		length int
		ok     bool
	}{
		{"zero length", s, AddressRef{Sym: "g_mode"}, 0, false},
		{"unresolvable", s, AddressRef{}, 1, false},
		{"above 32 bits", s, AddressRef{Addr: "0x100000000"}, 1, false},
		{"overflow", s, AddressRef{Addr: "0xFFFFFFFF"}, 2, false},
		{"past region end", s, AddressRef{Sym: "g_top"}, 4, false},
		{"fits", s, AddressRef{Sym: "g_top"}, 2, true},
		{"bad lo", addressScenario(t, MemoryRegion{Name: "x", Lo: "lo", Hi: "0x1"}), AddressRef{Addr: "0x0"}, 1, false},
		{"bad hi", addressScenario(t, MemoryRegion{Name: "x", Lo: "0x0", Hi: "hi"}), AddressRef{Addr: "0x0"}, 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.s.validateMemoryRange(tt.ref, tt.length); (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestValidatePredicateAddresses(t *testing.T) {
	s := addressScenario(t)
	if err := os.MkdirAll(filepath.Join(s.Dir, "checks"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "checks", "a.py"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	in := &AddressRef{Sym: "g_mode"}
	out := &AddressRef{Addr: "0x10"}
	inside := Predicate{Op: "mem_u8", At: in}

	tests := []struct {
		name string
		p    Predicate
		ok   bool
	}{
		{"mem_u8", inside, true},
		{"mem_u8 outside", Predicate{Op: "mem_u8", At: out}, false},
		{"mem_u16", Predicate{Op: "mem_u16", At: in}, true},
		{"mem_u32 outside", Predicate{Op: "mem_u32", At: out}, false},
		{"mem_bits default width", Predicate{Op: "mem_bits", At: in}, true},
		{"mem_bits width", Predicate{Op: "mem_bits", At: out, Width: 1}, false},
		{"mem", Predicate{Op: "mem", At: in, Len: 8}, true},
		{"mem_changed outside", Predicate{Op: "mem_changed", At: out, Len: 2}, false},
		{"script", Predicate{Op: "script", Entry: "checks/a.py:f"}, true},
		{"script bad entry", Predicate{Op: "script", Entry: "checks/a.py"}, false},
		{"script missing file", Predicate{Op: "script", Entry: "checks/b.py:f"}, false},
		{"commanded", Predicate{Op: "commanded", At: out}, true},
		{"commanded unresolvable", Predicate{Op: "commanded", At: &AddressRef{Sym: "nope"}}, false},
		{"all", Predicate{Op: "all", Of: of(t, []Predicate{inside, inside})}, true},
		{"any bad child", Predicate{Op: "any", Of: of(t, []Predicate{inside, {Op: "mem_u8", At: out}})}, false},
		{"any bad of", Predicate{Op: "any", Of: of(t, inside)}, false},
		{"not", Predicate{Op: "not", Of: of(t, inside)}, true},
		{"sustained bad child", Predicate{Op: "sustained", Of: of(t, Predicate{Op: "mem_u8", At: out})}, false},
		{"within bad of", Predicate{Op: "within", Of: of(t, []Predicate{inside})}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := s.validatePredicateAddresses(tt.p); (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestValidateObjectiveAddresses(t *testing.T) {
	good := Predicate{Op: "mem_u8", At: &AddressRef{Sym: "g_mode"}}
	bad := Predicate{Op: "mem_u8", At: &AddressRef{Addr: "0x10"}}

	tests := []struct {
		name      string
		objective Objective
		want      string
	}{
		{"valid", Objective{ID: "o", Success: good, Fail: &good, Partial: []Partial{{When: good}}}, ""},
		{"success", Objective{ID: "o", Success: bad}, "success"},
		{"fail", Objective{ID: "o", Success: good, Fail: &bad}, "fail"},
		{"partial", Objective{ID: "o", Success: good, Partial: []Partial{{When: bad}}}, "partial"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := addressScenario(t)
			s.Objectives = []Objective{tt.objective}
			err := s.validateObjectiveAddresses()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want mention of %q", err, tt.want)
			}
		})
	}
}

func TestValidateSetupAddresses(t *testing.T) {
	u8, u16, u32 := uint8(1), uint16(1), uint32(1)
	at := AddressRef{Sym: "g_mode"}

	tests := []struct {
		name  string
		write SetupWrite
		ok    bool
	}{
		{"u8", SetupWrite{At: at, U8: &u8}, true},
		{"u16", SetupWrite{At: at, U16: &u16}, true},
		{"u32", SetupWrite{At: at, U32: &u32}, true},
		{"hex", SetupWrite{At: at, Hex: "0102"}, true},
		{"bad ref", SetupWrite{U8: &u8}, false},
		{"bad hex", SetupWrite{At: at, Hex: "zz"}, false},
		{"odd hex", SetupWrite{At: at, Hex: "0"}, false},
		{"none", SetupWrite{At: at}, false},
		{"two values", SetupWrite{At: at, U8: &u8, U16: &u16}, false},
		{"outside", SetupWrite{At: AddressRef{Addr: "0x10"}, U32: &u32}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := addressScenario(t)
			s.Setup = &Setup{Writes: []SetupWrite{tt.write}}
			if err := s.validateSetupAddresses(); (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}

	if err := addressScenario(t).validateSetupAddresses(); err != nil {
		t.Fatalf("no setup: %v", err)
	}
}

func TestValidateObjectiveShape(t *testing.T) {
	success := Predicate{Op: "tlm"}
	tests := []struct {
		name      string
		objective Objective
		ok        bool
	}{
		{"valid", Objective{ID: "o", Title: "t", Brief: "b", Success: success, Partial: []Partial{{When: success, Text: "x"}}}, true},
		{"no title", Objective{ID: "o", Brief: "b", Success: success}, false},
		{"no brief", Objective{ID: "o", Title: "t", Success: success}, false},
		{"no success", Objective{ID: "o", Title: "t", Brief: "b"}, false},
		{"partial no when", Objective{ID: "o", Title: "t", Brief: "b", Success: success, Partial: []Partial{{Text: "x"}}}, false},
		{"partial no text", Objective{ID: "o", Title: "t", Brief: "b", Success: success, Partial: []Partial{{When: success}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateObjectiveShape([]Objective{tt.objective}); (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}

	if err := validateObjectiveIDs([]Objective{{ID: " "}}); err == nil {
		t.Fatal("expected blank objective id to be rejected")
	}
}

func TestValidateObjectivePredicatesFailAndPartial(t *testing.T) {
	good := Predicate{Op: "channel_present", ID: "HK"}
	bad := Predicate{Op: "channel_present"}

	if err := validateObjectivePredicates([]Objective{{ID: "o", Success: good, Fail: &bad}}); err == nil {
		t.Error("expected invalid fail predicate to be rejected")
	}
	if err := validateObjectivePredicates([]Objective{{ID: "o", Success: good, Partial: []Partial{{When: bad}}}}); err == nil {
		t.Error("expected invalid partial predicate to be rejected")
	}
	if err := validateObjectivePredicates([]Objective{{ID: "o", Success: good, Fail: &good, Partial: []Partial{{When: good}}}}); err != nil {
		t.Errorf("valid objective rejected: %v", err)
	}
}

func TestValidateManifestJSON(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{
			"format": 1, "id": "x", "title": "t", "revision": 1, "summary": "s",
			"firmware": map[string]any{"rom": "r", "symbols": "s", "memmap": "m", "app_crc32": "c"},
			"link":     map[string]any{"uplink_delay_s": 0, "downlink_delay_s": 0},
			"briefing": "b", "objectives": "o",
		}
	}

	tests := []struct {
		name   string
		mutate func(m map[string]any)
		ok     bool
	}{
		{"valid", func(m map[string]any) {}, true},
		{"missing key", func(m map[string]any) { delete(m, "summary") }, false},
		{"empty string", func(m map[string]any) { m["title"] = " " }, false},
		{"firmware not object", func(m map[string]any) { m["firmware"] = "rom" }, false},
		{"firmware missing key", func(m map[string]any) { delete(m["firmware"].(map[string]any), "app_crc32") }, false},
		{"firmware empty", func(m map[string]any) { m["firmware"].(map[string]any)["rom"] = "" }, false},
		{"link not object", func(m map[string]any) { m["link"] = []any{} }, false},
		{"link missing key", func(m map[string]any) { delete(m["link"].(map[string]any), "downlink_delay_s") }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			tt.mutate(m)
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			// The typed decode may fail for malformed objects; validation still runs on the raw form.
			var manifest Manifest
			_ = json.Unmarshal(data, &manifest)
			if err := validateManifestJSON(data, manifest); (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}

	if err := validateManifestJSON([]byte("[]"), Manifest{}); err == nil {
		t.Fatal("expected non-object manifest to be rejected")
	}
}

func TestValidateReferencedFiles(t *testing.T) {
	dir := t.TempDir()
	writeScenario(t, dir, map[string]string{"setup.json": "{}", "docs/a.md": "a"})

	if err := validateReferencedFiles(dir, Manifest{Setup: "setup.json", Docs: []string{"docs/a.md"}}); err != nil {
		t.Fatalf("valid references rejected: %v", err)
	}
	if err := validateReferencedFiles(dir, Manifest{Docs: []string{"docs"}}); err == nil {
		t.Fatal("expected a directory reference to be rejected")
	}
	if err := validateReferencedFiles(dir, Manifest{Setup: "missing.json"}); err == nil {
		t.Fatal("expected a missing reference to be rejected")
	}
}

func TestValidatePurity(t *testing.T) {
	pure := t.TempDir()
	impure := t.TempDir()
	if err := os.Mkdir(filepath.Join(impure, "checks"), 0755); err != nil {
		t.Fatal(err)
	}
	unreadable := t.TempDir()
	if err := os.WriteFile(filepath.Join(unreadable, "file"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		dir      string
		impure   bool
		pureMode bool
		ok       bool
	}{
		{"pure", pure, false, true, true},
		{"undeclared checks", impure, false, false, false},
		{"declared without checks", pure, true, false, false},
		{"impure allowed", impure, true, false, true},
		{"impure refused", impure, true, true, false},
		{"stat error", filepath.Join(unreadable, "file"), false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validatePurity(tt.dir, Manifest{Impure: tt.impure}, tt.pureMode); (err == nil) != tt.ok {
				t.Fatalf("err = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestLoadScenarioWithOptionsImpure(t *testing.T) {
	dir := t.TempDir()
	files := scenarioFiles("impure")
	files["manifest.json"] = strings.Replace(files["manifest.json"], `"revision": 1,`, `"revision": 1, "impure": true,`, 1)
	files["checks/a.py"] = "def f(frame): return True\n"
	files["objectives.json"] = `{"format": 1, "objectives": [{"id": "s", "title": "t", "brief": "b",
		"success": {"op": "script", "lang": "python", "entry": "checks/a.py:f"}}]}`
	writeScenario(t, dir, files)

	if _, err := LoadScenario(dir); err == nil {
		t.Fatal("expected impure scenario to be refused in pure mode")
	}
	if _, err := LoadScenarioWithOptions(dir, false); err != nil {
		t.Fatalf("LoadScenarioWithOptions: %v", err)
	}
}

func TestLoadScenarioErrors(t *testing.T) {
	objective := func(success string) string {
		return `{"format": 1, "objectives": [{"id": "o", "title": "t", "brief": "b", "success": ` + success + `}]}`
	}
	withSetup := func(files map[string]string, setup string) {
		files["manifest.json"] = strings.Replace(files["manifest.json"], `"briefing"`, `"setup": "setup.json", "briefing"`, 1)
		files["setup.json"] = setup
	}

	tests := []struct {
		name   string
		mutate func(files map[string]string)
	}{
		{"no manifest", func(f map[string]string) { f["manifest.json"] = "" }},
		{"bad manifest json", func(f map[string]string) { f["manifest.json"] = "{" }},
		{"manifest missing key", func(f map[string]string) { f["manifest.json"] = `{"format": 1}` }},
		{"manifest bad format", func(f map[string]string) {
			f["manifest.json"] = strings.Replace(f["manifest.json"], `"format": 1`, `"format": 2`, 1)
		}},
		{"undeclared checks", func(f map[string]string) { f["checks/a.py"] = "x" }},
		{"missing briefing", func(f map[string]string) { f["briefing.md"] = "" }},
		{"objectives format", func(f map[string]string) { f["objectives.json"] = `{"format": 2, "objectives": []}` }},
		{"objective ids", func(f map[string]string) { f["objectives.json"] = `{"format": 1, "objectives": [{"id": ""}]}` }},
		{"objective shape", func(f map[string]string) { f["objectives.json"] = `{"format": 1, "objectives": [{"id": "o"}]}` }},
		{"objective requires", func(f map[string]string) {
			f["objectives.json"] = `{"format": 1, "objectives": [{"id": "o", "title": "t", "brief": "b", "requires": ["x"], "success": {"op": "tlm"}}]}`
		}},
		{"objective cycle", func(f map[string]string) {
			f["objectives.json"] = `{"format": 1, "objectives": [{"id": "o", "title": "t", "brief": "b", "requires": ["o"], "success": {"op": "tlm"}}]}`
		}},
		{"objective predicate", func(f map[string]string) { f["objectives.json"] = objective(`{"op": "tlm"}`) }},
		{"objective address", func(f map[string]string) {
			f["objectives.json"] = objective(`{"op": "mem_u8", "at": {"addr": "0x10"}, "cmp": "eq", "value": 1}`)
		}},
		{"setup json", func(f map[string]string) { withSetup(f, "{") }},
		{"setup format", func(f map[string]string) { withSetup(f, `{"format": 2, "writes": []}`) }},
		{"setup address", func(f map[string]string) {
			withSetup(f, `{"format": 1, "writes": [{"at": {"addr": "0x10"}, "u8": 1}]}`)
		}},
		{"symbols json", func(f map[string]string) { f["firmware/symbols.json"] = "{" }},
		{"symbols format", func(f map[string]string) { f["firmware/symbols.json"] = `{"format": 2}` }},
		{"memmap json", func(f map[string]string) { f["firmware/memmap.json"] = "{" }},
		{"memmap format", func(f map[string]string) { f["firmware/memmap.json"] = `{"format": 2}` }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			files := scenarioFiles("errors")
			tt.mutate(files)
			writeScenario(t, dir, files)
			if _, err := LoadScenario(dir); err == nil {
				t.Fatal("expected LoadScenario to fail")
			}
		})
	}
}

func TestLoadScenarioWithSetupAndMemory(t *testing.T) {
	dir := t.TempDir()
	files := scenarioFiles("full")
	files["manifest.json"] = strings.Replace(files["manifest.json"], `"briefing"`, `"setup": "setup.json", "docs": ["docs/a.md"], "briefing"`, 1)
	files["setup.json"] = `{"format": 1, "writes": [{"at": {"sym": "g_mode"}, "hex": "0102"}], "settle_frames": 1}`
	files["docs/a.md"] = "doc"
	files["objectives.json"] = `{"format": 1, "objectives": [{"id": "o", "title": "t", "brief": "b",
		"success": {"op": "mem_u8", "at": {"sym": "g_mode", "field": "hi"}, "cmp": "eq", "value": 1}}]}`
	writeScenario(t, dir, files)

	s, err := LoadScenario(dir)
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	if s.Setup == nil || s.Setup.Writes[0].Hex != "0102" {
		t.Fatalf("setup = %+v", s.Setup)
	}
}

func TestLoadScenarioUnreadableFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("file permissions are not enforced for root")
	}

	for _, name := range []string{"objectives.json", "setup.json", "firmware/symbols.json", "firmware/memmap.json"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			files := scenarioFiles("unreadable")
			files["manifest.json"] = strings.Replace(files["manifest.json"], `"briefing"`, `"setup": "setup.json", "briefing"`, 1)
			files["setup.json"] = `{"format": 1, "writes": []}`
			writeScenario(t, dir, files)
			if err := os.Chmod(filepath.Join(dir, name), 0); err != nil {
				t.Fatal(err)
			}

			if _, err := LoadScenario(dir); err == nil {
				t.Fatalf("expected LoadScenario to fail with unreadable %s", name)
			}
		})
	}
}
