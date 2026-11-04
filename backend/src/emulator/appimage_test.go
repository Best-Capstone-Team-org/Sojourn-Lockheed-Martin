package emulator_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"sojourn/emulator"
	"testing"
)

const firstContactROM = "../../../deps/sojourn/scenarios/first-contact/firmware/probe_rom.elf"

func TestAppImageExtractsGoldenImage(t *testing.T) {
	image, err := emulator.AppImage(firstContactROM)
	if err != nil {
		t.Fatalf("AppImage: %v", err)
	}

	// apphdr_t: magic, entry, size, crc32 (firmware include/probe.h).
	if len(image) < 16 {
		t.Fatalf("image is %d bytes, shorter than its header", len(image))
	}
	if magic := binary.LittleEndian.Uint32(image[0:]); magic != 0x4E4A5253 {
		t.Errorf("magic = %#x, want SRJN", magic)
	}
	if size := binary.LittleEndian.Uint32(image[8:]); int(size) != len(image) {
		t.Errorf("header size = %d, image is %d bytes", size, len(image))
	}
}

func TestAppImageMissingROM(t *testing.T) {
	if _, err := emulator.AppImage(filepath.Join(t.TempDir(), "probe_rom.elf")); err == nil {
		t.Fatal("AppImage succeeded on a missing ROM")
	}
}

func TestAppImageNoSection(t *testing.T) {
	// The test binary is an ELF without an .app_image section.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := emulator.AppImage(self); err == nil {
		t.Fatal("AppImage succeeded on an ELF without .app_image")
	}
}
