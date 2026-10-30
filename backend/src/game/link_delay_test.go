package game

import (
	"testing"
	"time"
)

func scaleLinkDelays(t *testing.T, unit time.Duration) {
	t.Helper()
	prev := linkDelayUnit
	linkDelayUnit = unit
	t.Cleanup(func() { linkDelayUnit = prev })
}

// Each message is held for the delay from when it arrived, not from when the
// previous one left, so a burst arrives as a burst.
func TestDelayedIsLatencyNotThroughput(t *testing.T) {
	const delay = 100 * time.Millisecond
	in, out := make(chan int), make(chan int)
	go delayed(in, out, delay, nil)

	start := time.Now()
	for i := range 5 {
		in <- i
	}
	close(in)

	for i := range 5 {
		got, ok := <-out
		if !ok || got != i {
			t.Fatalf("message %d = %d (open %v), want in order", i, got, ok)
		}
		if elapsed := time.Since(start); elapsed < delay {
			t.Fatalf("message %d arrived after %s, before the %s delay", i, elapsed, delay)
		}
	}
	if elapsed := time.Since(start); elapsed > 2*delay {
		t.Errorf("burst of 5 took %s; delays are adding up instead of overlapping", elapsed)
	}
	if _, ok := <-out; ok {
		t.Error("out not closed after in closed")
	}
}

func TestDelayedFlushesOnStop(t *testing.T) {
	in, out := make(chan string), make(chan string)
	stop := make(chan struct{})
	go delayed(in, out, time.Hour, stop)

	in <- "late"
	close(stop)
	select {
	case got := <-out:
		if got != "late" {
			t.Errorf("got %q, want late", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued message not released after stop")
	}
	close(in)
	if _, ok := <-out; ok {
		t.Error("out not closed")
	}
}

func TestStartAppliesLinkDelays(t *testing.T) {
	scaleLinkDelays(t, 20*time.Millisecond)
	const delay = 100 * time.Millisecond

	launches := stubFirmware(t)
	state := readyState(t, Objective{
		ID:      "safe",
		Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "BOOT"},
	})
	state.Scenario.Manifest.Link.UplinkDelayS = 5
	state.Scenario.Manifest.Link.DownlinkDelayS = 5
	u8 := uint8(1)
	state.Scenario.Setup = &Setup{Writes: []SetupWrite{{At: AddressRef{Sym: "g_mode"}, U8: &u8}}}
	state.grader = NewGrader(state, "127.0.0.1:1")

	out := make(chan DownlinkMessage)
	up := make(chan []byte, 2)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	sent := time.Now()
	l.downlink <- []byte("SOJOURN FSW v1.0 boot\r\n")

	if got := recvUplink(t, l.uplink); got != "POKE 0x20000010 01" {
		t.Fatalf("firmware uplink = %q, want the setup write", got)
	}
	if elapsed := time.Since(sent); elapsed >= delay {
		t.Errorf("setup write took %s; it should not wait for the uplink delay", elapsed)
	}

	recvDownlink(t, out)
	if elapsed := time.Since(sent); elapsed < delay {
		t.Errorf("boot banner reached the player after %s, before the downlink delay", elapsed)
	}

	sent = time.Now()
	up <- []byte("PING")
	up <- []byte("NOOP")
	for _, want := range []string{"PING", "NOOP"} {
		if got := recvUplink(t, l.uplink); got != want {
			t.Fatalf("firmware uplink = %q, want %q", got, want)
		}
		if elapsed := time.Since(sent); elapsed < delay {
			t.Errorf("%s reached the firmware after %s, before the uplink delay", want, elapsed)
		}
	}
	if elapsed := time.Since(sent); elapsed > delay+delay/2 {
		t.Errorf("two commands took %s; uplink delays are adding up", elapsed)
	}

	// The player hears about the objective alongside the frame that completed
	// it, not ahead of it.
	sent = time.Now()
	l.downlink <- []byte(tlmLine(0) + "\r\n")
	stateMessage, ok := recvDownlink(t, out).(*StateDownlinkMessage)
	if !ok {
		t.Fatal("expected the state message first")
	}
	if elapsed := time.Since(sent); elapsed < delay {
		t.Errorf("state reached the player after %s, ahead of its telemetry", elapsed)
	}
	if len(stateMessage.Objectives) != 1 || stateMessage.Objectives[0].Status != ObjectiveStatusComplete {
		t.Errorf("state = %+v, want safe complete", stateMessage.Objectives)
	}
	if _, ok := recvDownlink(t, out).(*TLMDownlinkMessage); !ok {
		t.Fatal("expected the telemetry message")
	}

	close(up)
	close(l.downlink)
	for range out {
	}
}
