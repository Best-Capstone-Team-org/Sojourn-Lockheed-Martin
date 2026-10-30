package game

import (
	"maps"
	"testing"
	"time"
)

func TestChargedResource(t *testing.T) {
	for verb, want := range map[string]string{
		"POKE": ResourceWrites, "CALL": ResourceWrites, "TRIM": ResourceWrites, "SAFE": ResourceWrites,
		"PING": ResourceReads, "STAT": ResourceReads, "PEEK": ResourceReads, "DUMP": ResourceReads,
		"AUTH": ResourceReads, "NOOP": "", "poke": "", "": "",
	} {
		if got := chargedResource(verb); got != want {
			t.Errorf("chargedResource(%q) = %q, want %q", verb, got, want)
		}
	}
}

func TestAllowance(t *testing.T) {
	writes := 2
	a := newAllowance(BudgetConfig{Writes: &writes})

	for i := range 2 {
		if !a.admit(ResourceWrites) {
			t.Fatalf("write %d refused within the allowance", i)
		}
	}
	if a.admit(ResourceWrites) {
		t.Error("third write admitted past an allowance of 2")
	}
	for range 5 {
		if !a.admit(ResourceReads) || !a.admit("") {
			t.Fatal("unmetered command refused")
		}
	}

	if got, want := a.remaining(), map[string]int{ResourceWrites: 0}; !maps.Equal(got, want) {
		t.Errorf("remaining = %v, want %v", got, want)
	}
	if got := (*allowance)(nil).remaining(); len(got) != 0 {
		t.Errorf("nil allowance remaining = %v, want empty", got)
	}
}

func TestBudgetUsed(t *testing.T) {
	log := []CommandLogEntry{
		{Verb: "POKE", Charged: ResourceWrites, Result: "NAK E04"},
		{Verb: "POKE", Charged: ResourceWrites, Result: "ACK POKE 1"},
		{Verb: "PEEK", Charged: ResourceReads},
		{Verb: "NOOP"},
	}
	want := map[string]int{ResourceWrites: 2, ResourceReads: 1}
	if got := budgetUsed(log); !maps.Equal(got, want) {
		t.Errorf("budgetUsed = %v, want %v", got, want)
	}
	if got := budgetUsed(nil); !maps.Equal(got, map[string]int{ResourceWrites: 0, ResourceReads: 0}) {
		t.Errorf("budgetUsed(nil) = %v, want both resources at 0", got)
	}
}

func TestStartEnforcesAllowances(t *testing.T) {
	scaleLinkDelays(t, 20*time.Millisecond)
	const downlinkDelay = 100 * time.Millisecond

	launches := stubFirmware(t)
	state := readyState(t,
		Objective{ID: "one-write", Success: Predicate{Op: "budget", Resource: ResourceWrites, Cmp: "eq", Value: 1.0}},
		Objective{ID: "one-read", Success: Predicate{Op: "budget", Resource: ResourceReads, Cmp: "eq", Value: 1.0}},
	)
	writes, reads := 1, 1
	state.Scenario.Manifest.Link.Budget = BudgetConfig{Writes: &writes, Reads: &reads}
	state.Scenario.Manifest.Link.DownlinkDelayS = 5
	state.grader = NewGrader(state, "127.0.0.1:1")

	out := make(chan DownlinkMessage)
	up := make(chan []byte)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	l.downlink <- []byte("SOJOURN FSW v1.0 boot\r\n")
	recvDownlink(t, out)

	forwarded := func(raw string) {
		t.Helper()
		up <- []byte(raw)
		if got := recvUplink(t, l.uplink); got != raw {
			t.Fatalf("firmware uplink = %q, want %q", got, raw)
		}
	}
	refused := func(raw, reason string) {
		t.Helper()
		sent := time.Now()
		up <- []byte(raw)
		msg, ok := recvDownlink(t, out).(*CommandRefusedDownlinkMessage)
		if !ok || msg.Command != raw || msg.Reason != reason {
			t.Fatalf("downlink = %#v, want %q refused for %q", msg, raw, reason)
		}
		if elapsed := time.Since(sent); elapsed >= downlinkDelay {
			t.Errorf("refusal took %s; it should not wait for the downlink delay", elapsed)
		}
	}

	forwarded("POKE 0x20000010 01")
	refused("POKE 0x20000010 02", "writes allowance exhausted")
	forwarded("NOOP")
	forwarded("PEEK 0x20000010 1")
	refused("PING", "reads allowance exhausted")

	// The probe rejecting a command still costs the allowance.
	for _, line := range []string{"ACK POKE 1", "ACK NOOP", "NAK E05"} {
		l.downlink <- []byte(line + "\r\n")
		if msg, ok := recvDownlink(t, out).(*CommandResponseDownlinkMessage); !ok || msg.Response != line {
			t.Fatalf("downlink = %#v, want %q", msg, line)
		}
	}

	l.downlink <- []byte(tlmLine(0) + "\r\n")
	stateMessage, ok := recvDownlink(t, out).(*StateDownlinkMessage)
	if !ok {
		t.Fatal("expected a state message")
	}
	if want := map[string]int{ResourceWrites: 0, ResourceReads: 0}; !maps.Equal(stateMessage.Constraints, want) {
		t.Errorf("constraints = %v, want %v", stateMessage.Constraints, want)
	}
	for _, o := range stateMessage.Objectives {
		if o.Status != ObjectiveStatusComplete {
			t.Errorf("%s = %s, want complete: refused commands were charged or charged ones were not", o.ID, o.Status)
		}
	}

	close(up)
	close(l.downlink)
	for range out {
	}
}

// Leaving after the game has ended must not strand a refusal.
func TestRefusalAfterDownlinkEnds(t *testing.T) {
	launches := stubFirmware(t)
	state := readyState(t)
	writes := 0
	state.Scenario.Manifest.Link.Budget = BudgetConfig{Writes: &writes}

	out := make(chan DownlinkMessage)
	up := make(chan []byte)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	close(l.downlink)
	for range out {
	}

	select {
	case up <- []byte("POKE 0x20000010 01"):
	case <-time.After(5 * time.Second):
		t.Fatal("uplink blocked after the downlink ended")
	}
	close(up)
	if _, ok := <-l.uplink; ok {
		t.Error("refused command reached the firmware")
	}
}
