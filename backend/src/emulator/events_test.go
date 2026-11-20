package emulator

import (
	"slices"
	"testing"
)

// frame builds a decoded frame for event tests.
func eventFrame(mutate func(*TelemetryFrame)) *TelemetryFrame {
	f := &TelemetryFrame{
		CRCOK: true, Mode: "NOMINAL", LastFault: "-",
		Sensors: map[byte]int32{chMAG: 1, chIMU: 2},
		Comms:   &CommsData{Antenna: 0, Budget: 100, XStat: xstatLink, HGADeployPct: 100},
		HK:      &HKData{RecFillPct: 50},
	}
	if mutate != nil {
		mutate(f)
	}
	return f
}

// Every event the reference decoder (tools/tlm_decode.py) reports, worded as
// it words them: scenarios match event predicates against this text.
func TestTelemetryEvents(t *testing.T) {
	tests := []struct {
		name string
		cur  func(*TelemetryFrame)
		prev func(*TelemetryFrame)
		want []string
	}{
		{"nothing changed", nil, nil, nil},
		{"channel lost and acquired", func(f *TelemetryFrame) {
			delete(f.Sensors, chIMU)
			f.Sensors[chRAD] = 3
		}, nil, []string{"channel IMU LOST", "channel RAD ACQUIRED"}},
		{"reboot", func(f *TelemetryFrame) { f.Reboots, f.LastFault = 1, "WDG" }, nil,
			[]string{"PROBE REBOOTED (0 -> 1, fault=WDG)"}},
		{"mode", func(f *TelemetryFrame) { f.Mode = "SAFE" }, nil, []string{"mode NOMINAL -> SAFE"}},
		{"first camera capture", func(f *TelemetryFrame) {
			f.Camera = &CameraData{FrameID: 3, Target: 2, ExposureMS: 40, HistMean: 120, SatPct: 7, Stars: 12}
		}, nil, []string{"CAM capture #3: target=2 exp=40ms mean=120 sat=7% stars=12"}},
		{"same camera frame is not a capture", func(f *TelemetryFrame) { f.Camera = &CameraData{FrameID: 3} },
			func(f *TelemetryFrame) { f.Camera = &CameraData{FrameID: 3} }, nil},
		{"antenna", func(f *TelemetryFrame) { f.Comms.Antenna, f.Comms.Budget = 1, 40 }, nil,
			[]string{"ANTENNA HGA -> LGA (budget 100 -> 40 bytes)"}},
		{"downlink saturated", func(f *TelemetryFrame) { f.Comms.Dropped = 6 }, nil,
			[]string{"DOWNLINK SATURATED: 6 channels dropped"}},
		{"dropped count changes", func(f *TelemetryFrame) { f.Comms.Dropped = 5 },
			func(f *TelemetryFrame) { f.Comms.Dropped = 6 }, []string{"dropped channels 6 -> 5"}},
		{"jam", func(f *TelemetryFrame) { f.Comms.XStat |= xstatHGAJam; f.Comms.HGADeployPct = 40 }, nil,
			[]string{"HIGH GAIN ANTENNA JAMMED at 40% deployment", "HGA deployment 100% -> 40%"}},
		{"pointing error", func(f *TelemetryFrame) { f.Comms.XStat |= xstatPointErr }, nil,
			[]string{"HGA POINTING ERROR: boresight outside tolerance"}},
		{"boresight reacquired", nil, func(f *TelemetryFrame) { f.Comms.XStat |= xstatPointErr },
			[]string{"HGA boresight reacquired"}},
		{"link lost", func(f *TelemetryFrame) { f.Comms.XStat &^= xstatLink }, nil,
			[]string{"DOWNLINK LOST (no link)"}},
		{"load shed", func(f *TelemetryFrame) { f.HK.ShedCount = 2 }, nil,
			[]string{"POWER LOAD SHED (count 0 -> 2)"}},
		{"auth", func(f *TelemetryFrame) { f.HK.Auth = 1 }, nil,
			[]string{"ENGINEERING COMMAND UNLOCKED (auth)"}},
		{"heater on", func(f *TelemetryFrame) { f.HK.HeaterOn = 1 }, nil, []string{"heater ON"}},
		{"heater off", nil, func(f *TelemetryFrame) { f.HK.HeaterOn = 1 }, []string{"heater OFF"}},
		{"recorder full", func(f *TelemetryFrame) { f.HK.RecFillPct = 100 }, nil,
			[]string{"RECORDER BUFFER FULL"}},
		{"no COMMS or HK either side", func(f *TelemetryFrame) { f.Comms, f.HK = nil, nil },
			func(f *TelemetryFrame) { f.Comms, f.HK = nil, nil }, nil},
		{"in the reference's order", func(f *TelemetryFrame) {
			delete(f.Sensors, chMAG)
			f.Mode = "SAFE"
			f.Comms.Antenna, f.Comms.Budget = 1, 40
			f.HK.HeaterOn = 1
		}, nil, []string{"channel MAG LOST", "mode NOMINAL -> SAFE",
			"ANTENNA HGA -> LGA (budget 100 -> 40 bytes)", "heater ON"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TelemetryEvents(eventFrame(tc.prev), eventFrame(tc.cur))
			if !slices.Equal(got, tc.want) {
				t.Errorf("events = %q, want %q", got, tc.want)
			}
		})
	}

	if got := TelemetryEvents(nil, eventFrame(nil)); got != nil {
		t.Errorf("first frame had events %q", got)
	}
}
