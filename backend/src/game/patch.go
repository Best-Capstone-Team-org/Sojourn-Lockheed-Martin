package game

import (
	"errors"
	"fmt"
)

// The application runs from APP RAM: the bootloader copies the golden image
// there byte for byte on every boot (firmware spec §5), so byte i of an image
// lives at appRAMBase+i.
const (
	appRAMBase = 0x20001000
	appRAMEnd  = 0x20019000

	appHeaderSize = 16
	patchMergeGap = 4
)

var ErrPatchTooLarge = fmt.Errorf("patch must fit in APP RAM (%d bytes)", appRAMEnd-appRAMBase)

func patchPokes(running, uploaded []byte) ([]string, error) {
	if len(uploaded) == 0 {
		return nil, errors.New("patch is empty")
	}
	if len(uploaded) > appRAMEnd-appRAMBase {
		return nil, ErrPatchTooLarge
	}
	if len(running) < len(uploaded) {
		return nil, fmt.Errorf("running image covers %d bytes, patch has %d", len(running), len(uploaded))
	}

	var pokes []string
	for i := appHeaderSize; i < len(uploaded); i++ {
		if running[i] == uploaded[i] {
			continue
		}

		end := i + 1
		for j := end; j < len(uploaded) && j < end+patchMergeGap+1; j++ {
			if running[j] != uploaded[j] {
				end = j + 1
			}
		}

		for start := i; start < end; start += maxPokeBytes {
			chunk := uploaded[start:min(start+maxPokeBytes, end)]
			pokes = append(pokes, fmt.Sprintf("POKE 0x%08X %X", appRAMBase+start, chunk))
		}
		i = end - 1
	}

	return pokes, nil
}
