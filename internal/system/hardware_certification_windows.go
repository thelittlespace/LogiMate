//go:build windows

package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

const hardwareCertificationSchema = 1

// CertificationResult is one evidence item collected on a real wheel.
// Nothing in this file promotes a model automatically to Stable: user-visible
// physical tests remain evidence and the repository-level Stable gate stays
// separate in docs/HARDWARE_CERTIFICATION.json.
type CertificationResult struct {
	Check       string    `json:"check"`
	Passed      bool      `json:"passed"`
	Evidence    string    `json:"evidence"`
	CapturedAt  time.Time `json:"capturedAt"`
	WheelID     string    `json:"wheelId"`
	SessionID   string    `json:"sessionId"`
	Model       string    `json:"model"`
	NativePID   string    `json:"nativePid,omitempty"`
	AppVersion  string    `json:"appVersion"`
	InputSource string    `json:"inputSource,omitempty"`
	LayoutID    string    `json:"layoutId,omitempty"`
}

type wheelCertificationRecord struct {
	WheelID   string                         `json:"wheelId"`
	Model     string                         `json:"model"`
	StartedAt time.Time                      `json:"startedAt"`
	UpdatedAt time.Time                      `json:"updatedAt"`
	Results   map[string]CertificationResult `json:"results"`
}

type hardwareCertificationFile struct {
	SchemaVersion int                                 `json:"schemaVersion"`
	Records       map[string]wheelCertificationRecord `json:"records"`
}

type CertificationItem struct {
	Check    string
	Label    string
	Passed   bool
	Failed   bool
	Evidence string
	At       time.Time
}

type CertificationProgress struct {
	WheelID  string
	Model    string
	Required int
	Passed   int
	Failed   int
	Pending  int
	Complete bool
	Error    string
	Items    []CertificationItem
}

var certificationFileCache = struct {
	sync.Mutex
	path   string
	loaded time.Time
	value  hardwareCertificationFile
}{}

func certificationEvidencePath(dataDir string) string {
	return filepath.Join(dataDir, "Certification", "hardware-evidence.json")
}

func certificationRecordKey(w WheelDevice) string {
	id := strings.ToLower(strings.TrimSpace(w.ID))
	if id == "" {
		id = strings.ToLower(strings.TrimSpace(w.SessionID))
	}
	return id + "|" + string(ClassifyWheelModel(w.Model))
}

func loadHardwareCertification(dataDir string) (hardwareCertificationFile, error) {
	path := certificationEvidencePath(dataDir)
	certificationFileCache.Lock()
	if certificationFileCache.path == path && !certificationFileCache.loaded.IsZero() && time.Since(certificationFileCache.loaded) < 2*time.Second {
		f := certificationFileCache.value
		certificationFileCache.Unlock()
		return f, nil
	}
	certificationFileCache.Unlock()
	f := hardwareCertificationFile{SchemaVersion: hardwareCertificationSchema, Records: map[string]wheelCertificationRecord{}}
	found, err := ReadJSONConfigStrict(path, &f)
	if err != nil {
		return f, err
	}
	if !found {
		certificationFileCache.Lock()
		certificationFileCache.path = path
		certificationFileCache.loaded = time.Now()
		certificationFileCache.value = f
		certificationFileCache.Unlock()
		return f, nil
	}
	if f.SchemaVersion > hardwareCertificationSchema {
		return f, fmt.Errorf("Hardware-Zertifizierungsdaten verwenden ein neueres Schema %d", f.SchemaVersion)
	}
	if f.SchemaVersion <= 0 {
		f.SchemaVersion = hardwareCertificationSchema
	}
	if f.Records == nil {
		f.Records = map[string]wheelCertificationRecord{}
	}
	certificationFileCache.Lock()
	certificationFileCache.path = path
	certificationFileCache.loaded = time.Now()
	certificationFileCache.value = f
	certificationFileCache.Unlock()
	return f, nil
}

func saveHardwareCertification(dataDir string, f hardwareCertificationFile) error {
	f.SchemaVersion = hardwareCertificationSchema
	if f.Records == nil {
		f.Records = map[string]wheelCertificationRecord{}
	}
	path := certificationEvidencePath(dataDir)
	if err := WriteJSONConfigStrict(path, f, 0600); err != nil {
		return err
	}
	certificationFileCache.Lock()
	certificationFileCache.path = path
	certificationFileCache.loaded = time.Now()
	certificationFileCache.value = f
	certificationFileCache.Unlock()
	return nil
}

func certificationModelForState(s State) (WheelDevice, wheelengine.ModelID, error) {
	w, ok := SelectedWheel(s)
	if !ok {
		return WheelDevice{}, wheelengine.ModelUnknown, errors.New("kein eindeutig ausgewähltes Lenkrad")
	}
	model := ClassifyWheelModel(w.Model)
	if model != wheelengine.ModelG25 && model != wheelengine.ModelG27 && model != wheelengine.ModelDFGT {
		return WheelDevice{}, model, errors.New("Hardware-Zertifizierung ist nur für G25, G27 und Driving Force GT verfügbar")
	}
	if !w.PnPVerified || !w.ModelConfirmed {
		return WheelDevice{}, model, errors.New("Lenkrad ist noch nicht PnP-verifiziert und modellbestätigt")
	}
	return w, model, nil
}

var certificationWorkflowOrder = []string{
	"native-mode-switch", "steering", "pedals", "buttons", "shifter", "range",
	"constant-force", "spring", "damper", "friction", "autocenter", "game-ffb",
	"reconnect", "usb-removal", "process-kill", "suspend-resume", "modern-legacy-rollback",
}

func orderCertificationChecks(required []string) []string {
	wanted := map[string]bool{}
	for _, c := range required {
		wanted[c] = true
	}
	out := make([]string, 0, len(required))
	for _, c := range certificationWorkflowOrder {
		if wanted[c] {
			out = append(out, c)
			delete(wanted, c)
		}
	}
	if len(wanted) > 0 {
		extra := make([]string, 0, len(wanted))
		for c := range wanted {
			extra = append(extra, c)
		}
		sort.Strings(extra)
		out = append(out, extra...)
	}
	return out
}

func CertificationCheckLabel(check string) string {
	switch check {
	case "native-mode-switch":
		return "Native Mode / PID"
	case "steering":
		return "Lenkung"
	case "pedals":
		return "Pedale"
	case "buttons":
		return "Tasten & Wippen"
	case "shifter":
		return "H-Shifter"
	case "range":
		return "Lenkwinkel / Range"
	case "constant-force":
		return "Constant Force"
	case "spring":
		return "Spring"
	case "damper":
		return "Damper"
	case "friction":
		return "Friction"
	case "autocenter":
		return "Autocenter"
	case "game-ffb":
		return "Game FFB / Telemetrie"
	case "reconnect":
		return "Reconnect"
	case "usb-removal":
		return "USB-Abziehen unter Betrieb"
	case "process-kill":
		return "Prozess-Kill / Recovery"
	case "suspend-resume":
		return "Standby / Resume"
	case "modern-legacy-rollback":
		return "Modern ↔ Legacy ↔ Modern"
	default:
		return check
	}
}

func HardwareCertificationProgress(s State) CertificationProgress {
	w, model, err := certificationModelForState(s)
	if err != nil {
		return CertificationProgress{WheelID: s.SelectedWheelID, Model: s.WheelModel}
	}
	required := orderCertificationChecks(wheelengine.RequiredHardwareChecks(model))
	p := CertificationProgress{WheelID: w.ID, Model: w.Model, Required: len(required)}
	f, loadErr := loadHardwareCertification(s.DataDir)
	var rec wheelCertificationRecord
	if loadErr == nil {
		rec = f.Records[certificationRecordKey(w)]
	} else {
		p.Error = loadErr.Error()
	}
	for _, check := range required {
		item := CertificationItem{Check: check, Label: CertificationCheckLabel(check)}
		if r, ok := rec.Results[check]; ok {
			item.Passed = r.Passed
			item.Failed = !r.Passed
			item.Evidence = r.Evidence
			item.At = r.CapturedAt
			if r.Passed {
				p.Passed++
			} else {
				p.Failed++
			}
		} else {
			p.Pending++
		}
		p.Items = append(p.Items, item)
	}
	p.Complete = p.Required > 0 && p.Passed == p.Required
	return p
}

func RecordHardwareCertificationResult(s State, check string, passed bool, evidence string) error {
	w, model, err := certificationModelForState(s)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, c := range wheelengine.RequiredHardwareChecks(model) {
		allowed[c] = true
	}
	if !allowed[check] {
		return fmt.Errorf("Prüfung %q ist für %s nicht vorgesehen", check, w.Model)
	}
	evidence = strings.TrimSpace(evidence)
	if evidence == "" {
		return errors.New("Zertifizierungsevidenz darf nicht leer sein")
	}
	f, err := loadHardwareCertification(s.DataDir)
	if err != nil {
		return err
	}
	key := certificationRecordKey(w)
	rec := f.Records[key]
	if rec.Results == nil {
		rec.Results = map[string]CertificationResult{}
	}
	now := time.Now().UTC()
	if rec.StartedAt.IsZero() {
		rec.StartedAt = now
	}
	rec.WheelID, rec.Model, rec.UpdatedAt = w.ID, canonicalWheelModel(w.Model), now
	j := ReadPreferredWheelInput(s)
	rec.Results[check] = CertificationResult{
		Check: check, Passed: passed, Evidence: evidence, CapturedAt: now,
		WheelID: w.ID, SessionID: w.SessionID, Model: canonicalWheelModel(w.Model),
		NativePID: devicePID(w.InstanceID), AppVersion: s.AppVersion,
		InputSource: j.InputSource, LayoutID: j.LayoutID,
	}
	f.Records[key] = rec
	return saveHardwareCertification(s.DataDir, f)
}

func ClearHardwareCertificationResult(s State, check string) error {
	w, _, err := certificationModelForState(s)
	if err != nil {
		return err
	}
	f, err := loadHardwareCertification(s.DataDir)
	if err != nil {
		return err
	}
	key := certificationRecordKey(w)
	rec, ok := f.Records[key]
	if !ok || rec.Results == nil {
		return nil
	}
	delete(rec.Results, check)
	rec.UpdatedAt = time.Now().UTC()
	f.Records[key] = rec
	return saveHardwareCertification(s.DataDir, f)
}

func ResetHardwareCertification(s State) error {
	w, _, err := certificationModelForState(s)
	if err != nil {
		return err
	}
	f, err := loadHardwareCertification(s.DataDir)
	if err != nil {
		return err
	}
	delete(f.Records, certificationRecordKey(w))
	return saveHardwareCertification(s.DataDir, f)
}

// AutoCertificationEvidence records only facts LogiMate can prove without
// asking the user to judge physical force/feel. It intentionally does not
// auto-pass motor-effect checks or destructive recovery tests.
func AutoCertificationEvidence(s State) (string, bool, string) {
	w, model, err := certificationModelForState(s)
	if err != nil {
		return "", false, err.Error()
	}
	desc, ok := wheelengine.DescriptorForModel(model)
	if !ok {
		return "", false, "Modelldeskriptor fehlt"
	}
	currentPID := strings.ToUpper(strings.TrimSpace(devicePID(w.InstanceID)))
	if currentPID == strings.ToUpper(desc.NativePID) && w.PnPVerified && w.ModelConfirmed {
		evidence := fmt.Sprintf("PnP-verifiziert; Modell=%s; native PID=%s; Session=%s", canonicalWheelModel(w.Model), currentPID, w.SessionID)
		return "native-mode-switch", true, evidence
	}
	return "native-mode-switch", false, fmt.Sprintf("Erwartet native PID %s, aktuell %s", desc.NativePID, firstNonEmpty(currentPID, "unbekannt"))
}

func CertificationReportMarkdown(s State) (string, error) {
	w, model, err := certificationModelForState(s)
	if err != nil {
		return "", err
	}
	p := HardwareCertificationProgress(s)
	f, err := loadHardwareCertification(s.DataDir)
	if err != nil {
		return "", err
	}
	rec := f.Records[certificationRecordKey(w)]
	var b strings.Builder
	fmt.Fprintf(&b, "# LogiMate Hardware Certification Evidence\n\n")
	fmt.Fprintf(&b, "- App: %s\n- Wheel: %s\n- Wheel-ID: `%s`\n- Session: `%s`\n- Model-ID: `%s`\n- PnP verified: %v\n- Model confirmed: %v\n- Progress: %d/%d passed\n- Generated: %s\n\n", s.AppVersion, canonicalWheelModel(w.Model), redactIdentifier(w.ID), redactIdentifier(w.SessionID), model, w.PnPVerified, w.ModelConfirmed, p.Passed, p.Required, time.Now().UTC().Format(time.RFC3339))
	b.WriteString("| Check | Status | Evidence | Captured |\n|---|---|---|---|\n")
	for _, item := range p.Items {
		status := "PENDING"
		captured := ""
		evidence := ""
		if r, ok := rec.Results[item.Check]; ok {
			if r.Passed {
				status = "PASS"
			} else {
				status = "FAIL"
			}
			captured = r.CapturedAt.Format(time.RFC3339)
			evidence = r.Evidence
			if strings.TrimSpace(w.ID) != "" {
				evidence = strings.ReplaceAll(evidence, w.ID, redactIdentifier(w.ID))
			}
			if strings.TrimSpace(w.SessionID) != "" {
				evidence = strings.ReplaceAll(evidence, w.SessionID, redactIdentifier(w.SessionID))
			}
			evidence = strings.ReplaceAll(evidence, "|", "\\|")
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", item.Label, status, evidence, captured)
	}
	b.WriteString("\n> This evidence file does not by itself promote a Stable release. Repository Stable gates and per-model evidence review remain authoritative.\n")
	return b.String(), nil
}

func ExportHardwareCertificationReport(s State) (string, error) {
	w, _, err := certificationModelForState(s)
	if err != nil {
		return "", err
	}
	md, err := CertificationReportMarkdown(s)
	if err != nil {
		return "", err
	}
	f, err := loadHardwareCertification(s.DataDir)
	if err != nil {
		return "", err
	}
	rec := f.Records[certificationRecordKey(w)]
	dir := filepath.Join(s.DataDir, "Certification")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	base := fmt.Sprintf("LogiMate-%s-%s-certification", strings.ToLower(string(ClassifyWheelModel(w.Model))), stamp)
	mdPath := filepath.Join(dir, base+".md")
	jsonPath := filepath.Join(dir, base+".json")
	if err := AtomicWriteFile(mdPath, []byte(md), 0600); err != nil {
		return "", err
	}
	publicRec := rec
	publicRec.WheelID = redactIdentifier(rec.WheelID)
	publicRec.Results = make(map[string]CertificationResult, len(rec.Results))
	for key, result := range rec.Results {
		result.WheelID = redactIdentifier(result.WheelID)
		result.SessionID = redactIdentifier(result.SessionID)
		if strings.TrimSpace(w.ID) != "" {
			result.Evidence = strings.ReplaceAll(result.Evidence, w.ID, redactIdentifier(w.ID))
		}
		if strings.TrimSpace(w.SessionID) != "" {
			result.Evidence = strings.ReplaceAll(result.Evidence, w.SessionID, redactIdentifier(w.SessionID))
		}
		publicRec.Results[key] = result
	}
	jb, err := json.MarshalIndent(publicRec, "", "  ")
	if err != nil {
		return "", err
	}
	if err := AtomicWriteFile(jsonPath, jb, 0600); err != nil {
		return "", err
	}
	return mdPath, nil
}

func CertificationPendingChecks(s State) []string {
	p := HardwareCertificationProgress(s)
	var out []string
	for _, it := range p.Items {
		if !it.Passed {
			out = append(out, it.Check)
		}
	}
	sort.Strings(out)
	return out
}
