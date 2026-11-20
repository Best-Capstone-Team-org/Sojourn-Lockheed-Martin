package game

import (
	"encoding/binary"
	"errors"
	"strings"
	"testing"
	"time"
)

func appHeaderRAM(magic, crc uint32) memory {
	m := make(memory, appRAMBase-0x20000000+appHeaderLen)
	header := m[appRAMBase-0x20000000:]
	binary.LittleEndian.PutUint32(header[0:], magic)
	binary.LittleEndian.PutUint32(header[4:], 0x20001021)
	binary.LittleEndian.PutUint32(header[8:], 6491)
	binary.LittleEndian.PutUint32(header[appHeaderCRCField:], crc)
	return m
}

func TestGraderAppImageCRC(t *testing.T) {
	stub := newGDBStub(t, appHeaderRAM(appHeaderMagic, 0xf72478e0), false)
	g := NewGrader(readyState(t), stub.addr)
	defer g.Close()

	crc, err := g.AppImageCRC()
	if err != nil {
		t.Fatalf("AppImageCRC: %v", err)
	}
	if crc != 0xf72478e0 {
		t.Errorf("crc = %#08x, want 0xf72478e0", crc)
	}
}

func TestGraderAppImageCRCWithoutHeader(t *testing.T) {
	stub := newGDBStub(t, appHeaderRAM(0, 0xf72478e0), false)
	g := NewGrader(readyState(t), stub.addr)
	defer g.Close()

	if crc, err := g.AppImageCRC(); err == nil {
		t.Fatalf("AppImageCRC = %#08x with no application header, want an error", crc)
	}
}

func TestParseAppCRC(t *testing.T) {
	for in, want := range map[string]uint32{"0xf72478e0": 0xf72478e0, "0XF72478E0": 0xf72478e0, "f72478e0": 0xf72478e0} {
		if got, err := parseAppCRC(in); err != nil || got != want {
			t.Errorf("parseAppCRC(%q) = %#08x, %v; want %#08x", in, got, err, want)
		}
	}
	for _, in := range []string{"", "0x", "crc", "0x1ffffffff"} {
		if got, err := parseAppCRC(in); err == nil {
			t.Errorf("parseAppCRC(%q) = %#08x, want an error", in, got)
		}
	}
}

type bootedProbe struct {
	launch   firmwareLaunch
	downlink chan DownlinkMessage
	uplink   chan []byte
}

func startProbe(t *testing.T, state *State, read func(*State) (uint32, error)) bootedProbe {
	t.Helper()
	launches := stubFirmware(t)
	stubAppCRC(t, read)

	p := bootedProbe{downlink: make(chan DownlinkMessage), uplink: make(chan []byte)}
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	if err := state.Start(p.downlink, p.uplink, done); err != nil {
		t.Fatalf("Start: %v", err)
	}
	p.launch = <-launches

	p.uplink <- []byte("PING")
	return p
}

func noUplink(t *testing.T, ch chan []byte) {
	t.Helper()
	select {
	case got := <-ch:
		t.Fatalf("firmware received %q from a refused scenario", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func noDownlink(t *testing.T, ch chan DownlinkMessage) {
	t.Helper()
	select {
	case msg := <-ch:
		t.Fatalf("console received %#v from a refused scenario", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func recvRefusal(t *testing.T, ch chan DownlinkMessage) *ScenarioRefusedDownlinkMessage {
	t.Helper()
	msg := recvDownlink(t, ch)
	refusal, ok := msg.(*ScenarioRefusedDownlinkMessage)
	if !ok {
		t.Fatalf("downlink = %#v, want a scenario refusal", msg)
	}
	return refusal
}

func TestMismatchedAppImageRefusesScenario(t *testing.T) {
	state := readyState(t, Objective{ID: "safe", Success: Predicate{Op: "tlm", Path: "mode", Cmp: "eq", Value: "SAFE"}})
	p := startProbe(t, state, func(*State) (uint32, error) { return 0xdeadbeef, nil })

	p.launch.downlink <- []byte(bootBanner + " v1.0 boot  reboots=0 fault=0\r\n")
	refusal := recvRefusal(t, p.downlink)

	if refusal.Declared != "0xf72478e0" || refusal.Reported != "0xdeadbeef" {
		t.Errorf("refusal reports declared %q, reported %q; want 0xf72478e0 and 0xdeadbeef",
			refusal.Declared, refusal.Reported)
	}
	for _, value := range []string{"0xf72478e0", "0xdeadbeef"} {
		if !strings.Contains(refusal.Reason, value) {
			t.Errorf("reason %q does not report %s", refusal.Reason, value)
		}
	}

	noUplink(t, p.launch.uplink)
	p.launch.downlink <- []byte(tlmLine(0x02) + "\n")
	noDownlink(t, p.downlink)
	if got := status(t, &state.evaluation, "safe"); got != ObjectiveStatusActive {
		t.Errorf("objective = %s after refusal, want active (ungraded)", got)
	}
}

func TestMatchingAppImageStartsScenario(t *testing.T) {
	state := readyState(t)
	p := startProbe(t, state, func(*State) (uint32, error) { return 0xf72478e0, nil })

	p.launch.downlink <- []byte(bootBanner + " v1.0 boot\r\n")
	if msg, ok := recvDownlink(t, p.downlink).(*CommandResponseDownlinkMessage); !ok ||
		!strings.HasPrefix(msg.Response, bootBanner) {
		t.Fatalf("downlink = %#v, want the boot banner", msg)
	}
	if got := recvUplink(t, p.launch.uplink); got != "PING" {
		t.Errorf("firmware uplink = %q, want the held PING", got)
	}
}

func TestAppImageCheckRetriesUntilProbeAnswers(t *testing.T) {
	retry := appImageRetryInterval
	appImageRetryInterval = time.Millisecond
	t.Cleanup(func() { appImageRetryInterval = retry })

	state := readyState(t)
	reads := 0
	p := startProbe(t, state, func(*State) (uint32, error) {
		reads++
		if reads < 3 {
			return 0, ErrProbeMemoryUnavailable
		}
		return 0xf72478e0, nil
	})

	p.launch.downlink <- []byte(bootBanner + " v1.0 boot\r\n")
	recvDownlink(t, p.downlink)
	if got := recvUplink(t, p.launch.uplink); got != "PING" {
		t.Errorf("firmware uplink = %q, want the held PING", got)
	}
}

func shortCheckWindow(t *testing.T) {
	t.Helper()
	window, retry := appImageCheckWindow, appImageRetryInterval
	appImageCheckWindow, appImageRetryInterval = 50*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { appImageCheckWindow, appImageRetryInterval = window, retry })
}

func TestUnreadableAppImageRefusedAtDeadline(t *testing.T) {
	shortCheckWindow(t)
	state := readyState(t)
	p := startProbe(t, state, func(*State) (uint32, error) {
		return 0, errors.New("introspection unavailable: connecting to 127.0.0.1:1234")
	})

	p.launch.downlink <- []byte(bootBanner + " v1.0 boot\r\n")
	refusal := recvRefusal(t, p.downlink)
	if refusal.Declared != "0xf72478e0" || refusal.Reported != "" {
		t.Errorf("refusal = %+v, want declared 0xf72478e0 and nothing reported", refusal)
	}
	if strings.Contains(refusal.Reason, "introspection") || strings.Contains(refusal.Reason, "1234") {
		t.Errorf("reason %q exposes the introspection channel", refusal.Reason)
	}
	noUplink(t, p.launch.uplink)
}

func TestProbeThatNeverBootsRefusedAtDeadline(t *testing.T) {
	shortCheckWindow(t)
	state := readyState(t)
	p := startProbe(t, state, func(*State) (uint32, error) { return 0xf72478e0, nil })

	refusal := recvRefusal(t, p.downlink)
	if refusal.Declared != "0xf72478e0" || !strings.Contains(refusal.Reason, "0xf72478e0") {
		t.Errorf("refusal = %+v, want it to report the declared 0xf72478e0", refusal)
	}

	p.launch.downlink <- []byte(bootBanner + " v1.0 boot\r\n")
	noDownlink(t, p.downlink)
	noUplink(t, p.launch.uplink)
}

func TestUncheckableDeclaredCRCRefusesScenario(t *testing.T) {
	state := readyState(t)
	p := startProbe(t, state, func(*State) (uint32, error) { return 0xf72478e0, nil })
	state.Scenario.Manifest.Firmware.AppCRC32 = "not hex"

	p.launch.downlink <- []byte(bootBanner + " v1.0 boot\r\n")
	if refusal := recvRefusal(t, p.downlink); !strings.Contains(refusal.Reason, "not hex") {
		t.Errorf("reason %q does not name the bad app_crc32", refusal.Reason)
	}
	noUplink(t, p.launch.uplink)
}
