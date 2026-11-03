package game

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConformanceDefects(t *testing.T) {
	defectsDir := filepath.Join(
		"..", "..", "..",
		"deps", "sojourn", "conformance", "defects",
	)

	entries, err := os.ReadDir(defectsDir)
	if err != nil {
		t.Fatalf("Failed to read defects directory: %v", err)
	}

	total := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		total++

		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join(defectsDir, entry.Name())

			_, err := LoadScenario(path)
			if err == nil {
				t.Fatalf(
					"Defective scenario %q was accepted but should be rejected",
					entry.Name(),
				)
			}

			t.Logf(
				"PASS: Defective scenario %q rejected: %v",
				entry.Name(),
				err,
			)
		})
	}

	if total != 9 {
		t.Fatalf("Expected 9 defective packages, found %d", total)
	}
}
