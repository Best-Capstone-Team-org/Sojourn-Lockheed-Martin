package game

import "testing"

type firmwareLaunch struct {
	rom      string
	downlink chan []byte
	uplink   chan []byte
	done     chan struct{}
}

// stubFirmware replaces the firmware launcher for the duration of the test and
// returns a channel that receives each launch.
func stubFirmware(t *testing.T) <-chan firmwareLaunch {
	t.Helper()

	launches := make(chan firmwareLaunch, 1)
	prev := launchFirmware
	launchFirmware = func(rom string, downlink chan []byte, uplink chan []byte, done chan struct{}) {
		launches <- firmwareLaunch{rom, downlink, uplink, done}
	}
	t.Cleanup(func() { launchFirmware = prev })

	return launches
}

// startGame starts state without booting real firmware.
func startGame(t *testing.T, state *State) {
	t.Helper()
	stubFirmware(t)

	if err := state.Start(make(chan []byte), make(chan []byte), make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func TestStartLaunchesScenarioFirmware(t *testing.T) {
	launches := stubFirmware(t)

	s := frameScenario(t)
	s.Manifest.Firmware.ROM = "probe_rom.elf"
	state := &State{phase: PhaseReady, Scenario: s}

	downlink, uplink, done := make(chan []byte), make(chan []byte), make(chan struct{})
	if err := state.Start(downlink, uplink, done); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if state.Phase() != PhaseInProgress {
		t.Fatalf("phase = %v, want in progress", state.Phase())
	}

	l := <-launches
	if l.rom != "probe_rom.elf" {
		t.Errorf("rom = %q, want probe_rom.elf", l.rom)
	}
	if l.downlink != downlink || l.uplink != uplink || l.done != done {
		t.Error("firmware was not given the channels passed to Start")
	}
}

func TestStartRequiresReadyPhase(t *testing.T) {
	launches := stubFirmware(t)

	for _, phase := range []Phase{PhaseNone, PhaseLevelSelect, PhaseInProgress} {
		state := &State{phase: phase, Scenario: frameScenario(t)}
		if err := state.Start(nil, nil, nil); err == nil {
			t.Errorf("Start in phase %v succeeded, want error", phase)
		}
		if state.Phase() != phase {
			t.Errorf("phase changed from %v to %v", phase, state.Phase())
		}
	}

	select {
	case l := <-launches:
		t.Fatalf("firmware launched (%q) when Start failed", l.rom)
	default:
	}
}

func TestStopReturnsToLevelSelect(t *testing.T) {
	state := &State{phase: PhaseReady, Scenario: frameScenario(t)}
	startGame(t, state)

	if err := state.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if state.Phase() != PhaseLevelSelect {
		t.Fatalf("phase = %v, want level select", state.Phase())
	}

	if err := state.Stop(); err == nil {
		t.Fatal("expected Stop to fail when no game is running")
	}
}
