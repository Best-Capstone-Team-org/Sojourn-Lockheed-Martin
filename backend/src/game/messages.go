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
