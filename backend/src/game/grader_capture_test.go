package game

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"sojourn/emulator"
	"strings"
	"testing"
)

// gdbStub is a minimal GDB remote stub serving reads of mem, based at SRAM.
// With failReads set every memory read returns an error reply.
type gdbStub struct {
	addr      string
	mem       memory
	failReads bool
}

func newGDBStub(t *testing.T, mem memory, failReads bool) *gdbStub {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })

	s := &gdbStub{addr: ln.Addr().String(), mem: mem, failReads: failReads}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	return s
}

func (s *gdbStub) reply(conn net.Conn, payload string) {
	var sum byte
	for i := 0; i < len(payload); i++ {
		sum += payload[i]
	}
	fmt.Fprintf(conn, "$%s#%02x", payload, sum)
}

func (s *gdbStub) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return
		}
		switch b {
		case 0x03:
			s.reply(conn, "S05")
			continue
		case '$':
		default:
			continue
		}

		body, err := r.ReadString('#')
		if err != nil {
			return
		}
		if _, err := r.Discard(2); err != nil {
			return
		}
		conn.Write([]byte("+"))

		payload := strings.TrimSuffix(body, "#")
		switch {
		case payload == "qSupported":
			s.reply(conn, "PacketSize=400")
		case payload == "D":
			s.reply(conn, "OK")
			return
		case strings.HasPrefix(payload, "m"):
			var addr uint32
			var n int
			if _, err := fmt.Sscanf(payload, "m%x,%x", &addr, &n); err != nil || s.failReads {
				s.reply(conn, "E01")
				continue
			}
			data, _ := s.mem.Read(addr, n)
			s.reply(conn, hex.EncodeToString(data))
		}
	}
}

func TestGraderCapturesMemory(t *testing.T) {
	stub := newGDBStub(t, sram(0x10, 2), false)

	state := readyState(t, modeIs2)
	startGame(t, state)
	g := NewGrader(state, stub.addr)

	for range 2 {
		if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err != nil {
			t.Fatalf("Grade: %v", err)
		}
	}
	if got := status(t, &state.evaluation, "mode"); got != ObjectiveStatusComplete {
		t.Fatalf("status = %s, want complete", got)
	}

	if err := g.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if g.introspect != nil {
		t.Fatal("Close kept the introspection connection")
	}
	if err := g.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestGraderCaptureFailureDropsConnection(t *testing.T) {
	stub := newGDBStub(t, sram(0), true)

	state := readyState(t, modeIs2)
	startGame(t, state)
	g := NewGrader(state, stub.addr)
	defer g.Close()

	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err == nil {
		t.Fatal("expected Grade to fail when memory reads fail")
	}
	if g.introspect != nil {
		t.Fatal("failed capture kept the introspection connection")
	}
	if got := status(t, &state.evaluation, "mode"); got != ObjectiveStatusActive {
		t.Fatalf("status = %s, want active", got)
	}
}

func TestGraderEvaluationError(t *testing.T) {
	state := readyState(t, Objective{
		ID:      "bits",
		Success: Predicate{Op: "tlm_bits", Path: "mode", Mask: 1, Cmp: "eq", Value: 1.0},
	})
	startGame(t, state)
	g := NewGrader(state, "127.0.0.1:1")

	// mode is a string, which cannot be masked.
	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true, Mode: "SAFE"}); err == nil {
		t.Fatal("expected Grade to report the evaluation error")
	}
}

func TestGraderInProgressWithoutScenario(t *testing.T) {
	g := NewGrader(&State{phase: PhaseInProgress}, "127.0.0.1:1")
	if err := g.Grade(&emulator.TelemetryFrame{CRCOK: true}); err == nil {
		t.Fatal("expected Grade to fail with no scenario selected")
	}
}
