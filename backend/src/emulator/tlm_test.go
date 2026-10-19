package emulator

import (
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"
)

// fullPayload has every sensor channel plus CAM, HK, COMMS and AUX.
func fullPayload() []byte {
	return []byte{
		0x04, 0xD2, // frame 1234
		0x00, 0x00, 0x0E, 0x10, // uptime 3600
		0x02, 0x03, 0x01, // SAFE, 3 reboots, WDG
		0x13, 0x88, 0x01, 0xF4, // bus 5000mV, load 500mW
		0x00, 0x04, 0xFF, 0xFF, 0xFF, 0xF6, // MAG -10
		0x01, 0x04, 0x00, 0x00, 0x04, 0xD2, // IMU 1234 -> 12.34
		0x02, 0x04, 0xFF, 0xFF, 0xFF, 0x9C, // THM -100 -> -10.0
		0x03, 0x04, 0x00, 0x00, 0x13, 0x88, // PWR 5000
		0x04, 0x04, 0x00, 0x00, 0x00, 0x07, // RAD 7
		0x05, 0x04, 0x00, 0x00, 0x27, 0x10, // STR 10000 -> 1.0
		0x43, 0x0C, 0x00, 0x01, 0x00, 0x02, 0x00, 0x64, 0x00, 0x80, 0x00, 0x05, 0x00, 0x0B, // CAM
		0x60, 0x08, 0x01, 0x02, 0x03, 0xE8, 0xFF, 0xFE, 0x32, 0x01, // HK
		0x61, 0x06, 0x00, 0x04, 0x01, 0x00, 0x07, 0x64, // COMMS
		0x5A, 0x02, 0xBE, 0xEF, // AUX
	}
}

func decode(t *testing.T, payload []byte) *TelemetryFrame {
	t.Helper()
	frame, err := DecodeTelemetryFrame(encodeFrame(payload))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return frame
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	out := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		out <- string(b)
	}()

	fn()
	w.Close()
	return <-out
}

func TestDecodeTelemetryFrameAllChannels(t *testing.T) {
	f := decode(t, fullPayload())

	if !f.CRCOK || f.Frame != 1234 || f.UptimeS != 3600 || f.Mode != "SAFE" ||
		f.Reboots != 3 || f.LastFault != "WDG" || f.BusMV != 5000 || f.LoadMW != 500 {
		t.Fatalf("header = %+v", f)
	}

	wantSensors := map[byte]int32{chMAG: -10, chIMU: 1234, chTHM: -100, chPWR: 5000, chRAD: 7, chSTR: 10000}
	for cid, want := range wantSensors {
		if got, ok := f.Sensors[cid]; !ok || got != want {
			t.Errorf("sensor 0x%02X = %d (present %v), want %d", cid, got, ok, want)
		}
	}

	if want := (CameraData{1, 2, 100, 128, 5, 11}); f.Camera == nil || *f.Camera != want {
		t.Errorf("camera = %+v, want %+v", f.Camera, want)
	}
	if want := (HKData{1, 2, 1000, -2, 50, 1}); f.HK == nil || *f.HK != want {
		t.Errorf("hk = %+v, want %+v", f.HK, want)
	}
	if want := (CommsData{0, 4, 256, 7, 100}); f.Comms == nil || *f.Comms != want {
		t.Errorf("comms = %+v, want %+v", f.Comms, want)
	}
	if f.AUX == nil || *f.AUX != 0xBEEF {
		t.Errorf("aux = %v, want 0xBEEF", f.AUX)
	}
	if len(f.Unknown) != 0 {
		t.Errorf("unknown = %v, want none", f.Unknown)
	}
}

func TestDecodeTelemetryFrameWrongLengthChannelsAreUnknown(t *testing.T) {
	payload := append(fullPayload()[:13],
		0x00, 0x02, 0x12, 0x34, // MAG with the wrong length
		0x43, 0x01, 0xAA, // CAM too short
		0x60, 0x00, // HK empty
		0x61, 0x01, 0xBB, // COMMS too short
		0x5A, 0x01, 0xCC, // AUX too short
		0x7F, 0x03, 0x01, 0x02, // trailing channel runs past the payload
	)

	f := decode(t, payload)

	if len(f.Sensors) != 0 || f.Camera != nil || f.HK != nil || f.Comms != nil || f.AUX != nil {
		t.Fatalf("decoded malformed channels as known: %+v", f)
	}
	want := map[string]string{"CH_00": "1234", "CH_43": "aa", "CH_60": "", "CH_61": "bb", "CH_5A": "cc"}
	if len(f.Unknown) != len(want) {
		t.Fatalf("unknown = %v, want %v", f.Unknown, want)
	}
	for k, v := range want {
		if f.Unknown[k] != v {
			t.Errorf("unknown[%s] = %q, want %q", k, f.Unknown[k], v)
		}
	}
}

func TestDecodeTelemetryFrameBadCRC(t *testing.T) {
	raw, _ := hex.DecodeString(encodeFrame(fullPayload()))
	raw[len(raw)-1] ^= 0xFF

	f, err := DecodeTelemetryFrame(hex.EncodeToString(raw))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if f.CRCOK {
		t.Fatal("CRCOK = true for a corrupted checksum")
	}
	if f.Frame != 1234 {
		t.Fatalf("frame = %d, want fields still decoded", f.Frame)
	}
}

func TestDecodeTelemetryFrameRejectsMalformed(t *testing.T) {
	good := encodeFrame(fullPayload())

	cases := []struct {
		name, in, want string
	}{
		{"not hex", "zz" + good[2:], "invalid byte"},
		{"odd length hex", good[:len(good)-1], "odd length"},
		{"too short", good[:34], "too short"},
		{"bad sync", "EB91" + good[4:], "invalid sync"},
		{"trailing byte", good + "00", "length mismatch"},
		{"missing byte", good[:len(good)-2], "length mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := DecodeTelemetryFrame(tc.in)
			if err == nil || f != nil {
				t.Fatalf("got %+v, %v; want an error", f, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestEnumNames(t *testing.T) {
	cases := []struct {
		fn   func(byte) string
		in   byte
		want string
	}{
		{modeName, 0, "BOOT"},
		{modeName, 1, "NOMINAL"},
		{modeName, 2, "SAFE"},
		{modeName, 9, "?9"},
		{faultName, 0, "-"},
		{faultName, 1, "WDG"},
		{faultName, 2, "HARD"},
		{faultName, 3, "BADIMG"},
		{faultName, 200, "?200"},
		{antennaName, 0, "HGA"},
		{antennaName, 1, "LGA"},
		{antennaName, 2, "?2"},
	}
	for _, tc := range cases {
		if got := tc.fn(tc.in); got != tc.want {
			t.Errorf("name(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatSensor(t *testing.T) {
	cases := []struct {
		cid  byte
		v    int32
		want string
	}{
		{chMAG, -10, "MAG    -10 nT"},
		{chIMU, 1234, "IMU  12.34 °/s"},
		{chTHM, -100, "THM -10.0 °C"},
		{chPWR, 5000, "PWR  5000 mV"},
		{chRAD, 7, "RAD     7 ct"},
		{chSTR, 10000, "STR q=1.0000"},
		{0x7E, -1, "0x7E=-1"},
	}
	for _, tc := range cases {
		if got := formatSensor(tc.cid, tc.v); got != tc.want {
			t.Errorf("formatSensor(0x%02X, %d) = %q, want %q", tc.cid, tc.v, got, tc.want)
		}
	}
}

func TestPrintTelemetryFrame(t *testing.T) {
	f := decode(t, fullPayload())

	got := captureStdout(t, func() { PrintTelemetryFrame(f) })

	want := []string{
		"[1234] up=3600s | mode=SAFE | reboots=3 | fault=WDG | bus=5.000V | load=500mW",
		"       MAG    -10 nT",
		"       IMU  12.34 °/s",
		"       THM -10.0 °C",
		"       PWR  5000 mV",
		"       RAD     7 ct",
		"       STR q=1.0000",
		"       CAM frame=1 | target=2 | exp=100ms | mean=128 | sat=5% | stars=11",
		"       HK  heater=on | prop=1000mg | mom=-2 | rec=50% | shed=2 | auth=YES",
		"       LINK HGA | budget=256B | dropped=4 | hga_deploy=100%",
		"       AUX 48879",
	}
	if got != strings.Join(want, "\n")+"\n" {
		t.Fatalf("output:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

func TestPrintTelemetryFrameAlternateValues(t *testing.T) {
	f := &TelemetryFrame{
		CRCOK:   true,
		Frame:   7,
		Mode:    "BOOT",
		Sensors: map[byte]int32{chRAD: 1, 0x7E: 99},
		HK:      &HKData{},
		Comms:   &CommsData{Antenna: 1},
	}

	got := captureStdout(t, func() { PrintTelemetryFrame(f) })

	for _, want := range []string{"[0007] up=0s | mode=BOOT", "RAD     1 ct", "heater=off", "auth=no", "LINK LGA"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	// Only known sensors are printed, and absent sections are skipped.
	for _, absent := range []string{"0x7E", "CAM", "AUX", "MAG"} {
		if strings.Contains(got, absent) {
			t.Errorf("output contains %q:\n%s", absent, got)
		}
	}
}

func TestPrintTelemetryFrameBadCRC(t *testing.T) {
	f := decode(t, fullPayload())
	f.CRCOK = false

	got := captureStdout(t, func() { PrintTelemetryFrame(f) })
	if got != "[1234] *** BAD CRC ***\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestAsMapAllChannels(t *testing.T) {
	m := decode(t, fullPayload()).AsMap()
	channels := m["channels"].(map[string]any)

	for name, want := range map[string]int32{"MAG": -10, "IMU": 1234, "THM": -100, "PWR": 5000, "RAD": 7, "STR": 10000} {
		if channels[name] != want {
			t.Errorf("%s = %v, want %d", name, channels[name], want)
		}
	}

	cam := channels["CAM"].(map[string]any)
	if cam["frame_id"] != uint16(1) || cam["exposure_ms"] != uint16(100) || cam["stars"] != uint16(11) {
		t.Errorf("CAM = %v", cam)
	}
	hk := channels["HK"].(map[string]any)
	if hk["heater_on"] != byte(1) || hk["propellant_mg"] != uint16(1000) || hk["momentum"] != int16(-2) || hk["auth"] != byte(1) {
		t.Errorf("HK = %v", hk)
	}
	if ant := channels["COMMS"].(map[string]any)["antenna"]; ant != "HGA" {
		t.Errorf("antenna = %v, want HGA", ant)
	}
	if channels["AUX"] != "0xBEEF" {
		t.Errorf("AUX = %v", channels["AUX"])
	}
	if m["last_fault"] != "WDG" || m["bus_mv"] != uint16(5000) || m["reboots"] != byte(3) {
		t.Errorf("header = %v", m)
	}
}
