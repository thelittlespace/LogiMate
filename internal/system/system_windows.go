//go:build windows

package system

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type Device struct {
	Status, Name, Service, INF, InstanceID string
	// ParentID is the immediate Windows PnP parent. For HID collections of the
	// classic Logitech wheels this normally resolves to the USB wheel devnode.
	ParentID string
	// PhysicalID identifies the current Windows device-container/session view.
	// It is useful for grouping USB + HID function nodes, but can change when a
	// multimode Logitech wheel re-enumerates from C294 to its native PID.
	PhysicalID string
	// ContainerID is Windows' device-container GUID. All current devnodes that
	// belong to one physical device share it. Container IDs are therefore the
	// primary session-grouping key, but not LogiMate's persisted wheel identity.
	ContainerID string
	// LocationPath is Windows' device-tree location. For the USB wheel devnode it
	// remains tied to the physical USB topology and is the strongest available
	// stable identity input when the wheel itself exposes no serial number.
	LocationPath string
	// StableID is LogiMate's persisted identity. It prefers a hashed Windows
	// LocationPath and falls back to the USB devnode's topology suffix. Both
	// survive C294 <-> C299/C29A/C29B re-enumeration on the same USB port.
	StableID   string
	Model      string
	HIDProduct string
	HIDSerial  string
}

// WheelDevice is LogiMate's logical view of one physical supported wheel.
// Device remains the low-level PnP/Raw-Input record; WheelDevice is what the UI
// and destructive workflows use to keep multiple attached wheels separate.
type WheelDevice struct {
	// ID is the stable LogiMate identity used for persisted selection and all
	// per-wheel settings. SessionID is the current Windows container identity.
	ID        string
	SessionID string
	Name      string
	Model     string
	Mode      string
	// D0 typed identities are authoritative for backend control flow. The
	// presentation strings above remain for UI/backward-compatible persistence.
	ModelKind           WheelModelKind
	ModeKind            OperatingModeKind
	Evidence            string
	InstanceID          string
	InterfaceIDs        []string
	AliasIDs            []string
	Service             string
	INF                 string
	Supported           bool
	ModelConfirmed      bool
	PnPVerified         bool
	HardwareFingerprint string // only set when Windows exposes a serial-like hardware identity
	PersistentIdentity  bool   // true only when HardwareFingerprint is strong enough for cross-session trust
}

type LegacyDriver struct {
	PublishedName string `json:"Driver"`
	OriginalName  string `json:"OriginalFileName"`
	ProviderName  string `json:"ProviderName"`
}

// HIDCandidate is a read-only snapshot of one supported Logitech HID interface.
// It is captured during the normal background device scan so the UI can show
// complete OpenG27-style HID diagnostics without probing hardware from WM_PAINT.
type HIDCandidate struct {
	Path                string
	Product             string
	Serial              string
	VendorID            uint16
	ProductID           uint16
	VersionNumber       uint16
	UsagePage           uint16
	Usage               uint16
	InputReportLength   uint16
	OutputReportLength  uint16
	FeatureReportLength uint16
	ProbeOK             bool
}

type State struct {
	Devices              []Device
	Wheels               []WheelDevice
	SelectedWheelID      string
	SelectionStatus      string
	DeviceDetectionError string
	LegacyDrivers        []LegacyDriver
	LegacyDriverError    string
	ProfilerSummary      string
	ProfilerError        string
	ActiveMode           string
	HVCI                 bool // effective/running state
	HVCIConfigured       bool
	HVCIRestartRequired  bool
	HVCIStatusError      string
	LCoreRunning         bool
	WheelModel           string
	WheelPreference      string
	OperatingPreference  string
	DetectionEvidence    string
	BackupCount          int
	DataDir              string
	Portable             bool
	OS                   string
	LastError            string
	RawInputDevices      []string
	HIDCandidates        []HIDCandidate
	RawInputError        string
	Pedals               PedalMapping
	AppVersion           string
	Theme                string
	DPI                  uint32
	MonitorCount         int
	HighContrast         bool
	ReducedMotion        bool
	WindowsBuild         uint32
	Renderer             string
	RendererDetail       string
	MaterialActive       bool

	// D0 canonical engine view. These typed values are derived from the selected
	// logical wheel and replace display-string control flow at new boundaries.
	SelectedModel    WheelModelKind
	SelectedMode     OperatingModeKind
	EngineGeneration uint64

	// Physical certification evidence is user-/hardware-derived and remains
	// separate from software build readiness. The value is cached by the
	// certification store so fast UI refreshes do not repeatedly hit disk.
	Certification CertificationProgress
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func discoveryErrorText(rawErr, hidErr error) string {
	var parts []string
	if rawErr != nil {
		parts = append(parts, "Raw Input: "+rawErr.Error())
	}
	if hidErr != nil {
		parts = append(parts, "SetupAPI HID: "+hidErr.Error())
	}
	return strings.Join(parts, "; ")
}

func RunPowerShellTimeout(script string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return strings.TrimSpace(out.String()), fmt.Errorf("PowerShell-Zeitlimit nach %s überschritten", timeout)
	}
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return strings.TrimSpace(out.String()), errors.New(msg)
	}
	return strings.TrimSpace(out.String()), nil
}

func RunHidden(name string, args ...string) (string, error) {
	return RunHiddenTimeout(60*time.Second, name, args...)
}

func RunHiddenTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return strings.TrimSpace(out.String()), fmt.Errorf("%s-Zeitlimit nach %s überschritten", filepath.Base(name), timeout)
	}
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return strings.TrimSpace(out.String()), errors.New(msg)
	}
	return strings.TrimSpace(out.String()), nil
}

func GetDataDir() (string, bool) {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	if _, err := os.Stat(filepath.Join(exeDir, "portable.flag")); err == nil {
		d := filepath.Join(exeDir, "LogiMateData")
		_ = os.MkdirAll(d, 0755)
		return d, true
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = exeDir
	}
	d := filepath.Join(base, "LogiMate")
	_ = os.MkdirAll(d, 0755)
	return d, false
}

type wheelDiscoverySnapshot struct {
	Devices    []Device
	Paths      []string
	Candidates []HIDCandidate
	RawErr     error
	HIDErr     error
	DevErr     error
}

// discoveryNeedsSettle detects the short C294 <-> native re-enumeration window.
// OpenG27 avoids making a final decision during that window by explicitly waiting
// for the native HID node. LogiMate supports several models, so it uses the same
// principle but correlates by stable USB identity instead of assuming C294=G27.
func discoveryNeedsSettle(devices []Device, paths []string) bool {
	if len(devices) == 0 && len(paths) > 0 {
		return true
	}
	byStable := map[string]map[string]bool{}
	for _, d := range devices {
		stable := strings.ToLower(strings.TrimSpace(d.StableID))
		if stable == "" {
			continue
		}
		pid := devicePID(d.InstanceID)
		if pid == "" {
			continue
		}
		if byStable[stable] == nil {
			byStable[stable] = map[string]bool{}
		}
		byStable[stable][pid] = true
	}
	for _, pids := range byStable {
		if pids[pidCompat] && (pids[pidG25] || pids[pidDFGT] || pids[pidG27]) {
			return true
		}
	}
	// A single C294 devnode plus a native HID interface normally means Windows is
	// halfway through the native-mode switch. Give SetupAPI a moment to converge.
	if len(byStable) <= 1 {
		hasCompatDevice := false
		hasNativePath := false
		for _, d := range devices {
			if devicePID(d.InstanceID) == pidCompat {
				hasCompatDevice = true
			}
		}
		for _, path := range paths {
			pid := devicePID(path)
			if pid == pidG25 || pid == pidDFGT || pid == pidG27 {
				hasNativePath = true
				break
			}
		}
		if hasCompatDevice && hasNativePath {
			return true
		}
	}
	return false
}

// collectWheelDiscoveryNative takes one coherent HID-first discovery snapshot.
// The important difference from the older state collector is that SetupAPI HID,
// devnodes and Raw Input are no longer launched as unrelated parallel scans and
// then combined even if Windows re-enumerated the wheel between them.
func snapshotHIDCandidates(paths []string) []HIDCandidate {
	if len(paths) == 0 {
		return nil
	}
	out := make([]HIDCandidate, 0, len(paths))
	for _, path := range rankHIDPathsOpenG27Style(paths, "") {
		m := hidInterfaceMetadataCached(path, 2*time.Second)
		out = append(out, HIDCandidate{
			Path: m.Path, Product: m.Product, Serial: m.Serial, VendorID: m.VendorID, ProductID: m.ProductID, VersionNumber: m.VersionNumber,
			UsagePage: m.UsagePage, Usage: m.Usage, InputReportLength: m.InputReportLength, OutputReportLength: m.OutputReportLength,
			FeatureReportLength: m.FeatureReportLength, ProbeOK: m.ProbeOK,
		})
	}
	return out
}

func collectWheelDiscoveryNative(settle bool) wheelDiscoverySnapshot {
	attempts := 1
	if settle {
		attempts = 5
	}
	var snap wheelDiscoverySnapshot
	for attempt := 0; attempt < attempts; attempt++ {
		hidDevices, hidPaths, hidErr := enumerateHIDLogitechWheelInterfacesNative()
		devnodes, devErr := detectDevicesNative()
		rawPaths, rawErr := EnumerateRawInputLogitechWheelsStrict()

		var devices []Device
		switch {
		case devErr == nil && hidErr == nil:
			devices = mergeDeviceEvidence(devnodes, hidDevices)
		case devErr == nil:
			devices = devnodes
		case hidErr == nil && len(hidDevices) > 0:
			devices = hidDevices
		}
		// HID is intentionally first. Its candidate order is capability-ranked like
		// OpenG27/HidSharp (largest output report first); Raw Input is supplemental.
		paths := mergeHIDPaths(hidPaths, rawPaths)
		snap = wheelDiscoverySnapshot{Devices: devices, Paths: paths, Candidates: snapshotHIDCandidates(hidPaths), RawErr: rawErr, HIDErr: hidErr, DevErr: devErr}
		if !settle || !discoveryNeedsSettle(devices, paths) {
			break
		}
		time.Sleep(160 * time.Millisecond)
		invalidateHIDMetadataCache()
	}
	return snap
}

func discoveryFatalError(s wheelDiscoverySnapshot) error {
	if s.DevErr == nil || (s.HIDErr == nil && len(s.Devices) > 0) {
		return nil
	}
	if s.HIDErr == nil && len(s.Paths) > 0 {
		// Direct HID visibility is sufficient for read-only discovery and rescue.
		// Destructive actions remain guarded by PnPVerified downstream.
		return nil
	}
	return fmt.Errorf("SetupAPI devnodes: %v; HID interfaces: %v", s.DevErr, s.HIDErr)
}

func detectDevicesFusedNative() ([]Device, error) {
	devnodes, devErr := detectDevicesNative()
	hidDevices, _, hidErr := enumerateHIDLogitechWheelInterfacesNative()
	switch {
	case devErr == nil && hidErr == nil:
		return mergeDeviceEvidence(devnodes, hidDevices), nil
	case devErr == nil:
		// All-class SetupAPI enumeration is authoritative even when the HID
		// interface pass is unavailable on an unusual Windows image.
		return devnodes, nil
	case hidErr == nil && len(hidDevices) > 0:
		// This is the OpenG27/HidSharp-like rescue path: a present Logitech HID
		// interface with a backing SetupAPI devnode is sufficient PnP evidence.
		return hidDevices, nil
	default:
		return nil, fmt.Errorf("SetupAPI devnodes: %v; HID interfaces: %v", devErr, hidErr)
	}
}

func DetectDevicesStrict() ([]Device, error) {
	if ds, err := detectDevicesFusedNative(); err == nil {
		return ds, nil
	} else {
		nativeErr := err
		// Compatibility fallback for unusual Windows images where both native
		// SetupAPI passes fail. A successful empty native result never comes here.
		ps := "$ds=Get-PnpDevice -PresentOnly -ErrorAction Stop | Where-Object {$_.InstanceId -match 'VID_046D&PID_(C294|C299|C29A|C29B)'}; foreach($d in $ds){$svc=(Get-PnpDeviceProperty -InstanceId $d.InstanceId -KeyName 'DEVPKEY_Device_Service' -ErrorAction SilentlyContinue).Data;$inf=(Get-PnpDeviceProperty -InstanceId $d.InstanceId -KeyName 'DEVPKEY_Device_DriverInfPath' -ErrorAction SilentlyContinue).Data;$parent=(Get-PnpDeviceProperty -InstanceId $d.InstanceId -KeyName 'DEVPKEY_Device_Parent' -ErrorAction SilentlyContinue).Data;$container=(Get-PnpDeviceProperty -InstanceId $d.InstanceId -KeyName 'DEVPKEY_Device_ContainerId' -ErrorAction SilentlyContinue).Data;$loc=(Get-PnpDeviceProperty -InstanceId $d.InstanceId -KeyName 'DEVPKEY_Device_LocationPaths' -ErrorAction SilentlyContinue).Data;$containerText='';if($container){$containerText=$container.ToString()};$locText='';if($loc){$locText=[string]$loc[0]};[Console]::WriteLine(($d.Status+'`t'+$d.FriendlyName+'`t'+$svc+'`t'+$inf+'`t'+$d.InstanceId+'`t'+$parent+'`t'+$containerText+'`t'+$locText))}"
		out, psErr := RunPowerShellTimeout(ps, 6*time.Second)
		if psErr != nil {
			return nil, fmt.Errorf("Geräteerkennung fehlgeschlagen (SetupAPI: %v; PowerShell-Fallback: %v)", nativeErr, psErr)
		}
		var ds []Device
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			line = strings.ReplaceAll(line, "`t", "\t")
			parts := strings.SplitN(line, "\t", 8)
			for len(parts) < 8 {
				parts = append(parts, "")
			}
			name, id := strings.TrimSpace(parts[1]), strings.TrimSpace(parts[4])
			parentID := strings.TrimSpace(parts[5])
			containerID := strings.TrimSpace(parts[6])
			locationPath := strings.TrimSpace(parts[7])
			stableID := DeriveStableWheelIDWithLocation(id, parentID, locationPath)
			physical := id
			if containerID != "" {
				physical = "container:" + strings.ToLower(containerID)
			} else if stableID != "" {
				physical = stableID
			} else if supportedWheelPIDText(parentID) {
				physical = parentID
			}
			ds = append(ds, Device{Status: strings.TrimSpace(parts[0]), Name: name, Service: strings.TrimSpace(parts[2]), INF: strings.TrimSpace(parts[3]), InstanceID: id, ParentID: parentID, PhysicalID: physical, ContainerID: containerID, LocationPath: locationPath, StableID: stableID, Model: IdentifyWheelModel(name, id)})
		}
		return ds, nil
	}
}

// DetectDevices is the compatibility wrapper used by non-critical callers.
// State collection uses DetectDevicesStrict so "scan failed" is never confused
// with "no wheel connected".
func DetectDevices() []Device {
	ds, _ := DetectDevicesStrict()
	return ds
}

func IdentifyWheelModel(name, instanceID string) string {
	n := strings.ToUpper(name)
	id := strings.ToUpper(instanceID)
	// Native PIDs are authoritative. C294 is the older Logitech compatibility
	// identity and can be presented with misleading "Driving Force" / "Driving
	// Force GT" names even when the physical wheel is a G27. Only literal G25
	// or G27 names may disambiguate C294 automatically; DFGT requires C29A or a
	// manual confirmation.
	switch {
	case strings.Contains(id, "PID_"+pidG25):
		return modelG25
	case strings.Contains(id, "PID_"+pidDFGT):
		return modelDFGT
	case strings.Contains(id, "PID_"+pidG27):
		return modelG27
	case strings.Contains(id, "PID_"+pidCompat):
		// C294 is shared by more Logitech wheels than the three currently
		// integrated in LogiMate. If Windows gives us an explicit name for a known
		// non-integrated model, fail closed instead of inviting a wrong G25/G27/
		// DFGT confirmation and potentially sending the wrong native-mode command.
		if strings.Contains(n, "G29") {
			return "Logitech G29 (nicht integriert)"
		}
		if strings.Contains(n, "DRIVING FORCE PRO") {
			return "Logitech Driving Force Pro (nicht integriert)"
		}
		has25, has27 := strings.Contains(n, "G25"), strings.Contains(n, "G27")
		if has25 && !has27 {
			return modelG25
		}
		if has27 && !has25 {
			return modelG27
		}
		return modelCompat
	case strings.Contains(n, "G25"):
		return modelG25
	case strings.Contains(n, "G27"):
		return modelG27
	case strings.Contains(n, "DRIVING FORCE GT"):
		return modelDFGT
	default:
		return "Logitech wheel"
	}
}
func SummarizeWheelModel(devs []Device) string {
	if len(devs) == 0 {
		return "Kein kompatibles Lenkrad"
	}
	seen25, seen27, seenDFGT, ambiguous := false, false, false, false
	for _, d := range devs {
		switch d.Model {
		case modelG25:
			seen25 = true
		case modelG27:
			seen27 = true
		case modelDFGT:
			seenDFGT = true
		case modelCompat:
			ambiguous = true
		}
	}
	var models []string
	if seenDFGT {
		models = append(models, "Driving Force GT")
	}
	if seen25 {
		models = append(models, "G25")
	}
	if seen27 {
		models = append(models, "G27")
	}
	if len(models) > 1 {
		return strings.Join(models, " + ")
	}
	if seenDFGT {
		return modelDFGT
	}
	if seen27 {
		return modelG27
	}
	if seen25 {
		return modelG25
	}
	if ambiguous {
		return "Logitech C294 (Kompatibilitätsmodus)"
	}
	return devs[0].Model
}
func devicePID(instanceID string) string {
	u := strings.ToUpper(instanceID)
	for _, pid := range []string{pidG25, pidDFGT, pidG27, pidCompat} {
		if strings.Contains(u, "PID_"+pid) {
			return pid
		}
	}
	return ""
}

func devicesFromRawInput(paths []string) []Device {
	seen := map[string]bool{}
	var out []Device
	for _, path := range paths {
		pid := devicePID(path)
		if pid == "" {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(path))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		name := "Logitech wheel (Raw Input)"
		switch pid {
		case pidG25:
			name = "Logitech G25 (Raw Input)"
		case pidDFGT:
			name = "Logitech Driving Force GT (Raw Input)"
		case pidG27:
			name = "Logitech G27 (Raw Input)"
		case pidCompat:
			name = "Logitech compatibility device (Raw Input)"
		}
		instanceID := rawPathInstanceID(path)
		if instanceID == "" {
			instanceID = path
		}
		model := IdentifyWheelModel(name, instanceID)
		if pid == pidCompat {
			model = modelCompat
		}
		// Raw Input has no ContainerID here. Keep every unique HID interface as a
		// separate conservative fallback target; over-counting blocks destructive
		// actions, whereas collapsing two same-PID wheels could authorize one.
		out = append(out, Device{Status: "Present", Name: name, Service: "Raw Input fallback", InstanceID: instanceID, PhysicalID: "raw:" + key, Model: model})
	}
	return out
}

func explicitWinMMModel(devices []JoyState) string {
	bestModel := ""
	bestScore := 0
	tied := false
	for _, d := range devices {
		n := strings.ToLower(d.Name)
		model, score := "", 0
		if strings.Contains(n, "g27") {
			model, score = "Logitech G27", 120
		} else if strings.Contains(n, "g25") {
			model, score = "Logitech G25", 120
		} else if strings.Contains(n, "driving force gt") {
			// Keep this as read-only evidence. C294 has historically exposed
			// misleading "Driving Force" names, so DFGT is never auto-authorized
			// from WinMM alone; native C29A or explicit confirmation still wins.
			model, score = "Logitech Driving Force GT", 110
		}
		if score == 0 {
			continue
		}
		if score > bestScore {
			bestModel, bestScore, tied = model, score, false
		} else if score == bestScore && model != bestModel {
			tied = true
		}
	}
	if tied {
		return ""
	}
	return bestModel
}

func nativePIDModel(hasDFGT, has25, has27 bool) string {
	var models []string
	if hasDFGT {
		models = append(models, modelDFGT)
	}
	if has25 {
		models = append(models, modelG25)
	}
	if has27 {
		models = append(models, modelG27)
	}
	if len(models) == 1 {
		return models[0]
	}
	if len(models) > 1 {
		return strings.Join(models, " + ")
	}
	return ""
}

func nativePIDModelFromDevices(devs []Device) string {
	hasDFGT, has25, has27 := false, false, false
	for _, d := range devs {
		switch devicePID(d.InstanceID) {
		case pidDFGT:
			hasDFGT = true
		case pidG25:
			has25 = true
		case pidG27:
			has27 = true
		}
	}
	return nativePIDModel(hasDFGT, has25, has27)
}

func nativePIDModelFromRawInput(paths []string) string {
	hasDFGT, has25, has27 := false, false, false
	for _, path := range paths {
		switch devicePID(path) {
		case pidDFGT:
			hasDFGT = true
		case pidG25:
			has25 = true
		case pidG27:
			has27 = true
		}
	}
	return nativePIDModel(hasDFGT, has25, has27)
}
func resolveWheelModel(devs []Device, rawInput []string, preference string) (resolved string, effectiveDevices []Device, evidence string) {
	// Identity evidence is deliberately ranked. SetupAPI native PIDs are the
	// strongest signal. Raw Input native PIDs may disambiguate a PnP C294
	// interface. WinMM is used only after those Windows device-tree signals and
	// only when its product name literally says G25/G27. A saved manual choice (G25/G27/DFGT) is
	// the last C294 fallback and can never override a native PID.
	effectiveDevices = devs
	if native := nativePIDModelFromDevices(devs); native != "" {
		return native, effectiveDevices, "SetupAPI/PnP native PID"
	}

	if len(effectiveDevices) == 0 && len(rawInput) > 0 {
		effectiveDevices = devicesFromRawInput(rawInput)
		if native := nativePIDModelFromRawInput(rawInput); native != "" {
			return native, effectiveDevices, "Raw Input native PID (PnP fallback)"
		}
		if len(effectiveDevices) > 0 {
			evidence = "Raw Input C294 fallback"
		}
	}

	resolved = SummarizeWheelModel(effectiveDevices)
	if len(devs) > 0 && (resolved == modelG25 || resolved == modelG27 || resolved == modelDFGT || strings.Contains(resolved, " + ")) {
		// With no native PID this resolution came from an explicit Windows device
		// name on C294. Cross-check G25/G27 against WinMM. This gives LogiMate the
		// OpenG27-like convenience of recognizing a power-on C294 G27 without
		// persisting an unsafe USB-port-only model authorization.
		if resolved == modelG25 || resolved == modelG27 {
			if joys, err := enumerateWinMMJoysticksCached(30 * time.Second); err == nil && explicitWinMMModel(joys) == resolved {
				return resolved, effectiveDevices, "SetupAPI/PnP C294 + WinMM Modellkonsens"
			}
		}
		return resolved, effectiveDevices, "SetupAPI/PnP C294 + expliziter Gerätename"
	}

	if strings.Contains(resolved, "Kompatibilitätsmodus") {
		if native := nativePIDModelFromRawInput(rawInput); native != "" {
			return native, effectiveDevices, "PnP C294 + Raw Input native PID"
		}
		// C294 is a shared Logitech compatibility identity. A literal G25/G27 WinMM product name is
		// strong enough to disambiguate the active session; generic WinMM names
		// are never used for model identity.
		if joys, err := enumerateWinMMJoysticksCached(30 * time.Second); err == nil {
			if model := explicitWinMMModel(joys); model != "" {
				return model + " (WinMM bestätigt / C294)", effectiveDevices, "PnP/Raw C294 + expliziter WinMM-Name"
			}
		}
		if preference != "" {
			return preference + " (manuell bestätigt / C294)", effectiveDevices, "C294 + gespeicherte Bestätigung"
		}
		if evidence == "" {
			evidence = "C294 erkannt; Modell noch uneindeutig"
		}
		return resolved, effectiveDevices, evidence
	}

	if len(effectiveDevices) == 0 {
		return "Kein kompatibles Lenkrad", effectiveDevices, "Keine passende Logitech VID/PID in SetupAPI oder Raw Input"
	}
	if evidence == "" {
		evidence = "Logitech-Gerät erkannt; Modell nicht eindeutig"
	}
	return resolved, effectiveDevices, evidence
}

var legacyDriverCache = struct {
	sync.Mutex
	at      time.Time
	drivers []LegacyDriver
	err     error
}{}

func listLegacyDriversFresh() ([]LegacyDriver, error) {
	ps := `$x=Get-WindowsDriver -Online -ErrorAction Stop | Where-Object {$_.ProviderName -match 'Logitech' -and ([IO.Path]::GetFileName($_.OriginalFileName) -match '^(wm.*|lgjoyhid)\.inf$')} | Select-Object Driver,@{N='OriginalFileName';E={[IO.Path]::GetFileName($_.OriginalFileName)}},ProviderName; $x | ConvertTo-Json -Compress`
	out, err := RunPowerShellTimeout(ps, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("Logitech-Legacy-Treiber konnten nicht zuverlässig gelesen werden: %w", err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	var many []LegacyDriver
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &many); err != nil {
			return nil, fmt.Errorf("Treiberinventar konnte nicht ausgewertet werden: %w", err)
		}
		return many, nil
	}
	var one LegacyDriver
	if err := json.Unmarshal([]byte(trimmed), &one); err != nil {
		return nil, fmt.Errorf("Treiberinventar konnte nicht ausgewertet werden: %w", err)
	}
	if one.PublishedName == "" {
		return nil, errors.New("Treiberinventar enthielt einen unvollständigen Logitech-Eintrag")
	}
	return []LegacyDriver{one}, nil
}

// ListLegacyDriversStrict is used before every destructive driver operation.
// An inventory failure is never treated as "no drivers installed".
func ListLegacyDriversStrict() ([]LegacyDriver, error) {
	drivers, err := listLegacyDriversFresh()
	legacyDriverCache.Lock()
	legacyDriverCache.at = time.Now()
	legacyDriverCache.drivers = append([]LegacyDriver(nil), drivers...)
	legacyDriverCache.err = err
	legacyDriverCache.Unlock()
	return drivers, err
}

func listLegacyDriversCached(maxAge time.Duration) ([]LegacyDriver, error) {
	legacyDriverCache.Lock()
	if !legacyDriverCache.at.IsZero() && time.Since(legacyDriverCache.at) < maxAge {
		d := append([]LegacyDriver(nil), legacyDriverCache.drivers...)
		err := legacyDriverCache.err
		legacyDriverCache.Unlock()
		return d, err
	}
	legacyDriverCache.Unlock()
	return ListLegacyDriversStrict()
}

func InvalidateSystemCaches() {
	InvalidateInputCaches()
	legacyDriverCache.Lock()
	legacyDriverCache.at = time.Time{}
	legacyDriverCache.Unlock()
	invalidateProfilerCache()
}

// Compatibility helper for read-only UI call sites. Destructive operations
// must use ListLegacyDriversStrict and handle the error.
func ListLegacyDrivers() []LegacyDriver {
	d, _ := listLegacyDriversCached(30 * time.Second)
	return d
}

func IsProcessRunning(name string) bool { return IsProcessRunningNative(name) }

type HVCIStatus struct {
	Configured      bool
	Effective       bool
	RestartRequired bool
	Error           string
}

var hvciStatusCache = struct {
	sync.Mutex
	at     time.Time
	status HVCIStatus
}{}

func ReadHVCIStatus() HVCIStatus {
	hvciStatusCache.Lock()
	if !hvciStatusCache.at.IsZero() && time.Since(hvciStatusCache.at) < 30*time.Second {
		st := hvciStatusCache.status
		hvciStatusCache.Unlock()
		return st
	}
	hvciStatusCache.Unlock()

	st := HVCIStatus{Configured: HVCIEnabledNative()}
	ps := `$ErrorActionPreference='Stop'; $d=Get-CimInstance -Namespace 'root\Microsoft\Windows\DeviceGuard' -ClassName 'Win32_DeviceGuard'; if($null -eq $d){ throw 'Win32_DeviceGuard nicht verfügbar' }; if($d.SecurityServicesRunning -contains 2){ '1' } else { '0' }`
	out, err := RunPowerShellTimeout(ps, 6*time.Second)
	if err != nil {
		// Do not lie about effective state. Preserve the configured registry bit
		// separately and surface the probe failure to UI/diagnostics.
		st.Error = err.Error()
	} else {
		st.Effective = strings.TrimSpace(out) == "1"
		st.RestartRequired = st.Configured != st.Effective
	}
	hvciStatusCache.Lock()
	hvciStatusCache.at = time.Now()
	hvciStatusCache.status = st
	hvciStatusCache.Unlock()
	return st
}

func InvalidateHVCIStatusCache() {
	hvciStatusCache.Lock()
	hvciStatusCache.at = time.Time{}
	hvciStatusCache.Unlock()
}

func HVCIEnabled() bool { return ReadHVCIStatus().Effective }

func ReadWheelPreference(dataDir string) string {
	b, err := os.ReadFile(filepath.Join(dataDir, "wheel.model"))
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(b))
	if isManualWheelModel(v) {
		return v
	}
	return ""
}

func SaveWheelPreference(dataDir, model string) error {
	if !isManualWheelModel(model) {
		return errors.New("ungueltiges Wheel-Modell")
	}
	return AtomicWriteFile(filepath.Join(dataDir, "wheel.model"), []byte(model), 0644)
}

func ClearWheelPreference(dataDir string) error {
	err := os.Remove(filepath.Join(dataDir, "wheel.model"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func wheelDevicePreferenceFile(dataDir string) string {
	return filepath.Join(dataDir, "wheel.models.json")
}

// ReadWheelDevicePreferences returns per-physical-device model fallbacks. Keys
// are normalized stable wheel IDs (with backward-compatible legacy aliases) so
// two C294 wheels never inherit each other's manual identity.
func readWheelDevicePreferencesStrict(dataDir string) (map[string]string, error) {
	out := map[string]string{}
	raw := map[string]string{}
	_, err := ReadJSONConfigStrict(wheelDevicePreferenceFile(dataDir), &raw)
	if err != nil {
		return out, err
	}
	for id, model := range raw {
		id = strings.ToLower(strings.TrimSpace(id))
		if id != "" && isManualWheelModel(model) {
			out[id] = model
		}
	}
	return out, nil
}

func ReadWheelDevicePreferences(dataDir string) map[string]string {
	out, _ := readWheelDevicePreferencesStrict(dataDir)
	return out
}

func SaveWheelDevicePreference(dataDir, id, model string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return errors.New("ungueltige Lenkrad-ID")
	}
	if !isManualWheelModel(model) {
		return errors.New("ungueltiges Wheel-Modell")
	}
	all, err := readWheelDevicePreferencesStrict(dataDir)
	if err != nil {
		return err
	}
	all[id] = model
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(wheelDevicePreferenceFile(dataDir), b, 0644)
}

func ClearWheelDevicePreferences(dataDir string) error {
	err := os.Remove(wheelDevicePreferenceFile(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func ClearWheelDevicePreference(dataDir, id string) error {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return nil
	}
	all, err := readWheelDevicePreferencesStrict(dataDir)
	if err != nil {
		return err
	}
	if _, ok := all[id]; !ok {
		return nil
	}
	delete(all, id)
	if len(all) == 0 {
		return ClearWheelDevicePreferences(dataDir)
	}
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(wheelDevicePreferenceFile(dataDir), b, 0644)
}

func ClearWheelDetectionState(dataDir string) error {
	var errs []string
	if err := ClearSelectedWheelID(dataDir); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ClearWheelPreference(dataDir); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ClearWheelDevicePreferences(dataDir); err != nil {
		errs = append(errs, err.Error())
	}
	if err := ClearWheelNativeIdentityHistory(dataDir); err != nil {
		errs = append(errs, err.Error())
	}
	InvalidateInputCaches()
	ClosePreferredInput()
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func ReadSelectedWheelID(dataDir string) string {
	b, err := os.ReadFile(filepath.Join(dataDir, "wheel.selected"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func SaveSelectedWheelID(dataDir, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("ungueltige Lenkrad-ID")
	}
	return AtomicWriteFile(filepath.Join(dataDir, "wheel.selected"), []byte(id), 0644)
}

func ClearSelectedWheelID(dataDir string) error {
	err := os.Remove(filepath.Join(dataDir, "wheel.selected"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func ReadOperatingPreference(dataDir string) string {
	b, err := os.ReadFile(filepath.Join(dataDir, "wheel.mode"))
	if err != nil {
		return ""
	}
	v := strings.ToLower(strings.TrimSpace(string(b)))
	if v == "modern" || v == "legacy" {
		return v
	}
	return ""
}

func SaveOperatingPreference(dataDir, mode string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "modern" && mode != "legacy" {
		return errors.New("ungültiger Betriebsmodus")
	}
	return AtomicWriteFile(filepath.Join(dataDir, "wheel.mode"), []byte(mode), 0644)
}

func ClearOperatingPreference(dataDir string) error {
	err := os.Remove(filepath.Join(dataDir, "wheel.mode"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func backupRoot(dataDir string) string { return filepath.Join(dataDir, "DriverBackups") }

func CountBackupINFs(dataDir string) int {
	n := 0
	_ = filepath.Walk(backupRoot(dataDir), func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && strings.EqualFold(filepath.Ext(info.Name()), ".inf") {
			n++
		}
		return nil
	})
	return n
}

func OSString() string { return NativeOSString() }

func isStablePersistedWheelID(id string) bool {
	v := strings.ToLower(strings.TrimSpace(id))
	return strings.HasPrefix(v, "usbloc:") || strings.HasPrefix(v, "usbslot:")
}

func aggregateLogicalWheelModel(wheels []WheelDevice, fallback string) string {
	if len(wheels) == 0 {
		if strings.TrimSpace(fallback) != "" {
			return fallback
		}
		return "Kein kompatibles Lenkrad"
	}
	seen := map[string]bool{}
	var names []string
	for _, w := range wheels {
		name := w.Model
		switch {
		case IsG27Model(name):
			name = modelG27
		case IsG25Model(name):
			name = modelG25
		case IsDFGTModel(name):
			name = modelDFGT
		case IsCompatibilityModel(name):
			name = modelCompat
		}
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	if len(names) == 1 {
		return names[0]
	}
	if len(names) > 1 {
		return strings.Join(names, " + ")
	}
	return fallback
}

func aggregateLogicalWheelMode(wheels []WheelDevice, fallback string) string {
	if len(wheels) == 0 {
		if fallback != "" {
			return fallback
		}
		return "KEIN LOGITECH WHEEL"
	}
	first := wheels[0].Mode
	for _, w := range wheels[1:] {
		if w.Mode != first {
			return "Gemischt: Legacy + Modern"
		}
	}
	if first != "" {
		return first
	}
	return fallback
}

func migratePerWheelPreferencesToStableIDs(dataDir string, wheels []WheelDevice, prefs map[string]string) (map[string]string, error) {
	if len(wheels) == 0 || len(prefs) == 0 {
		return prefs, nil
	}
	// Work on a clone. If persistence fails, the current session must not silently
	// trust a migration that was never durably committed.
	migrated := make(map[string]string, len(prefs))
	for k, v := range prefs {
		migrated[k] = v
	}
	changed := false
	for _, w := range wheels {
		stableKey := strings.ToLower(strings.TrimSpace(w.ID))
		if stableKey == "" || isManualWheelModel(migrated[stableKey]) {
			continue
		}
		for _, alias := range w.AliasIDs {
			aliasKey := strings.ToLower(strings.TrimSpace(alias))
			if model := migrated[aliasKey]; isManualWheelModel(model) {
				migrated[stableKey] = model
				changed = true
				break
			}
		}
	}
	if !changed {
		return prefs, nil
	}
	b, err := json.MarshalIndent(migrated, "", "  ")
	if err != nil {
		return prefs, fmt.Errorf("Wheel-Präferenzmigration konnte nicht serialisiert werden: %w", err)
	}
	if err := AtomicWriteFile(wheelDevicePreferenceFile(dataDir), b, 0644); err != nil {
		return prefs, fmt.Errorf("Wheel-Präferenzmigration konnte nicht gespeichert werden: %w", err)
	}
	return migrated, nil
}

func retireLegacyGlobalWheelPreference(dataDir string) error {
	// wheel.model was a machine-global C294 fallback in early previews. It is
	// intentionally not migrated to a physical wheel: after unplug/replug it can
	// belong to a different C294 device and would authorize model-specific HID
	// commands for the wrong hardware. The caller may keep the value in memory
	// briefly as a schema-repair hint for an already-native wheel, then retire it.
	return ClearWheelPreference(dataDir)
}

func migratePerWheelState(dataDir, oldID, newID string) string {
	oldID, newID = strings.TrimSpace(oldID), strings.TrimSpace(newID)
	if oldID == "" || newID == "" || strings.EqualFold(oldID, newID) {
		return ""
	}
	var warnings []string
	prefs := ReadWheelDevicePreferences(dataDir)
	oldKey, newKey := strings.ToLower(oldID), strings.ToLower(newID)
	if model := prefs[oldKey]; isManualWheelModel(model) && !isManualWheelModel(prefs[newKey]) {
		if err := SaveWheelDevicePreference(dataDir, newID, model); err != nil {
			warnings = append(warnings, "Modellbestätigung konnte nicht migriert werden: "+err.Error())
		} else if err := ClearWheelDevicePreference(dataDir, oldID); err != nil {
			warnings = append(warnings, "alte Modellbestätigung konnte nicht bereinigt werden: "+err.Error())
		}
	}
	if err := MigratePedalMappingsWheelID(dataDir, oldID, newID); err != nil {
		warnings = append(warnings, "Pedal-Mapping konnte nicht migriert werden: "+err.Error())
	}
	return strings.Join(warnings, "; ")
}

func applySessionHIDProductConsensus(wheels []WheelDevice, devices []Device) []WheelDevice {
	if len(wheels) != 1 {
		return wheels
	}
	w := &wheels[0]
	if !w.Supported || !w.PnPVerified || w.ModelConfirmed || devicePID(w.InstanceID) != pidCompat {
		return wheels
	}
	model := ""
	for _, d := range devices {
		if !strings.EqualFold(deviceSessionGroupID(d), w.SessionID) && !strings.EqualFold(physicalWheelID(d), w.ID) {
			continue
		}
		product := strings.ToUpper(strings.TrimSpace(d.HIDProduct))
		switch {
		case strings.Contains(product, "G27"):
			if model != "" && model != modelG27 {
				return wheels
			}
			model = modelG27
		case strings.Contains(product, "G25"):
			if model != "" && model != modelG25 {
				return wheels
			}
			model = modelG25
		}
	}
	if model == "" {
		return wheels
	}
	w.Model = model + " (HID-Produkt bestätigt / C294)"
	w.ModelKind = ClassifyWheelModel(model)
	w.ModelConfirmed = true
	w.Evidence = "C294 + direkter HID-Produktname; nur aktuelle Sitzung"
	return wheels
}

func applySessionModelConsensus(wheels []WheelDevice, aggregateModel, aggregateEvidence string) []WheelDevice {
	if len(wheels) != 1 || !strings.Contains(strings.ToLower(aggregateEvidence), "winmm") {
		return wheels
	}
	w := &wheels[0]
	if !w.Supported || !w.PnPVerified || w.ModelConfirmed || devicePID(w.InstanceID) != pidCompat {
		return wheels
	}
	kind := ClassifyWheelModel(aggregateModel)
	// G25/G27 literal WinMM names are sufficiently specific for a session-only
	// C294 identity. DFGT intentionally remains manual/native-PID-only because
	// classic Logitech compatibility mode can expose misleading Driving Force
	// names on other physical wheels.
	var canonical string
	switch kind {
	case WheelModelG25:
		canonical = modelG25
	case WheelModelG27:
		canonical = modelG27
	default:
		return wheels
	}
	w.Model = canonical + " (Windows-Sitzung bestätigt / C294)"
	w.ModelKind = kind
	w.ModelConfirmed = true
	w.Evidence = "C294 + read-only Windows-Modellkonsens (PnP/WinMM); nur aktuelle Sitzung"
	return wheels
}

func reconcileWheelSelection(dataDir string, devices []Device, pref, aggregateModel, aggregateEvidence, aggregateMode string) ([]WheelDevice, string, string, string, string, string) {
	legacyStoredModel := strings.TrimSpace(pref)
	legacyPerDevicePrefs := ReadWheelDevicePreferences(dataDir)
	// First build without any manual model authorization. Manual C294 model
	// confirmation is trusted only after a second pass proves both the stable
	// wheel slot and the current Windows session/container identity still match.
	wheels := BuildWheelDevicesWithPreferences(devices, "", nil)
	// Learn only from an authoritative native PID (C299/C29A/C29B). This gives
	// LogiMate the same practical reconnect memory OpenG27 gets from being G27-only,
	// without blindly assuming that every future C294 on the same USB slot is the
	// same physical model. Current-session contradictory WinMM evidence still wins.
	rememberAuthoritativeNativeIdentities(dataDir, wheels)
	var selectionWarnings []string
	var prefMigrationErr error
	legacyPerDevicePrefs, prefMigrationErr = migratePerWheelPreferencesToStableIDs(dataDir, wheels, legacyPerDevicePrefs)
	if prefMigrationErr != nil {
		selectionWarnings = append(selectionWarnings, prefMigrationErr.Error())
	}
	trustedPrefs := trustedWheelDevicePreferences(dataDir, wheels)
	wheels = BuildWheelDevicesWithPreferences(devices, "", trustedPrefs)
	wheels = applySessionHIDProductConsensus(wheels, devices)
	wheels = applyLearnedNativeIdentity(dataDir, wheels, aggregateModel, aggregateEvidence)
	wheels = applySessionModelConsensus(wheels, aggregateModel, aggregateEvidence)
	if isManualWheelModel(pref) {
		if err := retireLegacyGlobalWheelPreference(dataDir); err != nil {
			selectionWarnings = append(selectionWarnings, "alte globale Wheel-Präferenz konnte nicht entfernt werden: "+err.Error())
		} else {
			pref = ""
		}
	}
	withSelectionWarnings := func(status string) string {
		if len(selectionWarnings) == 0 {
			return status
		}
		return status + " · " + strings.Join(selectionWarnings, " · ")
	}
	stored := ReadSelectedWheelID(dataDir)

	modelSummary := aggregateLogicalWheelModel(wheels, aggregateModel)
	modeSummary := aggregateLogicalWheelMode(wheels, aggregateMode)
	if len(wheels) == 0 {
		status := "Kein unterstütztes Lenkrad erkannt"
		if stored != "" {
			status = "Gespeichertes Ziel ist derzeit nicht verbunden"
		}
		return wheels, "", modelSummary, aggregateEvidence, modeSummary, withSelectionWarnings(status)
	}

	// A persisted stable target is authoritative. Legacy IDs (old container or
	// interface IDs) may be migrated, but once a stable usbloc:*/usbslot:* ID exists LogiMate will
	// never silently jump to another wheel.
	if stored != "" {
		for _, w := range wheels {
			if idMatchesWheel(stored, w) {
				if !strings.EqualFold(stored, w.ID) {
					status := "Gespeichertes Ziel auf stabile Geräte-ID migriert"
					if warning := migratePerWheelState(dataDir, stored, w.ID); warning != "" {
						status += " · " + warning
					}
					if err := SaveSelectedWheelID(dataDir, w.ID); err != nil {
						status += " · Speichern fehlgeschlagen: " + err.Error()
					}
					return wheels, w.ID, w.Model, w.Evidence, w.Mode, withSelectionWarnings(status)
				}
				return wheels, w.ID, w.Model, w.Evidence, w.Mode, withSelectionWarnings("Gespeichertes Ziel verbunden")
			}
		}

		// Older releases sometimes persisted the raw USB devnode. Unlike a HID
		// collection, that ID contains the same USB-slot suffix across Logitech's
		// C294 <-> native PID re-enumeration, so this migration is exact and safe.
		if len(wheels) == 1 {
			w := wheels[0]
			if oldStable := DeriveStableWheelID(stored, ""); oldStable != "" && strings.EqualFold(oldStable, w.ID) {
				status := "Alte USB-Geräte-ID auf stabile USB-ID migriert"
				if warning := migratePerWheelState(dataDir, stored, w.ID); warning != "" {
					status += " · " + warning
				}
				if err := SaveSelectedWheelID(dataDir, w.ID); err != nil {
					status += " · Speichern fehlgeschlagen: " + err.Error()
				}
				return wheels, w.ID, w.Model, w.Evidence + " · alte USB-Auswahl auf stabile ID migriert", w.Mode, withSelectionWarnings(status)
			}
		}

		// 0.1.2 could persist ContainerID. Windows may replace that container when
		// the same multimode wheel changes PID. Container GUIDs do not carry the
		// USB-slot token, so there is no mathematical correlation after the switch.
		// Permit one narrowly-scoped schema repair only for an old container:* ID,
		// exactly one current wheel, and authoritative native-PID evidence. Raw HID
		// IDs are deliberately NOT guessed: a stale HID target must never jump to a
		// different physical wheel. Once a stable ID has been stored, all future
		// missing-target cases remain fail-closed.
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(stored)), "container:") && len(wheels) == 1 && isStablePersistedWheelID(wheels[0].ID) {
			w := wheels[0]
			// A container GUID cannot be correlated after Windows has replaced it.
			// Auto-repair therefore requires a model hint written by the older
			// release. Without that evidence we fail closed and ask the user to pick
			// the sole wheel once instead of possibly attaching stale state to a
			// different physical wheel.
			legacyModel := legacyPerDevicePrefs[strings.ToLower(strings.TrimSpace(stored))]
			if !isManualWheelModel(legacyModel) {
				legacyModel = legacyStoredModel
			}
			modelConsistent := isManualWheelModel(legacyModel) && ((IsG27Model(legacyModel) && IsG27Model(w.Model)) ||
				(IsG25Model(legacyModel) && IsG25Model(w.Model)) ||
				(IsDFGTModel(legacyModel) && IsDFGTModel(w.Model)))
			nativeEvidence := strings.Contains(w.Evidence, "native PID")
			if modelConsistent && nativeEvidence {
				status := "Alte Container-ID nach Moduswechsel auf stabile USB-ID migriert"
				if warning := migratePerWheelState(dataDir, stored, w.ID); warning != "" {
					status += " · " + warning
				}
				if err := SaveSelectedWheelID(dataDir, w.ID); err != nil {
					status += " · Speichern fehlgeschlagen: " + err.Error()
				}
				return wheels, w.ID, w.Model, w.Evidence + " · alte Container-Auswahl automatisch repariert", w.Mode, withSelectionWarnings(status)
			}
		}

		return wheels, "", modelSummary, aggregateEvidence, modeSummary, withSelectionWarnings("Gespeichertes Ziel fehlt – bitte Gerät auswählen")
	}

	if len(wheels) == 1 {
		w := wheels[0]
		// direct:* is a transient emergency identity used only while SetupAPI is
		// momentarily empty. Never persist it: once Windows exposes the real USB
		// devnode again, the stable location/USB identity must become authoritative.
		status := "Einziges erkanntes Ziel automatisch ausgewählt"
		if CanPersistWheelSelection(w) {
			if err := SaveSelectedWheelID(dataDir, w.ID); err != nil {
				status += " · Auswahl konnte nicht gespeichert werden: " + err.Error()
			}
		} else {
			status += " · transienter Fallback wird nicht dauerhaft gespeichert"
		}
		return wheels, w.ID, w.Model, w.Evidence, w.Mode, withSelectionWarnings(status)
	}

	// Multiple wheels with no valid persisted target is deliberately not guessed.
	return wheels, "", modelSummary,
		fmt.Sprintf("%d unterstützte Lenkräder erkannt; kein Ziel ausgewählt", len(wheels)), modeSummary, withSelectionWarnings("Mehrere Ziele – Auswahl erforderlich")
}

func effectiveWheelPreference(wheels []WheelDevice, selectedWheelID string) string {
	for _, w := range wheels {
		if strings.EqualFold(w.ID, selectedWheelID) && w.ModelConfirmed && !w.PnPVerified {
			switch {
			case IsG25Model(w.Model):
				return modelG25
			case IsG27Model(w.Model):
				return modelG27
			case IsDFGTModel(w.Model):
				return modelDFGT
			}
		}
	}
	return ""
}

func CollectState() State {
	dataDir, portable := GetDataDir()

	// Independent Windows probes run concurrently. On systems where DISM/
	// PowerShell is slow this keeps initial status collection bounded by the
	// slowest probe instead of the sum of every probe.
	var (
		discovery   wheelDiscoverySnapshot
		ds          []Device
		detectErr   error
		legacy      []LegacyDriver
		legacyErr   error
		profiler    ProfilerInfo
		profilerErr error
		hvciStatus  HVCIStatus
		lcore       bool
		osName      string
	)
	var wg sync.WaitGroup
	wg.Add(6)
	go func() { defer wg.Done(); discovery = collectWheelDiscoveryNative(true) }()
	go func() { defer wg.Done(); legacy, legacyErr = listLegacyDriversCached(30 * time.Second) }()
	go func() { defer wg.Done(); profiler, profilerErr = detectProfilerCached(30 * time.Second) }()
	go func() { defer wg.Done(); hvciStatus = ReadHVCIStatus() }()
	go func() { defer wg.Done(); lcore = IsProcessRunning("LCore.exe") }()
	go func() { defer wg.Done(); osName = OSString() }()

	backupCount := CountBackupINFs(dataDir)
	wg.Wait()
	ds = discovery.Devices
	detectErr = discoveryFatalError(discovery)
	// Full refresh may use the slower PowerShell compatibility fallback only if
	// both native Windows discovery layers really failed. Normal detection never
	// pays for a second unrelated scan.
	if detectErr != nil {
		if fallback, err := DetectDevicesStrict(); err == nil {
			ds, detectErr = fallback, nil
		}
	}
	liveHIDPaths := append([]string(nil), discovery.Paths...)
	rawInputErr := discovery.RawErr
	hidInterfaceErr := discovery.HIDErr

	mode := "KEIN LOGITECH WHEEL"
	if len(ds) > 0 {
		mode = "Generic HID / Modern"
		for _, d := range ds {
			x := strings.ToLower(d.Service + " " + d.INF)
			if strings.Contains(x, "wm") || strings.Contains(x, "lgjoyhid") {
				mode = "Logitech Legacy"
				break
			}
		}
	}

	detectionErrText := ""
	if detectErr != nil {
		detectionErrText = detectErr.Error()
	}
	pref := ReadWheelPreference(dataDir)
	operatingPref := ReadOperatingPreference(dataDir)
	model, effectiveDevices, evidence := resolveWheelModel(ds, liveHIDPaths, "")
	ds = effectiveDevices

	// A live Direct-HID handle is stronger evidence than a transient empty
	// SetupAPI/Raw-Input refresh. This keeps an already confirmed G27 visible
	// while it is physically connected, even when Windows returns a short-lived
	// empty enumeration during an idle/rescan window.
	if (model == "" || model == "Kein kompatibles Lenkrad") && G27DirectHIDConnected() {
		model = modelG27
		evidence = "LogiMate Direct HID verbunden"
		if len(ds) == 0 {
			stable := ReadSelectedWheelID(dataDir)
			if stable == "" {
				stable = "direct:g27"
			}
			ds = []Device{{Status: "Synthetic", Name: "Logitech G27 (Direct HID continuity)", Service: "HID", InstanceID: `HID\VID_046D&PID_C29B\DIRECT`, PhysicalID: "direct:g27", StableID: stable, Model: modelG27}}
		}
		mode = "Generic HID / Modern"
	}

	// Mode inference happens after device reconciliation so a Raw-Input fallback
	// still yields a coherent "modern" state instead of "no wheel". Legacy is
	// only asserted when the PnP service/driver evidence says so.
	if len(ds) > 0 && mode == "KEIN LOGITECH WHEEL" {
		mode = "Generic HID / Modern"
	}
	wheels, selectedWheelID, model, evidence, mode, selectionStatus := reconcileWheelSelection(dataDir, ds, pref, model, evidence, mode)
	displayPreference := effectiveWheelPreference(wheels, selectedWheelID)
	if detectErr != nil {
		selectedWheelID = ""
		model = "Geräteerkennung fehlgeschlagen"
		evidence = detectErr.Error()
		mode = "UNBEKANNT"
		selectionStatus = "Erkennung fehlgeschlagen – keine Geräteaktion zulässig"
	}
	pedals := ReadPedalMappingForWheel(dataDir, selectedWheelID, model, mode)
	legacyErrText := ""
	if legacyErr != nil {
		legacyErrText = legacyErr.Error()
	}
	profilerErrText := ""
	if profilerErr != nil {
		profilerErrText = profilerErr.Error()
	}
	lastErr := ""
	if autoErr := wheelAutoSwitchError(); autoErr != "" {
		lastErr = "Automatischer Native-Mode-Switch fehlgeschlagen: " + autoErr
	}
	state := State{
		Devices: ds, Wheels: wheels, SelectedWheelID: selectedWheelID, SelectionStatus: selectionStatus, DeviceDetectionError: detectionErrText, LegacyDrivers: legacy, LegacyDriverError: legacyErrText, ProfilerSummary: profilerSummary(profiler, profilerErr), ProfilerError: profilerErrText, ActiveMode: mode, HVCI: hvciStatus.Effective, HVCIConfigured: hvciStatus.Configured, HVCIRestartRequired: hvciStatus.RestartRequired, HVCIStatusError: hvciStatus.Error,
		LCoreRunning: lcore, BackupCount: backupCount,
		WheelModel: model, WheelPreference: displayPreference, OperatingPreference: operatingPref, DetectionEvidence: evidence,
		DataDir: dataDir, Portable: portable, OS: osName, LastError: lastErr, RawInputDevices: liveHIDPaths, HIDCandidates: append([]HIDCandidate(nil), discovery.Candidates...), RawInputError: discoveryErrorText(rawInputErr, hidInterfaceErr), Pedals: pedals,
	}
	syncNativeWheelManager(&state)
	state.Certification = HardwareCertificationProgress(state)
	// OpenG27-like lifecycle convenience: once Windows has safely identified a
	// single remembered Modern wheel, restore its native PID in the background
	// immediately after discovery. This no longer depends on the user opening the
	// live-input page first.
	maybePrepareRememberedModernWheel(state)
	return state
}

// CollectStateFast refreshes volatile connection/input/process state without
// re-running the expensive PowerShell/DISM-style driver or Profiler inventory.
// Those inventories are refreshed by CollectState on startup, explicit refresh,
// device changes and before every destructive operation.
func CollectStateFast(previous State) State {
	dataDir, portable := GetDataDir()
	var (
		discovery  wheelDiscoverySnapshot
		ds         []Device
		detectErr  error
		hvciStatus HVCIStatus
		lcore      bool
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); discovery = collectWheelDiscoveryNative(true) }()
	go func() { defer wg.Done(); hvciStatus = ReadHVCIStatus() }()
	go func() { defer wg.Done(); lcore = IsProcessRunning("LCore.exe") }()
	wg.Wait()
	ds = discovery.Devices
	detectErr = discoveryFatalError(discovery)
	liveHIDPaths := append([]string(nil), discovery.Paths...)
	rawInputErr := discovery.RawErr
	hidInterfaceErr := discovery.HIDErr

	if rawInputErr != nil && hidInterfaceErr != nil {
		liveHIDPaths = append([]string(nil), previous.RawInputDevices...)
		discovery.Candidates = append([]HIDCandidate(nil), previous.HIDCandidates...)
	}
	lastErr := previous.LastError
	if detectErr != nil {
		// Never invoke the PowerShell compatibility fallback from the periodic fast
		// path. Keep the last good device tree until a full refresh can verify it.
		ds = append([]Device(nil), previous.Devices...)
		lastErr = "Schneller Geräte-Scan fehlgeschlagen: " + detectErr.Error()
	} else if strings.HasPrefix(lastErr, "Schneller Geräte-Scan fehlgeschlagen:") {
		lastErr = ""
	}
	if autoErr := wheelAutoSwitchError(); autoErr != "" && lastErr == "" {
		lastErr = "Automatischer Native-Mode-Switch fehlgeschlagen: " + autoErr
	}

	mode := "KEIN LOGITECH WHEEL"
	if len(ds) > 0 {
		mode = "Generic HID / Modern"
		for _, d := range ds {
			x := strings.ToLower(d.Service + " " + d.INF)
			if strings.Contains(x, "wm") || strings.Contains(x, "lgjoyhid") {
				mode = "Logitech Legacy"
				break
			}
		}
	}
	pref := ReadWheelPreference(dataDir)
	operatingPref := ReadOperatingPreference(dataDir)
	model, effectiveDevices, evidence := resolveWheelModel(ds, liveHIDPaths, "")
	ds = effectiveDevices
	if (model == "" || model == "Kein kompatibles Lenkrad") && G27DirectHIDConnected() {
		model = modelG27
		evidence = "LogiMate Direct HID verbunden"
		if len(ds) == 0 {
			stable := ReadSelectedWheelID(dataDir)
			if stable == "" {
				stable = "direct:g27"
			}
			ds = []Device{{Status: "Synthetic", Name: "Logitech G27 (Direct HID continuity)", Service: "HID", InstanceID: `HID\VID_046D&PID_C29B\DIRECT`, PhysicalID: "direct:g27", StableID: stable, Model: modelG27}}
		}
		mode = "Generic HID / Modern"
	}
	if len(ds) > 0 && mode == "KEIN LOGITECH WHEEL" {
		mode = "Generic HID / Modern"
	}
	wheels, selectedWheelID, model, evidence, mode, selectionStatus := reconcileWheelSelection(dataDir, ds, pref, model, evidence, mode)
	displayPreference := effectiveWheelPreference(wheels, selectedWheelID)
	deviceDetectionError := ""
	if detectErr != nil {
		// The fast path preserved the last known-good device tree above. Keep the
		// prior strict error (if any), otherwise expose this transient failure.
		deviceDetectionError = previous.DeviceDetectionError
		if deviceDetectionError == "" {
			deviceDetectionError = detectErr.Error()
		}
	}

	osName := previous.OS
	if osName == "" {
		osName = OSString()
	}
	pedals := ReadPedalMappingForWheel(dataDir, selectedWheelID, model, mode)
	state := State{
		Devices: ds, Wheels: wheels, SelectedWheelID: selectedWheelID, SelectionStatus: selectionStatus, DeviceDetectionError: deviceDetectionError,
		LegacyDrivers: append([]LegacyDriver(nil), previous.LegacyDrivers...), LegacyDriverError: previous.LegacyDriverError,
		ProfilerSummary: previous.ProfilerSummary, ProfilerError: previous.ProfilerError,
		ActiveMode: mode, HVCI: hvciStatus.Effective, HVCIConfigured: hvciStatus.Configured, HVCIRestartRequired: hvciStatus.RestartRequired, HVCIStatusError: hvciStatus.Error, LCoreRunning: lcore,
		WheelModel: model, WheelPreference: displayPreference, OperatingPreference: operatingPref, DetectionEvidence: evidence,
		BackupCount: previous.BackupCount, DataDir: dataDir, Portable: portable, OS: osName, LastError: lastErr,
		RawInputDevices: liveHIDPaths, HIDCandidates: append([]HIDCandidate(nil), discovery.Candidates...), RawInputError: discoveryErrorText(rawInputErr, hidInterfaceErr), Pedals: pedals,
	}
	syncNativeWheelManager(&state)
	state.Certification = HardwareCertificationProgress(state)
	maybePrepareRememberedModernWheel(state)
	return state
}

func BackupDrivers(dataDir string) (string, error) {
	drivers, scanErr := ListLegacyDriversStrict()
	if scanErr != nil {
		return "", scanErr
	}
	if len(drivers) == 0 {
		return "Keine Logitech WingMan/lgjoyhid-Treiber gefunden.", nil
	}
	stamp := time.Now().Format("2006-01-02_150405")
	dst := filepath.Join(backupRoot(dataDir), stamp)
	if err := os.MkdirAll(dst, 0755); err != nil {
		return "", err
	}
	var log []string
	for _, d := range drivers {
		out, err := RunHidden("pnputil.exe", "/export-driver", d.PublishedName, dst)
		log = append(log, d.PublishedName+" / "+d.OriginalName+": "+out)
		if err != nil {
			return strings.Join(log, "\r\n"), fmt.Errorf("Treiber-Backup unvollständig; es werden keine Treiber entfernt: %w", err)
		}
	}

	var backedFiles []string
	_ = filepath.Walk(dst, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			rel, _ := filepath.Rel(dst, path)
			backedFiles = append(backedFiles, rel)
		}
		return nil
	})
	if len(backedFiles) == 0 {
		return "", errors.New("Treiberexport meldete Erfolg, aber der Backup-Ordner enthält keine Dateien")
	}
	sort.Strings(backedFiles)
	hashes := make(map[string]string, len(backedFiles))
	for _, rel := range backedFiles {
		sum, err := fileSHA256(filepath.Join(dst, rel))
		if err != nil {
			return "", fmt.Errorf("Backup-Datei konnte nicht verifiziert werden (%s): %w", rel, err)
		}
		hashes[filepath.ToSlash(rel)] = sum
	}

	wheelDevices := DetectDevices()
	manifest := map[string]any{
		"created":      time.Now(),
		"complete":     true,
		"driverCount":  len(drivers),
		"drivers":      drivers,
		"wheelDevices": wheelDevices,
		"wheelModel":   SummarizeWheelModel(wheelDevices),
		"files":        backedFiles,
		"sha256":       hashes,
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	if err := AtomicWriteFile(filepath.Join(dst, "backup-manifest.json"), b, 0644); err != nil {
		return "", fmt.Errorf("Backup wurde exportiert, Manifest konnte aber nicht geschrieben werden: %w", err)
	}
	return fmt.Sprintf("Backup erstellt: %s\r\n%d Treiberpaket(e) vollständig gesichert.", dst, len(drivers)), nil
}

func latestBackupDir(dataDir string) string {
	entries, _ := os.ReadDir(backupRoot(dataDir))
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(backupRoot(dataDir), e.Name())
		manifestPath := filepath.Join(dir, "backup-manifest.json")
		b, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var m struct {
			Complete bool `json:"complete"`
		}
		if json.Unmarshal(b, &m) != nil || !m.Complete {
			continue
		}
		dirs = append(dirs, dir)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	if len(dirs) == 0 {
		return ""
	}
	return dirs[0]
}

func validateBackupHashes(dir string) error {
	b, err := os.ReadFile(filepath.Join(dir, "backup-manifest.json"))
	if err != nil {
		return err
	}
	var m struct {
		Complete bool              `json:"complete"`
		SHA256   map[string]string `json:"sha256"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if !m.Complete {
		return errors.New("Backup-Manifest ist nicht als vollständig markiert")
	}
	// Backward compatibility: v0.4.0 and older manifests did not contain file
	// hashes. They remain restorable; all new backups are verified file-by-file.
	if len(m.SHA256) == 0 {
		return nil
	}
	for rel, want := range m.SHA256 {
		clean := filepath.Clean(filepath.FromSlash(rel))
		if filepath.IsAbs(clean) || clean == "." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
			return fmt.Errorf("ungültiger Pfad im Backup-Manifest: %s", rel)
		}
		got, err := fileSHA256(filepath.Join(dir, clean))
		if err != nil {
			return fmt.Errorf("Backup-Datei fehlt/beschädigt (%s): %w", rel, err)
		}
		if !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(want)) {
			return fmt.Errorf("SHA-256-Prüfung fehlgeschlagen: %s", rel)
		}
	}
	return nil
}

// RestoreDriversForSelected performs the user-facing Legacy restore only after
// re-checking the current physical-wheel safety boundary in the elevated
// process. Rollback code uses RestoreDrivers directly because recovery may run
// while Windows is temporarily re-enumerating the wheel.
func RestoreDriversForSelected(dataDir string) (string, error) {
	if _, err := requireMigrationWheel(false); err != nil {
		return "", err
	}
	return RestoreDrivers(dataDir)
}

func RestoreDrivers(dataDir string) (string, error) {
	dir := latestBackupDir(dataDir)
	if dir == "" {
		return "", errors.New("Kein Treiber-Backup vorhanden")
	}
	if err := validateBackupHashes(dir); err != nil {
		return "", fmt.Errorf("Backup-Integritätsprüfung fehlgeschlagen: %w", err)
	}
	var infs []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".inf") {
			infs = append(infs, path)
		}
		return nil
	})
	if len(infs) == 0 {
		return "", errors.New("Backup enthaelt keine INF-Dateien")
	}
	var lines []string
	for _, inf := range infs {
		out, err := RunHidden("pnputil.exe", "/add-driver", inf, "/install")
		lines = append(lines, filepath.Base(inf)+": "+out)
		if err != nil {
			return strings.Join(lines, "\r\n"), err
		}
	}
	if out, err := RunHidden("pnputil.exe", "/scan-devices"); err != nil {
		return strings.Join(lines, "\r\n"), fmt.Errorf("Treiber wurden installiert, aber Windows konnte die Geräte nicht neu einlesen: %s: %w", out, err)
	}
	if err := SaveOperatingPreference(dataDir, "legacy"); err != nil {
		return strings.Join(lines, "\r\n"), fmt.Errorf("Treiber wurden wiederhergestellt, aber der LogiMate-Betriebsmodus konnte nicht gespeichert werden: %w", err)
	}
	InvalidateSystemCaches()
	return fmt.Sprintf("Logitech-Treiber aus %s wiederhergestellt (%d INF-Dateien).", dir, len(infs)), nil
}

func removeLegacyDriverPackages(drivers []LegacyDriver) (string, error) {
	// Best effort: LCore may not be running. A failure to stop it is not by
	// itself a driver-store failure; pnputil below remains authoritative.
	_, _ = RunHidden("taskkill.exe", "/IM", "LCore.exe", "/F")
	if len(drivers) == 0 {
		out, err := RunHidden("pnputil.exe", "/scan-devices")
		if err != nil {
			return "", fmt.Errorf("Windows-Gerätescan fehlgeschlagen: %s: %w", out, err)
		}
		InvalidateSystemCaches()
		return "Keine Logitech-Legacy-Treiberpakete gefunden. Generic HID ist bereits vorbereitet.", nil
	}
	var lines []string
	for _, d := range drivers {
		out, err := RunHidden("pnputil.exe", "/delete-driver", d.PublishedName, "/uninstall", "/force")
		lines = append(lines, d.PublishedName+": "+out)
		if err != nil {
			return strings.Join(lines, "\r\n"), err
		}
	}
	out, scanErr := RunHidden("pnputil.exe", "/scan-devices")
	if scanErr != nil {
		return strings.Join(lines, "\r\n"), fmt.Errorf("Legacy-Treiber wurden entfernt, aber Windows-Gerätescan fehlgeschlagen: %s: %w", out, scanErr)
	}
	InvalidateSystemCaches()
	return fmt.Sprintf("Legacy-Treiber entfernt: %d. Windows Generic HID wird verwendet, sobald das Logitech-Lenkrad neu enumeriert ist.", len(drivers)), nil
}

func RemoveLegacyDriversAfterBackup(dataDir string) (string, error) {
	state, err := requireMigrationWheel(false)
	if err != nil {
		return "", err
	}
	if latestBackupDir(dataDir) == "" {
		return "", errors.New("Sicherheitsabbruch: kein als vollständig markiertes Treiber-Backup gefunden")
	}
	drivers, err := ListLegacyDriversStrict()
	if err != nil {
		return "", err
	}
	if len(drivers) > 0 {
		if state.ActiveMode != "Logitech Legacy" {
			return "Legacy-Pakete sind nicht an das aktuelle Lenkrad gebunden und wurden nicht gelöscht.", nil
		}
		if err := ValidateLegacyDriverBinding(drivers, SelectedWheelDevices(state)); err != nil {
			return "", fmt.Errorf("Sicherheitsabbruch vor Treiberentfernung: %w", err)
		}
	}
	return removeLegacyDriverPackages(drivers)
}

func RemoveLegacyDrivers(dataDir string) (string, error) {
	state, err := requireMigrationWheel(false)
	if err != nil {
		return "", err
	}
	drivers, scanErr := ListLegacyDriversStrict()
	if scanErr != nil {
		return "", scanErr
	}
	if len(drivers) == 0 {
		return removeLegacyDriverPackages(nil)
	}
	if state.ActiveMode != "Logitech Legacy" {
		if err := SaveOperatingPreference(dataDir, "modern"); err != nil {
			return "Das Lenkrad verwendet bereits keinen Legacy-Treiber; alte ungebundene Pakete wurden nicht gelöscht.", err
		}
		return "Das Lenkrad verwendet bereits Generic HID / Modern. Alte ungebundene Legacy-Pakete wurden aus Sicherheitsgründen nicht gelöscht.", nil
	}
	if err := ValidateLegacyDriverBinding(drivers, SelectedWheelDevices(state)); err != nil {
		return "", fmt.Errorf("Sicherheitsabbruch vor Treiberentfernung: %w", err)
	}
	backupMsg, err := BackupDrivers(dataDir)
	if err != nil {
		return backupMsg, err
	}
	removeMsg, err := RemoveLegacyDriversAfterBackup(dataDir)
	if err != nil {
		return backupMsg + "\r\n\r\n" + removeMsg, err
	}
	if err := SaveOperatingPreference(dataDir, "modern"); err != nil {
		return backupMsg + "\r\n\r\n" + removeMsg, fmt.Errorf("Treiber wurden umgestellt, aber der LogiMate-Betriebsmodus konnte nicht gespeichert werden: %w", err)
	}
	return backupMsg + "\r\n\r\n" + removeMsg, nil
}

func sanitize(s string) string {
	repl := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return repl.Replace(s)
}

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	const maxFiles = 5000
	const maxSingle = uint64(256 << 20)
	const maxTotal = uint64(512 << 20)
	if len(r.File) > maxFiles {
		return fmt.Errorf("ZIP enthält zu viele Dateien (%d)", len(r.File))
	}
	var total uint64
	for _, f := range r.File {
		if f.UncompressedSize64 > maxSingle {
			return fmt.Errorf("ZIP-Datei ist ungewöhnlich groß: %s", f.Name)
		}
		total += f.UncompressedSize64
		if total > maxTotal {
			return errors.New("ZIP-Inhalt überschreitet das Sicherheitslimit")
		}
	}
	cleanDst, _ := filepath.Abs(dst)
	for _, f := range r.File {
		p := filepath.Join(dst, f.Name)
		abs, _ := filepath.Abs(p)
		if !strings.HasPrefix(strings.ToLower(abs), strings.ToLower(cleanDst)+strings.ToLower(string(os.PathSeparator))) && !strings.EqualFold(abs, cleanDst) {
			return errors.New("unsicherer ZIP-Pfad")
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(p, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(p)
		if err != nil {
			rc.Close()
			return err
		}
		_, e1 := io.Copy(out, rc)
		e2 := out.Close()
		e3 := rc.Close()
		if e1 != nil {
			return e1
		}
		if e2 != nil {
			return e2
		}
		if e3 != nil {
			return e3
		}
	}
	return nil
}

func Launch(path string) error {
	if path == "" {
		return errors.New("Pfad fehlt")
	}
	return exec.Command(path).Start()
}

func OpenFolder(path string) error { return exec.Command("explorer.exe", path).Start() }

func BuildDiagnosticReport(s State) string {
	var b strings.Builder
	fmt.Fprintf(&b, "LogiMate Diagnostic Report\r\nGenerated: %s\r\nVersion: %s\r\nTheme: %s\r\nDPI: %d\r\nMonitors: %d\r\nHigh Contrast: %v\r\nReduced motion: %v\r\nWindows build: %d\r\nRenderer: %s\r\nWindows material active: %v\r\nRenderer detail: %s\r\n\r\n", time.Now().Format(time.RFC3339), s.AppVersion, s.Theme, s.DPI, s.MonitorCount, s.HighContrast, s.ReducedMotion, s.WindowsBuild, s.Renderer, s.MaterialActive, s.RendererDetail)
	fmt.Fprintf(&b, "OS: %s\r\nArchitecture: %s\r\nPortable: %v\r\nDataDir: %s\r\n\r\n", s.OS, runtime.GOARCH, s.Portable, redactPath(s.DataDir))
	out := NativeOutputSnapshot()
	lastWrite := "—"
	if !out.LastWrite.IsZero() {
		lastWrite = out.LastWrite.Format(time.RFC3339)
	}
	fmt.Fprintf(&b, "Native output: active=%v wheel=%s model=%s commands=%d emergencyStops=%d watchdogStops=%d\r\nNative output last command: %s\r\nNative output last error: %s\r\nNative output last write: %s\r\n\r\n", out.Active, redactIdentifier(out.WheelID), out.Model, out.CommandCount, out.EmergencyStops, out.WatchdogStops, out.LastCommand, out.LastError, lastWrite)
	ffb := NativeFFBSnapshot()
	ffbHeartbeat := "OK"
	if !NativeFFBHeartbeatHealthy(time.Now()) {
		ffbHeartbeat = "STALE"
	}
	ffbLast := "—"
	if !ffb.LastHeartbeat.IsZero() {
		ffbLast = ffb.LastHeartbeat.Format(time.RFC3339Nano)
	}
	engineProfile := ReadActiveNativeEngineProfile(s.DataDir, s.SelectedWheelID)
	fmt.Fprintf(&b, "Native FFB engine: active=%v generation=%d wheel=%s model=%s effect=%s profile=%s requested=%+d%% constant=%+d%% spring=%d%% damper=%d%% friction=%d%% frames=%d clips=%d slewLimited=%d transitions=%d watchdogStops=%d emergencyStops=%d heartbeat=%s\r\nNative engine profile: %s\r\nNative FFB config: %s\r\nNative FFB last heartbeat: %s\r\nNative FFB last error: %s\r\n\r\n", ffb.Active, ffb.Generation, redactIdentifier(ffb.WheelID), ffb.Model, ffb.Effect, ffb.ProfileName, ffb.Requested, ffb.Applied, ffb.SpringApplied, ffb.DamperApplied, ffb.FrictionApplied, ffb.Frames, ffb.ClipEvents, ffb.SlewLimited, ffb.EffectTransitions, ffb.WatchdogStops, ffb.EmergencyStops, ffbHeartbeat, NativeEngineProfileSummary(engineProfile), NativeFFBConfigSummary(ffb.Config), ffbLast, ffb.LastError)
	gs := GameSessionSnapshot()
	tel := TelemetrySnapshot()
	rec, recDetail := RuntimeOutputRecoveryNeeded(s.DataDir)
	ready := BuildReadinessReport(s)
	fmt.Fprintf(&b, "Game session: active=%v process=%s profile=%s engine=%s autoApplied=%v status=%s\r\n", gs.Active, gs.Process, gs.ProfileName, gs.EngineProfile, gs.AutoApplied, gs.Message)
	fmt.Fprintf(&b, "Telemetry: running=%v adapter=%s address=%s packets=%d frames=%d invalid=%d stale=%v rpm=%d force=%.3f physics=%v playerControl=%v ffbEnabled=%v error=%s\r\n", tel.Running, tel.Adapter, tel.Address, tel.Packets, tel.Frames, tel.Invalid, tel.Stale, tel.LastFrame.RPM, tel.LastFrame.Force, tel.LastFrame.Physics, tel.LastFrame.PlayerControl, tel.LastFrame.FFBEnabled, tel.LastError)
	fmt.Fprintf(&b, "Runtime output recovery marker: %v %s\r\nReadiness: %d/100 · codeAuditComplete=%v\r\n", rec, recDetail, ready.Score, ready.CodeAuditComplete)
	b.WriteString("\r\nLegacy builder · Native protocol comparison:\r\n")
	b.WriteString(FusionC2ProtocolShadowSummary())
	b.WriteString("\r\n\r\nNative scheduler diagnostics:\r\n")
	b.WriteString(FusionC3SchedulerSummary())
	b.WriteString("\r\n\r\nNative G27 parser / device lifecycle:\r\n")
	b.WriteString(FusionC4ParserSummary())
	b.WriteString("\r\n")
	b.WriteString(FusionC4DeviceLifecycleSummary(s))
	b.WriteString("\r\n\r\nLogiMate Native Game Output:\r\n")
	b.WriteString(FusionC6Summary())
	b.WriteString("\r\n\r\nStandalone Native Engine:\r\n")
	b.WriteString(FusionC7ParitySummary(s))
	b.WriteString("\r\n\r\nRelease Trust / Authenticode:\r\n")
	b.WriteString(ReleaseTrustSummary(s.AppVersion))
	b.WriteString("\r\n")
	for _, issue := range ValidateStateInvariants(s) {
		fmt.Fprintf(&b, "Invariant: %s/%s · %s\r\n", issue.Severity, issue.Code, issue.Message)
	}
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "Wheel model: %s\r\nDetection evidence: %s\r\nSelection status: %s\r\nDevice detection error: %s\r\nSelected wheel ID: %s\r\nLogical wheels: %d\r\nStored model preference: %s\r\nStored operating preference: %s\r\nActive mode: %s\r\nProfiler: %s\r\nPedal mapping: Gas=%s Brake=%s Clutch=%s\r\nHVCI configured: %v\r\nHVCI effective: %v\r\nHVCI restart required: %v\r\nHVCI probe error: %s\r\nLCore running: %v\r\nBacked-up INF files: %d\r\n\r\n", s.WheelModel, s.DetectionEvidence, s.SelectionStatus, s.DeviceDetectionError, redactIdentifier(s.SelectedWheelID), len(s.Wheels), s.WheelPreference, s.OperatingPreference, s.ActiveMode, s.ProfilerSummary, s.Pedals.Gas, s.Pedals.Brake, s.Pedals.Clutch, s.HVCIConfigured, s.HVCI, s.HVCIRestartRequired, s.HVCIStatusError, s.LCoreRunning, s.BackupCount)
	if strings.TrimSpace(s.SelectedWheelID) != "" {
		profile := ReadControlProfile(s.DataDir, s.SelectedWheelID)
		buttons := strings.Join(SortedButtonMappings(profile), ", ")
		if buttons == "" {
			buttons = "none"
		}
		fmt.Fprintf(&b, "Learned buttons: %s\r\n", buttons)
		fmt.Fprintf(&b, "Learned H-shifter signatures: %d\r\n", len(profile.Gears))
		if profile.Steering.RangeDegrees > 0 {
			fmt.Fprintf(&b, "Steering calibration: left=%d center=%d right=%d range=%d°\r\n", profile.Steering.Left, profile.Steering.Center, profile.Steering.Right, profile.Steering.RangeDegrees)
		} else {
			b.WriteString("Steering calibration: none\r\n")
		}
		fmt.Fprintf(&b, "Pedal calibration: Gas[min=%d max=%d inv=%v dz=%.0f%% curve=%s] Brake[min=%d max=%d inv=%v dz=%.0f%% curve=%s] Clutch[min=%d max=%d inv=%v dz=%.0f%% curve=%s]\r\n\r\n",
			s.Pedals.GasCalibration.Min, s.Pedals.GasCalibration.Max, s.Pedals.GasCalibration.Inverted, s.Pedals.GasCalibration.Deadzone*100, s.Pedals.GasCalibration.Curve,
			s.Pedals.BrakeCalibration.Min, s.Pedals.BrakeCalibration.Max, s.Pedals.BrakeCalibration.Inverted, s.Pedals.BrakeCalibration.Deadzone*100, s.Pedals.BrakeCalibration.Curve,
			s.Pedals.ClutchCalibration.Min, s.Pedals.ClutchCalibration.Max, s.Pedals.ClutchCalibration.Inverted, s.Pedals.ClutchCalibration.Deadzone*100, s.Pedals.ClutchCalibration.Curve)
	}
	if s.LastError != "" {
		fmt.Fprintf(&b, "Last state error: %s\r\n", s.LastError)
	}
	b.WriteString("Logical wheel targets:\r\n")
	for _, w := range s.Wheels {
		selected := ""
		if s.SelectedWheelID != "" && strings.EqualFold(w.ID, s.SelectedWheelID) {
			selected = " [SELECTED]"
		}
		fmt.Fprintf(&b, "- %s | model=%s | mode=%s | supported=%v | modelConfirmed=%v | pnpVerified=%v | evidence=%s | stable=%s | session=%s%s\r\n", w.Name, w.Model, w.Mode, w.Supported, w.ModelConfirmed, w.PnPVerified, w.Evidence, redactIdentifier(w.ID), redactIdentifier(w.SessionID), selected)
	}
	b.WriteString("\r\n")
	if s.LegacyDriverError != "" {
		fmt.Fprintf(&b, "Legacy driver scan error: %s\r\n", s.LegacyDriverError)
	}
	if s.ProfilerError != "" {
		fmt.Fprintf(&b, "Profiler detection error: %s\r\n", s.ProfilerError)
	}
	b.WriteString("Logitech wheel devices:\r\n")
	for _, d := range s.Devices {
		fmt.Fprintf(&b, "- %s | %s | service=%s | inf=%s | stable=%s | container=%s | location=%s | physical=%s | parent=%s | id=%s\r\n", d.Name, d.Status, d.Service, d.INF, redactIdentifier(d.StableID), redactIdentifier(d.ContainerID), redactIdentifier(d.LocationPath), redactIdentifier(d.PhysicalID), redactIdentifier(d.ParentID), redactIdentifier(d.InstanceID))
	}
	b.WriteString("\r\nLegacy driver packages:\r\n")
	for _, d := range s.LegacyDrivers {
		fmt.Fprintf(&b, "- %s | %s | %s\r\n", d.PublishedName, d.OriginalName, d.ProviderName)
	}
	joy := ReadPreferredWheelInput(s)
	b.WriteString("\r\nPreferred live-input snapshot:\r\n")
	b.WriteString(FormatJoy(joy))
	b.WriteString("\r\nRaw Input Logitech wheel paths:\r\n")
	if len(s.RawInputDevices) == 0 {
		b.WriteString("- none\r\n")
	} else {
		for _, path := range s.RawInputDevices {
			fmt.Fprintf(&b, "- raw:%s\r\n", redactIdentifier(path))
		}
	}

	return redactDiagnosticText(b.String())
}

func redactIdentifier(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.ToLower(v)))
	return "sha256:" + hex.EncodeToString(sum[:6])
}

var (
	diagnosticGUIDPattern     = regexp.MustCompile(`(?i)\{?[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\}?`)
	diagnosticDevicePattern   = regexp.MustCompile(`(?i)(?:\\\\\?\\|HID\\|USB\\)[^\r\n\s]+`)
	diagnosticUserPathPattern = regexp.MustCompile(`(?i)[A-Z]:\\Users\\[^\\\r\n]+`)
)

func redactDiagnosticText(text string) string {
	// Final defense-in-depth pass for diagnostics leaving the application.
	// Earlier fields are already redacted structurally; this catches identifiers
	// embedded inside long error strings returned by Windows or drivers.
	home, _ := os.UserHomeDir()
	if home != "" {
		text = strings.ReplaceAll(text, home, "%USERPROFILE%")
		text = strings.ReplaceAll(text, strings.ToLower(home), "%USERPROFILE%")
	}
	text = diagnosticUserPathPattern.ReplaceAllStringFunc(text, func(v string) string {
		parts := strings.SplitN(v, `\\`, 4)
		if len(parts) >= 3 {
			return `%USERPROFILE%`
		}
		return `%USERPROFILE%`
	})
	text = diagnosticDevicePattern.ReplaceAllStringFunc(text, redactIdentifier)
	text = diagnosticGUIDPattern.ReplaceAllStringFunc(text, redactIdentifier)
	return text
}

func redactPath(p string) string {
	home, _ := os.UserHomeDir()
	if home != "" {
		if len(p) >= len(home) && strings.EqualFold(p[:len(home)], home) {
			p = "%USERPROFILE%" + p[len(home):]
		}
	}
	return p
}

func ExportDiagnostics(dataDir string, s State) (string, error) {
	dir := filepath.Join(dataDir, "Diagnostics")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102-150405")
	report := BuildDiagnosticReport(s)
	txt := filepath.Join(dir, "LogiMate-Diagnostic-"+stamp+".txt")
	if err := os.WriteFile(txt, []byte(report), 0644); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(report))
	if err := os.WriteFile(txt+".sha256", []byte(hex.EncodeToString(sum[:])+"  "+filepath.Base(txt)+"\r\n"), 0644); err != nil {
		return "", fmt.Errorf("Diagnose-Prüfsumme konnte nicht geschrieben werden: %w", err)
	}
	zipPath := filepath.Join(dir, "LogiMate-Diagnostic-"+stamp+".zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(zf)
	add := func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.Base(path))
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	if err = add(txt); err == nil {
		err = add(txt + ".sha256")
	}
	if e := zw.Close(); err == nil {
		err = e
	}
	if e := zf.Close(); err == nil {
		err = e
	}
	if err != nil {
		return "", err
	}
	return zipPath, nil
}

func ParseUint(s string) uint32 {
	n, _ := strconv.ParseUint(strings.TrimSpace(s), 0, 32)
	return uint32(n)
}

// Joystick support (winmm) for live input diagnostics.
//
// WinMM is intentionally used only as an input-diagnostics source. Model
// identity and driver mode come from PnP (VID/PID), because joyGetNumDevs
// reports driver-supported slots rather than physically attached devices.
// We therefore probe every valid WinMM slot with joyGetPosEx first and only
// then read its capabilities/name.
var winmm = syscall.NewLazyDLL("winmm.dll")
var procJoyGetNumDevs = winmm.NewProc("joyGetNumDevs")
var procJoyGetDevCapsW = winmm.NewProc("joyGetDevCapsW")
var procJoyGetPosEx = winmm.NewProc("joyGetPosEx")

const (
	joyReturnAll          = 0xFF
	maxWinMMJoystickSlots = 16 // joyGetDevCaps accepts joystick IDs 0..15.
)

type JOYCAPSW struct {
	WMid, WPid                                                                                                                          uint16
	SzPname                                                                                                                             [32]uint16
	W_Xmin, W_Xmax, W_Ymin, W_Ymax, W_Zmin, W_Zmax                                                                                      uint32
	WNumButtons, WPeriodMin, WPeriodMax, W_Rmin, W_Rmax, W_Umin, W_Umax, W_Vmin, W_Vmax, WCapabilities, WMaxAxes, WNumAxes, WMaxButtons uint32
	SzRegKey                                                                                                                            [32]uint16
	SzOEMVxD                                                                                                                            [260]uint16
}

type JOYINFOEX struct {
	DwSize, DwFlags, DwXpos, DwYpos, DwZpos, DwRpos, DwUpos, DwVpos, DwButtons, DwButtonNumber, DwPOV, DwReserved1, DwReserved2 uint32
}

type JoyState struct {
	Found                          bool
	ID                             uint32
	Name                           string
	X, Y, Z, R, U, V, Buttons, POV uint32
	NumAxes, NumButtons            uint32
	XMin, XMax, YMin, YMax         uint32
	ZMin, ZMax, RMin, RMax         uint32
	UMin, UMax, VMin, VMax         uint32
	Selection                      string
	ConnectionState                string
	Connected                      []string
	Error                          string
	RawReport                      []byte
	ReportRateHz                   float64
	LastReportAge                  time.Duration
	Reconnects                     int
	LastInputError                 string
	SteeringDegrees                float64
	SteeringCalibrated             bool
	SteeringRangeDegrees           int
	WheelID                        string
	SessionID                      string
	InputSource                    string
	LayoutID                       string
	SampleValid                    bool
	SampleAt                       time.Time
	SampleGeneration               uint64

	// Native G27 controls. These stay zero-value for generic WinMM sources.
	// Gear: -1=reverse, 0=neutral, 1..6=H-pattern gear. DPad: 0=neutral,
	// 1=N, 2=NE, 3=E, 4=SE, 5=S, 6=SW, 7=W, 8=NW.
	Gear                    int
	DPad                    int
	PaddleLeft, PaddleRight bool
	WheelButtons            [6]bool
	ShifterButtons          [8]bool
	NativeControls          bool
}

func enumerateWinMMJoysticks() ([]JoyState, error) {
	if err := winmm.Load(); err != nil {
		return nil, fmt.Errorf("winmm.dll konnte nicht geladen werden: %w", err)
	}
	n, _, _ := procJoyGetNumDevs.Call()
	slots := int(n)
	if slots > maxWinMMJoystickSlots {
		slots = maxWinMMJoystickSlots
	}
	if slots <= 0 {
		return nil, nil
	}

	out := make([]JoyState, 0, slots)
	for i := 0; i < slots; i++ {
		// Microsoft documents joyGetPos/joyGetPosEx as the presence check. Do
		// this before joyGetDevCaps so empty driver slots never look attached.
		ji := JOYINFOEX{DwSize: uint32(unsafe.Sizeof(JOYINFOEX{})), DwFlags: joyReturnAll}
		r, _, _ := procJoyGetPosEx.Call(uintptr(i), uintptr(unsafe.Pointer(&ji)))
		if r != 0 {
			continue
		}

		caps := JOYCAPSW{}
		capsResult, _, _ := procJoyGetDevCapsW.Call(uintptr(i), uintptr(unsafe.Pointer(&caps)), unsafe.Sizeof(caps))
		name := fmt.Sprintf("WinMM Controller %d", i)
		if capsResult == 0 {
			if n := strings.TrimSpace(syscall.UTF16ToString(caps.SzPname[:])); n != "" {
				name = n
			}
		}
		out = append(out, JoyState{
			Found: true, ID: uint32(i), Name: name, ConnectionState: "Connected",
			X: ji.DwXpos, Y: ji.DwYpos, Z: ji.DwZpos, R: ji.DwRpos, U: ji.DwUpos, V: ji.DwVpos,
			Buttons: ji.DwButtons, POV: ji.DwPOV,
			NumAxes: caps.WNumAxes, NumButtons: caps.WNumButtons,
			XMin: caps.W_Xmin, XMax: caps.W_Xmax, YMin: caps.W_Ymin, YMax: caps.W_Ymax,
			ZMin: caps.W_Zmin, ZMax: caps.W_Zmax, RMin: caps.W_Rmin, RMax: caps.W_Rmax,
			UMin: caps.W_Umin, UMax: caps.W_Umax, VMin: caps.W_Vmin, VMax: caps.W_Vmax,
		})
	}
	return out, nil
}

var winMMCapsCache = struct {
	sync.Mutex
	at      time.Time
	devices []JoyState
	err     error
}{}

type winMMTelemetryState struct {
	lastPoll         time.Time
	lastError        string
	reconnects       int
	hadFailure       bool
	rateStart        time.Time
	rateCount        int
	rateHz           float64
	sampleGeneration uint64
}

var winMMTelemetry = struct {
	sync.Mutex
	byID map[uint32]*winMMTelemetryState
}{byID: map[uint32]*winMMTelemetryState{}}

func InvalidateInputCaches() {
	invalidateHIDMetadataCache()
	winMMCapsCache.Lock()
	winMMCapsCache.at = time.Time{}
	winMMCapsCache.devices = nil
	winMMCapsCache.err = nil
	winMMCapsCache.Unlock()
}

func enumerateWinMMJoysticksCached(maxAge time.Duration) ([]JoyState, error) {
	winMMCapsCache.Lock()
	if !winMMCapsCache.at.IsZero() && time.Since(winMMCapsCache.at) < maxAge {
		out := append([]JoyState(nil), winMMCapsCache.devices...)
		err := winMMCapsCache.err
		winMMCapsCache.Unlock()
		return out, err
	}
	winMMCapsCache.Unlock()

	devices, err := enumerateWinMMJoysticks()
	winMMCapsCache.Lock()
	winMMCapsCache.at = time.Now()
	winMMCapsCache.devices = append([]JoyState(nil), devices...)
	winMMCapsCache.err = err
	winMMCapsCache.Unlock()
	return devices, err
}

func pollWinMMJoystick(base JoyState) (JoyState, error) {
	ji := JOYINFOEX{DwSize: uint32(unsafe.Sizeof(JOYINFOEX{})), DwFlags: joyReturnAll}
	r, _, _ := procJoyGetPosEx.Call(uintptr(base.ID), uintptr(unsafe.Pointer(&ji)))
	if r != 0 {
		err := fmt.Errorf("joyGetPosEx(ID %d) fehlgeschlagen (MMRESULT %d)", base.ID, r)
		winMMTelemetry.Lock()
		t := winMMTelemetry.byID[base.ID]
		if t == nil {
			t = &winMMTelemetryState{}
			winMMTelemetry.byID[base.ID] = t
		}
		t.lastError = err.Error()
		t.hadFailure = true
		winMMTelemetry.Unlock()
		return JoyState{}, err
	}
	now := time.Now()
	winMMTelemetry.Lock()
	t := winMMTelemetry.byID[base.ID]
	if t == nil {
		t = &winMMTelemetryState{}
		winMMTelemetry.byID[base.ID] = t
	}
	if t.hadFailure {
		t.reconnects++
		t.hadFailure = false
	}
	if t.rateStart.IsZero() {
		t.rateStart = now
	}
	t.rateCount++
	if d := now.Sub(t.rateStart); d >= time.Second {
		t.rateHz = float64(t.rateCount) / d.Seconds()
		t.rateStart = now
		t.rateCount = 0
	}
	t.lastPoll = now
	t.sampleGeneration++
	hz, rec, lastErr, sampleGen := t.rateHz, t.reconnects, t.lastError, t.sampleGeneration
	winMMTelemetry.Unlock()
	base.Found = true
	base.X, base.Y, base.Z, base.R, base.U, base.V = ji.DwXpos, ji.DwYpos, ji.DwZpos, ji.DwRpos, ji.DwUpos, ji.DwVpos
	base.Buttons, base.POV = ji.DwButtons, ji.DwPOV
	base.ConnectionState = "Connected"
	base.InputSource = "winmm"
	base.LayoutID = "winmm-joyinfoex-v1"
	base.SampleValid = true
	base.SampleAt = now
	base.SampleGeneration = sampleGen
	base.ReportRateHz = hz
	base.Reconnects = rec
	base.LastInputError = lastErr
	return base, nil
}

func joyNameScore(name, wheelModel string) int {
	n := strings.ToLower(name)
	m := strings.ToLower(wheelModel)
	// In multi-controller systems only an explicit model name is strong enough
	// to select a WinMM slot. "Driving Force", "Racing Wheel" and "Logitech"
	// can describe other Logitech products and must not win a guess.
	if strings.Contains(n, "g27") {
		if strings.Contains(m, "g25") && !strings.Contains(m, "g27") {
			return -100
		}
		return 120
	}
	if strings.Contains(n, "g25") {
		if strings.Contains(m, "g27") && !strings.Contains(m, "g25") {
			return -100
		}
		return 120
	}
	if strings.Contains(m, "driving force gt") && strings.Contains(n, "driving force gt") {
		return 120
	}
	return 0
}

func isCorrelatableSingleWinMMName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return true
	}
	// These names are accepted only when there is exactly one physical WinMM
	// controller AND PnP/Raw Input already established that a supported wheel exists.
	// They are never identity evidence on their own.
	tokens := []string{
		"usb input device",
		"hid-compliant game controller",
		"hid-konformer gamecontroller",
		"game controller",
		"winmm controller",
		"driving force",
		"racing wheel",
	}
	for _, token := range tokens {
		if strings.Contains(n, token) {
			return true
		}
	}
	return n == "controller"
}

func selectWinMMJoystick(devices []JoyState, wheelModel string) JoyState {
	connected := make([]string, 0, len(devices))
	for _, d := range devices {
		connected = append(connected, fmt.Sprintf("ID %d: %s", d.ID, d.Name))
	}
	if len(devices) == 0 {
		return JoyState{Connected: connected, Error: "Kein physisch angeschlossener WinMM-Controller erkannt."}
	}

	bestIdx, bestScore, ties := -1, -1000, 0
	for i, d := range devices {
		score := joyNameScore(d.Name, wheelModel)
		if score > bestScore {
			bestIdx, bestScore, ties = i, score, 1
		} else if score == bestScore {
			ties++
		}
	}
	if bestIdx >= 0 && bestScore >= 70 && ties == 1 {
		j := devices[bestIdx]
		j.Connected = connected
		j.Selection = "WinMM-Gerätename"
		return j
	}

	knownWheel := IsSupportedWheelModel(wheelModel)
	if knownWheel && len(devices) == 1 && isCorrelatableSingleWinMMName(devices[0].Name) {
		j := devices[0]
		j.Connected = connected
		j.Selection = "PnP-Fallback: einziger generischer WinMM-Controller"
		return j
	}

	msg := "WinMM-Controller gefunden, aber kein unterstütztes Lenkrad eindeutig zugeordnet."
	if len(devices) > 1 {
		msg += " Mehrere Controller sind angeschlossen; LogiMate rät deshalb nicht."
	}
	return JoyState{Connected: connected, Error: msg}
}

func ReadJoystickForModel(wheelModel string) JoyState {
	devices, err := enumerateWinMMJoysticksCached(30 * time.Second)
	if err != nil {
		return JoyState{ConnectionState: "Read error", Error: err.Error()}
	}
	selected := selectWinMMJoystick(devices, wheelModel)
	if !selected.Found {
		return selected
	}
	j, err := pollWinMMJoystick(selected)
	if err == nil {
		return j
	}

	// The selected slot may have disappeared between cached capability discovery
	// and this poll. Invalidate once, rebuild the mapping, then poll only the new
	// selected slot. The 250 ms live loop never scans/caps every slot repeatedly.
	InvalidateInputCaches()
	devices, scanErr := enumerateWinMMJoysticksCached(30 * time.Second)
	if scanErr != nil {
		return JoyState{ConnectionState: "Read error", Error: scanErr.Error()}
	}
	selected = selectWinMMJoystick(devices, wheelModel)
	if !selected.Found {
		if selected.Error == "" {
			selected.Error = err.Error()
		}
		return selected
	}
	j, err = pollWinMMJoystick(selected)
	if err != nil {
		selected.Found = false
		selected.ConnectionState = "Read error"
		selected.Error = err.Error()
		return selected
	}
	return j
}

func axisPercent(v, min, max uint32) string {
	if max <= min || v < min {
		return ""
	}
	p := float64(v-min) * 100 / float64(max-min)
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return fmt.Sprintf("%5.1f%%", p)
}

func activeButtons(mask uint32, maxButtons uint32) string {
	if mask == 0 {
		return "keine"
	}
	limit := maxButtons
	if limit == 0 || limit > 32 {
		limit = 32
	}
	var items []string
	for i := uint32(0); i < limit; i++ {
		if mask&(1<<i) != 0 {
			items = append(items, strconv.Itoa(int(i+1)))
		}
	}
	if len(items) == 0 {
		return "keine"
	}
	return strings.Join(items, ", ")
}

func povText(v uint32) string {
	if v == 0xFFFF || v == 0xFFFFFFFF {
		return "zentriert"
	}
	return fmt.Sprintf("%d (1/100°)", v)
}

func FormatJoy(j JoyState) string {
	sharedHID := strings.Contains(strings.ToLower(j.Selection), "direct hid") || strings.Contains(strings.ToLower(j.Selection), "shared hid")
	sourceTitle := "WINMM-LIVE-TEST"
	if sharedHID {
		sourceTitle = "LOGIMATE / DIRECT-HID-LIVE-TEST"
	}
	telemetry := func() string {
		rate := "—"
		if j.ReportRateHz > 0 {
			rate = fmt.Sprintf("%.1f Hz", j.ReportRateHz)
		}
		age := "—"
		if j.LastReportAge > 0 {
			age = fmt.Sprintf("%.2f s", j.LastReportAge.Seconds())
		}
		lastErr := strings.TrimSpace(j.LastInputError)
		if lastErr == "" {
			lastErr = "keiner"
		}
		return fmt.Sprintf("Rate: %s  •  letzter Report: %s  •  Reconnects: %d\r\nLetzter HID/WinMM-Fehler: %s", rate, age, j.Reconnects, lastErr)
	}
	if !j.Found {
		var b strings.Builder
		b.WriteString(sourceTitle + "\r\n")
		if j.ConnectionState != "" {
			b.WriteString("Status: " + j.ConnectionState + "\r\n")
		}
		if j.Error != "" {
			b.WriteString(j.Error)
		} else {
			b.WriteString("Kein kompatibles Live-Eingabegerät erkannt.")
		}
		if j.LastInputError != "" {
			b.WriteString("\r\nLetzter Input-Fehler: " + j.LastInputError)
		}
		if len(j.Connected) > 0 {
			b.WriteString("\r\n\r\nVerbundene Controller:\r\n")
			for _, name := range j.Connected {
				b.WriteString("• " + name + "\r\n")
			}
		}
		b.WriteString("\r\nModellidentität kommt aus SetupAPI/PnP und Raw Input; native G25/G27/DFGT-Pfade bevorzugen LogiMates Direct HID, WinMM bleibt der konservative Fallback.")
		return b.String()
	}

	fmtAxis := func(label string, v, min, max uint32) string {
		pct := axisPercent(v, min, max)
		if pct == "" {
			return fmt.Sprintf("%-11s: %5d", label, v)
		}
		return fmt.Sprintf("%-11s: %5d  (%s)", label, v, pct)
	}
	steeringLine := fmtAxis("Lenken", j.X, j.XMin, j.XMax)
	if j.SteeringCalibrated {
		steeringLine += fmt.Sprintf("  => %+.1f° / %d°", j.SteeringDegrees, j.SteeringRangeDegrees)
	}

	if sharedHID {
		gear := "N"
		if j.Gear == -1 {
			gear = "R"
		} else if j.Gear > 0 {
			gear = strconv.Itoa(j.Gear)
		}
		raw := RawReportHex(j)
		if len(raw) > 96 {
			raw = raw[:96] + "…"
		}
		if raw == "" {
			raw = "—"
		}
		return fmt.Sprintf("%s\r\nGerät: %s\r\nQuelle: %s\r\nStatus: %s\r\nAchsen: %d  •  Tasten: %d\r\n%s\r\n\r\n%s\r\n%s\r\n%s\r\n%s\r\n\r\nGang: %s  •  D-Pad: %d\r\nWippen: L=%v R=%v\r\nButton-Maske: 0x%08X\r\nAktiv: %s\r\nRohreport: %s\r\n\r\nLogiMate liest den nativen HID-Input direkt. Der Rohdatenmodus kann die aktuelle Hardwarebelegung sichtbar machen; gelernte Zuordnungen werden pro physischem Wheel gespeichert.",
			sourceTitle, j.Name, j.Selection, j.ConnectionState, j.NumAxes, j.NumButtons, telemetry(),
			steeringLine,
			fmtAxis("Gas", j.Y, j.YMin, j.YMax),
			fmtAxis("Bremse", j.Z, j.ZMin, j.ZMax),
			fmtAxis("Kupplung", j.R, j.RMin, j.RMax),
			gear, j.DPad, j.PaddleLeft, j.PaddleRight, j.Buttons, activeButtons(j.Buttons, j.NumButtons), raw)
	}
	return fmt.Sprintf("%s\r\nGerät: %s\r\nJoystick-ID: %d\r\nZuordnung: %s\r\nStatus: %s\r\nAchsen: %d  •  Tasten: %d\r\n%s\r\n\r\n%s\r\n%s\r\n%s\r\n%s\r\n%s\r\n%s\r\n\r\nButtons: 0x%08X\r\nAktiv: %s\r\nPOV: %s\r\n\r\nWinMM liefert normalisierte Controllerwerte, aber keinen HID-Rohreport. Bei einem nativen G25/G27/DFGT versucht LogiMate Direct HID zuerst und nutzt WinMM nur als Fallback.",
		sourceTitle, j.Name, j.ID, j.Selection, j.ConnectionState, j.NumAxes, j.NumButtons, telemetry(),
		steeringLine,
		fmtAxis("Y", j.Y, j.YMin, j.YMax),
		fmtAxis("Z", j.Z, j.ZMin, j.ZMax),
		fmtAxis("R", j.R, j.RMin, j.RMax),
		fmtAxis("U", j.U, j.UMin, j.UMax),
		fmtAxis("V", j.V, j.VMin, j.VMax),
		j.Buttons, activeButtons(j.Buttons, j.NumButtons), povText(j.POV))
}

func OpenGameControllers() error {
	cmd := exec.Command("control.exe", "joy.cpl")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Windows Game Controllers (joy.cpl) konnte nicht geöffnet werden: %w", err)
	}
	return nil
}
