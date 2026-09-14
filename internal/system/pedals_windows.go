//go:build windows

package system

import (
	"path/filepath"
	"strings"
)

type PedalMapping struct {
	InputSource       string          `json:"inputSource,omitempty"`
	LayoutID          string          `json:"layoutId,omitempty"`
	Gas               string          `json:"gas,omitempty"`
	Brake             string          `json:"brake,omitempty"`
	Clutch            string          `json:"clutch,omitempty"`
	GasCalibration    AxisCalibration `json:"gasCalibration,omitempty"`
	BrakeCalibration  AxisCalibration `json:"brakeCalibration,omitempty"`
	ClutchCalibration AxisCalibration `json:"clutchCalibration,omitempty"`
}

func legacyPedalKey(model, mode string) string {
	s := strings.ToLower(strings.TrimSpace(model + "|" + mode))
	r := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "|", "__", "(", "", ")", "")
	return r.Replace(s)
}

func canonicalPedalModel(model string) string {
	switch {
	case IsG27Model(model):
		return "g27"
	case IsG25Model(model):
		return "g25"
	case IsDFGTModel(model):
		return "dfgt"
	case IsCompatibilityModel(model):
		return "c294"
	default:
		return strings.ToLower(strings.TrimSpace(model))
	}
}

func canonicalPedalMode(mode string) string {
	l := strings.ToLower(strings.TrimSpace(mode))
	switch {
	case strings.Contains(l, "legacy"):
		return "legacy"
	case strings.Contains(l, "generic"), strings.Contains(l, "modern"):
		return "modern"
	default:
		return l
	}
}

func pedalKey(model, mode string) string {
	return legacyPedalKey(canonicalPedalModel(model), canonicalPedalMode(mode))
}

func sanitizePedalWheelID(wheelID string) string {
	wheelID = strings.ToLower(strings.TrimSpace(wheelID))
	r := strings.NewReplacer("\\", "_", "/", "_", "#", "_", "&", "_", ":", "_")
	return r.Replace(wheelID)
}

func pedalDevicePrefix(wheelID string) string {
	if strings.TrimSpace(wheelID) == "" {
		return ""
	}
	return "device__" + sanitizePedalWheelID(wheelID) + "__"
}

func pedalDeviceKey(wheelID, model, mode string) string {
	prefix := pedalDevicePrefix(wheelID)
	if prefix == "" {
		return pedalKey(model, mode)
	}
	return prefix + pedalKey(model, mode)
}

func legacyPedalDeviceKey(wheelID, model, mode string) string {
	prefix := pedalDevicePrefix(wheelID)
	if prefix == "" {
		return legacyPedalKey(model, mode)
	}
	return prefix + legacyPedalKey(model, mode)
}

func pedalFile(dataDir string) string { return filepath.Join(dataDir, "pedals.json") }

func ReadPedalMapping(dataDir, model, mode string) PedalMapping {
	all := map[string]PedalMapping{}
	if _, err := ReadJSONConfigStrict(pedalFile(dataDir), &all); err != nil {
		return PedalMapping{}
	}
	if m, ok := all[pedalKey(model, mode)]; ok {
		return m
	}
	return all[legacyPedalKey(model, mode)]
}

func ReadPedalMappingForWheel(dataDir, wheelID, model, mode string) PedalMapping {
	all := map[string]PedalMapping{}
	if _, err := ReadJSONConfigStrict(pedalFile(dataDir), &all); err != nil {
		return PedalMapping{}
	}
	for _, key := range []string{pedalDeviceKey(wheelID, model, mode), legacyPedalDeviceKey(wheelID, model, mode)} {
		if m, ok := all[key]; ok {
			return m
		}
	}

	// Older per-device files included presentation suffixes such as
	// "manuell bestätigt / C294". If exactly one mapping for this wheel matches
	// the current canonical model+mode family, accept it rather than losing a
	// user's calibration after the wheel re-enumerates to its native PID.
	prefix := pedalDevicePrefix(wheelID)
	modelToken := canonicalPedalModel(model)
	modeToken := canonicalPedalMode(mode)
	var candidate PedalMapping
	matches := 0
	for key, m := range all {
		lk := strings.ToLower(key)
		if prefix == "" || !strings.HasPrefix(lk, prefix) {
			continue
		}
		modelOK := modelToken == "" || strings.Contains(lk, modelToken) ||
			(modelToken == "g27" && strings.Contains(lk, "logitech_g27")) ||
			(modelToken == "g25" && strings.Contains(lk, "logitech_g25")) ||
			(modelToken == "dfgt" && strings.Contains(lk, "driving_force_gt"))
		modeOK := modeToken == "" || strings.Contains(lk, modeToken) ||
			(modeToken == "modern" && strings.Contains(lk, "generic_hid")) ||
			(modeToken == "legacy" && strings.Contains(lk, "logitech_legacy"))
		if modelOK && modeOK {
			candidate = m
			matches++
		}
	}
	if matches == 1 {
		return candidate
	}

	// Backward compatibility with mappings written before per-device storage.
	for _, key := range []string{pedalKey(model, mode), legacyPedalKey(model, mode)} {
		if m, ok := all[key]; ok {
			return m
		}
	}
	return PedalMapping{}
}

func SavePedalMappingForWheel(dataDir, wheelID, model, mode string, m PedalMapping) error {
	all := map[string]PedalMapping{}
	if _, err := ReadJSONConfigStrict(pedalFile(dataDir), &all); err != nil {
		return err
	}
	all[pedalDeviceKey(wheelID, model, mode)] = m
	return WriteJSONConfigStrict(pedalFile(dataDir), all, 0644)
}

func SavePedalMapping(dataDir, model, mode string, m PedalMapping) error {
	all := map[string]PedalMapping{}
	if _, err := ReadJSONConfigStrict(pedalFile(dataDir), &all); err != nil {
		return err
	}
	all[pedalKey(model, mode)] = m
	return WriteJSONConfigStrict(pedalFile(dataDir), all, 0644)
}

// MigratePedalMappingsWheelID carries every old per-device mapping to a new
// identity prefix. It is used when LogiMate upgrades from interface/container
// IDs to stable wheel IDs. Existing keys are kept as rollback compatibility;
// new keys win on subsequent reads/writes.
func MigratePedalMappingsWheelID(dataDir, oldID, newID string) error {
	oldPrefix, newPrefix := pedalDevicePrefix(oldID), pedalDevicePrefix(newID)
	if oldPrefix == "" || newPrefix == "" || oldPrefix == newPrefix {
		return nil
	}
	all := map[string]PedalMapping{}
	exists, err := ReadJSONConfigStrict(pedalFile(dataDir), &all)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	changed := false
	for key, m := range all {
		if !strings.HasPrefix(strings.ToLower(key), oldPrefix) {
			continue
		}
		tail := key[len(oldPrefix):]
		dst := newPrefix + tail
		if _, exists := all[dst]; !exists {
			all[dst] = m
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return WriteJSONConfigStrict(pedalFile(dataDir), all, 0644)
}

func AxisValues(j JoyState) map[string]float64 {
	norm := func(v, min, max uint32) float64 {
		if max <= min {
			return 0
		}
		// Convert before subtraction. Unsigned subtraction would wrap when a
		// noisy/raw sample falls slightly below the advertised minimum and could
		// otherwise be clamped to 100% instead of 0%.
		x := float64(int64(v)-int64(min)) / float64(int64(max)-int64(min))
		if x < 0 {
			x = 0
		}
		if x > 1 {
			x = 1
		}
		return x
	}
	return map[string]float64{
		"Y": norm(j.Y, j.YMin, j.YMax), "Z": norm(j.Z, j.ZMin, j.ZMax),
		"R": norm(j.R, j.RMin, j.RMax), "U": norm(j.U, j.UMin, j.UMax), "V": norm(j.V, j.VMin, j.VMax),
	}
}

func StrongestMovedAxis(before, after JoyState, excluded map[string]bool) string {
	a, b := AxisValues(before), AxisValues(after)
	best := ""
	delta := 0.0
	for _, k := range []string{"Y", "Z", "R", "U", "V"} {
		if excluded[k] {
			continue
		}
		d := a[k] - b[k]
		if d < 0 {
			d = -d
		}
		if d > delta {
			delta, best = d, k
		}
	}
	if delta < 0.20 {
		return ""
	}
	return best
}
