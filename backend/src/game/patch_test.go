package game

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func image(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i * 7)
	}
	return b
}

func patched(base []byte, at int, data ...byte) []byte {
	b := bytes.Clone(base)
	copy(b[at:], data)
	return b
}

func TestPatchPokes(t *testing.T) {
	running := image(0x100)

	tests := []struct {
		name     string
		uploaded []byte
		want     []string
	}{
		{"identical", running, nil},
		{"one byte", patched(running, 0x40, 0xAB), []string{"POKE 0x20001040 AB"}},
		{"header ignored", patched(running, 0x04, 0xFF, 0xFF), nil},
		// A merged run rewrites the unchanged bytes between its changes.
		{"close changes merge", patched(patched(running, 0x40, 0xAB), 0x44, 0xCD),
			[]string{fmt.Sprintf("POKE 0x20001040 AB%02X%02X%02XCD", running[0x41], running[0x42], running[0x43])}},
		{"distant changes split", patched(patched(running, 0x40, 0xAB), 0x46, 0xCD),
			[]string{"POKE 0x20001040 AB", "POKE 0x20001046 CD"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := patchPokes(running, tc.uploaded)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("pokes = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPatchPokesSplitsLongRuns(t *testing.T) {
	running := image(0x100)
	uploaded := bytes.Clone(running)
	for i := 0x40; i < 0x40+40; i++ {
		uploaded[i] ^= 0xFF
	}

	got, err := patchPokes(running, uploaded)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		fmt.Sprintf("POKE 0x20001040 %X", uploaded[0x40:0x60]),
		fmt.Sprintf("POKE 0x20001060 %X", uploaded[0x60:0x68]),
	}
	if !slices.Equal(got, want) {
		t.Errorf("pokes = %q, want %q", got, want)
	}
}

// Every POKE must be one the firmware accepts as a single line.
func TestPatchPokesFitTheFirmware(t *testing.T) {
	running := image(0x400)
	uploaded := bytes.Clone(running)
	for i := appHeaderSize; i < len(uploaded); i += 3 {
		uploaded[i] ^= 0x5A
	}

	pokes, err := patchPokes(running, uploaded)
	if err != nil {
		t.Fatal(err)
	}

	rebuilt := bytes.Clone(running)
	for _, poke := range pokes {
		fields := strings.Fields(poke)
		if len(fields) != 3 || fields[0] != "POKE" {
			t.Fatalf("malformed %q", poke)
		}
		if line := poke + " *0000"; len(line) > 95 {
			t.Errorf("%q is %d characters with its checksum; the firmware takes at most 95", poke, len(line))
		}
		if n := len(fields[2]) / 2; n > maxPokeBytes {
			t.Errorf("%q writes %d bytes; the firmware takes at most %d", poke, n, maxPokeBytes)
		}

		data, err := hex.DecodeString(fields[2])
		if err != nil {
			t.Fatalf("%q: %v", poke, err)
		}
		copy(rebuilt[parseCommand(poke).Addr-appRAMBase:], data)
	}

	if !bytes.Equal(rebuilt[appHeaderSize:], uploaded[appHeaderSize:]) {
		t.Error("applying the POKEs does not reproduce the uploaded image")
	}
}

func TestPatchPokesErrors(t *testing.T) {
	if _, err := patchPokes(image(0x100), nil); err == nil {
		t.Error("expected an error for an empty patch")
	}
	if _, err := patchPokes(image(0x20), image(0x40)); err == nil {
		t.Error("expected an error when the running image is shorter than the patch")
	}
	big := appRAMEnd - appRAMBase + 1
	if _, err := patchPokes(image(big), image(big)); !errors.Is(err, ErrPatchTooLarge) {
		t.Errorf("err = %v, want ErrPatchTooLarge", err)
	}
}

// appRAM is probe memory from SRAM base with img loaded at APP RAM.
func appRAM(img []byte) memory {
	m := make(memory, appRAMEnd-0x20000000)
	copy(m[appRAMBase-0x20000000:], img)
	return m
}

func runningGame(t *testing.T, ram memory) *State {
	t.Helper()
	if _, err := os.Stat(commsTriageDir); err != nil {
		t.Skipf("comms-triage scenario not checked out: %v", err)
	}
	state := readyState(t)
	state.Scenario.Dir = commsTriageDir
	state.Scenario.Manifest.Firmware.ROM = filepath.Join("firmware", "probe_rom.elf")
	startGame(t, state)
	state.grader = NewGrader(state, newGDBStub(t, ram, false).addr)
	t.Cleanup(func() { state.grader.Close() })
	return state
}

func TestRunningImageIncludesLivePatches(t *testing.T) {
	probe := readyState(t)
	probe.Scenario.Dir = commsTriageDir
	probe.Scenario.Manifest.Firmware.ROM = filepath.Join("firmware", "probe_rom.elf")
	startGame(t, probe)
	golden, err := probe.GoldenImage()
	if err != nil {
		t.Skipf("golden image unavailable: %v", err)
	}

	live := patched(golden, 0x100, 0xDE, 0xAD)
	state := runningGame(t, appRAM(live))

	running, err := state.RunningImage()
	if err != nil {
		t.Fatalf("RunningImage: %v", err)
	}
	if !bytes.Equal(running, live) {
		t.Error("running image does not match APP RAM")
	}
	if again, _ := state.GoldenImage(); !bytes.Equal(again, golden) {
		t.Error("golden image changed")
	}
}

// The patch is diffed against what runs now, so edits already live cost
// nothing and only the new ones are sent.
func TestPatchPokesDiffAgainstRunningImage(t *testing.T) {
	probe := readyState(t)
	probe.Scenario.Dir = commsTriageDir
	probe.Scenario.Manifest.Firmware.ROM = filepath.Join("firmware", "probe_rom.elf")
	startGame(t, probe)
	golden, err := probe.GoldenImage()
	if err != nil {
		t.Skipf("golden image unavailable: %v", err)
	}

	live := patched(golden, 0x100, 0xDE, 0xAD)
	state := runningGame(t, appRAM(live))

	upload := patched(live, 0x200, 0x46, 0xC0)
	pokes, err := state.PatchPokes(upload)
	if err != nil {
		t.Fatalf("PatchPokes: %v", err)
	}
	if want := []string{"POKE 0x20001200 46C0"}; !slices.Equal(pokes, want) {
		t.Errorf("pokes = %q, want %q", pokes, want)
	}

	// Appending code past the image's end diffs against what is in RAM there.
	longer := append(bytes.Clone(upload), 0x01, 0x02)
	pokes, err = state.PatchPokes(longer)
	if err != nil {
		t.Fatalf("PatchPokes: %v", err)
	}
	if last := pokes[len(pokes)-1]; last != fmt.Sprintf("POKE 0x%08X 0102", appRAMBase+len(upload)) {
		t.Errorf("last poke = %q, want the appended bytes", last)
	}
}

func TestPatchRequiresReadableProbe(t *testing.T) {
	state := readyState(t)
	state.Scenario.Dir = commsTriageDir
	state.Scenario.Manifest.Firmware.ROM = filepath.Join("firmware", "probe_rom.elf")
	if _, err := os.Stat(commsTriageDir); err != nil {
		t.Skip("comms-triage scenario not checked out")
	}

	if _, err := state.RunningImage(); !errors.Is(err, ErrNoGameInProgress) {
		t.Errorf("before start: err = %v, want ErrNoGameInProgress", err)
	}

	startGame(t, state)
	if _, err := state.RunningImage(); !errors.Is(err, ErrProbeMemoryUnavailable) {
		t.Errorf("without a grader: err = %v, want ErrProbeMemoryUnavailable", err)
	}

	state.grader = NewGrader(state, "127.0.0.1:1")
	if _, err := state.PatchPokes([]byte{1}); !errors.Is(err, ErrProbeMemoryUnavailable) {
		t.Errorf("unreachable: err = %v, want ErrProbeMemoryUnavailable", err)
	}
	if _, err := state.PatchPokes(make([]byte, appRAMEnd-appRAMBase+1)); !errors.Is(err, ErrPatchTooLarge) {
		t.Errorf("oversize: err = %v, want ErrPatchTooLarge", err)
	}
}

func TestRemainingWrites(t *testing.T) {
	state := readyState(t)
	if _, metered := state.RemainingWrites(); metered {
		t.Error("writes metered before a budget was set")
	}

	writes := 3
	state.Scenario.Manifest.Link.Budget = BudgetConfig{Writes: &writes}
	startGame(t, state)
	if remaining, metered := state.RemainingWrites(); !metered || remaining != 3 {
		t.Errorf("RemainingWrites = %d, %v; want 3, metered", remaining, metered)
	}
}
