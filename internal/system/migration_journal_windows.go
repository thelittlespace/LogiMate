//go:build windows

package system

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type MigrationStep struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // pending/running/done/warning/failed/rollback
	Detail    string `json:"detail,omitempty"`
	Timestamp string `json:"timestamp"`
}

type MigrationResult struct {
	ID        string          `json:"id"`
	Action    string          `json:"action"`
	Started   string          `json:"started"`
	Finished  string          `json:"finished,omitempty"`
	Status    string          `json:"status"` // pending/running/success/failed/cancelled
	Success   bool            `json:"success"`
	Completed bool            `json:"completed"`
	Message   string          `json:"message,omitempty"`
	Error     string          `json:"error,omitempty"`
	Steps     []MigrationStep `json:"steps,omitempty"`
	Snapshot  struct {
		HVCI                bool   `json:"hvci"`
		OperatingPreference string `json:"operatingPreference,omitempty"`
		WheelModel          string `json:"wheelModel,omitempty"`
		ActiveMode          string `json:"activeMode,omitempty"`
		ProfilerInstalled   bool   `json:"profilerInstalled"`
	} `json:"snapshot"`
}

func migrationRoot(dataDir string) string { return filepath.Join(dataDir, "Migrations") }
func migrationPath(dataDir, id string) string {
	return filepath.Join(migrationRoot(dataDir), id+".json")
}

func newMigrationID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

func writeMigration(dataDir string, m MigrationResult) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(migrationPath(dataDir, m.ID), b, 0644)
}

func BeginMigration(dataDir, action string) (string, error) {
	if pending, _ := ListIncompleteMigrations(dataDir); len(pending) > 0 {
		return "", fmt.Errorf("eine frühere Migration ist noch ungeklärt (%s); zuerst Recovery/Diagnose abschließen", pending[0].ID)
	}
	id := newMigrationID()
	m := MigrationResult{
		ID: id, Action: action, Started: time.Now().Format(time.RFC3339), Status: "pending",
	}
	if err := os.MkdirAll(migrationRoot(dataDir), 0755); err != nil {
		return "", err
	}
	if err := writeMigration(dataDir, m); err != nil {
		return "", err
	}
	cleanupOldMigrations(dataDir, 30*24*time.Hour)
	return id, nil
}

func LoadMigration(dataDir, id string) (MigrationResult, error) {
	var m MigrationResult
	if id == "" {
		return m, errors.New("leere Migrations-ID")
	}
	b, err := os.ReadFile(migrationPath(dataDir, id))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

func MarkMigrationRunning(dataDir, id, action string) error {
	m, err := LoadMigration(dataDir, id)
	if err != nil {
		return fmt.Errorf("Migrationsjournal %s konnte nicht geladen werden: %w", id, err)
	}
	m.Action = action
	m.Status = "running"
	m.Snapshot.HVCI = HVCIEnabled()
	m.Snapshot.OperatingPreference = ReadOperatingPreference(dataDir)
	ds := DetectDevices()
	raw := EnumerateRawInputLogitechWheels()
	model, effective, _ := resolveWheelModel(ds, raw, "")
	m.Snapshot.WheelModel = model
	mode := "KEIN LOGITECH WHEEL"
	if len(effective) > 0 {
		mode = "Generic HID / Modern"
		for _, d := range effective {
			x := strings.ToLower(d.Service + " " + d.INF)
			if strings.Contains(x, "wm") || strings.Contains(x, "lgjoyhid") {
				mode = "Logitech Legacy"
				break
			}
		}
	}
	m.Snapshot.ActiveMode = mode
	if p, pErr := DetectProfilerStrict(); pErr == nil {
		m.Snapshot.ProfilerInstalled = p.Installed
	}
	return writeMigration(dataDir, m)
}

func AppendMigrationStep(dataDir, id, name, status, detail string) error {
	if id == "" {
		return errors.New("leere Migrations-ID")
	}
	m, err := LoadMigration(dataDir, id)
	if err != nil {
		return err
	}
	if m.Completed {
		return fmt.Errorf("Migration %s ist bereits abgeschlossen", id)
	}
	m.Steps = append(m.Steps, MigrationStep{Name: name, Status: status, Detail: detail, Timestamp: time.Now().Format(time.RFC3339)})
	return writeMigration(dataDir, m)
}

func CompleteMigration(dataDir, id string, success bool, message string, cause error) error {
	if id == "" {
		return errors.New("leere Migrations-ID")
	}
	m, err := LoadMigration(dataDir, id)
	if err != nil {
		return err
	}
	m.Completed = true
	m.Success = success
	m.Finished = time.Now().Format(time.RFC3339)
	m.Message = message
	if success {
		m.Status = "success"
		m.Error = ""
	} else {
		m.Status = "failed"
		if cause != nil {
			m.Error = cause.Error()
		}
	}
	return writeMigration(dataDir, m)
}

func CancelMigration(dataDir, id, reason string) error {
	if id == "" {
		return errors.New("leere Migrations-ID")
	}
	m, err := LoadMigration(dataDir, id)
	if err != nil {
		return err
	}
	m.Completed = true
	m.Success = false
	m.Status = "cancelled"
	m.Error = reason
	m.Finished = time.Now().Format(time.RFC3339)
	return writeMigration(dataDir, m)
}

func ListIncompleteMigrations(dataDir string) ([]MigrationResult, error) {
	entries, err := os.ReadDir(migrationRoot(dataDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []MigrationResult
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, readErr := os.ReadFile(filepath.Join(migrationRoot(dataDir), e.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var m MigrationResult
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, fmt.Errorf("Migrationsjournal %s ist beschädigt: %w", e.Name(), err)
		}
		if !m.Completed || m.Status == "pending" || m.Status == "running" {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Started < out[j].Started })
	return out, nil
}

func migrationTargetSatisfied(dataDir string, m MigrationResult) (bool, string) {
	s := CollectState()
	if s.DeviceDetectionError != "" || !HasActionableSelectedWheel(s) {
		return false, "Gerätezustand ist nicht sicher verifizierbar"
	}
	a := strings.ToLower(m.Action)
	switch {
	case strings.Contains(a, "setup-modern"):
		ok := strings.Contains(strings.ToLower(s.ActiveMode), "generic hid") && ReadOperatingPreference(dataDir) == "modern"
		return ok, s.ActiveMode
	case strings.Contains(a, "setup-legacy"):
		ok := strings.Contains(strings.ToLower(s.ActiveMode), "legacy") && ReadOperatingPreference(dataDir) == "legacy"
		return ok, s.ActiveMode
	default:
		return false, "unbekannte Migrationsaktion"
	}
}

// RecoverInterruptedMigrations is non-destructive. It only marks an interrupted
// journal successful when a fresh device scan proves the requested terminal
// state. Otherwise the journal remains unresolved and blocks new migrations.
func RecoverInterruptedMigrations(dataDir string) ([]string, error) {
	pending, err := ListIncompleteMigrations(dataDir)
	if err != nil {
		return nil, err
	}
	var notes []string
	for _, m := range pending {
		ok, detail := migrationTargetSatisfied(dataDir, m)
		if !ok {
			notes = append(notes, fmt.Sprintf("%s (%s) bleibt ungeklärt: %s", m.ID, m.Action, detail))
			continue
		}
		msg := "Unterbrochene Migration beim nächsten Start anhand des verifizierten Zielzustands abgeschlossen: " + detail
		if err := CompleteMigration(dataDir, m.ID, true, msg, nil); err != nil {
			return notes, err
		}
		notes = append(notes, fmt.Sprintf("%s recovered: %s", m.ID, detail))
	}
	if remaining, _ := ListIncompleteMigrations(dataDir); len(remaining) > 0 {
		return notes, fmt.Errorf("%d unvollständige Migration(en) benötigen manuelle Prüfung", len(remaining))
	}
	return notes, nil
}

func cleanupOldMigrations(dataDir string, maxAge time.Duration) {
	entries, err := os.ReadDir(migrationRoot(dataDir))
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		p := filepath.Join(migrationRoot(dataDir), e.Name())
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			continue
		}
		var m MigrationResult
		if json.Unmarshal(b, &m) != nil || !m.Completed {
			continue // unresolved evidence is never aged away
		}
		if st, statErr := os.Stat(p); statErr == nil && st.ModTime().Before(cutoff) {
			_ = os.Remove(p)
		}
	}
}
