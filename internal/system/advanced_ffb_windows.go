//go:build windows

package system

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

const advancedFFBSchemaVersion = 2

// AdvancedFFBConfig shapes semantic game force before the existing hard
// NativeFFBConfig safety mixer. These values can change feel but can never
// increase the final per-wheel motor ceiling.
type AdvancedFFBConfig struct {
	Enabled              bool    `json:"enabled"`
	Preset               string  `json:"preset"`
	ConstantGainPercent  int     `json:"constantGainPercent"`
	TransientGainPercent int     `json:"transientGainPercent"`
	DeadbandPercent      int     `json:"deadbandPercent"`
	MinimumForcePercent  int     `json:"minimumForcePercent"`
	ResponseExponent     float64 `json:"responseExponent"`
	LowPassHz            int     `json:"lowPassHz"`
	SmoothingPercent     int     `json:"smoothingPercent"`
	OutputLimitPercent   int     `json:"outputLimitPercent"`
}

type advancedFFBFile struct {
	Version int                          `json:"version"`
	Wheels  map[string]AdvancedFFBConfig `json:"wheels"`
}

type AdvancedFFBPreset struct {
	Name        string
	Description string
	Config      AdvancedFFBConfig
}

func advancedFFBPath(dataDir string) string { return filepath.Join(dataDir, "advanced-ffb.json") }

func defaultAdvancedFFBConfig(model WheelModelKind) AdvancedFFBConfig {
	name := "Model Neutral"
	if d, ok := wheelengine.DescriptorForModel(model); ok && strings.TrimSpace(d.DisplayName) != "" {
		name = d.DisplayName + " · Neutral"
	}
	return AdvancedFFBConfig{
		Enabled: true, Preset: name, ConstantGainPercent: 100, TransientGainPercent: 100,
		ResponseExponent: 1, OutputLimitPercent: 100,
	}
}

func migrateAdvancedFFBConfig(c AdvancedFFBConfig, fromVersion int, model WheelModelKind) AdvancedFFBConfig {
	// Schema v1 treated zero Constant/Transient gain as "missing" and
	// silently restored 100 %. D6.2 makes 0 % a real, user-selectable value.
	// Preserve old files exactly by applying the historical default once while
	// migrating them to schema v2.
	if fromVersion > 0 && fromVersion < 2 {
		d := defaultAdvancedFFBConfig(model)
		if c.ConstantGainPercent == 0 {
			c.ConstantGainPercent = d.ConstantGainPercent
		}
		if c.TransientGainPercent == 0 {
			c.TransientGainPercent = d.TransientGainPercent
		}
	}
	return c
}

func normalizeAdvancedFFBConfig(c AdvancedFFBConfig, model WheelModelKind) AdvancedFFBConfig {
	d := defaultAdvancedFFBConfig(model)
	if strings.TrimSpace(c.Preset) == "" {
		c.Preset = d.Preset
	}
	if c.ResponseExponent == 0 {
		c.ResponseExponent = d.ResponseExponent
	}
	if c.OutputLimitPercent == 0 {
		c.OutputLimitPercent = d.OutputLimitPercent
	}
	c.ConstantGainPercent = clampInt(c.ConstantGainPercent, 0, 150)
	c.TransientGainPercent = clampInt(c.TransientGainPercent, 0, 150)
	c.DeadbandPercent = clampInt(c.DeadbandPercent, 0, 20)
	c.MinimumForcePercent = clampInt(c.MinimumForcePercent, 0, 20)
	if c.ResponseExponent < .5 {
		c.ResponseExponent = .5
	}
	if c.ResponseExponent > 2 {
		c.ResponseExponent = 2
	}
	if c.LowPassHz != 0 {
		c.LowPassHz = clampInt(c.LowPassHz, 5, 100)
	}
	c.SmoothingPercent = clampInt(c.SmoothingPercent, 0, 90)
	c.OutputLimitPercent = clampInt(c.OutputLimitPercent, 10, 100)
	if len(c.Preset) > 64 {
		c.Preset = c.Preset[:64]
	}
	return c
}

func BuiltinAdvancedFFBPresets(model WheelModelKind) []AdvancedFFBPreset {
	neutral := defaultAdvancedFFBConfig(model)
	return []AdvancedFFBPreset{
		{Name: "Neutral", Description: "Keine Klang-/Kraftformung · nur Diagnose; harte Safety-Limits bleiben aktiv", Config: neutral},
		{Name: "Smooth", Description: "35 Hz Low-Pass · 12 % Smoothing · keine Kraftanhebung", Config: AdvancedFFBConfig{Enabled: true, Preset: "Smooth", ConstantGainPercent: 100, TransientGainPercent: 90, ResponseExponent: 1, LowPassHz: 35, SmoothingPercent: 12, OutputLimitPercent: 100}},
		{Name: "Responsive", Description: "60 Hz Low-Pass · 4 % Smoothing · Transienten 100 %", Config: AdvancedFFBConfig{Enabled: true, Preset: "Responsive", ConstantGainPercent: 100, TransientGainPercent: 100, ResponseExponent: .95, LowPassHz: 60, SmoothingPercent: 4, OutputLimitPercent: 100}},
		{Name: "Compensated (Experimental)", Description: "1 % Deadband · 3 % Minimum Force · 45 Hz · bleibt vor dem Safety-Cap", Config: AdvancedFFBConfig{Enabled: true, Preset: "Compensated (Experimental)", ConstantGainPercent: 100, TransientGainPercent: 95, DeadbandPercent: 1, MinimumForcePercent: 3, ResponseExponent: .90, LowPassHz: 45, SmoothingPercent: 6, OutputLimitPercent: 100}},
		{Name: "Disabled", Description: "Signalformung deaktiviert; direkter Game-Forcepfad bleibt erhalten", Config: AdvancedFFBConfig{Enabled: false, Preset: "Disabled", ConstantGainPercent: 100, TransientGainPercent: 100, ResponseExponent: 1, OutputLimitPercent: 100}},
	}
}

func ReadAdvancedFFBConfig(dataDir, wheelID string, model WheelModelKind) AdvancedFFBConfig {
	d := defaultAdvancedFFBConfig(model)
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(wheelID) == "" {
		return d
	}
	var f advancedFFBFile
	exists, err := ReadJSONConfigStrict(advancedFFBPath(dataDir), &f)
	if err != nil || !exists || f.Wheels == nil {
		return d
	}
	c, ok := f.Wheels[inputProfileKey(wheelID)]
	if !ok {
		return d
	}
	c = migrateAdvancedFFBConfig(c, f.Version, model)
	return normalizeAdvancedFFBConfig(c, model)
}

func SaveAdvancedFFBConfig(dataDir, wheelID string, model WheelModelKind, c AdvancedFFBConfig) error {
	if strings.TrimSpace(wheelID) == "" {
		return errors.New("keine stabile Wheel-ID")
	}
	c = normalizeAdvancedFFBConfig(c, model)
	path := advancedFFBPath(dataDir)
	f := advancedFFBFile{Version: advancedFFBSchemaVersion, Wheels: map[string]AdvancedFFBConfig{}}
	var current advancedFFBFile
	if exists, err := ReadJSONConfigStrict(path, &current); err != nil {
		return err
	} else if exists {
		if current.Version > advancedFFBSchemaVersion {
			return fmt.Errorf("advanced-ffb.json verwendet Schema %d; unterstützt wird maximal %d", current.Version, advancedFFBSchemaVersion)
		}
		if current.Wheels != nil {
			f.Wheels = current.Wheels
			if current.Version > 0 && current.Version < advancedFFBSchemaVersion {
				for key, old := range f.Wheels {
					// Wheel model is unknown for sibling entries; v1 migration only
					// needs the model-independent 100 % gain defaults.
					if old.ConstantGainPercent == 0 {
						old.ConstantGainPercent = 100
					}
					if old.TransientGainPercent == 0 {
						old.TransientGainPercent = 100
					}
					f.Wheels[key] = old
				}
			}
		}
	}
	f.Wheels[inputProfileKey(wheelID)] = c
	return WriteJSONConfigStrict(path, f, 0644)
}

func AdvancedFFBTuning(c AdvancedFFBConfig, model WheelModelKind) wheelengine.FFBTuning {
	c = normalizeAdvancedFFBConfig(c, model)
	return wheelengine.FFBTuning{
		Enabled:          c.Enabled,
		ConstantGain:     float64(c.ConstantGainPercent) / 100,
		TransientGain:    float64(c.TransientGainPercent) / 100,
		Deadband:         float64(c.DeadbandPercent) / 100,
		MinimumForce:     float64(c.MinimumForcePercent) / 100,
		ResponseExponent: c.ResponseExponent,
		LowPassHz:        float64(c.LowPassHz),
		Smoothing:        float64(c.SmoothingPercent) / 100,
		OutputLimit:      float64(c.OutputLimitPercent) / 100,
	}
}

func AdvancedFFBConfigSummary(c AdvancedFFBConfig, model WheelModelKind) string {
	c = normalizeAdvancedFFBConfig(c, model)
	state := "AN"
	if !c.Enabled {
		state = "AUS"
	}
	return fmt.Sprintf("FFB %s · %s · C/T %d/%d%% · Deadband %d%% · MinForce %d%% · Curve %.2f · LPF %dHz · Smooth %d%% · Pre-Safety-Limit %d%%", state, c.Preset, c.ConstantGainPercent, c.TransientGainPercent, c.DeadbandPercent, c.MinimumForcePercent, c.ResponseExponent, c.LowPassHz, c.SmoothingPercent, c.OutputLimitPercent)
}

func ReadSelectedAdvancedFFBConfig(s State) (AdvancedFFBConfig, bool) {
	w, ok := SelectedWheel(s)
	if !ok {
		return AdvancedFFBConfig{}, false
	}
	return ReadAdvancedFFBConfig(s.DataDir, w.ID, wheelModelKind(w)), true
}

func SelectedAdvancedFFBPresets(s State) ([]AdvancedFFBPreset, bool) {
	w, ok := SelectedWheel(s)
	if !ok {
		return nil, false
	}
	return BuiltinAdvancedFFBPresets(wheelModelKind(w)), true
}

func SaveSelectedAdvancedFFBConfig(s State, c AdvancedFFBConfig) error {
	w, ok := SelectedWheel(s)
	if !ok {
		return errors.New("kein physisches Wheel ausgewählt")
	}
	return SaveAdvancedFFBConfig(s.DataDir, w.ID, wheelModelKind(w), c)
}

func SelectedAdvancedFFBConfigSummary(s State) string {
	w, ok := SelectedWheel(s)
	if !ok {
		return "FFB: kein Wheel ausgewählt"
	}
	c := ReadAdvancedFFBConfig(s.DataDir, w.ID, wheelModelKind(w))
	return AdvancedFFBConfigSummary(c, wheelModelKind(w))
}
