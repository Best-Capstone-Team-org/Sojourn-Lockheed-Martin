package game

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// The player's read-out decodes only the scenario's console.decode channels;
// grading still sees every channel (charter R25.3).
func TestTelemetryReadOutHonoursDecodeList(t *testing.T) {
	launches := stubFirmware(t)
	state := readyState(t, Objective{
		ID:      "lga",
		Success: Predicate{Op: "tlm", Path: "channels.COMMS.antenna", Cmp: "eq", Value: "LGA"},
	})
	state.Scenario.Manifest.Console.Decode = []string{"MAG"}
	state.grader = NewGrader(state, "127.0.0.1:1")

	out := make(chan DownlinkMessage)
	up := make(chan []byte)
	if err := state.Start(out, up, make(chan struct{})); err != nil {
		t.Fatalf("Start: %v", err)
	}
	l := <-launches

	payload := []byte{
		0x00, 0x01, 0x00, 0x00, 0x00, 0x2A, 0x01, 0x00, 0x00, 0x13, 0x88, 0x01, 0xF4,
		0x00, 0x04, 0xFF, 0xFF, 0xFF, 0xF6, // MAG -10
		0x61, 0x06, 0x01, 0x00, 0x00, 0x40, 0x05, 0x32, // COMMS on LGA
	}
	body := append([]byte{byte(len(payload))}, payload...)
	frame := binary.BigEndian.AppendUint16(append([]byte{0xEB, 0x90}, body...), crcCCITT(body))
	line := "TLM " + hex.EncodeToString(frame)

	l.downlink <- []byte(line + "\r\n")
	stateMessage := recvDownlink(t, out).(*StateDownlinkMessage)
	tlm := recvDownlink(t, out).(*TLMDownlinkMessage)

	if tlm.TLM != line {
		t.Errorf("raw line = %q, want it unchanged", tlm.TLM)
	}
	if tlm.Telemetry.Comms != nil {
		t.Errorf("COMMS decoded for the player without being listed: %+v", tlm.Telemetry.Comms)
	}
	if len(tlm.Telemetry.Sensors) != 1 {
		t.Errorf("sensors = %v, want only MAG", tlm.Telemetry.Sensors)
	}
	if stateMessage.Objectives[0].Status != ObjectiveStatusComplete {
		t.Errorf("lga = %s: grading must still see the undecoded COMMS channel", stateMessage.Objectives[0].Status)
	}

	close(up)
	close(l.downlink)
	for range out {
	}
}
