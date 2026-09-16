package emulator

type TLMUplinkMessage interface {
	Encode()
}

type TLMDownlinkMessage interface {
}
