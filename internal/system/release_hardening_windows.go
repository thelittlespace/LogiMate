//go:build windows

package system

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

type StateInvariantIssue struct{ Code, Severity, Message string }
type SelfTestResult struct {
	Name   string
	Passed bool
	Detail string
}
type ReadinessReport struct {
	Score             int
	CodeAuditComplete bool
	Issues            []string
	ExternalGates     []string
}
type StableReleaseGate struct {
	Ready    bool
	Blockers []string
}

var supportedConfigSnapshotFiles = []string{"settings.json", "state.json", "wheel-preferences.json", "wheel.models.json", "wheel-confirmations.json", "wheel-native-history.json", "pedal-mappings.json", "input-profiles.json", "native-output.json", "native-ffb.json", "native-wheel-engine.json", "native-wheel-profiles.json", "game-profiles.json", "data-schema.json"}

func ValidateStateInvariants(s State) []StateInvariantIssue {
	var x []StateInvariantIssue
	add := func(c, v, m string) { x = append(x, StateInvariantIssue{c, v, m}) }
	if len(s.Wheels) == 1 && s.SelectedWheelID == "" {
		add("single-no-selection", "warning", "Ein eindeutiges Wheel ist vorhanden, aber nicht ausgewählt")
	}
	if s.SelectedWheelID != "" {
		found := false
		for _, w := range s.Wheels {
			if strings.EqualFold(w.ID, s.SelectedWheelID) {
				found = true
			}
		}
		if !found {
			add("stale-selection", "error", "Gespeicherte Wheel-ID ist nicht verbunden")
		}
	}
	lease := NativeOutputLeaseSnapshot()
	ffb := NativeFFBSnapshot()
	out := NativeOutputSnapshot()
	if ffb.Active && !lease.Active {
		add("ffb-without-lease", "critical", "FFB ist aktiv, aber der zentrale Native-Output-Lease fehlt")
	}
	if out.Active && !lease.Active {
		add("output-without-lease", "critical", "Motor-Output ist aktiv, aber der zentrale Native-Output-Lease fehlt")
	}
	if lease.Active {
		w, ok := SelectedWheel(s)
		if !ok || !strings.EqualFold(w.ID, lease.WheelID) || !strings.EqualFold(w.SessionID, lease.SessionID) || !sameKnownWheelModel(w.Model, lease.Model) {
			add("lease-target-mismatch", "critical", "Aktiver Output-Lease passt nicht mehr zum ausgewählten physischen Wheel")
		}
	}
	if ffb.Active && !NativeFFBHeartbeatHealthy(time.Now()) {
		add("stale-ffb-heartbeat", "critical", "FFB-Heartbeat ist veraltet")
	}
	if len(s.Wheels) > 1 && strings.Contains(strings.ToLower(s.SelectionStatus), "bereit") {
		add("multiwheel-ready", "warning", "Mehrere Wheels dürfen keinen eindeutigen Änderungszustand vortäuschen")
	}
	return x
}
func RunInternalSelfTests(dataDir string) []SelfTestResult {
	tests := []SelfTestResult{}
	add := func(n string, e error) {
		tests = append(tests, SelfTestResult{Name: n, Passed: e == nil, Detail: func() string {
			if e != nil {
				return e.Error()
			}
			return "OK"
		}()})
	}
	_, e := BuildClassicRangeReport(900)
	add("Native range report", e)
	_, e = BuildClassicConstantForceReport(5, false)
	add("Native constant force", e)
	_, e = BuildClassicSpringReport(10, false)
	add("Native spring", e)
	_, e = ParseLogiMateJSONTelemetry([]byte(`{"rpm":5000,"rpmRedline":7000,"force":0.5,"physics":true,"playerControl":true}`))
	add("Telemetry parser", e)

	p := NativeEngineProfile{Name: "test", RotationDegrees: 900, MasterGainPercent: 100, ConstantGainPercent: 100, SpringGainPercent: 100, DamperGainPercent: 100, FrictionGainPercent: 100}
	m := MixNativeEffects(NativeEffectRequest{Constant: 100}, p, NativeFFBConfig{MasterGainPercent: 100, ConstantLimit: 10, SpringGain: 10, DamperGain: 10, FrictionGain: 10, SlewPerTick: 2, WatchdogMS: 1000})
	if m.Constant > 10 || m.Constant < -10 {
		add("Force mixer limits", fmt.Errorf("hard limit verletzt: %d", m.Constant))
	} else {
		add("Force mixer limits", nil)
	}

	tmp := filepath.Join(dataDir, ".selftest-atomic.tmp")
	e = AtomicWriteFile(tmp, []byte("ok"), 0644)
	if e == nil {
		_ = os.Remove(tmp)
	}
	add("Atomic write", e)

	golden := []struct {
		name      string
		got, want []byte
	}{
		{"constant", wheelengine.ConstantForce(0x80), []byte{0x11, 0x08, 0x80, 0x80, 0, 0, 0}},
		{"range", wheelengine.SetRange(900), []byte{0xF8, 0x81, 0x84, 0x03, 0, 0, 0}},
		{"native-g27", wheelengine.NativeSwitch(0x04), []byte{0xF8, 0x09, 0x04, 0x01, 0, 0, 0}},
		{"spring", wheelengine.SpringSet(0x07, 0x80), []byte{0xFE, 0x0D, 0x07, 0x07, 0x80, 0, 0}},
		{"damper", wheelengine.Damper(0xFF, 0x80), []byte{0x41, 0x0C, 0x0F, 0x00, 0x0F, 0x00, 0x80}},
		{"friction", wheelengine.Friction(0x40, 0xFF), []byte{0x81, 0x0E, 0x40, 0x40, 0xFF, 0, 0}},
		{"led", wheelengine.SetLeds(0x1F), []byte{0xF8, 0x12, 0x1F, 0, 0, 0, 0}},
	}
	for _, v := range golden {
		if string(v.got) != string(v.want) {
			add("Native protocol "+v.name, fmt.Errorf("got % X want % X", v.got, v.want))
		} else {
			add("Native protocol "+v.name, nil)
		}
	}
	for _, model := range []wheelengine.ModelID{wheelengine.ModelG25, wheelengine.ModelG27, wheelengine.ModelDFGT} {
		add("Model adapter "+string(model), wheelengine.ValidateModelAdapter(model))
	}
	g27 := []byte{0x00, 0x08, 0x00, 0x00, 0x00, 0x80, 0xFF, 0xFF, 0xFF, 0x67, 0x78, 0x98}
	if _, err := wheelengine.ParseNativeInputReport(wheelengine.ModelG27, g27); err != nil {
		add("Native G27 parser", err)
	} else {
		add("Native G27 parser", nil)
	}
	add("Native engine core gate", NativeEngineCoreGate())
	_, stressErr := HIDStressSoftwareSelfTest(2048)
	add("HID adverse-I/O policy stress", stressErr)
	return tests
}

func snapshotManifestEntry(name string, b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]) + "  " + name
}
func CreateConfigSnapshot(dataDir string) (string, error) {
	dir := filepath.Join(dataDir, "Snapshots")
	if e := os.MkdirAll(dir, 0755); e != nil {
		return "", e
	}
	path := filepath.Join(dir, "LogiMate-Config-"+time.Now().Format("20060102-150405")+".zip")
	f, e := os.Create(path)
	if e != nil {
		return "", e
	}
	zw := zip.NewWriter(f)
	var manifest []string
	for _, name := range supportedConfigSnapshotFiles {
		b, e := os.ReadFile(filepath.Join(dataDir, name))
		if e != nil {
			if os.IsNotExist(e) {
				continue
			}
			zw.Close()
			f.Close()
			return "", e
		}
		w, _ := zw.Create(name)
		_, _ = w.Write(b)
		manifest = append(manifest, snapshotManifestEntry(name, b))
	}
	mw, _ := zw.Create("MANIFEST.SHA256")
	_, _ = mw.Write([]byte(strings.Join(manifest, "\n") + "\n"))
	if e = zw.Close(); e != nil {
		f.Close()
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	return path, nil
}
func RestoreConfigSnapshot(dataDir, zipPath string) error {
	backup, _ := CreateConfigSnapshot(dataDir)
	_ = backup
	r, e := zip.OpenReader(zipPath)
	if e != nil {
		return e
	}
	defer r.Close()
	allowed := map[string]bool{}
	for _, n := range supportedConfigSnapshotFiles {
		allowed[n] = true
	}
	files := map[string][]byte{}
	var manifest []byte
	for _, z := range r.File {
		name := filepath.Base(z.Name)
		if name != z.Name {
			return fmt.Errorf("ungültiger Snapshot-Pfad")
		}
		if z.UncompressedSize64 > 2<<20 {
			return fmt.Errorf("Snapshot-Datei zu groß: %s", name)
		}
		rc, e := z.Open()
		if e != nil {
			return e
		}
		b, e := io.ReadAll(io.LimitReader(rc, 2<<20+1))
		rc.Close()
		if e != nil {
			return e
		}
		if name == "MANIFEST.SHA256" {
			manifest = b
			continue
		}
		if !allowed[name] {
			return fmt.Errorf("nicht erlaubte Snapshot-Datei: %s", name)
		}
		files[name] = b
	}
	if len(manifest) == 0 {
		return fmt.Errorf("Snapshot ohne Prüfsummenmanifest")
	}
	expected := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			expected[parts[len(parts)-1]] = parts[0]
		}
	}
	for n, b := range files {
		h := sha256.Sum256(b)
		if !strings.EqualFold(expected[n], hex.EncodeToString(h[:])) {
			return fmt.Errorf("Prüfsumme stimmt nicht: %s", n)
		}
	}
	for n, b := range files {
		if e := AtomicWriteFile(filepath.Join(dataDir, n), b, 0644); e != nil {
			return e
		}
	}
	return nil
}

type outputRecoveryMarker struct {
	Active              bool      `json:"active"`
	Effect              string    `json:"effect"`
	WheelID             string    `json:"wheelId"`
	SessionID           string    `json:"sessionId,omitempty"`
	Model               string    `json:"model,omitempty"`
	HardwareFingerprint string    `json:"hardwareFingerprint,omitempty"`
	At                  time.Time `json:"at"`
}

func runtimeOutputMarkerPath(dataDir string) string {
	return filepath.Join(dataDir, "runtime-output.pending")
}

// MarkRuntimeOutputActiveTarget persists enough identity to prevent a recovery
// command being sent to a different wheel that later occupies the same USB
// topology slot.
func MarkRuntimeOutputActiveTarget(dataDir string, w WheelDevice, effect string) error {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(w.ID) == "" || strings.TrimSpace(w.SessionID) == "" || !IsKnownWheelModel(w.Model) {
		return fmt.Errorf("unvollständiger Output-Recovery-Fingerprint")
	}
	b, err := json.Marshal(outputRecoveryMarker{
		Active: true, Effect: effect, WheelID: w.ID, SessionID: w.SessionID, Model: w.Model,
		HardwareFingerprint: w.HardwareFingerprint, At: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return AtomicWriteFile(runtimeOutputMarkerPath(dataDir), b, 0644)
}

// MarkRuntimeOutputActive remains for compatibility with older tests/callers,
// but intentionally fails closed because a wheel ID alone is not a sufficient
// recovery fingerprint after the C8 audit.
func MarkRuntimeOutputActive(dataDir, wheelID, effect string) error {
	return fmt.Errorf("legacy output marker without session/model fingerprint rejected for %s (%s)", wheelID, effect)
}

func ClearRuntimeOutputMarker(dataDir string) error {
	if strings.TrimSpace(dataDir) == "" {
		return nil
	}
	err := os.Remove(runtimeOutputMarkerPath(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func readRuntimeOutputRecoveryMarker(dataDir string) (outputRecoveryMarker, error) {
	var m outputRecoveryMarker
	b, err := os.ReadFile(runtimeOutputMarkerPath(dataDir))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if !m.Active {
		return m, fmt.Errorf("Recovery-Marker ist nicht aktiv")
	}
	return m, nil
}

func RuntimeOutputRecoveryNeeded(dataDir string) (bool, string) {
	m, err := readRuntimeOutputRecoveryMarker(dataDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, ""
		}
		return true, "Unlesbarer/ungültiger Output-Recovery-Marker: " + err.Error()
	}
	return true, fmt.Sprintf("%s · %s · %s", m.Effect, m.Model, redactIdentifier(m.WheelID))
}

// RecoverRuntimeOutputIfNeeded retries recovery whenever a fresh state becomes
// available. The marker is cleared only after exact target correlation and
// successful neutral/stop writes.
func RecoverRuntimeOutputIfNeeded(s State) (bool, string, error) {
	m, err := readRuntimeOutputRecoveryMarker(s.DataDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		return false, "", err
	}
	w, ok := SelectedWheel(s)
	if !ok || !w.PnPVerified || !w.ModelConfirmed || !strings.EqualFold(w.ID, m.WheelID) || !strings.EqualFold(w.SessionID, m.SessionID) || !sameKnownWheelModel(w.Model, m.Model) {
		return false, fmt.Sprintf("%s · Ziel-Fingerprint noch nicht bestätigt", m.Effect), fmt.Errorf("Recovery-Ziel stimmt nicht mit dem aktuell verifizierten Wheel überein")
	}
	// When Windows exposes a serial-derived hardware fingerprint, recovery must
	// bind to it as well. A marker that claims one may never be recovered on a
	// wheel without the exact same strong identity.
	if strings.TrimSpace(m.HardwareFingerprint) != "" && !strings.EqualFold(m.HardwareFingerprint, w.HardwareFingerprint) {
		return false, fmt.Sprintf("%s · Hardware-Fingerprint stimmt nicht", m.Effect), fmt.Errorf("Recovery-Hardware-Fingerprint stimmt nicht mit dem aktuell verifizierten Wheel überein")
	}
	pid, ok := nativeOutputPID(w.Model)
	if !ok {
		return false, m.Effect, fmt.Errorf("kein Recovery-Protokoll für %s", w.Model)
	}
	paths := rawPathsForWheel(w, s.RawInputDevices, pid)
	if len(paths) != 1 {
		return false, m.Effect, fmt.Errorf("Recovery-HID-Pfad nicht eindeutig (%d Treffer)", len(paths))
	}
	if err := writeNativeOutputPath(paths[0], neutralReportsForModel(w.Model)...); err != nil {
		return false, m.Effect, err
	}
	if err := ClearRuntimeOutputMarker(s.DataDir); err != nil {
		return false, m.Effect, err
	}
	return true, m.Effect, nil
}

func sameKnownWheelModel(a, b string) bool {
	return (IsG25Model(a) && IsG25Model(b)) || (IsG27Model(a) && IsG27Model(b)) || (IsDFGTModel(a) && IsDFGTModel(b))
}

const CurrentDataSchemaVersion = 2

type schemaFile struct {
	Version    int       `json:"version"`
	MigratedAt time.Time `json:"migratedAt"`
}

func EnsureDataSchema(dataDir string) error {
	path := filepath.Join(dataDir, "data-schema.json")
	ver := 0
	if b, e := os.ReadFile(path); e == nil {
		var f schemaFile
		if json.Unmarshal(b, &f) == nil {
			ver = f.Version
		}
	}
	if ver > CurrentDataSchemaVersion {
		return fmt.Errorf("Daten-Schema %d ist neuer als diese LogiMate-Version (%d); Downgrade-Schreibzugriffe werden blockiert", ver, CurrentDataSchemaVersion)
	}
	if ver == CurrentDataSchemaVersion {
		return nil
	}
	if ver > 0 {
		if _, e := CreateConfigSnapshot(dataDir); e != nil {
			return fmt.Errorf("Schema-Backup fehlgeschlagen: %w", e)
		}
	}
	b, _ := json.MarshalIndent(schemaFile{CurrentDataSchemaVersion, time.Now()}, "", "  ")
	return AtomicWriteFile(path, b, 0644)
}
func BuildReadinessReport(s State) ReadinessReport {
	issues := ValidateStateInvariants(s)
	score := 100
	var msgs []string
	for _, i := range issues {
		msgs = append(msgs, i.Severity+": "+i.Message)
		switch i.Severity {
		case "critical":
			score -= 25
		case "error":
			score -= 15
		default:
			score -= 5
		}
	}
	for _, t := range RunInternalSelfTests(s.DataDir) {
		if !t.Passed {
			score -= 10
			msgs = append(msgs, "Selftest: "+t.Name+": "+t.Detail)
		}
	}
	if score < 0 {
		score = 0
	}
	return ReadinessReport{Score: score, CodeAuditComplete: true, Issues: msgs, ExternalGates: []string{
		"Physische G25/G27/DFGT-Matrix",
		"Crash/USB-Yank/Suspend-Recovery auf echter Hardware",
		"Modern↔Legacy/HVCI-Migration auf echtem Windows",
		"HID-Stresstest / adverse I/O auf echter Hardware",
		"Vollständige Windows UI/Accessibility-Validierung",
		"Authenticode: signierte App + Installer mit gleichem Publisher",
	}}
}
func EvaluateStableReleaseGate(s State, hardwareValidated, signedBinary, fullUIA, hidStressValidated bool) StableReleaseGate {
	r := BuildReadinessReport(s)
	var b []string
	if r.Score < 100 {
		b = append(b, "interne Readiness unter 100")
	}
	if !hardwareValidated {
		b = append(b, "Hardware-Matrix nicht vollständig validiert")
	}
	if !signedBinary {
		b = append(b, "Release nicht Authenticode-signiert")
	}
	if !fullUIA {
		b = append(b, "UI Automation nicht vollständig")
	}
	if !hidStressValidated {
		b = append(b, "HID-Stress/adverse-I/O nicht physisch validiert")
	}
	return StableReleaseGate{Ready: len(b) == 0, Blockers: b}
}
func SortedInvariantMessages(v []StateInvariantIssue) []string {
	out := make([]string, len(v))
	for i, x := range v {
		out[i] = x.Severity + ":" + x.Code + ":" + x.Message
	}
	sort.Strings(out)
	return out
}
