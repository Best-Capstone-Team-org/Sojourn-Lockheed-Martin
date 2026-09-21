package emulator

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

const (
	telemetrySync1 = 0xEB
	telemetrySync2 = 0x90
)

const (
	chMAG   = 0x00
	chIMU   = 0x01
	chTHM   = 0x02
	chPWR   = 0x03
	chRAD   = 0x04
	chSTR   = 0x05
	chCAM   = 0x43
	chAUX   = 0x5A
	chHK    = 0x60
	chCOMMS = 0x61
)

type CameraData struct {
	FrameID    uint16 `json:"frame"`
	Target     uint16 `json:"target"`
	ExposureMS uint16 `json:"exposure"`
	HistMean   uint16 `json:"hist"`
	SatPct     uint16 `json:"sat"`
	Stars      uint16 `json:"stars"`
}

type HKData struct {
	HeaterOn     byte   `json:"heaterOn"`
	ShedCount    byte   `json:"shed"`
	PropellantMG uint16 `json:"prop"`
	Momentum     int16  `json:"mom"`
	RecFillPct   byte   `json:"rec"`
	Auth         byte   `json:"auth"`
}

type CommsData struct {
	Antenna      byte   `json:"antenna"`
	Dropped      byte   `json:"dropped"`
	Budget       uint16 `json:"budget"`
	XStat        byte   `json:"xStat"`
	HGADeployPct byte   `json:"hgaDeploy"`
}

type TelemetryFrame struct {
	CRCOK     bool              `json:"crcOK"`
	Frame     uint16            `json:"frame"`
	UptimeS   uint32            `json:"uptime"`
	Mode      string            `json:"mode"`
	Reboots   byte              `json:"reboots"`
	LastFault string            `json:"lastFault"`
	BusMV     uint16            `json:"bus"`
	LoadMW    uint16            `json:"load"`
	Sensors   map[byte]int32    `json:"sensors"`
	Camera    *CameraData       `json:"camera"`
	HK        *HKData           `json:"hk"`
	Comms     *CommsData        `json:"comms"`
	AUX       *uint16           `json:"AUX"`
	Unknown   map[string]string `json:"unknown"`
}

func modeName(v byte) string {
	switch v {
	case 0:
		return "BOOT"
	case 1:
		return "NOMINAL"
	case 2:
		return "SAFE"
	default:
		return fmt.Sprintf("?%d", v)
	}
}

func faultName(v byte) string {
	switch v {
	case 0:
		return "-"
	case 1:
		return "WDG"
	case 2:
		return "HARD"
	case 3:
		return "BADIMG"
	default:
		return fmt.Sprintf("?%d", v)
	}
}

func cr16CCITT(data []byte) uint16 {
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

func DecodeTelemetryFrame(hexStr string) (*TelemetryFrame, error) {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}

	if len(data) < 18 {
		return nil, fmt.Errorf("frame too short")
	}

	if data[0] != telemetrySync1 || data[1] != telemetrySync2 {
		return nil, fmt.Errorf("invalid sync bytes")
	}

	paylen := int(data[2])

	if len(data) != 3+paylen+2 {
		return nil, fmt.Errorf(
			"length mismatch: expected %d, got %d",
			3+paylen+2,
			len(data),
		)
	}

	payload := data[3 : 3+paylen]

	want := binary.BigEndian.Uint16(
		data[3+paylen : 3+paylen+2],
	)

	got := uint16(
		cr16CCITT(data[2 : 3+paylen]),
	)

	frame := &TelemetryFrame{
		CRCOK:     want == got,
		Frame:     binary.BigEndian.Uint16(payload[0:2]),
		UptimeS:   binary.BigEndian.Uint32(payload[2:6]),
		Mode:      modeName(payload[6]),
		Reboots:   payload[7],
		LastFault: faultName(payload[8]),
		BusMV:     binary.BigEndian.Uint16(payload[9:11]),
		LoadMW:    binary.BigEndian.Uint16(payload[11:13]),
		Sensors:   make(map[byte]int32),
		Unknown:   make(map[string]string),
	}

	i := 13

	for i+2 <= len(payload) {
		cid := payload[i]
		clen := int(payload[i+1])

		i += 2

		if i+clen > len(payload) {
			break
		}

		value := payload[i : i+clen]
		i += clen

		if cid >= chMAG && cid <= chSTR && clen == 4 {

			sensorvalue := int32(binary.BigEndian.Uint32(value))
			frame.Sensors[cid] = sensorvalue

		} else if cid == chCAM && clen == 12 {

			frame.Camera = &CameraData{
				FrameID:    binary.BigEndian.Uint16(value[0:2]),
				Target:     binary.BigEndian.Uint16(value[2:4]),
				ExposureMS: binary.BigEndian.Uint16(value[4:6]),
				HistMean:   binary.BigEndian.Uint16(value[6:8]),
				SatPct:     binary.BigEndian.Uint16(value[8:10]),
				Stars:      binary.BigEndian.Uint16(value[10:12]),
			}

		} else if cid == chHK && clen == 8 {

			frame.HK = &HKData{
				HeaterOn:     value[0],
				ShedCount:    value[1],
				PropellantMG: binary.BigEndian.Uint16(value[2:4]),
				Momentum:     int16(binary.BigEndian.Uint16(value[4:6])),
				RecFillPct:   value[6],
				Auth:         value[7],
			}

		} else if cid == chCOMMS && clen == 6 {

			frame.Comms = &CommsData{
				Antenna:      value[0],
				Dropped:      value[1],
				Budget:       binary.BigEndian.Uint16(value[2:4]),
				XStat:        value[4],
				HGADeployPct: value[5],
			}

		} else if cid == chAUX && clen == 2 {

			auxValue := binary.BigEndian.Uint16(value)
			frame.AUX = &auxValue

		} else {

			name := fmt.Sprintf("CH_%02X", cid)
			frame.Unknown[name] = hex.EncodeToString(value)
		}
	}

	return frame, nil
}

func formatSensor(cid byte, v int32) string {
	switch cid {
	case chMAG:
		return fmt.Sprintf("MAG %6d nT", v)

	case chIMU:
		return fmt.Sprintf("IMU %6.2f °/s", float64(v)/100)

	case chTHM:
		return fmt.Sprintf("THM %5.1f °C", float64(v)/10)

	case chPWR:
		return fmt.Sprintf("PWR %5d mV", v)

	case chRAD:
		return fmt.Sprintf("RAD %5d ct", v)

	case chSTR:
		return fmt.Sprintf("STR q=%.4f", float64(v)/10000)

	default:
		return fmt.Sprintf("0x%02X=%d", cid, v)
	}
}

func PrintTelemetryFrame(frame *TelemetryFrame) {
	if !frame.CRCOK {
		fmt.Printf("[%04d] *** BAD CRC ***\n", frame.Frame)
		return
	}

	fmt.Printf(
		"[%04d] up=%ds | mode=%s | reboots=%d | fault=%s | bus=%.3fV | load=%dmW\n",
		frame.Frame,
		frame.UptimeS,
		frame.Mode,
		frame.Reboots,
		frame.LastFault,
		float64(frame.BusMV)/1000,
		frame.LoadMW,
	)
	sensorOrder := []byte{chMAG, chIMU, chTHM, chPWR, chRAD, chSTR}

	for _, cid := range sensorOrder {
		if value, exists := frame.Sensors[cid]; exists {
			fmt.Printf("       %s\n", formatSensor(cid, value))
		}
	}

	if frame.Camera != nil {
		fmt.Printf(
			"       CAM frame=%d | target=%d | exp=%dms | mean=%d | sat=%d%% | stars=%d\n",
			frame.Camera.FrameID,
			frame.Camera.Target,
			frame.Camera.ExposureMS,
			frame.Camera.HistMean,
			frame.Camera.SatPct,
			frame.Camera.Stars,
		)
	}

	if frame.HK != nil {
		heater := "off"
		if frame.HK.HeaterOn != 0 {
			heater = "on"
		}

		auth := "no"
		if frame.HK.Auth != 0 {
			auth = "YES"
		}

		fmt.Printf(
			"       HK  heater=%s | prop=%dmg | mom=%d | rec=%d%% | shed=%d | auth=%s\n",
			heater,
			frame.HK.PropellantMG,
			frame.HK.Momentum,
			frame.HK.RecFillPct,
			frame.HK.ShedCount,
			auth,
		)
	}

	if frame.Comms != nil {
		antenna := "HGA"

		if frame.Comms.Antenna == 1 {
			antenna = "LGA"
		}

		fmt.Printf(
			"       LINK %s | budget=%dB | dropped=%d | hga_deploy=%d%%\n",
			antenna,
			frame.Comms.Budget,
			frame.Comms.Dropped,
			frame.Comms.HGADeployPct,
		)
	}

	if frame.AUX != nil {
		fmt.Printf("       AUX %d\n", *frame.AUX)
	}
}
