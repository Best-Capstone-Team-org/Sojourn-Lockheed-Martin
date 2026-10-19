package emulator_test

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

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
