package emulator

import (
	"debug/elf"
	"fmt"
)

// appImageSection is the golden application image: what the bootloader copies
// into the APP region, and the binary the player is handed. The ELF's symbols
// and the detector image store stay with the author (firmware spec §6.4a, §15).
const appImageSection = ".app_image"

// AppImage extracts the golden application image from a ROM ELF.
func AppImage(romPath string) ([]byte, error) {
	rom, err := elf.Open(romPath)
	if err != nil {
		return nil, fmt.Errorf("opening firmware %s: %w", romPath, err)
	}
	defer rom.Close()

	section := rom.Section(appImageSection)
	if section == nil {
		return nil, fmt.Errorf("firmware %s has no %s section", romPath, appImageSection)
	}

	return section.Data()
}
