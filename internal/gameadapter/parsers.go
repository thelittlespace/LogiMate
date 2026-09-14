package gameadapter

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

type jsonWireFrame struct {
	Game          string  `json:"game"`
	RPM           int     `json:"rpm"`
	RPMMax        int     `json:"rpmMax"`
	RPMRedline    int     `json:"rpmRedline"`
	SpeedKPH      float64 `json:"speedKph"`
	Force         float64 `json:"force"`
	Physics       bool    `json:"physics"`
	PlayerControl bool    `json:"playerControl"`
	FFBEnabled    bool    `json:"ffbEnabled,omitempty"`
}

func ParseLocalJSON(b []byte, now time.Time) (Frame, error) {
	if len(b) == 0 || len(b) > 65536 {
		return Frame{}, fmt.Errorf("invalid packet size")
	}
	var w jsonWireFrame
	if err := json.Unmarshal(b, &w); err != nil {
		return Frame{}, err
	}
	if math.IsNaN(w.Force) || math.IsInf(w.Force, 0) || math.IsNaN(w.SpeedKPH) || math.IsInf(w.SpeedKPH, 0) {
		return Frame{}, fmt.Errorf("invalid numeric telemetry value")
	}
	w.Force = math.Max(-1, math.Min(1, w.Force))
	if w.RPM < 0 {
		w.RPM = 0
	}
	if w.RPMMax < 0 {
		w.RPMMax = 0
	}
	if w.RPMRedline < 0 {
		w.RPMRedline = 0
	}
	return Frame{Game: w.Game, RPM: w.RPM, RPMMax: w.RPMMax, RPMRedline: w.RPMRedline, SpeedKPH: w.SpeedKPH, Force: w.Force, Physics: w.Physics, PlayerControl: w.PlayerControl, FFBEnabled: w.FFBEnabled, ReceivedAt: now}, nil
}

func ParseWreckfestPino(b []byte, now time.Time) (Frame, error) {
	d, ok := wheelengine.ParseWreckfestPinoMain(b)
	if !ok {
		return Frame{}, fmt.Errorf("invalid Wreckfest 2 Pino main packet")
	}
	force := float64(d.FFBForce)
	if math.IsNaN(force) || math.IsInf(force, 0) {
		return Frame{}, fmt.Errorf("invalid Pino force")
	}
	force = math.Max(-1, math.Min(1, force))
	rpm, maxRPM, redline := d.RPM, d.RPMMax, d.RPMRedline
	if rpm < 0 {
		rpm = 0
	}
	if maxRPM < 0 {
		maxRPM = 0
	}
	if redline < 0 {
		redline = 0
	}
	return Frame{Game: "Wreckfest 2", RPM: rpm, RPMMax: maxRPM, RPMRedline: redline, Force: force, Physics: d.PhysicsRunning(), PlayerControl: d.PlayerInControl(), FFBEnabled: d.FFBEnabled, ReceivedAt: now}, nil
}
