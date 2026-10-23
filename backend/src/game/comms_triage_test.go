package game

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const commsTriageDir = "../../../deps/sojourn/scenarios/comms-triage"

// probeMemory applies POKEs so memory predicates see the player's writes.
type probeMemory map[uint32]byte

func (m probeMemory) Read(addr uint32, length int) ([]byte, error) {
	data := make([]byte, length)
	for i := range data {
		data[i] = m[addr+uint32(i)]
	}
	return data, nil
}

func (m probeMemory) poke(command CommandLogEntry) {
	value, _ := strconv.ParseUint(strings.Fields(command.Raw)[2], 16, 8)
	m[command.Addr] = byte(value)
}

// Replays comms-triage's solution.txt the way the grader sees it: commands
// accumulate in the log and objectives are evaluated once per telemetry frame.
func TestCommsTriageSolution(t *testing.T) {
	if _, err := os.Stat(commsTriageDir); err != nil {
		t.Skipf("comms-triage scenario not checked out: %v", err)
	}
	scenario, err := LoadScenario(commsTriageDir)
	if err != nil {
		t.Fatalf("LoadScenario: %v", err)
	}
	e, err := NewEvaluation(scenario)
	if err != nil {
		t.Fatalf("NewEvaluation: %v", err)
	}

	memory := probeMemory{}
	var log []CommandLogEntry
	command := func(raw, result string) {
		entry := parseCommand(raw)
		entry.Result = result
		if entry.Verb == "POKE" && strings.HasPrefix(result, "ACK") {
			memory.poke(entry)
		}
		log = append(log, entry)
	}
	frame := func(radPresent bool) {
		t.Helper()
		channels := map[string]any{
			"COMMS": map[string]any{"antenna": "LGA", "hga_deploy_pct": 40.0, "dropped": 2.0},
		}
		if radPresent {
			channels["RAD"] = map[string]any{"counts": 12.0}
		}
		f := Frame{Telemetry: map[string]any{"channels": channels}, Log: slices.Clone(log), Memory: memory}
		if err := e.Evaluate(f); err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
	}
	expect := func(want map[string]ObjectiveStatus) {
		t.Helper()
		for id, w := range want {
			if got := status(t, &e, id); got != w {
				t.Errorf("%s = %s, want %s", id, got, w)
			}
		}
	}

	frame(false)
	expect(map[string]ObjectiveStatus{"diagnose": ObjectiveStatusActive, "attempt-redeploy": ObjectiveStatusLocked})

	command("PEEK 0x2001E200 16", "ACK PEEK 01000000030000000000000064000000")
	frame(false)
	expect(map[string]ObjectiveStatus{"diagnose": ObjectiveStatusComplete, "attempt-redeploy": ObjectiveStatusActive})

	command("POKE 0x2001E200 09", "ACK POKE 1")
	frame(false)
	expect(map[string]ObjectiveStatus{"attempt-redeploy": ObjectiveStatusComplete, "restore-radiation": ObjectiveStatusActive})

	command("POKE 0x2001943A 04", "ACK POKE 1")
	command("POKE 0x2001943E 00", "ACK POKE 1")
	frame(true)
	frame(true)
	expect(map[string]ObjectiveStatus{"restore-radiation": ObjectiveStatusActive})

	// A survey between frames must not reset the three-frame streak.
	command("PEEK 0x20019438 10", "ACK PEEK 00010304020500436061")
	frame(true)
	expect(map[string]ObjectiveStatus{"restore-radiation": ObjectiveStatusComplete})
}

// Plays the first objective of comms-triage on the real firmware: the setup
// write has to make the antenna fail before a PEEK can complete diagnose.
// It boots QEMU on the loader's fixed ports, so it only runs when asked.
func TestCommsTriageOnFirmware(t *testing.T) {
	if os.Getenv("SOJOURN_FIRMWARE_TESTS") == "" {
		t.Skip("set SOJOURN_FIRMWARE_TESTS=1 to boot the real firmware")
	}
	if _, err := exec.LookPath("qemu-system-arm"); err != nil {
		t.Skip("qemu-system-arm not installed")
	}

	state := InitState()
	if err := state.LoadScenarios(filepath.Dir(commsTriageDir)); err != nil {
		t.Fatalf("LoadScenarios: %v", err)
	}
	if err := state.SelectScenario("sojourn.comms.triage"); err != nil {
		t.Fatalf("SelectScenario: %v", err)
	}

	out := make(chan DownlinkMessage)
	up := make(chan []byte, 1)
	done := make(chan struct{})
	if err := state.Start(out, up, done); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		close(done)
		close(up)
		for range out {
		}
	}()

	deadline := time.After(90 * time.Second)
	peeked := false
	for {
		select {
		case msg := <-out:
			switch msg := msg.(type) {
			case *CommandResponseDownlinkMessage:
				if strings.HasPrefix(msg.Response, "ACK POKE") {
					t.Fatalf("setup write reply %q reached the player's console", msg.Response)
				}
			case *TLMDownlinkMessage:
				antenna := msg.Telemetry.AsMap()["channels"].(map[string]any)["COMMS"].(map[string]any)["antenna"]
				t.Logf("uptime %3ds antenna %v", msg.Telemetry.UptimeS, antenna)
				if antenna == "LGA" && !peeked {
					up <- []byte("PEEK 0x2001E200 16")
					peeked = true
				}
			case *StateDownlinkMessage:
				for _, objective := range msg.Objectives {
					if objective.ID == "diagnose" && objective.Status == ObjectiveStatusComplete {
						return
					}
				}
			}
		case <-deadline:
			t.Fatalf("diagnose not complete after 90s (antenna failed and PEEK sent: %v)", peeked)
		}
	}
}
