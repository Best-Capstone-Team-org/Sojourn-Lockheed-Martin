package tests

import (
	"encoding/binary"
	"encoding/hex"
	"reflect"
	"sojourn/emulator"
	"testing"
)

func crc16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func encodeFrame(payload []byte) string {
	body := append([]byte{byte(len(payload))}, payload...)
	sum := binary.BigEndian.AppendUint16(nil, crc16(body))
	return hex.EncodeToString(append(append([]byte{0xEB, 0x90}, body...), sum...))
}

// Scenario paths address the frame as tlm_decode.py --json shapes it.
func TestAsMapMatchesReferenceDecoder(t *testing.T) {
	payload := []byte{
		0x00, 0x07, // frame
		0x00, 0x00, 0x00, 0x2A, // uptime
		0x01, 0x00, 0x00, // NOMINAL, reboots, no fault
		0x13, 0x88, 0x01, 0xF4, // bus, load
		0x00, 0x04, 0xFF, 0xFF, 0xFF, 0xF6, // MAG -10
		0x61, 0x06, 0x01, 0x00, 0x00, 0x40, 0x05, 0x32, // COMMS
		0x5A, 0x02, 0x56, 0xA0, // AUX
		0x7E, 0x02, 0xAB, 0xCD, // unknown
	}

	frame, err := emulator.DecodeTelemetryFrame(encodeFrame(payload))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	got := frame.AsMap()
	channels := got["channels"].(map[string]any)

	checks := map[string]any{
		"mode":     got["mode"],
		"uptime_s": got["uptime_s"],
		"crc_ok":   got["crc_ok"],
		"MAG":      channels["MAG"],
		"AUX":      channels["AUX"],
		"0x7E":     channels["0x7E"],
		"antenna":  channels["COMMS"].(map[string]any)["antenna"],
		"budget":   channels["COMMS"].(map[string]any)["budget"],
		"xstat":    channels["COMMS"].(map[string]any)["xstat"],
	}
	want := map[string]any{
		"mode":     "NOMINAL",
		"uptime_s": uint32(42),
		"crc_ok":   true,
		"MAG":      int32(-10),
		"AUX":      "0x56A0",
		"0x7E":     "ABCD",
		"antenna":  "LGA",
		"budget":   uint16(64),
		"xstat":    byte(0x05),
	}

	if !reflect.DeepEqual(checks, want) {
		t.Fatalf("got %v\nwant %v", checks, want)
	}
}
