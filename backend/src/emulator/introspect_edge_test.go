package emulator_test

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"sojourn/emulator"
)

// scripted runs handle against the first connection to a fresh listener.
func scripted(t *testing.T, handle func(conn net.Conn, r *bufio.Reader)) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	accepted := make(chan net.Conn, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		ln.Close()
		select {
		case conn := <-accepted:
			conn.Close()
		default:
		}
		<-done
	})

	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
		defer conn.Close()
		handle(conn, bufio.NewReader(conn))
	}()

	return ln.Addr().String()
}

// readPacket returns the payload of the next client packet, skipping acks and
// interrupts.
func readPacket(r *bufio.Reader) (string, error) {
	if _, err := r.ReadString('$'); err != nil {
		return "", err
	}
	payload, err := r.ReadString('#')
	if err != nil {
		return "", err
	}
	if _, err := r.Discard(2); err != nil {
		return "", err
	}
	return payload[:len(payload)-1], nil
}

func reply(conn net.Conn, payload string) {
	fmt.Fprintf(conn, "$%s#%02x", payload, checksum(payload))
}

// reset closes conn with an RST so the client's next write fails.
func reset(conn net.Conn) {
	conn.(*net.TCPConn).SetLinger(0)
	conn.Close()
}

func TestConnectFailsWhenStubHangsUp(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply string
	}{
		{"before reply", ""},
		{"mid body", "$PacketSize=1000"},
		{"mid checksum", "$OK#9"},
		{"bad checksum", "$OK#00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr := scripted(t, func(conn net.Conn, r *bufio.Reader) {
				if _, err := readPacket(r); err != nil {
					return
				}
				conn.Write([]byte(tc.reply))
			})

			g, err := emulator.Connect(addr)
			if err == nil {
				g.Close()
				t.Fatal("expected Connect to fail")
			}
			if g != nil {
				t.Fatal("Connect returned a connection alongside its error")
			}
		})
	}
}

// A stub that omits or garbles PacketSize leaves the client on its 1024-byte
// default: reads are split into (1024-4)/2 = 510 byte chunks.
func TestDefaultPacketSize(t *testing.T) {
	for _, features := range []string{"qXfer:features:read+", "PacketSize=zz;swbreak+", ""} {
		t.Run(features, func(t *testing.T) {
			requests := make(chan string, 16)
			addr := scripted(t, func(conn net.Conn, r *bufio.Reader) {
				defer close(requests)
				for {
					if b, err := r.Peek(1); err == nil && b[0] == 0x03 {
						r.ReadByte()
						reply(conn, "T05")
						continue
					}
					p, err := readPacket(r)
					if err != nil {
						return
					}
					conn.Write([]byte("+"))
					switch {
					case strings.HasPrefix(p, "qSupported"):
						reply(conn, features)
					case strings.HasPrefix(p, "m"):
						requests <- p
						var addr, n int
						fmt.Sscanf(p, "m%x,%x", &addr, &n)
						reply(conn, strings.Repeat("ab", n))
					case p == "D":
						reply(conn, "OK")
						return
					}
				}
			})

			g, err := emulator.Connect(addr)
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}
			halt(t, g)

			data, err := g.Read(sramBase, 1100)
			if err != nil || len(data) != 1100 {
				t.Fatalf("Read = %d bytes, %v", len(data), err)
			}
			g.Close()

			var got []string
			for p := range requests {
				got = append(got, p)
			}
			want := []string{"m20000000,1fe", "m200001fe,1fe", "m200003fc,50"}
			if strings.Join(got, " ") != strings.Join(want, " ") {
				t.Fatalf("requests = %v, want %v", got, want)
			}
		})
	}
}

func TestHaltAndResumeAreIdempotent(t *testing.T) {
	s := newStub(t, stubConfig{})
	g := connect(t, s)

	if err := g.Resume(); err != nil {
		t.Fatalf("Resume while running: %v", err)
	}
	halt(t, g)
	halt(t, g)

	// Still halted after the second Halt, and the connection is in sync.
	got, err := g.Read(sramBase+0x10, 4)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if want := s.get(sramBase+0x10, 4); string(got) != string(want) {
		t.Fatalf("Read = %x, want %x", got, want)
	}

	if err := g.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if err := g.Resume(); err != nil {
		t.Fatalf("second Resume: %v", err)
	}
}

func TestOperationsAfterCloseFail(t *testing.T) {
	s := newStub(t, stubConfig{})
	g, err := emulator.Connect(s.addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	g.Close()

	if err := g.Halt(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Halt after Close = %v, want net.ErrClosed", err)
	}

	snap, err := emulator.NewMemorySnapshot(g, []emulator.MemoryRange{{Addr: sramBase, Len: 4}})
	if err == nil || snap != nil || !strings.Contains(err.Error(), "halting") {
		t.Fatalf("NewMemorySnapshot after Close = %v, %v; want a halting error", snap, err)
	}
}

func TestReadAfterCloseWhileHaltedFails(t *testing.T) {
	s := newStub(t, stubConfig{})
	g, err := emulator.Connect(s.addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	halt(t, g)
	g.Close()

	data, err := g.Read(sramBase, 4)
	requireError(t, data, err)
	if !errors.Is(err, net.ErrClosed) {
		t.Fatalf("err = %v, want net.ErrClosed", err)
	}
	if err := g.Resume(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Resume after Close = %v, want net.ErrClosed", err)
	}
}

func TestHaltFailsWithoutStopReply(t *testing.T) {
	addr := scripted(t, func(conn net.Conn, r *bufio.Reader) {
		readPacket(r)
		conn.Write([]byte("+"))
		reply(conn, "PacketSize=1000")
		readPacket(r) // c
		r.ReadByte()  // interrupt
		conn.Close()
	})

	g, err := emulator.Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer g.Close()

	if err := g.Halt(); err == nil {
		t.Fatal("Halt succeeded without a stop reply")
	}
	data, err := g.Read(sramBase, 4)
	requireError(t, data, err)
}

func TestSnapshotReportsResumeFailure(t *testing.T) {
	addr := scripted(t, func(conn net.Conn, r *bufio.Reader) {
		readPacket(r)
		conn.Write([]byte("+"))
		reply(conn, "PacketSize=1000")
		readPacket(r) // c
		r.ReadByte()  // interrupt
		reply(conn, "T05")
		readPacket(r) // m
		reset(conn)
	})

	g, err := emulator.Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer g.Close()

	snap, err := emulator.NewMemorySnapshot(g, []emulator.MemoryRange{{Addr: sramBase, Len: 4}})
	if err == nil || snap != nil {
		t.Fatalf("NewMemorySnapshot = %v, %v; want an error", snap, err)
	}
	if !strings.Contains(err.Error(), "reading receive") || !strings.Contains(err.Error(), "resuming") {
		t.Fatalf("err = %v, want both the read and the resume failure", err)
	}
}

func TestConnectClosesConnectionOnFailure(t *testing.T) {
	closed := make(chan struct{})
	addr := scripted(t, func(conn net.Conn, r *bufio.Reader) {
		defer close(closed)
		if _, err := readPacket(r); err != nil {
			return
		}
		conn.Write([]byte("$OK#00"))
		io.Copy(io.Discard, r)
	})

	if g, err := emulator.Connect(addr); err == nil {
		g.Close()
		t.Fatal("expected Connect to fail")
	}

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Connect left the connection open after failing")
	}
}

// windowStub answers like QEMU, but delays its stop reply and memory replies,
// and records every packet and interrupt the client sends.
func windowStub(t *testing.T, stopDelay, readDelay time.Duration, answerStop bool) (string, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string

	addr := scripted(t, func(conn net.Conn, r *bufio.Reader) {
		for {
			b, err := r.ReadByte()
			if err != nil {
				return
			}
			switch b {
			case 0x03:
				mu.Lock()
				seen = append(seen, "halt")
				mu.Unlock()
				if answerStop {
					time.Sleep(stopDelay)
					reply(conn, "T05")
				}
				continue
			case '$':
			default:
				continue
			}

			body, err := r.ReadString('#')
			if err != nil {
				return
			}
			r.Discard(2)
			conn.Write([]byte("+"))
			payload := body[:len(body)-1]

			mu.Lock()
			seen = append(seen, payload)
			mu.Unlock()

			switch {
			case payload == "qSupported":
				reply(conn, "PacketSize=1000")
			case strings.HasPrefix(payload, "m"):
				time.Sleep(readDelay)
				reply(conn, "00")
			case payload == "D":
				reply(conn, "OK")
				return
			}
		}
	})

	return addr, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(seen)
	}
}

func TestSnapshotWindowIsBounded(t *testing.T) {
	for _, tc := range []struct {
		name       string
		stopDelay  time.Duration
		readDelay  time.Duration
		answerStop bool
	}{
		{"slow reads", 0, 400 * time.Millisecond, true},
		{"stop reply never arrives", 0, 0, false},
		{"slow stop reply", 400 * time.Millisecond, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, seen := windowStub(t, tc.stopDelay, tc.readDelay, tc.answerStop)
			in, err := emulator.Connect(addr)
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}
			defer in.Close()

			start := time.Now()
			snap, err := emulator.NewMemorySnapshot(in, []emulator.MemoryRange{{Addr: 0x20000000, Len: 1}})
			elapsed := time.Since(start)

			if !errors.Is(err, emulator.ErrSnapshotWindow) || snap != nil {
				t.Fatalf("NewMemorySnapshot = %v, %v; want ErrSnapshotWindow", snap, err)
			}
			if elapsed > emulator.SnapshotWindow+150*time.Millisecond {
				t.Errorf("gave up after %s; the window is %s", elapsed, emulator.SnapshotWindow)
			}

			// The guest was asked to halt, so it must be told to continue,
			// even though the stop reply may never have arrived.
			deadline := time.Now().Add(time.Second)
			for {
				packets := seen()
				halt := slices.Index(packets, "halt")
				if halt >= 0 && slices.Contains(packets[halt:], "c") {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("no continue after the halt: %q", packets)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

func TestSnapshotWithinWindowSucceeds(t *testing.T) {
	addr, _ := windowStub(t, 0, 50*time.Millisecond, true)
	in, err := emulator.Connect(addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer in.Close()

	snap, err := emulator.NewMemorySnapshot(in, []emulator.MemoryRange{{Addr: 0x20000000, Len: 1}})
	if err != nil {
		t.Fatalf("NewMemorySnapshot: %v", err)
	}
	if data, err := snap.Read(0x20000000, 1); err != nil || data[0] != 0 {
		t.Errorf("Read = %v, %v", data, err)
	}

	// The window's deadline must not leak into later exchanges.
	time.Sleep(emulator.SnapshotWindow + 50*time.Millisecond)
	if _, err := emulator.NewMemorySnapshot(in, []emulator.MemoryRange{{Addr: 0x20000000, Len: 1}}); err != nil {
		t.Errorf("second capture: %v", err)
	}
}
