//go:build windows

package system

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// NativeEngineProfile contains user-facing wheel-engine tuning. Hard motor
// safety ceilings stay in NativeFFBConfig and can only reduce these values.
type NativeEngineProfile struct {
	Name                string `json:"name"`
	RotationDegrees     int    `json:"rotationDegrees"`
	MasterGainPercent   int    `json:"masterGainPercent"`
	ConstantGainPercent int    `json:"constantGainPercent"`
	SpringGainPercent   int    `json:"springGainPercent"`
	DamperGainPercent   int    `json:"damperGainPercent"`
	FrictionGainPercent int    `json:"frictionGainPercent"`
}

type NativeEffectRequest struct {
	Constant int
	Spring   int
	Damper   int
	Friction int
}

type NativeEffectMix struct {
	Constant   int
	Spring     int
	Damper     int
	Friction   int
	ClipEvents int
}

const nativeEngineProfileSchemaVersion = 1

type nativeEngineProfileFile struct {
	Version  int                                       `json:"version"`
	Active   map[string]string                         `json:"active"`
	Profiles map[string]map[string]NativeEngineProfile `json:"profiles"`
}

func nativeEngineProfilePath(dataDir string) string {
	return filepath.Join(dataDir, "native-wheel-profiles.json")
}

func normalizeEngineProfileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Balanced"
	}
	if len(name) > 48 {
		name = name[:48]
	}
	return name
}

func normalizeRotationDegrees(v int) int {
	supported := []int{270, 360, 540, 720, 900}
	best := supported[0]
	bestD := absInt(v - best)
	for _, x := range supported[1:] {
		if d := absInt(v - x); d < bestD {
			best, bestD = x, d
		}
	}
	return best
}

func normalizeNativeEngineProfile(p NativeEngineProfile) NativeEngineProfile {
	p.Name = normalizeEngineProfileName(p.Name)
	if p.RotationDegrees == 0 {
		p.RotationDegrees = 900
	}
	p.RotationDegrees = normalizeRotationDegrees(p.RotationDegrees)
	if p.MasterGainPercent == 0 {
		p.MasterGainPercent = 75
	}
	p.MasterGainPercent = clampInt(p.MasterGainPercent, 1, 100)
	p.ConstantGainPercent = clampInt(p.ConstantGainPercent, 0, 100)
	p.SpringGainPercent = clampInt(p.SpringGainPercent, 0, 100)
	p.DamperGainPercent = clampInt(p.DamperGainPercent, 0, 100)
	p.FrictionGainPercent = clampInt(p.FrictionGainPercent, 0, 100)
	return p
}

func BuiltinNativeEngineProfiles() []NativeEngineProfile {
	return []NativeEngineProfile{
		normalizeNativeEngineProfile(NativeEngineProfile{Name: "Gentle", RotationDegrees: 900, MasterGainPercent: 50, ConstantGainPercent: 50, SpringGainPercent: 40, DamperGainPercent: 35, FrictionGainPercent: 30}),
		normalizeNativeEngineProfile(NativeEngineProfile{Name: "Balanced", RotationDegrees: 900, MasterGainPercent: 75, ConstantGainPercent: 75, SpringGainPercent: 65, DamperGainPercent: 55, FrictionGainPercent: 50}),
		normalizeNativeEngineProfile(NativeEngineProfile{Name: "Direct", RotationDegrees: 900, MasterGainPercent: 100, ConstantGainPercent: 100, SpringGainPercent: 85, DamperGainPercent: 70, FrictionGainPercent: 65}),
	}
}

func defaultNativeEngineProfileFile() nativeEngineProfileFile {
	return nativeEngineProfileFile{Version: nativeEngineProfileSchemaVersion, Active: map[string]string{}, Profiles: map[string]map[string]NativeEngineProfile{}}
}

func readNativeEngineProfileFileStrict(dataDir string) (nativeEngineProfileFile, error) {
	f := defaultNativeEngineProfileFile()
	exists, err := ReadJSONConfigStrict(nativeEngineProfilePath(dataDir), &f)
	if err != nil {
		return defaultNativeEngineProfileFile(), err
	}
	if exists && f.Version > nativeEngineProfileSchemaVersion {
		return defaultNativeEngineProfileFile(), fmt.Errorf("native-wheel-profiles.json verwendet Schema %d; unterstützt wird maximal %d", f.Version, nativeEngineProfileSchemaVersion)
	}
	if f.Active == nil {
		f.Active = map[string]string{}
	}
	if f.Profiles == nil {
		f.Profiles = map[string]map[string]NativeEngineProfile{}
	}
	f.Version = nativeEngineProfileSchemaVersion
	return f, nil
}

func loadNativeEngineProfileFile(dataDir string) nativeEngineProfileFile {
	f, err := readNativeEngineProfileFileStrict(dataDir)
	if err != nil {
		return defaultNativeEngineProfileFile()
	}
	return f
}

func saveNativeEngineProfileFile(dataDir string, f nativeEngineProfileFile) error {
	f.Version = nativeEngineProfileSchemaVersion
	return WriteJSONConfigStrict(nativeEngineProfilePath(dataDir), f, 0644)
}

func ListNativeEngineProfiles(dataDir, wheelID string) []NativeEngineProfile {
	byName := map[string]NativeEngineProfile{}
	for _, p := range BuiltinNativeEngineProfiles() {
		byName[strings.ToLower(p.Name)] = p
	}
	f := loadNativeEngineProfileFile(dataDir)
	for _, p := range f.Profiles[inputProfileKey(wheelID)] {
		p = normalizeNativeEngineProfile(p)
		byName[strings.ToLower(p.Name)] = p
	}
	out := make([]NativeEngineProfile, 0, len(byName))
	for _, p := range byName {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func ReadNativeEngineProfile(dataDir, wheelID, name string) (NativeEngineProfile, bool) {
	name = normalizeEngineProfileName(name)
	for _, p := range ListNativeEngineProfiles(dataDir, wheelID) {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return NativeEngineProfile{}, false
}

func ReadActiveNativeEngineProfile(dataDir, wheelID string) NativeEngineProfile {
	f := loadNativeEngineProfileFile(dataDir)
	name := f.Active[inputProfileKey(wheelID)]
	if p, ok := ReadNativeEngineProfile(dataDir, wheelID, name); ok {
		return p
	}
	p, _ := ReadNativeEngineProfile(dataDir, wheelID, "Balanced")
	return p
}

func SaveNativeEngineProfile(dataDir, wheelID string, p NativeEngineProfile) error {
	if strings.TrimSpace(wheelID) == "" {
		return fmt.Errorf("keine stabile Wheel-ID")
	}
	p = normalizeNativeEngineProfile(p)
	f, err := readNativeEngineProfileFileStrict(dataDir)
	if err != nil {
		return err
	}
	key := inputProfileKey(wheelID)
	if f.Profiles[key] == nil {
		f.Profiles[key] = map[string]NativeEngineProfile{}
	}
	f.Profiles[key][strings.ToLower(p.Name)] = p
	return saveNativeEngineProfileFile(dataDir, f)
}

func SetActiveNativeEngineProfile(dataDir, wheelID, name string) error {
	if strings.TrimSpace(wheelID) == "" {
		return fmt.Errorf("keine stabile Wheel-ID")
	}
	p, ok := ReadNativeEngineProfile(dataDir, wheelID, name)
	if !ok {
		return fmt.Errorf("Wheel-Engine-Profil %q wurde nicht gefunden", name)
	}
	f, err := readNativeEngineProfileFileStrict(dataDir)
	if err != nil {
		return err
	}
	f.Active[inputProfileKey(wheelID)] = p.Name
	return saveNativeEngineProfileFile(dataDir, f)
}

// ApplyNativeEngineProfile is intentionally transactional from the user's
// perspective: the active profile is only persisted after the hardware range
// command succeeded. It never starts motor force by itself.
func ApplyNativeEngineProfile(s State, name string) error {
	w, _, err := validateNativeOutputTarget(s)
	if err != nil {
		return err
	}
	next, ok := ReadNativeEngineProfile(s.DataDir, w.ID, name)
	if !ok {
		return fmt.Errorf("Wheel-Engine-Profil %q wurde nicht gefunden", name)
	}
	previous := ReadActiveNativeEngineProfile(s.DataDir, w.ID)

	// Profile changes are a hardware transaction. First prove that no previous
	// effect remains active; a timed-out worker must block the profile change.
	if err := NativeOutputEmergencyStop(s); err != nil {
		return fmt.Errorf("Profil nicht aktiviert; bestehende Ausgabe konnte nicht sicher neutralisiert werden: %w", err)
	}
	if err := NativeOutputApplyRange(s, next.RotationDegrees); err != nil {
		return fmt.Errorf("Profil nicht aktiviert; Lenkwinkel konnte nicht angewendet werden: %w", err)
	}
	if err := SetActiveNativeEngineProfile(s.DataDir, w.ID, next.Name); err != nil {
		// Persistence is part of the transaction. Restore the previous hardware
		// range so UI/config and actual wheel cannot silently diverge.
		rbErr := NativeOutputApplyRange(s, previous.RotationDegrees)
		if rbErr != nil {
			return errors.Join(fmt.Errorf("Profilstatus konnte nicht gespeichert werden: %w", err), fmt.Errorf("Rollback des vorherigen Lenkwinkels %d° fehlgeschlagen: %w", previous.RotationDegrees, rbErr))
		}
		return fmt.Errorf("Profilstatus konnte nicht gespeichert werden; vorheriger Lenkwinkel wurde wiederhergestellt: %w", err)
	}
	return nil
}

func scaleProfileValue(v, gain, master int) int {
	v = clampInt(v, -100, 100)
	gain = clampInt(gain, 0, 100)
	master = clampInt(master, 0, 100)
	return v * gain * master / 10000
}

// MixNativeEffects is a pure safety stage used before native packets are built.
// User/profile gains can only reduce requests; NativeFFBConfig applies the hard
// per-wheel ceiling last.
func MixNativeEffects(req NativeEffectRequest, profile NativeEngineProfile, cfg NativeFFBConfig) NativeEffectMix {
	profile = normalizeNativeEngineProfile(profile)
	cfg = normalizeNativeFFBConfig(cfg)
	out := NativeEffectMix{}
	clip := func(v, lo, hi int) int {
		c := clampInt(v, lo, hi)
		if c != v {
			out.ClipEvents++
		}
		return c
	}
	safetyScale := func(v int) int { return v * cfg.MasterGainPercent / 100 }
	out.Constant = clip(safetyScale(scaleProfileValue(req.Constant, profile.ConstantGainPercent, profile.MasterGainPercent)), -cfg.ConstantLimit, cfg.ConstantLimit)
	out.Spring = clip(absInt(safetyScale(scaleProfileValue(req.Spring, profile.SpringGainPercent, profile.MasterGainPercent))), 0, cfg.SpringGain)
	out.Damper = clip(absInt(safetyScale(scaleProfileValue(req.Damper, profile.DamperGainPercent, profile.MasterGainPercent))), 0, cfg.DamperGain)
	out.Friction = clip(absInt(safetyScale(scaleProfileValue(req.Friction, profile.FrictionGainPercent, profile.MasterGainPercent))), 0, cfg.FrictionGain)
	return out
}

func NativeEngineProfileSummary(p NativeEngineProfile) string {
	p = normalizeNativeEngineProfile(p)
	return fmt.Sprintf("%s · %d° · Master %d%% · Constant/Spring/Damper/Friction %d/%d/%d/%d%%", p.Name, p.RotationDegrees, p.MasterGainPercent, p.ConstantGainPercent, p.SpringGainPercent, p.DamperGainPercent, p.FrictionGainPercent)
}

func NativeEngineDryRun(dataDir, wheelID string) string {
	cfg := ReadNativeFFBConfig(dataDir, wheelID)
	p := ReadActiveNativeEngineProfile(dataDir, wheelID)
	mix := MixNativeEffects(NativeEffectRequest{Constant: 100, Spring: 100, Damper: 100, Friction: 100}, p, cfg)
	checks := []string{"slot0-constant", "slot1-spring", "slot2-damper", "slot3-friction", "stop-slots"}
	ok := true
	if r, e := BuildClassicConstantForceSlotReport(0, mix.Constant, false); e != nil || len(r) != 8 {
		ok = false
	}
	if r, e := BuildClassicSpringSlotReport(1, mix.Spring, false); e != nil || len(r) != 8 {
		ok = false
	}
	if r, e := BuildClassicDamperSlotReport(2, mix.Damper, false); e != nil || len(r) != 8 {
		ok = false
	}
	if r, e := BuildClassicFrictionSlotReport(3, mix.Friction, false); e != nil || len(r) != 8 {
		ok = false
	}
	for slot := 0; slot < 4; slot++ {
		if r, e := BuildClassicEffectStopReport(slot); e != nil || len(r) != 8 {
			ok = false
		}
	}
	status := "PASS"
	if !ok {
		status = "FAIL"
	}
	return fmt.Sprintf("Native Engine Dry Run: %s\r\nChecks: %s\r\nProfile: %s\r\nSafety: %s\r\nExtreme request -> constant=%+d%% spring=%d%% damper=%d%% friction=%d%% clipped=%d", status, strings.Join(checks, ", "), NativeEngineProfileSummary(p), NativeFFBConfigSummary(cfg), mix.Constant, mix.Spring, mix.Damper, mix.Friction, mix.ClipEvents)
}
