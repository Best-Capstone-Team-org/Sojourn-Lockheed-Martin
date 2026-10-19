package emulator

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUARTReaderClosesDownlinkOnReadError(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()

	downlink := make(chan []byte)
	go uartReader(local, downlink)

	go remote.Write([]byte("TLM 01\n"))
	if got := <-downlink; string(got) != "TLM 01\n" {
		t.Fatalf("downlink = %q", got)
	}

	// Closing our own end is a read error, not EOF.
	local.Close()

	select {
	case msg, ok := <-downlink:
		if ok {
			t.Fatalf("unexpected downlink %q", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("downlink was not closed")
	}
}

func TestUARTWriterSurvivesWriteErrors(t *testing.T) {
	local, remote := net.Pipe()
	remote.Close()

	uplink := make(chan []byte)
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		uartWriter(local, uplink)
	}()

	// Writes fail, but the writer keeps draining uplink until it is closed.
	for range 3 {
		select {
		case uplink <- []byte("PING"):
		case <-time.After(5 * time.Second):
			t.Fatal("writer stopped accepting uplink messages")
		}
	}
	close(uplink)

	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("uartWriter did not return after uplink closed")
	}
}

func TestDialUARTTimesOut(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	start := time.Now()
	conn, err := dialUART(addr, 300*time.Millisecond, make(chan error))
	if err == nil {
		conn.Close()
		t.Fatal("expected dialUART to fail")
	}
	if !strings.Contains(err.Error(), "timed out after 300ms") {
		t.Fatalf("err = %v, want a timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("dialUART took %s", elapsed)
	}
}

func TestFindQEMUDefaultsToQEMUSystemARMOnPATH(t *testing.T) {
	binDir := t.TempDir()
	want := filepath.Join(binDir, "qemu-system-arm")
	if err := os.WriteFile(want, nil, 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv(qemuBinEnv, "")
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty")+string(os.PathListSeparator)+binDir)

	got, err := findQEMUSystemARM()
	if err != nil {
		t.Fatalf("findQEMUSystemARM: %v", err)
	}
	if got != want {
		t.Fatalf("found %q, want %q", got, want)
	}
}
