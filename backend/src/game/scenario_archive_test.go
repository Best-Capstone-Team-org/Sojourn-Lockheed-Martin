package game

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"sojourn/emulator"
)

type tarEntry struct {
	name     string
	body     string
	typeflag byte
	size     int64 // overrides len(body) when set, for a header that lies
}

func writeArchive(t *testing.T, path string, entries []tarEntry) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		header := &tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.body)), Typeflag: e.typeflag}
		switch e.typeflag {
		case tar.TypeDir:
			header.Size = 0
		case tar.TypeSymlink:
			header.Size, header.Linkname = 0, e.body
		}
		if e.size > 0 {
			header.Size = e.size
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if header.Size > 0 {
			if e.size > 0 {
				tw.Write(make([]byte, e.size))
			} else {
				tw.Write([]byte(e.body))
			}
		}
	}
	tw.Close()
	gz.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// packageEntries archives a package's files at the archive root, under prefix.
func packageEntries(files map[string]string, prefix string) []tarEntry {
	var entries []tarEntry
	for name, body := range files {
		entries = append(entries, tarEntry{name: prefix + name, body: body, typeflag: tar.TypeReg})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries
}

func loadedIDs(t *testing.T, root string) []string {
	t.Helper()
	state := InitState()
	if err := state.LoadScenarios([]ScenariosDir{{Dir: root, Required: true}}); err != nil {
		t.Fatalf("LoadScenarios: %v", err)
	}
	var ids []string
	for id := range state.AvailableScenarios {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func TestLoadScenariosFromArchives(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	root := t.TempDir()

	writeScenario(t, filepath.Join(root, "alpha"), scenarioFiles("alpha"))
	writeArchive(t, filepath.Join(root, "beta.tar.gz"), packageEntries(scenarioFiles("beta"), ""))

	// tar -C pkg . writes ./-prefixed names and directory entries.
	gamma := append([]tarEntry{{name: "./", typeflag: tar.TypeDir}, {name: "./firmware/", typeflag: tar.TypeDir}},
		packageEntries(scenarioFiles("gamma"), "./")...)
	writeArchive(t, filepath.Join(root, "gamma.tar.gz"), gamma)

	// Not packages: ignored.
	os.WriteFile(filepath.Join(root, "README.md"), []byte("notes"), 0o644)
	os.WriteFile(filepath.Join(root, "delta.zip"), []byte("PK"), 0o644)

	state := InitState()
	if err := state.LoadScenarios([]ScenariosDir{{Dir: root, Required: true}}); err != nil {
		t.Fatalf("LoadScenarios: %v", err)
	}
	var ids []string
	for id := range state.AvailableScenarios {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if want := []string{"sojourn.test.alpha", "sojourn.test.beta", "sojourn.test.gamma"}; strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("loaded %v, want %v", ids, want)
	}

	// The extracted package is real files the emulator can be pointed at.
	beta := state.AvailableScenarios["sojourn.test.beta"]
	rom, err := os.ReadFile(filepath.Join(beta.Dir, beta.Manifest.Firmware.ROM))
	if err != nil || string(rom) != "rom" {
		t.Errorf("extracted ROM = %q, %v", rom, err)
	}
}

func TestLoadScenariosSkipsBadArchives(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	root := t.TempDir()

	good := scenarioFiles("good")
	writeScenario(t, filepath.Join(root, "good"), good)

	bad := map[string][]tarEntry{
		// The package must be the archive root, not one directory down.
		"nested": packageEntries(scenarioFiles("nested"), "nested/"),
		"traversal": append(packageEntries(scenarioFiles("traversal"), ""),
			tarEntry{name: "../escaped.txt", body: "x", typeflag: tar.TypeReg}),
		"absolute": append(packageEntries(scenarioFiles("absolute"), ""),
			tarEntry{name: "/tmp/escaped.txt", body: "x", typeflag: tar.TypeReg}),
		"symlink": append(packageEntries(scenarioFiles("symlink"), ""),
			tarEntry{name: "firmware/link", body: "/etc/passwd", typeflag: tar.TypeSymlink}),
		"bomb": append(packageEntries(scenarioFiles("bomb"), ""),
			tarEntry{name: "huge.bin", typeflag: tar.TypeReg, size: maxScenarioArchiveBytes + 1}),
	}
	for name, entries := range bad {
		writeArchive(t, filepath.Join(root, name+".tar.gz"), entries)
	}
	os.WriteFile(filepath.Join(root, "corrupt.tar.gz"), []byte("not gzip at all"), 0o644)

	if ids := loadedIDs(t, root); strings.Join(ids, ",") != "sojourn.test.good" {
		t.Errorf("loaded %v, want only the good directory package", ids)
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escaped.txt")); err == nil {
		t.Error("an archive wrote outside its package root")
	}
	if _, err := os.Stat("/tmp/escaped.txt"); err == nil {
		t.Error("an archive wrote to an absolute path")
	}

	// Rejected archives clean up after themselves, including "nested", which
	// extracts fine but has no manifest at its root.
	leftovers, _ := filepath.Glob(filepath.Join(tmp, "sojourn-scenario-*"))
	if len(leftovers) != 0 {
		t.Errorf("rejected archives left %d temporary directories behind", len(leftovers))
	}
}

func TestLoadScenariosFromArchivedRealPackage(t *testing.T) {
	if _, err := os.Stat(commsTriageDir); err != nil {
		t.Skipf("comms-triage scenario not checked out: %v", err)
	}
	t.Setenv("TMPDIR", t.TempDir())

	files := map[string]string{}
	filepath.WalkDir(commsTriageDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			data, _ := os.ReadFile(path)
			rel, _ := filepath.Rel(commsTriageDir, path)
			files[filepath.ToSlash(rel)] = string(data)
		}
		return nil
	})
	root := t.TempDir()
	writeArchive(t, filepath.Join(root, "comms-triage.tar.gz"), packageEntries(files, "./"))

	state := InitState()
	if err := state.LoadScenarios([]ScenariosDir{{Dir: root, Required: true}}); err != nil {
		t.Fatalf("LoadScenarios: %v", err)
	}
	scenario, ok := state.AvailableScenarios["sojourn.comms.triage"]
	if !ok {
		t.Fatal("archived comms-triage did not load")
	}
	if _, err := emulator.AppImage(filepath.Join(scenario.Dir, scenario.Manifest.Firmware.ROM)); err != nil {
		t.Errorf("firmware unusable from the extracted package: %v", err)
	}
}
