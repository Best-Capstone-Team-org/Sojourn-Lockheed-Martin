package game

import (
	"os"
	"sojourn/app"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	app.InitLoggers()
	os.Exit(m.Run())
}

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

	if err := state.Start(make(chan DownlinkMessage), make(chan []byte), make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
}

func TestStartLaunchesScenarioFirmware(t *testing.T) {
	launches := stubFirmware(t)

	state := readyState(t)
	state.Scenario.Manifest.Firmware.ROM = "probe_rom.elf"

	downlink, uplink, done := make(chan DownlinkMessage), make(chan []byte), make(chan struct{})
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
	if l.uplink != uplink || l.done != done {
		t.Error("firmware was not given the channels passed to Start")
	}

	l.downlink <- []byte("ACK PING\n")
	select {
	case msg := <-downlink:
		resp, ok := msg.(*CommandResponseDownlinkMessage)
		if !ok || resp.Response != "ACK PING" {
			t.Errorf("downlink = %#v, want command response %q", msg, "ACK PING")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for translated downlink")
	}

	close(l.downlink)
	select {
	case _, ok := <-downlink:
		if ok {
			t.Error("downlink received a message after the firmware downlink closed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("downlink was not closed after the firmware downlink closed")
	}
}

func TestStartRequiresReadyPhase(t *testing.T) {
	launches := stubFirmware(t)

	for _, phase := range []Phase{PhaseNone, PhaseLevelSelect, PhaseInProgress} {
		state := readyState(t)
		state.phase = phase
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
	state := readyState(t)
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
