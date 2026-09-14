package openg27port

import (
	"encoding/json"
	"strings"
)

// OpenG27GameProfile is the Phase-C compatibility model for OpenG27 1.0.4
// GameProfile.cs. It intentionally mirrors the upstream profile semantics,
// while LogiMate keeps its own broader persisted profile schema.
type OpenG27GameProfile struct {
	Name                 string `json:"Name"`
	ProcessMatch         string `json:"ProcessMatch"`
	RotationDeg          int    `json:"RotationDeg"`
	AutocenterStrength   int    `json:"AutocenterStrength"`
	DamperStrength       int    `json:"DamperStrength"`
	TelemetryFormat      string `json:"TelemetryFormat"`
	GameFfbEnabled       bool   `json:"GameFfbEnabled"`
	GameFfbGain          int    `json:"GameFfbGain"`
	GameFfbInvert        bool   `json:"GameFfbInvert"`
	LedsFromTelemetryRpm bool   `json:"LedsFromTelemetryRpm"`
}

// ParseOpenG27GameProfiles accepts OpenG27's root JSON array. The pointer
// fields preserve the C# record-constructor defaults for properties omitted by
// older profile files (notably GameFfbGain=100).
func ParseOpenG27GameProfiles(b []byte) ([]OpenG27GameProfile, error) {
	type raw struct {
		Name                 string  `json:"Name"`
		ProcessMatch         string  `json:"ProcessMatch"`
		RotationDeg          int     `json:"RotationDeg"`
		AutocenterStrength   int     `json:"AutocenterStrength"`
		DamperStrength       int     `json:"DamperStrength"`
		TelemetryFormat      *string `json:"TelemetryFormat"`
		GameFfbEnabled       *bool   `json:"GameFfbEnabled"`
		GameFfbGain          *int    `json:"GameFfbGain"`
		GameFfbInvert        *bool   `json:"GameFfbInvert"`
		LedsFromTelemetryRpm *bool   `json:"LedsFromTelemetryRpm"`
	}
	var in []raw
	if err := json.Unmarshal(b, &in); err != nil {
		return nil, err
	}
	out := make([]OpenG27GameProfile, 0, len(in))
	for _, r := range in {
		p := OpenG27GameProfile{Name: r.Name, ProcessMatch: r.ProcessMatch, RotationDeg: r.RotationDeg,
			AutocenterStrength: r.AutocenterStrength, DamperStrength: r.DamperStrength, GameFfbGain: 100}
		if r.TelemetryFormat != nil {
			p.TelemetryFormat = *r.TelemetryFormat
		}
		if r.GameFfbEnabled != nil {
			p.GameFfbEnabled = *r.GameFfbEnabled
		}
		if r.GameFfbGain != nil {
			p.GameFfbGain = *r.GameFfbGain
		}
		if r.GameFfbInvert != nil {
			p.GameFfbInvert = *r.GameFfbInvert
		}
		if r.LedsFromTelemetryRpm != nil {
			p.LedsFromTelemetryRpm = *r.LedsFromTelemetryRpm
		}
		out = append(out, p)
	}
	return out, nil
}

// MatchOpenG27GameProfile reproduces GameProfile.MatchProcess: profile order
// wins, matching is case-insensitive substring matching against running names.
func MatchOpenG27GameProfile(running []string, profiles []OpenG27GameProfile) (OpenG27GameProfile, bool) {
	for _, p := range profiles {
		needle := strings.TrimSpace(p.ProcessMatch)
		if needle == "" {
			continue
		}
		needle = strings.ToLower(needle)
		for _, name := range running {
			if name != "" && strings.Contains(strings.ToLower(name), needle) {
				return p, true
			}
		}
	}
	return OpenG27GameProfile{}, false
}

func OpenG27Wreckfest2Preset() OpenG27GameProfile {
	return OpenG27GameProfile{Name: "Wreckfest 2", ProcessMatch: "Wreckfest2", RotationDeg: 900,
		AutocenterStrength: 0, DamperStrength: 0, TelemetryFormat: "WreckfestPino",
		GameFfbEnabled: true, GameFfbGain: 100, GameFfbInvert: true, LedsFromTelemetryRpm: true}
}
