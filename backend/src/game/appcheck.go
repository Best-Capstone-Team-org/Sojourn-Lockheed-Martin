package game

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sojourn/app"
	"strconv"
	"strings"
	"time"
)

// The bootloader copies the golden image, header first, to the APP region and
// checks the header's CRC32 against the image before jumping to it (firmware
// include/probe.h apphdr_t: magic, entry, size, crc32). Once the application
// prints its boot banner, the header in APP RAM is the CRC the probe booted.
const (
	appHeaderLen      = 16
	appHeaderMagic    = 0x4E4A5253
	appHeaderCRCField = 12
)

var appImageCheckWindow = 10 * time.Second

var appImageRetryInterval = 250 * time.Millisecond

// Tests replace readAppCRC, since the real one needs a running probe.
var readAppCRC = func(s *State) (uint32, error) {
	if s.grader == nil {
		return 0, ErrProbeMemoryUnavailable
	}
	return s.grader.AppImageCRC()
}

func (g *Grader) AppImageCRC() (uint32, error) {
	header, err := g.ReadAppRAM(appHeaderLen)
	if err != nil {
		return 0, err
	}
	if len(header) < appHeaderLen {
		return 0, fmt.Errorf("short application header: %d bytes", len(header))
	}
	if magic := binary.LittleEndian.Uint32(header); magic != appHeaderMagic {
		return 0, fmt.Errorf("no application header in APP RAM (magic %#08x)", magic)
	}
	return binary.LittleEndian.Uint32(header[appHeaderCRCField:]), nil
}

type AppImageMismatch struct {
	Declared string
	Reported string
	Reason   string
}

func (m *AppImageMismatch) Error() string { return m.Reason }

func parseAppCRC(s string) (uint32, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	crc, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("app_crc32 %q is not a 32-bit hex value", s)
	}
	return uint32(crc), nil
}

func formatAppCRC(crc uint32) string { return fmt.Sprintf("0x%08x", crc) }

func (s *State) verifyAppImage(deadline time.Time, stop <-chan struct{}) *AppImageMismatch {
	declared := s.Scenario.Manifest.Firmware.AppCRC32

	want, err := parseAppCRC(declared)
	if err != nil {
		return &AppImageMismatch{Declared: declared,
			Reason: fmt.Sprintf("Scenario refused: the package's firmware CRC can't be checked: %v.", err)}
	}

	for {
		got, err := readAppCRC(s)
		if err == nil {
			if got == want {
				return nil
			}
			return &AppImageMismatch{Declared: formatAppCRC(want), Reported: formatAppCRC(got),
				Reason: fmt.Sprintf("Scenario refused: the probe booted firmware with CRC32 %s, "+
					"but the package was written for %s.", formatAppCRC(got), formatAppCRC(want))}
		}

		wait := min(appImageRetryInterval, time.Until(deadline))
		if wait <= 0 {
			return appImageUnconfirmed(want, err)
		}
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-stop:
			timer.Stop()
			return appImageUnconfirmed(want, errors.New("session stopped"))
		}
	}
}

// err is logged, not shown: it may name the introspection channel (R24.19).
func appImageUnconfirmed(want uint32, err error) *AppImageMismatch {
	app.ErrorLogger.Printf("Confirming the application image CRC: %v", err)
	return &AppImageMismatch{Declared: formatAppCRC(want),
		Reason: fmt.Sprintf("Scenario refused: the probe didn't report its firmware CRC32 within %s "+
			"of session start, so it couldn't be checked against the package's %s.",
			appImageCheckWindow, formatAppCRC(want))}
}

func (s *State) appImageDeclared() uint32 {
	crc, _ := parseAppCRC(s.Scenario.Manifest.Firmware.AppCRC32)
	return crc
}
