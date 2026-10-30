package game

import "sojourn/emulator"

type DownlinkMessage interface {
	downlinkMessage()
}

type TLMDownlinkMessage struct {
	TLM       string
	Telemetry *emulator.TelemetryFrame
}

func (m *TLMDownlinkMessage) downlinkMessage() {}

type CommandResponseDownlinkMessage struct {
	Response string
}

func (m *CommandResponseDownlinkMessage) downlinkMessage() {}

type StateDownlinkMessage struct {
	Objectives  []ObjectiveStateDownlinkMessage
	Constraints map[string]int
}

type ObjectiveStateDownlinkMessage struct {
	ID     string          `json:"id"`
	Status ObjectiveStatus `json:"status"`
}

func (m *StateDownlinkMessage) downlinkMessage() {}

type CommandRefusedDownlinkMessage struct {
	Command     string
	Reason      string
	Constraints map[string]int
}

func (m *CommandRefusedDownlinkMessage) downlinkMessage() {}
