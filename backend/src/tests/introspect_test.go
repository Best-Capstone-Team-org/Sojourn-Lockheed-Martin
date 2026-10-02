package tests

import (
	"bufio"
	"bytes"
	"debug/elf"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"sojourn/introspect"
)

// ---- fake GDB stub ----
//
// A minimal RSP server backed by one region of memory. It answers the packets
// a conforming daemon may send, and can be configured to misbehave the ways a
// real stub (or a broken connection) might.

const (
	sramBase = 0x20000000
	sramSize = 0x4000
)

type stubFault int

const (
	faultNone stubFault = iota
	faultShortRead
	faultNonHex
	faultBadChecksum
	faultEmptyReply
	faultOddLengthHex
	faultTrailingRLE
	faultNegativeRLE
	faultCloseAfterAttach
)

type stubConfig struct {
	packetSize int
	rle        bool
	replyDelay time.Duration
	fault      stubFault
}

type stub struct {
	cfg  stubConfig
	addr string

	mu  sync.Mutex
	mem []byte
}

func newStub(t *testing.T, cfg stubConfig) *stub {
	t.Helper()

	if cfg.packetSize == 0 {
		cfg.packetSize = 0x1000
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	s := &stub{cfg: cfg, addr: ln.Addr().String(), mem: make([]byte, sramSize)}
	for i := range s.mem {
		s.mem[i] = byte(i * 7)
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		s.serve(conn)
	}()

	return s
}

func (s *stub) set(addr uint32, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy(s.mem[addr-sramBase:], data)
}

func (s *stub) get(addr uint32, n int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return bytes.Clone(s.mem[addr-sramBase : int(addr-sramBase)+n])
}

func (s *stub) serve(conn net.Conn) {
	r := bufio.NewReader(conn)

	for {
		b, err := r.ReadByte()
		if err != nil {
			return
		}

		switch b {
		case '+', '-':
			continue
		case 0x03:
			s.send(conn, "T05thread:01;", false)
			continue
		case '$':
		default:
			continue
		}

		payload, err := r.ReadString('#')
		if err != nil {
			return
		}
		payload = payload[:len(payload)-1]

		// A stub that cannot trust the client's framing hangs up, which
		// fails whichever call sent the packet.
		var sum [2]byte
		if _, err := io.ReadFull(r, sum[:]); err != nil {
			return
		}
		if want, err := strconv.ParseUint(string(sum[:]), 16, 8); err != nil || byte(want) != checksum(payload) {
			return
		}
		conn.Write([]byte("+"))

		if !s.handle(conn, payload) {
			return
		}
	}
}

// handle answers one packet; false closes the connection.
func (s *stub) handle(conn net.Conn, payload string) bool {
	switch {
	case strings.HasPrefix(payload, "qSupported"):
		s.send(conn, fmt.Sprintf("PacketSize=%x;qXfer:features:read+", s.cfg.packetSize), false)
		return s.cfg.fault != faultCloseAfterAttach
	case payload == "c":
		return true
	case payload == "D":
		s.send(conn, "OK", false)
		return false
	case payload == "?":
		s.send(conn, "S05", false)
		return true
	case strings.HasPrefix(payload, "m"):
		s.readMem(conn, payload[1:])
		return true
	default:
		s.send(conn, "", false)
		return true
	}
}

func (s *stub) readMem(conn net.Conn, args string) {
	time.Sleep(s.cfg.replyDelay)

	addrHex, lenHex, _ := strings.Cut(args, ",")
	addr, err1 := strconv.ParseUint(addrHex, 16, 32)
	n, err2 := strconv.ParseUint(lenHex, 16, 32)
	if err1 != nil || err2 != nil {
		s.send(conn, "E01", false)
		return
	}

	if addr < sramBase || addr+n > sramBase+sramSize {
		s.send(conn, "E14", false)
		return
	}

	// Like QEMU, refuse a read whose reply would not fit in one packet.
	if int(n)*2+4 > s.cfg.packetSize {
		s.send(conn, "E22", false)
		return
	}

	reply := hex.EncodeToString(s.get(uint32(addr), int(n)))

	switch s.cfg.fault {
	case faultShortRead:
		if n > 1 {
			reply = reply[:len(reply)-2]
		}
	case faultNonHex:
		reply = "zz" + reply[2:]
	case faultEmptyReply:
		reply = ""
	case faultOddLengthHex:
		reply = reply[:len(reply)-1]
	}

	if s.cfg.rle {
		reply = rleEncode(reply)
	}

	switch s.cfg.fault {
	case faultTrailingRLE:
		// "*" with no repeat count after it.
		reply += "*"
	case faultNegativeRLE:
		// A repeat count below 29 encodes a negative run.
		reply = reply[:2] + "*\x1c" + reply[2:]
	}

	s.send(conn, reply, s.cfg.fault == faultBadChecksum)
}

func (s *stub) send(conn net.Conn, payload string, corrupt bool) {
	sum := checksum(payload)
	if corrupt {
		sum++
	}
	fmt.Fprintf(conn, "$%s#%02x", payload, sum)
}

func checksum(payload string) byte {
	var sum byte
	for i := 0; i < len(payload); i++ {
		sum += payload[i]
	}
	return sum
}

// rleEncode applies RSP run-length encoding. Repeat counts are capped so the
// count character never lands on '#' or '$'.
func rleEncode(data string) string {
	var b strings.Builder
	for i := 0; i < len(data); {
		j := i + 1
		for j < len(data) && data[j] == data[i] && j-i < 6 {
			j++
		}
		run := j - i
		b.WriteByte(data[i])
		if run >= 4 {
			b.WriteByte('*')
			b.WriteByte(byte(29 + run - 1))
		} else {
			b.WriteString(strings.Repeat(string(data[i]), run-1))
		}
		i = j
	}
	return b.String()
}

// ---- helpers ----

func connect(t *testing.T, s *stub) *introspect.GDB {
	t.Helper()
	g, err := introspect.Connect(s.addr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

func halt(t *testing.T, g *introspect.GDB) {
	t.Helper()
	if err := g.Halt(); err != nil {
		t.Fatalf("Halt: %v", err)
	}
}

func requireError(t *testing.T, data []byte, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error, got data %x", data)
	}
	if data != nil {
		t.Fatalf("expected no data alongside the error, got %x", data)
	}
}

// ---- reads ----

func TestReadReturnsMemoryContents(t *testing.T) {
	cases := []struct {
		name string
		addr uint32
		n    int
	}{
		{"one byte", sramBase + 0x433, 1},
		{"u16", sramBase + 0x10, 2},
		{"u32", sramBase + 0x20, 4},
		{"unaligned", sramBase + 0x101, 13},
		{"exactly one chunk", sramBase, 1024},
		{"one past a chunk", sramBase, 1025},
		{"frame buffer size", sramBase + 0x100, 9216},
		{"last byte of region", sramBase + sramSize - 1, 1},
	}

	s := newStub(t, stubConfig{})
	g := connect(t, s)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			halt(t, g)
			defer g.Resume()

			got, err := g.Read(tc.addr, tc.n)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if want := s.get(tc.addr, tc.n); !bytes.Equal(got, want) {
				t.Fatalf("got %d bytes that differ from memory", len(got))
			}
		})
	}
}

func TestReadWithRunLengthEncodedReplies(t *testing.T) {
	s := newStub(t, stubConfig{rle: true})
	s.set(sramBase, make([]byte, 256))
	s.set(sramBase+0x200, bytes.Repeat([]byte{0xAA}, 64))
	s.set(sramBase+0x300, []byte{0x11, 0x11, 0x11, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12})

	g := connect(t, s)
	halt(t, g)
	defer g.Resume()

	for _, r := range []struct {
		addr uint32
		n    int
	}{{sramBase, 256}, {sramBase + 0x200, 64}, {sramBase + 0x300, 11}, {sramBase + 0x400, 2048}} {
		got, err := g.Read(r.addr, r.n)
		if err != nil {
			t.Fatalf("Read(0x%08X, %d): %v", r.addr, r.n, err)
		}
		if want := s.get(r.addr, r.n); !bytes.Equal(got, want) {
			t.Fatalf("Read(0x%08X, %d) = %x, want %x", r.addr, r.n, got, want)
		}
	}
}

func TestReadWithSmallPacketSize(t *testing.T) {
	// A stub that advertises a small PacketSize rejects any single read that
	// would not fit; the client must still return the full range.
	s := newStub(t, stubConfig{packetSize: 0x100})
	g := connect(t, s)
	halt(t, g)
	defer g.Resume()

	got, err := g.Read(sramBase, 4096)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if want := s.get(sramBase, 4096); !bytes.Equal(got, want) {
		t.Fatal("data differs from memory")
	}
}

func TestEachHaltSeesCurrentMemory(t *testing.T) {
	s := newStub(t, stubConfig{})
	g := connect(t, s)
	addr := uint32(sramBase + 0x80)

	for i := 0; i < 50; i++ {
		want := []byte{byte(i), byte(i >> 8)}
		s.set(addr, want)

		halt(t, g)
		got, err := g.Read(addr, 2)
		if err != nil {
			t.Fatalf("halt %d: %v", i, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("halt %d: got %x, want %x", i, got, want)
		}
		if err := g.Resume(); err != nil {
			t.Fatalf("halt %d Resume: %v", i, err)
		}
	}
}

// ---- failures are errors, never data ----

func TestReadOutsideMappedMemoryFails(t *testing.T) {
	s := newStub(t, stubConfig{})
	g := connect(t, s)

	for _, r := range []struct {
		name string
		addr uint32
		n    int
	}{
		{"below region", sramBase - 4, 4},
		{"above region", sramBase + sramSize, 4},
		{"straddles end", sramBase + sramSize - 2, 4},
		{"later chunk runs off end", sramBase + sramSize - 1500, 3000},
	} {
		t.Run(r.name, func(t *testing.T) {
			halt(t, g)
			defer g.Resume()
			data, err := g.Read(r.addr, r.n)
			requireError(t, data, err)

			// An error reply must not desynchronise the connection.
			got, err := g.Read(sramBase+0x10, 8)
			if err != nil {
				t.Fatalf("Read after failed read: %v", err)
			}
			if want := s.get(sramBase+0x10, 8); !bytes.Equal(got, want) {
				t.Fatalf("Read after failed read = %x, want %x", got, want)
			}
		})
	}
}

func TestMalformedRepliesFail(t *testing.T) {
	for _, tc := range []struct {
		name  string
		fault stubFault
	}{
		{"short read", faultShortRead},
		{"non-hex data", faultNonHex},
		{"bad checksum", faultBadChecksum},
		{"empty reply", faultEmptyReply},
		{"odd-length hex", faultOddLengthHex},
		{"trailing run-length marker", faultTrailingRLE},
		{"negative run length", faultNegativeRLE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStub(t, stubConfig{fault: tc.fault})
			g := connect(t, s)
			halt(t, g)
			defer g.Resume()

			data, err := g.Read(sramBase, 16)
			requireError(t, data, err)
		})
	}
}

func TestLostConnectionFails(t *testing.T) {
	// The stub hangs up right after qSupported. Connect may or may not notice
	// (the "c" it sends has no reply), but no later read may produce data.
	s := newStub(t, stubConfig{fault: faultCloseAfterAttach})
	g, err := introspect.Connect(s.addr)
	if err != nil {
		if g != nil {
			t.Fatal("Connect returned a connection alongside its error")
		}
		return
	}
	defer g.Close()

	if err := g.Halt(); err == nil {
		t.Fatal("Halt succeeded on a closed connection")
	}

	data, err := g.Read(sramBase, 4)
	requireError(t, data, err)
}

func TestReadWhileRunningFails(t *testing.T) {
	s := newStub(t, stubConfig{})
	g := connect(t, s)

	data, err := g.Read(sramBase, 4)
	requireError(t, data, err)
}

func TestReadAfterResumeFails(t *testing.T) {
	s := newStub(t, stubConfig{})
	g := connect(t, s)
	halt(t, g)
	if err := g.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	data, err := g.Read(sramBase+0x500, 4)
	requireError(t, data, err)
}

func TestConnectWithNothingListeningFails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	g, err := introspect.Connect(addr)
	if err == nil {
		g.Close()
		t.Fatal("expected Connect to fail")
	}
	if g != nil {
		t.Fatal("Connect returned a connection alongside its error")
	}
}

// ---- against the real emulator ----

// Set SOJOURN_FIRMWARE to a probe ELF (e.g. deps/sojourn/firmware/build/probe_rom.elf)
// to run against QEMU's GDB stub.
func TestAgainstQEMU(t *testing.T) {
	firmware := os.Getenv("SOJOURN_FIRMWARE")
	if firmware == "" {
		t.Skip("SOJOURN_FIRMWARE not set")
	}
	qemu, err := exec.LookPath("qemu-system-arm")
	if err != nil {
		t.Skip("qemu-system-arm not found")
	}

	rom := loadSegments(t, firmware)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	cmd := exec.Command(qemu, "-M", "mps2-an386", "-nographic", "-monitor", "none",
		"-kernel", firmware, "-serial", "null", "-gdb", "tcp:"+addr)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start qemu: %v", err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })

	var g *introspect.GDB
	for deadline := time.Now().Add(5 * time.Second); ; {
		if g, err = introspect.Connect(addr); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Connect: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	defer g.Close()

	for i := 0; i < 3; i++ {
		time.Sleep(200 * time.Millisecond)
		halt(t, g)

		for _, seg := range rom {
			got, err := g.Read(seg.addr, len(seg.data))
			if err != nil {
				t.Fatalf("read ROM at 0x%08X: %v", seg.addr, err)
			}
			if !bytes.Equal(got, seg.data) {
				t.Fatalf("%d bytes at 0x%08X differ from the ELF", len(seg.data), seg.addr)
			}
		}

		sram, err := g.Read(0x20000000, 9216)
		if err != nil || len(sram) != 9216 {
			t.Fatalf("read 9216 bytes of SRAM: %d bytes, %v", len(sram), err)
		}

		data, err := g.Read(0x3FFFFFFC, 64)
		requireError(t, data, err)

		if err := g.Resume(); err != nil {
			t.Fatalf("Resume: %v", err)
		}
	}
}

type segment struct {
	addr uint32
	data []byte
}

// loadSegments returns the firmware's read-only loadable segments, which the
// emulator's memory must match byte for byte.
func loadSegments(t *testing.T, path string) []segment {
	t.Helper()

	f, err := elf.Open(path)
	if err != nil {
		t.Fatalf("open firmware: %v", err)
	}
	defer f.Close()

	var segs []segment
	for _, p := range f.Progs {
		if p.Type != elf.PT_LOAD || p.Flags&elf.PF_W != 0 || p.Filesz == 0 {
			continue
		}
		data := make([]byte, p.Filesz)
		if _, err := p.ReadAt(data, 0); err != nil {
			t.Fatalf("read segment at 0x%08X: %v", p.Paddr, err)
		}
		segs = append(segs, segment{uint32(p.Paddr), data})
	}

	if len(segs) == 0 {
		t.Fatal("firmware has no read-only segments")
	}
	return segs
}
