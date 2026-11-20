package emulator

import "fmt"

// COMMS XSTAT bits that events report on (firmware spec §6.4b).
const (
	xstatLink     = 0x01
	xstatHGAJam   = 0x04
	xstatPointErr = 0x08
)

// TelemetryEvents lists what changed from prev to cur, the frame before it.
// Scenarios match event predicates against this text, so it is worded and
// ordered exactly as the reference decoder (tools/tlm_decode.py) has it. A
// first frame, with no previous one, has no events.
func TelemetryEvents(prev, cur *TelemetryFrame) []string {
	if prev == nil || cur == nil {
		return nil
	}

	var events []string
	add := func(format string, args ...any) {
		events = append(events, fmt.Sprintf(format, args...))
	}

	for cid := byte(chMAG); cid <= chSTR; cid++ {
		_, had := prev.Sensors[cid]
		_, has := cur.Sensors[cid]
		switch {
		case had && !has:
			add("channel %s LOST", sensorNames[cid])
		case !had && has:
			add("channel %s ACQUIRED", sensorNames[cid])
		}
	}

	if cur.Reboots != prev.Reboots {
		add("PROBE REBOOTED (%d -> %d, fault=%s)", prev.Reboots, cur.Reboots, cur.LastFault)
	}
	if cur.Mode != prev.Mode {
		add("mode %s -> %s", prev.Mode, cur.Mode)
	}

	if c1, c2 := prev.Camera, cur.Camera; c2 != nil && (c1 == nil || c2.FrameID != c1.FrameID) {
		add("CAM capture #%d: target=%d exp=%dms mean=%d sat=%d%% stars=%d",
			c2.FrameID, c2.Target, c2.ExposureMS, c2.HistMean, c2.SatPct, c2.Stars)
	}

	if k1, k2 := prev.Comms, cur.Comms; k1 != nil && k2 != nil {
		if k2.Antenna != k1.Antenna {
			add("ANTENNA %s -> %s (budget %d -> %d bytes)",
				antennaName(k1.Antenna), antennaName(k2.Antenna), k1.Budget, k2.Budget)
		}
		if k2.Dropped != 0 && k1.Dropped == 0 {
			add("DOWNLINK SATURATED: %d channels dropped", k2.Dropped)
		} else if k2.Dropped != k1.Dropped {
			add("dropped channels %d -> %d", k1.Dropped, k2.Dropped)
		}
		x1, x2 := k1.XStat, k2.XStat
		if x2&xstatHGAJam != 0 && x1&xstatHGAJam == 0 {
			add("HIGH GAIN ANTENNA JAMMED at %d%% deployment", k2.HGADeployPct)
		}
		if x2&xstatPointErr != 0 && x1&xstatPointErr == 0 {
			add("HGA POINTING ERROR: boresight outside tolerance")
		} else if x1&xstatPointErr != 0 && x2&xstatPointErr == 0 {
			add("HGA boresight reacquired")
		}
		if k2.HGADeployPct != k1.HGADeployPct {
			add("HGA deployment %d%% -> %d%%", k1.HGADeployPct, k2.HGADeployPct)
		}
		if x2&xstatLink == 0 && x1&xstatLink != 0 {
			add("DOWNLINK LOST (no link)")
		}
	}

	if h1, h2 := prev.HK, cur.HK; h1 != nil && h2 != nil {
		if h2.ShedCount != h1.ShedCount {
			add("POWER LOAD SHED (count %d -> %d)", h1.ShedCount, h2.ShedCount)
		}
		if h2.Auth != 0 && h1.Auth == 0 {
			add("ENGINEERING COMMAND UNLOCKED (auth)")
		}
		if h2.HeaterOn != h1.HeaterOn {
			if h2.HeaterOn != 0 {
				add("heater ON")
			} else {
				add("heater OFF")
			}
		}
		if h2.RecFillPct >= 100 && h1.RecFillPct < 100 {
			add("RECORDER BUFFER FULL")
		}
	}

	return events
}
