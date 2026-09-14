//go:build windows

package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const gameProfileSchemaVersion = 2

type legacyOpenG27GameProfile struct {
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

func parseLegacyOpenG27GameProfiles(b []byte) ([]legacyOpenG27GameProfile, error) {
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
	out := make([]legacyOpenG27GameProfile, 0, len(in))
	for _, r := range in {
		p := legacyOpenG27GameProfile{Name: r.Name, ProcessMatch: r.ProcessMatch, RotationDeg: r.RotationDeg, AutocenterStrength: r.AutocenterStrength, DamperStrength: r.DamperStrength, GameFfbGain: 100}
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

func legacyWreckfest2Preset() legacyOpenG27GameProfile {
	return legacyOpenG27GameProfile{Name: "Wreckfest 2", ProcessMatch: "Wreckfest2", RotationDeg: 900, TelemetryFormat: "WreckfestPino", GameFfbEnabled: true, GameFfbGain: 100, GameFfbInvert: true, LedsFromTelemetryRpm: true}
}

type GameProfile struct {
	Name                string    `json:"name"`
	Executables         []string  `json:"executables"`
	ProcessMatch        string    `json:"processMatch,omitempty"`
	Source              string    `json:"source,omitempty"`
	Enabled             bool      `json:"enabled"`
	AutoApply           bool      `json:"autoApply"`
	ForegroundOnly      bool      `json:"foregroundOnly"`
	EngineProfile       string    `json:"engineProfile"`
	MasterGainPercent   int       `json:"masterGainPercent"`
	ConstantGainPercent int       `json:"constantGainPercent"`
	SpringGainPercent   int       `json:"springGainPercent"`
	DamperGainPercent   int       `json:"damperGainPercent"`
	FrictionGainPercent int       `json:"frictionGainPercent"`
	LEDPolicy           string    `json:"ledPolicy"`
	TelemetryAdapter    string    `json:"telemetryAdapter"`
	TelemetryPort       int       `json:"telemetryPort,omitempty"`
	Priority            int       `json:"priority,omitempty"`
	GameFFBEnabled      bool      `json:"gameFfbEnabled,omitempty"`
	GameFFBGainPercent  int       `json:"gameFfbGainPercent,omitempty"`
	GameFFBInvert       bool      `json:"gameFfbInvert,omitempty"`
	AutocenterPercent   int       `json:"autocenterPercent,omitempty"`
	DamperPercent       int       `json:"damperPercent,omitempty"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type gameProfileFile struct {
	Version  int           `json:"version"`
	Profiles []GameProfile `json:"profiles"`
}

type GameSessionState struct {
	Active        bool
	Process       string
	ProfileName   string
	EngineProfile string
	AutoApplied   bool
	Message       string
	UpdatedAt     time.Time
}

var gameSessionRuntime struct {
	sync.Mutex
	state       GameSessionState
	lastApplied string
}

func gameProfilesPath(dataDir string) string { return filepath.Join(dataDir, "game-profiles.json") }
func normalizeGameProfile(p GameProfile) GameProfile {
	p.Name = strings.TrimSpace(p.Name)
	if len(p.Name) > 64 {
		p.Name = p.Name[:64]
	}
	seen := map[string]bool{}
	out := []string{}
	for _, x := range p.Executables {
		x = strings.TrimSpace(x)
		if x == "" {
			continue
		}
		x = strings.TrimSpace(strings.TrimSuffix(filepath.Base(x), ".exe"))
		if x != "" && x != "." && !seen[strings.ToLower(x)] {
			seen[strings.ToLower(x)] = true
			out = append(out, x)
		}
	}
	p.Executables = out
	p.ProcessMatch = strings.TrimSpace(p.ProcessMatch)
	if p.ProcessMatch != "" {
		p.ProcessMatch = strings.TrimSpace(strings.TrimSuffix(filepath.Base(p.ProcessMatch), ".exe"))
		if p.ProcessMatch == "." {
			p.ProcessMatch = ""
		}
	}
	if len(p.ProcessMatch) > 128 {
		p.ProcessMatch = p.ProcessMatch[:128]
	}
	p.Source = strings.ToLower(strings.TrimSpace(p.Source))
	if p.EngineProfile == "" {
		p.EngineProfile = "Balanced"
	}
	if p.MasterGainPercent == 0 {
		p.MasterGainPercent = 100
	}
	p.MasterGainPercent = clampInt(p.MasterGainPercent, 1, 100)
	if p.ConstantGainPercent == 0 {
		p.ConstantGainPercent = 100
	}
	if p.SpringGainPercent == 0 {
		p.SpringGainPercent = 100
	}
	if p.DamperGainPercent == 0 {
		p.DamperGainPercent = 100
	}
	if p.FrictionGainPercent == 0 {
		p.FrictionGainPercent = 100
	}
	p.ConstantGainPercent = clampInt(p.ConstantGainPercent, 0, 100)
	p.SpringGainPercent = clampInt(p.SpringGainPercent, 0, 100)
	p.DamperGainPercent = clampInt(p.DamperGainPercent, 0, 100)
	p.FrictionGainPercent = clampInt(p.FrictionGainPercent, 0, 100)
	switch strings.ToLower(strings.TrimSpace(p.LEDPolicy)) {
	case "manual", "telemetry":
		p.LEDPolicy = strings.ToLower(strings.TrimSpace(p.LEDPolicy))
	default:
		p.LEDPolicy = "off"
	}
	p.TelemetryAdapter = canonicalTelemetryAdapterID(p.TelemetryAdapter)
	if p.TelemetryPort < 0 || p.TelemetryPort > 65535 {
		p.TelemetryPort = 0
	}
	p.Priority = clampInt(p.Priority, 0, 1000)
	if p.GameFFBGainPercent == 0 && p.Source != "openg27" {
		p.GameFFBGainPercent = 100
	}
	p.GameFFBGainPercent = clampInt(p.GameFFBGainPercent, 0, 150)
	p.AutocenterPercent = clampInt(p.AutocenterPercent, 0, 100)
	p.DamperPercent = clampInt(p.DamperPercent, 0, 100)
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now()
	}
	return p
}
func loadGameProfiles(dataDir string) gameProfileFile {
	f := gameProfileFile{Version: gameProfileSchemaVersion}
	_, _ = ReadJSONConfigStrict(gameProfilesPath(dataDir), &f)
	if f.Version < 1 {
		f.Version = 1
	}
	for i := range f.Profiles {
		f.Profiles[i] = normalizeGameProfile(f.Profiles[i])
	}
	return f
}
func saveGameProfiles(dataDir string, f gameProfileFile) error {
	path := gameProfilesPath(dataDir)
	var current gameProfileFile
	if exists, err := ReadJSONConfigStrict(path, &current); err != nil {
		return err
	} else if exists && current.Version > gameProfileSchemaVersion {
		return fmt.Errorf("game-profiles.json verwendet Schema %d; diese LogiMate-Version unterstützt maximal %d", current.Version, gameProfileSchemaVersion)
	}
	f.Version = gameProfileSchemaVersion
	return WriteJSONConfigStrict(path, f, 0644)
}
func ListGameProfiles(dataDir string) []GameProfile {
	f := loadGameProfiles(dataDir)
	out := append([]GameProfile(nil), f.Profiles...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}
func SaveGameProfile(dataDir string, p GameProfile) error {
	p = normalizeGameProfile(p)
	if p.Name == "" {
		return fmt.Errorf("Profilname fehlt")
	}
	if len(p.Executables) == 0 && p.ProcessMatch == "" {
		return fmt.Errorf("mindestens eine EXE oder ProcessMatch ist erforderlich")
	}
	f := loadGameProfiles(dataDir)
	replaced := false
	for i := range f.Profiles {
		if strings.EqualFold(f.Profiles[i].Name, p.Name) {
			f.Profiles[i] = p
			replaced = true
		}
	}
	if !replaced {
		f.Profiles = append(f.Profiles, p)
	}
	return saveGameProfiles(dataDir, f)
}
func DeleteGameProfile(dataDir, name string) error {
	f := loadGameProfiles(dataDir)
	out := f.Profiles[:0]
	for _, p := range f.Profiles {
		if !strings.EqualFold(p.Name, name) {
			out = append(out, p)
		}
	}
	f.Profiles = out
	return saveGameProfiles(dataDir, f)
}
func FindGameProfileByExecutable(dataDir, exe string) (GameProfile, bool) {
	exe = strings.TrimSpace(strings.TrimSuffix(filepath.Base(exe), ".exe"))
	for _, p := range ListGameProfiles(dataDir) {
		if !p.Enabled {
			continue
		}
		for _, x := range p.Executables {
			if strings.EqualFold(x, exe) {
				return p, true
			}
		}
		if p.ProcessMatch != "" && strings.Contains(strings.ToLower(exe), strings.ToLower(p.ProcessMatch)) {
			return p, true
		}
	}
	return GameProfile{}, false
}
func ActiveGameProfile(dataDir string) (GameProfile, string, bool) {
	fg := ForegroundProcessNameNative()
	if fg != "" {
		if p, ok := FindGameProfileByExecutable(dataDir, fg); ok {
			return p, fg, true
		}
	}
	running := ListRunningProcessNamesNative()
	exes := make([]string, 0, len(running))
	for exe := range running {
		exes = append(exes, exe)
	}
	sort.Slice(exes, func(i, j int) bool { return strings.ToLower(exes[i]) < strings.ToLower(exes[j]) })
	for _, exe := range exes {
		if p, ok := FindGameProfileByExecutable(dataDir, exe); ok && !p.ForegroundOnly {
			return p, exe, true
		}
	}
	return GameProfile{}, "", false
}
func ApplyGameProfileToEngine(s State, p GameProfile) error {
	p = normalizeGameProfile(p)
	if !HasActionableSelectedWheel(s) {
		return fmt.Errorf("kein sicher bestätigtes Wheel ausgewählt")
	}
	return ApplyNativeEngineProfile(s, p.EngineProfile)
}
func ExportGameProfilesJSON(dataDir string) ([]byte, error) {
	f := loadGameProfiles(dataDir)
	return json.MarshalIndent(f, "", "  ")
}
func ImportGameProfilesJSON(dataDir string, b []byte) error {
	var f gameProfileFile
	if len(b) > 1<<20 {
		return fmt.Errorf("Profilimport zu groß")
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	if len(f.Profiles) > 200 {
		return fmt.Errorf("zu viele Profile")
	}
	if f.Version > gameProfileSchemaVersion {
		return fmt.Errorf("Profilimport verwendet unbekanntes Schema %d", f.Version)
	}
	clean := gameProfileFile{Version: gameProfileSchemaVersion}
	for _, p := range f.Profiles {
		p = normalizeGameProfile(p)
		if p.Name == "" || (len(p.Executables) == 0 && p.ProcessMatch == "") {
			return fmt.Errorf("ungültiges Profil im Import")
		}
		clean.Profiles = append(clean.Profiles, p)
	}
	return saveGameProfiles(dataDir, clean)
}
func GameSessionSnapshot() GameSessionState {
	gameSessionRuntime.Lock()
	defer gameSessionRuntime.Unlock()
	return gameSessionRuntime.state
}
func UpdateGameSession(s State, nativeOutputEnabled bool) {
	p, exe, ok := ActiveGameProfile(s.DataDir)
	st := GameSessionState{UpdatedAt: time.Now()}
	if !ok {
		StopAutomaticGameTelemetry()
		StopNativeGameOutput("game-deactivated")
		st.Message = "Kein passendes Spielprofil aktiv"
		gameSessionRuntime.Lock()
		gameSessionRuntime.state = st
		gameSessionRuntime.lastApplied = ""
		gameSessionRuntime.Unlock()
		return
	}
	st.Active = true
	st.Process = exe
	if err := EnsureGameProfileTelemetry(p); err != nil {
		st.ProfileName = p.Name
		st.EngineProfile = p.EngineProfile
		st.Message = "Spielprofil erkannt, Telemetrie blockiert: " + err.Error()
		gameSessionRuntime.Lock()
		gameSessionRuntime.state = st
		gameSessionRuntime.Unlock()
		StopNativeGameOutput("telemetry-start-failed")
		return
	}
	c6 := NativeGameOutputSnapshot()
	if c6.Active && (!strings.EqualFold(c6.Profile, p.Name) || !sameTelemetryAdapter(c6.Adapter, p.TelemetryAdapter)) {
		StopNativeGameOutput("profile-changed")
	}
	st.ProfileName = p.Name
	st.EngineProfile = p.EngineProfile
	st.Message = "Spielprofil erkannt"
	key := strings.ToLower(exe + "|" + p.Name + "|" + s.SelectedWheelID)
	gameSessionRuntime.Lock()
	already := gameSessionRuntime.lastApplied == key
	gameSessionRuntime.Unlock()
	if p.AutoApply && nativeOutputEnabled && !p.ForegroundOnly || (p.AutoApply && nativeOutputEnabled && p.ForegroundOnly && strings.EqualFold(ForegroundProcessNameNative(), exe)) {
		if !already && HasActionableSelectedWheel(s) && strings.Contains(strings.ToLower(s.ActiveMode), "generic") {
			if err := ApplyGameProfileToEngine(s, p); err == nil {
				st.AutoApplied = true
				st.Message = "Spielprofil sicher angewendet (kein FFB-Autostart)"
				gameSessionRuntime.Lock()
				gameSessionRuntime.lastApplied = key
				gameSessionRuntime.Unlock()
			} else {
				st.Message = "Profil erkannt, Auto-Apply blockiert: " + err.Error()
			}
		}
	}
	gameSessionRuntime.Lock()
	gameSessionRuntime.state = st
	gameSessionRuntime.Unlock()
}

type OpenG27ImportReport struct {
	Path           string
	Imported       int
	EngineProfiles int
	Skipped        int
	Profiles       []string
}

func OpenG27DefaultProfilePaths() []string {
	root, _ := os.UserConfigDir()
	if root == "" {
		return nil
	}
	return []string{
		filepath.Join(root, "OpenG27", "game-profiles.json"),
		filepath.Join(root, "OpenG27FFB", "game-profiles.json"),
	}
}

func FindOpenG27ProfileFile() string {
	for _, p := range OpenG27DefaultProfilePaths() {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func ImportOpenG27GameProfilesFile(dataDir, wheelID, path string) (OpenG27ImportReport, error) {
	var r OpenG27ImportReport
	path = strings.TrimSpace(path)
	if path == "" {
		path = FindOpenG27ProfileFile()
	}
	if path == "" {
		return r, fmt.Errorf("keine OpenG27 game-profiles.json gefunden")
	}
	st, err := os.Stat(path)
	if err != nil {
		return r, err
	}
	if st.Size() > 1<<20 {
		return r, fmt.Errorf("OpenG27-Profilimport ist größer als 1 MiB")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	r, err = ImportOpenG27GameProfilesJSON(dataDir, wheelID, b)
	r.Path = path
	return r, err
}

func ImportOpenG27GameProfilesJSON(dataDir, wheelID string, b []byte) (OpenG27ImportReport, error) {
	var report OpenG27ImportReport
	if len(b) == 0 || len(b) > 1<<20 {
		return report, fmt.Errorf("ungültige OpenG27-Profildatei")
	}
	src, err := parseLegacyOpenG27GameProfiles(b)
	if err != nil {
		return report, fmt.Errorf("OpenG27-Profilformat: %w", err)
	}
	if len(src) > 200 {
		return report, fmt.Errorf("zu viele OpenG27-Profile")
	}
	existing := loadGameProfiles(dataDir)
	byName := map[string]int{}
	for i, p := range existing.Profiles {
		byName[strings.ToLower(p.Name)] = i
	}
	for _, op := range src {
		op.Name = strings.TrimSpace(op.Name)
		op.ProcessMatch = strings.TrimSpace(op.ProcessMatch)
		if op.Name == "" || op.ProcessMatch == "" {
			report.Skipped++
			continue
		}
		engineName := "OpenG27 · " + op.Name
		if len(engineName) > 48 {
			engineName = engineName[:48]
		}
		gp := normalizeGameProfile(GameProfile{
			Name: op.Name, ProcessMatch: op.ProcessMatch, Enabled: true, AutoApply: true,
			ForegroundOnly: false, EngineProfile: engineName, Source: "openg27",
			MasterGainPercent: 100, ConstantGainPercent: 100,
			SpringGainPercent: clampInt(op.AutocenterStrength, 0, 100),
			DamperGainPercent: clampInt(op.DamperStrength, 0, 100), FrictionGainPercent: 100,
			GameFFBEnabled: op.GameFfbEnabled, GameFFBGainPercent: clampInt(op.GameFfbGain, 0, 150), GameFFBInvert: op.GameFfbInvert,
			AutocenterPercent: clampInt(op.AutocenterStrength, 0, 100), DamperPercent: clampInt(op.DamperStrength, 0, 100),
		})
		if strings.EqualFold(op.TelemetryFormat, "WreckfestPino") {
			gp.TelemetryAdapter = TelemetryAdapterWreckfestPino
		}
		if op.LedsFromTelemetryRpm {
			gp.LEDPolicy = "telemetry"
		}
		if gp.LEDPolicy == "" {
			gp.LEDPolicy = "off"
		}
		if wheelID != "" {
			ep := NativeEngineProfile{Name: engineName, RotationDegrees: op.RotationDeg, MasterGainPercent: 100,
				ConstantGainPercent: 100, SpringGainPercent: clampInt(op.AutocenterStrength, 0, 100),
				DamperGainPercent: clampInt(op.DamperStrength, 0, 100), FrictionGainPercent: 100}
			if err := SaveNativeEngineProfile(dataDir, wheelID, ep); err != nil {
				return report, err
			}
			report.EngineProfiles++
		} else {
			gp.EngineProfile = "Balanced"
		}
		key := strings.ToLower(gp.Name)
		if idx, ok := byName[key]; ok {
			existing.Profiles[idx] = gp
		} else {
			byName[key] = len(existing.Profiles)
			existing.Profiles = append(existing.Profiles, gp)
		}
		report.Imported++
		report.Profiles = append(report.Profiles, gp.Name)
	}
	if report.Imported == 0 {
		return report, fmt.Errorf("keine gültigen OpenG27-Profile gefunden")
	}
	if err := saveGameProfiles(dataDir, existing); err != nil {
		return report, err
	}
	return report, nil
}

func OpenG27ProfileImportSummary(r OpenG27ImportReport) string {
	return fmt.Sprintf("OpenG27 Import · %d Profile · %d Wheel-Engine-Profile · %d übersprungen · Quelle=%s", r.Imported, r.EngineProfiles, r.Skipped, r.Path)
}

func InstallOpenG27Wreckfest2Preset(dataDir, wheelID string) (OpenG27ImportReport, error) {
	b, err := json.Marshal([]legacyOpenG27GameProfile{legacyWreckfest2Preset()})
	if err != nil {
		return OpenG27ImportReport{}, err
	}
	r, err := ImportOpenG27GameProfilesJSON(dataDir, wheelID, b)
	if err == nil {
		r.Path = "built-in OpenG27 1.0.4 Wreckfest 2 preset"
	}
	return r, err
}
