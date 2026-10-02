package tests

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"sojourn/app"
	"sojourn/emulator"
)

// The loader starts QEMU through the QEMU_SYSTEM_ARM_BIN env var. These tests
// point it at the test binary itself, which acts as a fake qemu-system-arm when
// fakeQEMUEnv is set: it records its arguments, serves the UART over TCP like
// QEMU's `-serial tcp:...,server=on`, and echoes every received line back.
const (
	qemuBinEnv       = "QEMU_SYSTEM_ARM_BIN"
	fakeQEMUEnv      = "SOJOURN_FAKE_QEMU"
	fakeQEMUArgsEnv  = "SOJOURN_FAKE_QEMU_ARGS"
	fakeQEMUModeEnv  = "SOJOURN_FAKE_QEMU_MODE"
	fakeQEMUDelayEnv = "SOJOURN_FAKE_QEMU_LISTEN_DELAY"
	loaderFatalEnv   = "SOJOURN_LOADER_FATAL"
	firmwareELFEnv   = "SOJOURN_FIRMWARE_ELF"
	fakeQEMUGreeting = "BOOT OK"
)

// Fake QEMU modes.
const (
	modeEcho      = "echo"       // echo lines until killed
	modeExitAfter = "exit-after" // exit cleanly after the first echoed line
	modeFail      = "fail"       // exit with a non-zero status after the first echoed line
	modeNoListen  = "no-listen"  // exit with a non-zero status before opening the UART
)

func TestMain(m *testing.M) {
	if name := os.Getenv(loaderFatalEnv); name != "" {
		// Child process for fatal-path tests: LoadAndStartFirmware calls
		// log.Fatalf (os.Exit) on load failures, so it must run in its own process.
		// Unset the marker so the fake QEMU it spawns doesn't run the loader too.
		os.Unsetenv(loaderFatalEnv)
		app.InitLoggers()
		emulator.LoadAndStartFirmware(name, make(chan []byte), make(chan []byte), make(chan struct{}))
		os.Exit(0)
	}

	if os.Getenv(fakeQEMUEnv) == "1" {
		os.Exit(runFakeQEMU(os.Args[1:]))
	}

	app.InitLoggers()

	os.Exit(m.Run())
}

func runFakeQEMU(args []string) int {
	if path := os.Getenv(fakeQEMUArgsEnv); path != "" {
		os.WriteFile(path, []byte(strings.Join(args, "\n")), 0o644)
	}

	addr, err := serialAddr(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if os.Getenv(fakeQEMUModeEnv) == modeNoListen {
		return 1
	}

	if delay, err := time.ParseDuration(os.Getenv(fakeQEMUDelayEnv)); err == nil {
		time.Sleep(delay)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer ln.Close()

	conn, err := ln.Accept()
	if err != nil {
		return 2
	}
	defer conn.Close()

	fmt.Fprintf(conn, "%s\n", fakeQEMUGreeting)

	mode := os.Getenv(fakeQEMUModeEnv)
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return 0
		}
		fmt.Fprintf(conn, "ECHO %s", line)

		switch mode {
		case modeExitAfter:
			return 0
		case modeFail:
			return 3
		}
	}
}

// serialAddr extracts host:port from `-serial tcp:HOST:PORT,opts...`.
func serialAddr(args []string) (string, error) {
	i := slices.Index(args, "-serial")
	if i < 0 || i+1 >= len(args) {
		return "", fmt.Errorf("missing -serial argument")
	}
	spec, ok := strings.CutPrefix(args[i+1], "tcp:")
	if !ok {
		return "", fmt.Errorf("unexpected -serial spec %q", args[i+1])
	}
	addr, _, _ := strings.Cut(spec, ",")
	return addr, nil
}

type loaderRun struct {
	downlink chan []byte
	uplink   chan []byte
	done     chan struct{}
	returned chan struct{}
	argsFile string
}

// startLoader runs LoadAndStartFirmware against the fake QEMU in the given mode.
func startLoader(t *testing.T, mode string) *loaderRun {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	firmware := filepath.Join(dir, "probe_rom.elf")
	if err := os.WriteFile(firmware, []byte("not really an elf"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &loaderRun{
		downlink: make(chan []byte),
		uplink:   make(chan []byte),
		done:     make(chan struct{}),
		returned: make(chan struct{}),
		argsFile: filepath.Join(dir, "qemu-args"),
	}

	t.Setenv(qemuBinEnv, self)
	t.Setenv(fakeQEMUEnv, "1")
	t.Setenv(fakeQEMUArgsEnv, r.argsFile)
	t.Setenv(fakeQEMUModeEnv, mode)

	go func() {
		defer close(r.returned)
		emulator.LoadAndStartFirmware(firmware, r.downlink, r.uplink, r.done)
	}()

	t.Cleanup(func() {
		r.stop()
		r.wait(t)
		close(r.uplink)
	})

	return r
}

func (r *loaderRun) stop() {
	select {
	case <-r.done:
	default:
		close(r.done)
	}
}

func (r *loaderRun) wait(t *testing.T) {
	t.Helper()
	select {
	case <-r.returned:
	case <-time.After(10 * time.Second):
		t.Fatal("LoadAndStartFirmware did not return")
	}
}

func (r *loaderRun) recv(t *testing.T) string {
	t.Helper()
	select {
	case msg, ok := <-r.downlink:
		if !ok {
			t.Fatal("downlink channel closed unexpectedly")
		}
		return string(msg)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for downlink message")
	}
	return ""
}

func (r *loaderRun) send(t *testing.T, msg string) {
	t.Helper()
	select {
	case r.uplink <- []byte(msg):
	case <-time.After(5 * time.Second):
		t.Fatal("timed out sending uplink message")
	}
}

func (r *loaderRun) drainUntilClosed(t *testing.T) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case _, ok := <-r.downlink:
			if !ok {
				return
			}
		case <-timeout:
			t.Fatal("downlink channel was never closed")
		}
	}
}

// crc16CCITT is an independent CRC-16/CCITT-FALSE implementation used to check
// the checksum the loader appends to uplink messages.
func crc16CCITT(s string) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range []byte(s) {
		for i := 7; i >= 0; i-- {
			bit := (b>>i)&1 == 1
			top := crc&0x8000 != 0
			crc <<= 1
			if bit != top {
				crc ^= 0x1021
			}
		}
	}
	return crc
}

func TestCRC16Reference(t *testing.T) {
	// Standard check value for CRC-16/CCITT-FALSE.
	if got := crc16CCITT("123456789"); got != 0x29B1 {
		t.Fatalf("crc16CCITT check value = %04X, want 29B1", got)
	}
}

func TestLoaderStartsQEMUWithExpectedArgs(t *testing.T) {
	r := startLoader(t, modeEcho)

	if got := r.recv(t); got != fakeQEMUGreeting+"\n" {
		t.Fatalf("first downlink = %q, want greeting", got)
	}

	raw, err := os.ReadFile(r.argsFile)
	if err != nil {
		t.Fatalf("fake qemu did not record its args: %v", err)
	}
	args := strings.Split(string(raw), "\n")

	flagValue := func(flag string) string {
		i := slices.Index(args, flag)
		if i < 0 || i+1 >= len(args) {
			t.Fatalf("qemu args %q missing %s", args, flag)
		}
		return args[i+1]
	}

	if got := flagValue("-M"); got != "mps2-an386" {
		t.Errorf("-M = %q, want mps2-an386", got)
	}
	if !slices.Contains(args, "-nographic") {
		t.Errorf("qemu args %q missing -nographic", args)
	}
	if got := flagValue("-kernel"); filepath.Base(got) != "probe_rom.elf" {
		t.Errorf("-kernel = %q, want the firmware path", got)
	}

	wantSerial := fmt.Sprintf("tcp:%s:%d,server=on,wait=on", emulator.UartTCPAddr, emulator.UartTCPPort)
	if got := flagValue("-serial"); got != wantSerial {
		t.Errorf("-serial = %q, want %q", got, wantSerial)
	}
}

func TestLoaderUplinkAppendsCRC(t *testing.T) {
	r := startLoader(t, modeEcho)
	r.recv(t) // greeting

	frameRe := regexp.MustCompile(`^ECHO (.*\S) \*([0-9A-F]{4})\n$`)

	tests := []struct {
		name string
		in   string
		body string
	}{
		{"plain", "PING", "PING"},
		{"trailing newline", "MODE SAFE\n", "MODE SAFE"},
		{"surrounding whitespace", "  CAM 3 100  \r\n", "CAM 3 100"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r.send(t, tc.in)
			got := r.recv(t)

			m := frameRe.FindStringSubmatch(got)
			if m == nil {
				t.Fatalf("uplink frame %q does not match `<body> *XXXX\\n`", got)
			}
			if m[1] != tc.body {
				t.Errorf("body = %q, want %q", m[1], tc.body)
			}

			// The CRC covers the trimmed body plus the trailing space.
			want := fmt.Sprintf("%04X", crc16CCITT(tc.body+" "))
			if m[2] != want {
				t.Errorf("crc = %s, want %s", m[2], want)
			}
		})
	}
}

func TestLoaderDownlinkPreservesLines(t *testing.T) {
	r := startLoader(t, modeEcho)
	r.recv(t) // greeting

	for i := range 20 {
		r.send(t, fmt.Sprintf("SEQ %d", i))
	}
	for i := range 20 {
		got := r.recv(t)
		if !strings.HasPrefix(got, fmt.Sprintf("ECHO SEQ %d *", i)) || !strings.HasSuffix(got, "\n") {
			t.Fatalf("downlink %d = %q, want in-order newline-terminated echo", i, got)
		}
	}
}

func TestLoaderDoneKillsFirmware(t *testing.T) {
	r := startLoader(t, modeEcho)
	r.recv(t) // greeting

	r.stop()
	r.wait(t)

	// Killing QEMU drops the UART connection, which must close the downlink.
	r.drainUntilClosed(t)
}

func TestLoaderReturnsWhenFirmwareExits(t *testing.T) {
	for _, mode := range []string{modeExitAfter, modeFail} {
		t.Run(mode, func(t *testing.T) {
			r := startLoader(t, mode)
			r.recv(t) // greeting
			r.send(t, "SHUTDOWN")

			if got := r.recv(t); !strings.HasPrefix(got, "ECHO SHUTDOWN *") {
				t.Fatalf("downlink = %q, want echo", got)
			}

			// done is never closed here: the loader must return on its own.
			r.wait(t)
			r.drainUntilClosed(t)
		})
	}
}

func TestLoaderSerializesFirmwareInstances(t *testing.T) {
	first := startLoader(t, modeEcho)
	first.recv(t)

	// A second instance would fight over the UART port, so it must wait for
	// the first one to finish before starting QEMU.
	second := &loaderRun{
		downlink: make(chan []byte),
		uplink:   make(chan []byte),
		done:     make(chan struct{}),
		returned: make(chan struct{}),
	}
	firmware := filepath.Join(t.TempDir(), "second.elf")
	os.WriteFile(firmware, nil, 0o644)

	go func() {
		defer close(second.returned)
		emulator.LoadAndStartFirmware(firmware, second.downlink, second.uplink, second.done)
	}()
	t.Cleanup(func() {
		second.stop()
		second.wait(t)
		close(second.uplink)
	})

	select {
	case msg := <-second.downlink:
		t.Fatalf("second instance produced %q while the first was still running", msg)
	case <-time.After(1500 * time.Millisecond):
	}

	first.stop()
	first.wait(t)
	first.drainUntilClosed(t)

	if got := second.recv(t); got != fakeQEMUGreeting+"\n" {
		t.Fatalf("second instance greeting = %q", got)
	}
}

// runLoaderFatal runs LoadAndStartFirmware in a child process and returns its
// exit error and stderr.
func runLoaderFatal(t *testing.T, firmware string, env ...string) (error, string) {
	t.Helper()

	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(self, "-test.run=^$")
	cmd.Env = append(os.Environ(), loaderFatalEnv+"="+firmware)
	cmd.Env = append(cmd.Env, env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	return err, stderr.String()
}

func TestLoaderFatalOnMissingFirmware(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.elf")

	err, stderr := runLoaderFatal(t, missing)
	if err == nil {
		t.Fatal("expected loader to exit with an error")
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr = %q, want firmware not found error", stderr)
	}
}

func TestLoaderFatalOnMissingQEMU(t *testing.T) {
	firmware := filepath.Join(t.TempDir(), "probe_rom.elf")
	os.WriteFile(firmware, nil, 0o644)

	err, stderr := runLoaderFatal(t, firmware,
		qemuBinEnv+"=definitely-not-qemu-system-arm",
		"PATH="+t.TempDir())
	if err == nil {
		t.Fatal("expected loader to exit with an error")
	}
	if !strings.Contains(stderr, "Unable to find QEMU") {
		t.Errorf("stderr = %q, want missing QEMU error", stderr)
	}
}

func TestLoaderRetriesUntilUARTListens(t *testing.T) {
	// QEMU takes a while to open the UART socket; the loader must keep retrying.
	t.Setenv(fakeQEMUDelayEnv, "750ms")

	r := startLoader(t, modeEcho)
	if got := r.recv(t); got != fakeQEMUGreeting+"\n" {
		t.Fatalf("greeting = %q", got)
	}
}

func TestLoaderFatalWhenQEMUExitsBeforeUART(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	firmware := filepath.Join(t.TempDir(), "probe_rom.elf")
	os.WriteFile(firmware, nil, 0o644)

	start := time.Now()
	err, stderr := runLoaderFatal(t, firmware,
		qemuBinEnv+"="+self,
		fakeQEMUEnv+"=1",
		fakeQEMUModeEnv+"="+modeNoListen)
	if err == nil {
		t.Fatal("expected loader to exit with an error")
	}
	if !strings.Contains(stderr, "qemu exited before UART was ready") {
		t.Errorf("stderr = %q, want early qemu exit error", stderr)
	}
	// It should notice the exit rather than waiting out the dial timeout.
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("loader took %s to notice qemu exited", elapsed)
	}
}

func TestLoaderFindsQEMUOnPATH(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	// Expose the fake QEMU as a bare name that only resolves through PATH.
	binDir := t.TempDir()
	if err := os.Symlink(self, filepath.Join(binDir, "fake-qemu-system-arm")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	firmware := filepath.Join(t.TempDir(), "probe_rom.elf")
	os.WriteFile(firmware, nil, 0o644)

	t.Setenv(qemuBinEnv, "fake-qemu-system-arm")
	t.Setenv("PATH", binDir)
	t.Setenv(fakeQEMUEnv, "1")
	t.Setenv(fakeQEMUModeEnv, modeEcho)

	r := &loaderRun{
		downlink: make(chan []byte),
		uplink:   make(chan []byte),
		done:     make(chan struct{}),
		returned: make(chan struct{}),
	}
	go func() {
		defer close(r.returned)
		emulator.LoadAndStartFirmware(firmware, r.downlink, r.uplink, r.done)
	}()
	t.Cleanup(func() {
		r.stop()
		r.wait(t)
		close(r.uplink)
	})

	if got := r.recv(t); got != fakeQEMUGreeting+"\n" {
		t.Fatalf("greeting = %q", got)
	}
}

// TestLoaderRealQEMU boots the real firmware under qemu-system-arm. It only
// runs when SOJOURN_FIRMWARE_ELF points at a built probe_rom.elf.
func TestLoaderRealQEMU(t *testing.T) {
	firmware := os.Getenv(firmwareELFEnv)
	if firmware == "" {
		t.Skipf("set %s to a built firmware ELF to run", firmwareELFEnv)
	}
	if _, err := exec.LookPath("qemu-system-arm"); err != nil && os.Getenv(qemuBinEnv) == "" {
		t.Skip("qemu-system-arm not installed")
	}

	r := &loaderRun{
		downlink: make(chan []byte),
		uplink:   make(chan []byte),
		done:     make(chan struct{}),
		returned: make(chan struct{}),
	}
	go func() {
		defer close(r.returned)
		emulator.LoadAndStartFirmware(firmware, r.downlink, r.uplink, r.done)
	}()

	got := r.recv(t)
	if !strings.HasSuffix(got, "\n") {
		t.Errorf("downlink %q is not newline terminated", got)
	}
	if line := strings.TrimSpace(got); strings.HasPrefix(line, "TLM ") {
		if _, err := emulator.DecodeTelemetryFrame(line[4:]); err != nil {
			t.Errorf("failed to decode telemetry frame %q: %v", line, err)
		}
	}

	r.stop()
	r.wait(t)
	r.drainUntilClosed(t)
	close(r.uplink)
}
