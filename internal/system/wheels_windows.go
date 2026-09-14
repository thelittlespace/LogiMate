//go:build windows

package system

import (
	"crypto/sha256"
	"encoding/hex"
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

const (
	pidCompat = "C294"
	pidG25    = "C299"
	pidDFGT   = "C29A"
	pidG27    = "C29B"

	modelG25    = "Logitech G25"
	modelG27    = "Logitech G27"
	modelDFGT   = "Logitech Driving Force GT"
	modelCompat = "Logitech C294 compatibility mode"
)

const (
	ModelG25  = modelG25
	ModelG27  = modelG27
	ModelDFGT = modelDFGT
)

type WheelModelConfirmation struct {
	StableID            string    `json:"stableId"`
	SessionID           string    `json:"sessionId"`
	HardwareFingerprint string    `json:"hardwareFingerprint,omitempty"`
	Model               string    `json:"model"`
	ConfirmedAt         time.Time `json:"confirmedAt"`
}

type wheelModelConfirmationFile struct {
	Version       int                               `json:"version"`
	Confirmations map[string]WheelModelConfirmation `json:"confirmations"`
}

func wheelModelConfirmationPath(dataDir string) string {
	return filepath.Join(dataDir, "wheel-confirmations.json")
}

type WheelNativeIdentityBinding struct {
	StableID            string    `json:"stableId"`
	SessionID           string    `json:"sessionId,omitempty"`
	HardwareFingerprint string    `json:"hardwareFingerprint,omitempty"`
	Model               string    `json:"model"`
	NativePID           string    `json:"nativePid"`
	LearnedAt           time.Time `json:"learnedAt"`
}

type wheelNativeIdentityFile struct {
	Version  int                                   `json:"version"`
	Bindings map[string]WheelNativeIdentityBinding `json:"bindings"`
}

func wheelNativeIdentityPath(dataDir string) string {
	return filepath.Join(dataDir, "wheel-native-history.json")
}

func defaultWheelNativeIdentityFile() wheelNativeIdentityFile {
	return wheelNativeIdentityFile{Version: 2, Bindings: map[string]WheelNativeIdentityBinding{}}
}

func readWheelNativeIdentitiesStrict(dataDir string) (wheelNativeIdentityFile, error) {
	f := defaultWheelNativeIdentityFile()
	found, err := ReadJSONConfigStrict(wheelNativeIdentityPath(dataDir), &f)
	if err != nil {
		return defaultWheelNativeIdentityFile(), err
	}
	if !found {
		return f, nil
	}
	if f.Version > 2 {
		return defaultWheelNativeIdentityFile(), fmt.Errorf("wheel-native-history.json verwendet Schema %d; unterstützt wird maximal 2", f.Version)
	}
	if f.Version != 2 || f.Bindings == nil {
		// Version 1 remembered only the USB location and is intentionally not
		// trusted for automatic model authorization. A different C294 wheel can
		// be plugged into the same port. Version 2 also binds the observed native
		// model to the Windows device session and/or strong hardware fingerprint.
		return defaultWheelNativeIdentityFile(), nil
	}
	return f, nil
}

func loadWheelNativeIdentities(dataDir string) wheelNativeIdentityFile {
	f, err := readWheelNativeIdentitiesStrict(dataDir)
	if err != nil {
		return defaultWheelNativeIdentityFile()
	}
	return f
}

func rememberAuthoritativeNativeIdentities(dataDir string, wheels []WheelDevice) {
	f, err := readWheelNativeIdentitiesStrict(dataDir)
	if err != nil {
		// Discovery must remain available, but a corrupt/newer history file must
		// never be silently replaced by an automatic background observation.
		return
	}
	changed := false
	for _, w := range wheels {
		if !w.PnPVerified || strings.TrimSpace(w.ID) == "" {
			continue
		}
		pid := devicePID(w.InstanceID)
		if pid != pidG25 && pid != pidG27 && pid != pidDFGT {
			continue
		}
		if !IsKnownWheelModel(w.Model) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(w.ID))
		old, ok := f.Bindings[key]
		canonical := canonicalWheelModel(w.Model)
		if ok && strings.EqualFold(old.Model, canonical) && strings.EqualFold(old.NativePID, pid) &&
			strings.EqualFold(old.SessionID, w.SessionID) && strings.EqualFold(old.HardwareFingerprint, w.HardwareFingerprint) {
			continue
		}
		f.Version = 2
		f.Bindings[key] = WheelNativeIdentityBinding{StableID: w.ID, SessionID: w.SessionID, HardwareFingerprint: w.HardwareFingerprint, Model: canonical, NativePID: pid, LearnedAt: time.Now().UTC()}
		changed = true
	}
	if changed {
		if err := WriteJSONConfigStrict(wheelNativeIdentityPath(dataDir), f, 0644); err != nil {
			return
		}
	}
}

func canonicalWheelModel(model string) string {
	switch ClassifyWheelModel(model) {
	case WheelModelG25:
		return modelG25
	case WheelModelG27:
		return modelG27
	case WheelModelDFGT:
		return modelDFGT
	default:
		return strings.TrimSpace(model)
	}
}

func applyLearnedNativeIdentity(dataDir string, wheels []WheelDevice, aggregateModel, aggregateEvidence string) []WheelDevice {
	f := loadWheelNativeIdentities(dataDir)
	if len(f.Bindings) == 0 {
		return wheels
	}
	var explicitSessionKind WheelModelKind
	if strings.Contains(strings.ToLower(aggregateEvidence), "winmm") {
		k := ClassifyWheelModel(aggregateModel)
		if k == WheelModelG25 || k == WheelModelG27 {
			explicitSessionKind = k
		}
	}
	for i := range wheels {
		w := &wheels[i]
		if !w.PnPVerified || w.ModelConfirmed || devicePID(w.InstanceID) != pidCompat {
			continue
		}
		b, ok := f.Bindings[strings.ToLower(strings.TrimSpace(w.ID))]
		if !ok || !isManualWheelModel(b.Model) {
			continue
		}
		kind := ClassifyWheelModel(b.Model)
		// A literal current-session G25/G27 WinMM name outranks remembered history.
		if explicitSessionKind != WheelModelUnknown && explicitSessionKind != kind {
			continue
		}
		strongMatch := strings.TrimSpace(b.HardwareFingerprint) != "" && strings.EqualFold(b.HardwareFingerprint, w.HardwareFingerprint)
		sessionMatch := strings.TrimSpace(b.SessionID) != "" && strings.EqualFold(b.SessionID, w.SessionID)
		currentKind := ClassifyWheelModel(aggregateModel)
		currentSupportsHistory := currentKind == kind && currentKind != WheelModelUnknown && currentKind != WheelModelCompatibility
		if !strongMatch && !sessionMatch && !currentSupportsHistory {
			// Stable USB location alone is not a physical identity. Keep the history as
			// a diagnostic hint but do not authorize a model-specific C294 command.
			continue
		}
		w.Model = canonicalWheelModel(b.Model) + " (zuvor nativ erkannt / C294)"
		w.ModelKind = kind
		w.ModelConfirmed = true
		w.Evidence = "C294 + gelernte native Identitaet (" + strings.ToUpper(b.NativePID) + ")"
		if strongMatch {
			w.Evidence += " + Hardware-Fingerprint"
		} else if sessionMatch {
			w.Evidence += " + gleicher Windows-Gerätecontainer"
		} else {
			w.Evidence += " + aktuelle Modell-Evidenz"
		}
	}
	return wheels
}

func ClearWheelNativeIdentityHistory(dataDir string) error {
	err := os.Remove(wheelNativeIdentityPath(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

var ephemeralWheelConfirmations = struct {
	sync.Mutex
	bySession map[string]WheelModelConfirmation
}{bySession: map[string]WheelModelConfirmation{}}

func ephemeralConfirmationKey(w WheelDevice) string {
	return strings.ToLower(strings.TrimSpace(w.ID)) + "|" + strings.ToLower(strings.TrimSpace(w.SessionID))
}

func InvalidateEphemeralWheelConfirmations() {
	ephemeralWheelConfirmations.Lock()
	ephemeralWheelConfirmations.bySession = map[string]WheelModelConfirmation{}
	ephemeralWheelConfirmations.Unlock()
}

func defaultWheelModelConfirmationFile() wheelModelConfirmationFile {
	return wheelModelConfirmationFile{Version: 2, Confirmations: map[string]WheelModelConfirmation{}}
}

func readWheelModelConfirmationsStrict(dataDir string) (wheelModelConfirmationFile, error) {
	f := defaultWheelModelConfirmationFile()
	found, err := ReadJSONConfigStrict(wheelModelConfirmationPath(dataDir), &f)
	if err != nil {
		return defaultWheelModelConfirmationFile(), err
	}
	if !found {
		return f, nil
	}
	if f.Version > 2 {
		return defaultWheelModelConfirmationFile(), fmt.Errorf("wheel-confirmations.json verwendet Schema %d; unterstützt wird maximal 2", f.Version)
	}
	if f.Version != 2 || f.Confirmations == nil {
		// Version 1 was keyed only by USB topology/session and is intentionally
		// not trusted after C8 because another C294 wheel on the same port could
		// inherit that authorization.
		return defaultWheelModelConfirmationFile(), nil
	}
	return f, nil
}

func loadWheelModelConfirmations(dataDir string) wheelModelConfirmationFile {
	f, err := readWheelModelConfirmationsStrict(dataDir)
	if err != nil {
		return defaultWheelModelConfirmationFile()
	}
	return f
}

func SaveWheelModelConfirmation(dataDir string, w WheelDevice, model string) error {
	if strings.TrimSpace(w.ID) == "" || strings.TrimSpace(w.SessionID) == "" || !w.PnPVerified {
		return errors.New("Modellbestätigung benötigt ein echtes PnP-verifiziertes Wheel mit stabiler und aktueller Session-ID")
	}
	if !isManualWheelModel(model) {
		return errors.New("ungueltiges Wheel-Modell")
	}
	c := WheelModelConfirmation{
		StableID: w.ID, SessionID: w.SessionID, HardwareFingerprint: w.HardwareFingerprint,
		Model: model, ConfirmedAt: time.Now().UTC(),
	}
	// Always authorize the current in-process device session. Without a real
	// serial-like hardware fingerprint this confirmation deliberately does not
	// survive PnP changes/restarts, because USB topology alone is not identity.
	ephemeralWheelConfirmations.Lock()
	ephemeralWheelConfirmations.bySession[ephemeralConfirmationKey(w)] = c
	ephemeralWheelConfirmations.Unlock()
	if !w.PersistentIdentity || strings.TrimSpace(w.HardwareFingerprint) == "" {
		// Remove any older topology-only record for this slot so it can never be
		// inherited by a replacement C294 wheel.
		f, err := readWheelModelConfirmationsStrict(dataDir)
		if err != nil {
			return err
		}
		key := strings.ToLower(strings.TrimSpace(w.ID))
		if _, ok := f.Confirmations[key]; ok {
			delete(f.Confirmations, key)
			if err := WriteJSONConfigStrict(wheelModelConfirmationPath(dataDir), f, 0644); err != nil {
				return err
			}
		}
		return nil
	}
	f, err := readWheelModelConfirmationsStrict(dataDir)
	if err != nil {
		return err
	}
	f.Version = 2
	key := strings.ToLower(strings.TrimSpace(w.ID))
	f.Confirmations[key] = c
	return WriteJSONConfigStrict(wheelModelConfirmationPath(dataDir), f, 0644)
}

func ClearWheelModelConfirmation(dataDir, stableID string) error {
	key := strings.ToLower(strings.TrimSpace(stableID))
	ephemeralWheelConfirmations.Lock()
	for k, c := range ephemeralWheelConfirmations.bySession {
		if strings.EqualFold(c.StableID, stableID) || strings.HasPrefix(k, key+"|") {
			delete(ephemeralWheelConfirmations.bySession, k)
		}
	}
	ephemeralWheelConfirmations.Unlock()
	if key == "" {
		return nil
	}
	f, err := readWheelModelConfirmationsStrict(dataDir)
	if err != nil {
		return err
	}
	if _, ok := f.Confirmations[key]; !ok {
		return nil
	}
	delete(f.Confirmations, key)
	if len(f.Confirmations) == 0 {
		err := os.Remove(wheelModelConfirmationPath(dataDir))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return WriteJSONConfigStrict(wheelModelConfirmationPath(dataDir), f, 0644)
}

// trustedWheelDevicePreferences only authorizes a manual C294 model when both
// the stable slot identity and the current Windows device-container/session
// identity match the explicit confirmation. A different wheel inserted into
// the same USB port therefore cannot inherit the old model authorization.
func trustedWheelDevicePreferences(dataDir string, wheels []WheelDevice) map[string]string {
	out := map[string]string{}
	f := loadWheelModelConfirmations(dataDir)
	for _, w := range wheels {
		key := strings.ToLower(strings.TrimSpace(w.ID))
		// First allow only an exact current-process session confirmation.
		ephemeralWheelConfirmations.Lock()
		e, eok := ephemeralWheelConfirmations.bySession[ephemeralConfirmationKey(w)]
		ephemeralWheelConfirmations.Unlock()
		if eok && strings.EqualFold(e.StableID, w.ID) && strings.EqualFold(e.SessionID, w.SessionID) && isManualWheelModel(e.Model) {
			out[key] = e.Model
			continue
		}
		// Cross-session persistence requires an independently strong hardware
		// fingerprint. Stable USB location and ContainerID are not sufficient.
		c, ok := f.Confirmations[key]
		if !ok || !w.PersistentIdentity || strings.TrimSpace(w.HardwareFingerprint) == "" ||
			!strings.EqualFold(c.StableID, w.ID) || !strings.EqualFold(c.HardwareFingerprint, w.HardwareFingerprint) ||
			!isManualWheelModel(c.Model) {
			continue
		}
		out[key] = c.Model
	}
	return out
}

func deviceIsPnPVerified(d Device) bool {
	id := strings.ToUpper(strings.TrimSpace(d.InstanceID))
	physical := strings.ToLower(strings.TrimSpace(d.PhysicalID))
	if strings.Contains(id, `\DIRECT`) || strings.HasPrefix(physical, "direct:") || strings.EqualFold(strings.TrimSpace(d.Status), "Synthetic") {
		return false
	}
	return strings.HasPrefix(id, `USB\`) || strings.HasPrefix(id, `HID\`)
}

var supportedWheelPIDs = [...]string{pidCompat, pidG25, pidDFGT, pidG27}

func supportedWheelPIDText(s string) bool {
	u := strings.ToUpper(s)
	for _, pid := range supportedWheelPIDs {
		if strings.Contains(u, "VID_046D&PID_"+pid) {
			return true
		}
	}
	return false
}

func isManualWheelModel(model string) bool {
	return model == modelG25 || model == modelG27 || model == modelDFGT
}

func IsG25Model(model string) bool  { return ClassifyWheelModel(model) == WheelModelG25 }
func IsG27Model(model string) bool  { return ClassifyWheelModel(model) == WheelModelG27 }
func IsDFGTModel(model string) bool { return ClassifyWheelModel(model) == WheelModelDFGT }

func IsCompatibilityModel(model string) bool {
	return ClassifyWheelModel(model) == WheelModelCompatibility
}

func IsExplicitlyUnsupportedWheelModel(model string) bool {
	l := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(l, "nicht integriert") || strings.Contains(l, "unsupported")
}

func CanPersistWheelSelection(w WheelDevice) bool {
	return w.PnPVerified && isStablePersistedWheelID(w.ID) && strings.TrimSpace(w.SessionID) != ""
}

func IsSupportedWheelModel(model string) bool {
	return IsG25Model(model) || IsG27Model(model) || IsDFGTModel(model) || IsCompatibilityModel(model)
}

// IsKnownWheelModel means LogiMate knows which model-specific command set is
// safe to use. C294 compatibility mode is detectable/readable but deliberately
// not "known" until G25/G27/DFGT has been confirmed or a native PID appears.
func IsKnownWheelModel(model string) bool {
	return (IsG25Model(model) || IsG27Model(model) || IsDFGTModel(model)) && !IsCompatibilityModel(model)
}

// isLegacyPersistedWheelID recognizes identifiers written by LogiMate before
// stable usbloc:*/usbslot:* identities were introduced. Only these known old schemas may
// receive the one-time migration to a new stable ID; arbitrary stale IDs must
// never be allowed to attach themselves to a different wheel.
func isLegacyPersistedWheelID(id string) bool {
	v := strings.TrimSpace(id)
	if v == "" || isStablePersistedWheelID(v) {
		return false
	}
	l := strings.ToLower(v)
	u := strings.ToUpper(v)
	return strings.HasPrefix(l, "container:") || strings.HasPrefix(u, `USB\`) || strings.HasPrefix(u, `HID\`) || strings.EqualFold(v, "DIRECT_HID") || strings.HasPrefix(l, "direct:")
}

// ValidateLegacyDriverBinding ensures destructive driver-store cleanup is
// anchored to the currently selected/supported wheel. We still back up all
// Logitech wheel-stack packages, but we refuse deletion when none of the
// candidate published INFs can be correlated with the active wheel nodes.
func ValidateLegacyDriverBinding(drivers []LegacyDriver, devices []Device) error {
	if len(drivers) == 0 {
		return nil
	}
	published := make(map[string]bool, len(drivers))
	for _, d := range drivers {
		if n := strings.ToLower(strings.TrimSpace(filepath.Base(d.PublishedName))); n != "" {
			published[n] = true
		}
	}
	var legacyWheelSeen bool
	for _, d := range devices {
		if !IsSupportedWheelModel(d.Model) && !supportedWheelPIDText(d.InstanceID) {
			continue
		}
		inf := strings.ToLower(strings.TrimSpace(filepath.Base(d.INF)))
		service := strings.ToLower(d.Service)
		if strings.Contains(service, "wm") || strings.Contains(service, "lgjoyhid") || inf != "" {
			legacyWheelSeen = legacyWheelSeen || strings.Contains(service, "wm") || strings.Contains(service, "lgjoyhid")
		}
		if inf != "" && published[inf] {
			return nil
		}
	}
	if legacyWheelSeen {
		return errors.New("Legacy-Treiber sind aktiv, aber der gebundene Wheel-INF konnte keinem zu löschenden Paket sicher zugeordnet werden")
	}
	return errors.New("kein aktiver Logitech-Wheel-Treiber ist an eines der zu löschenden Legacy-Pakete gebunden")
}

func wheelModeForDevice(d Device) string {
	x := strings.ToLower(d.Service + " " + d.INF)
	if strings.Contains(x, "wm") || strings.Contains(x, "lgjoyhid") {
		return "Logitech Legacy"
	}
	return "Generic HID / Modern"
}

func logicalWheelModel(d Device, preference string) (string, string) {
	pid := devicePID(d.InstanceID)
	switch pid {
	case pidG25:
		return modelG25, "SetupAPI/PnP native PID C299"
	case pidDFGT:
		return modelDFGT, "SetupAPI/PnP native PID C29A"
	case pidG27:
		return modelG27, "SetupAPI/PnP native PID C29B"
	case pidCompat:
		if IsExplicitlyUnsupportedWheelModel(d.Model) {
			return d.Model, "C294 + expliziter Name eines nicht integrierten Logitech-Wheels"
		}
		if d.Model == modelG25 || d.Model == modelG27 {
			return d.Model + " (Gerätename / C294)", "SetupAPI/PnP C294 + expliziter Gerätename"
		}
		if isManualWheelModel(preference) {
			return preference + " (manuell bestätigt / C294)", "C294 + gespeicherte Bestätigung"
		}
		return "Logitech C294 (Kompatibilitätsmodus)", "C294 erkannt; exaktes Modell noch unbestätigt"
	}
	if IsSupportedWheelModel(d.Model) {
		return d.Model, "Windows-Geräteerkennung"
	}
	return d.Model, "Logitech-Gerät erkannt; Modell nicht eindeutig"
}

// BuildWheelDevices converts low-level Windows device nodes to logical wheels.
// Windows ContainerID is excellent for grouping the current USB/HID devnodes,
// but multimode Logitech wheels can receive a new ContainerID when they switch
// PID. Persisted identity therefore uses a stable USB-slot token instead.
func usbWheelSlotID(instanceID string) string {
	u := strings.TrimSpace(instanceID)
	if u == "" || !strings.HasPrefix(strings.ToUpper(u), `USB\VID_046D&PID_`) || !supportedWheelPIDText(u) {
		return ""
	}
	parts := strings.Split(u, `\`)
	if len(parts) < 3 {
		return ""
	}
	token := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	if token == "" {
		return ""
	}
	return "usbslot:" + token
}

func stableWheelLocationID(locationPath string) string {
	v := strings.ToLower(strings.TrimSpace(locationPath))
	if v == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(v))
	// 96 bits keeps the persisted token compact while leaving collision risk
	// negligible for USB topology identifiers on one machine.
	return "usbloc:" + hex.EncodeToString(sum[:12])
}

// DeriveStableWheelIDWithLocation prefers Windows' explicit device-tree
// location over parsing an instance-ID suffix. The suffix remains a fallback
// for unusual images or compatibility enumeration paths without LocationPaths.
func DeriveStableWheelIDWithLocation(instanceID, parentID, locationPath string) string {
	if id := stableWheelLocationID(locationPath); id != "" {
		return id
	}
	return DeriveStableWheelID(instanceID, parentID)
}

// DeriveStableWheelID is intentionally exported for diagnostics/tests. The USB
// wheel devnode keeps the same port/topology instance suffix while a classic
// Logitech wheel changes from the shared C294 mode to C299/C29A/C29B. The HID
// collection can use its USB parent to reach the same token.
func DeriveStableWheelID(instanceID, parentID string) string {
	if id := usbWheelSlotID(instanceID); id != "" {
		return id
	}
	if id := usbWheelSlotID(parentID); id != "" {
		return id
	}
	return ""
}

func deviceStableID(d Device) string {
	if id := strings.ToLower(strings.TrimSpace(d.StableID)); id != "" {
		return id
	}
	return DeriveStableWheelIDWithLocation(d.InstanceID, d.ParentID, d.LocationPath)
}

func deviceSessionGroupID(d Device) string {
	// For the current Windows enumeration, ContainerID is the strongest grouping
	// key because the USB devnode and all HID child collections of one physical
	// device share it. Persisted identity remains separate (StableID/USB slot), so
	// a container replacement across C294 <-> native re-enumeration cannot break
	// cross-session selection while same-session USB/HID nodes still collapse.
	if id := strings.TrimSpace(d.ContainerID); id != "" {
		return "container:" + strings.ToLower(id)
	}
	// Some snapshots do not expose ContainerID but do carry the physical USB
	// parent in PhysicalID. Normalize that parent to the same USB-slot token used
	// by the USB devnode before falling back to interface-specific identities.
	if id := DeriveStableWheelID(d.PhysicalID, ""); id != "" {
		return id
	}
	if id := DeriveStableWheelID(d.InstanceID, d.ParentID); id != "" {
		return id
	}
	if id := deviceStableID(d); id != "" {
		return id
	}
	if id := strings.TrimSpace(d.PhysicalID); id != "" {
		return strings.ToLower(id)
	}
	return strings.ToLower(strings.TrimSpace(d.InstanceID))
}

// physicalWheelID is the persisted LogiMate identity. Keep the historical name
// because several safety helpers use it, but prefer StableID over session IDs.
func physicalWheelID(d Device) string {
	if id := deviceStableID(d); id != "" {
		return id
	}
	return deviceSessionGroupID(d)
}

func uniqueIDs(values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		k := strings.ToLower(v)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	return out
}

func wheelAliases(stableID, sessionID string, members []Device) []string {
	var ids []string
	ids = append(ids, stableID, sessionID)
	for _, d := range members {
		// Include both the new LocationPath-based ID and the historical USB-slot
		// derivation so upgrades migrate selections/settings without user action.
		ids = append(ids, d.StableID, deviceStableID(d), DeriveStableWheelID(d.InstanceID, d.ParentID), d.PhysicalID, d.InstanceID)
		if d.ContainerID != "" {
			ids = append(ids, "container:"+strings.ToLower(strings.TrimSpace(d.ContainerID)))
		}
	}
	return uniqueIDs(ids...)
}

func idMatchesWheel(id string, w WheelDevice) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	if strings.EqualFold(id, w.ID) || strings.EqualFold(id, w.SessionID) {
		return true
	}
	for _, alias := range w.AliasIDs {
		if strings.EqualFold(id, alias) {
			return true
		}
	}
	for _, interfaceID := range w.InterfaceIDs {
		if strings.EqualFold(id, interfaceID) {
			return true
		}
	}
	return false
}

func interfacePreference(perDevice map[string]string, stableID, sessionID string, members []Device) string {
	if perDevice == nil {
		return ""
	}
	for _, id := range wheelAliases(stableID, sessionID, members) {
		if p := perDevice[strings.ToLower(strings.TrimSpace(id))]; isManualWheelModel(p) {
			return p
		}
	}
	return ""
}

func preferredRepresentative(members []Device) Device {
	if len(members) == 0 {
		return Device{}
	}
	best := members[0]
	score := func(d Device) int {
		n := strings.ToLower(d.Name)
		id := strings.ToUpper(d.InstanceID)
		s := 0
		// The USB node is the strongest identity carrier (stable USB slot), while
		// HID is often the best live-input interface. Identity wins here; live HID
		// selection is handled separately.
		if strings.HasPrefix(id, `USB\`) {
			s += 35
		} else if strings.HasPrefix(id, `HID\`) {
			s += 25
		}
		if devicePID(id) != pidCompat {
			s += 20
		}
		if strings.Contains(n, "g25") || strings.Contains(n, "g27") || strings.Contains(n, "driving force gt") {
			s += 10
		}
		if d.Service != "" {
			s += 3
		}
		if d.INF != "" {
			s += 2
		}
		return s
	}
	for _, d := range members[1:] {
		if score(d) > score(best) {
			best = d
		}
	}
	return best
}

func groupMode(members []Device) string {
	for _, d := range members {
		if wheelModeForDevice(d) == "Logitech Legacy" {
			return "Logitech Legacy"
		}
	}
	return "Generic HID / Modern"
}

func usbSerialToken(instanceID string) string {
	v := strings.TrimSpace(instanceID)
	if !strings.HasPrefix(strings.ToUpper(v), `USB\VID_046D&PID_`) {
		return ""
	}
	parts := strings.Split(v, `\`)
	if len(parts) < 3 {
		return ""
	}
	token := strings.TrimSpace(parts[len(parts)-1])
	// Windows-generated topology instance tokens commonly contain '&'. A token
	// without it is the conservative signal that the USB device exposed a serial.
	if token == "" || strings.Contains(token, "&") {
		return ""
	}
	return strings.ToLower(token)
}

func strongWheelHardwareFingerprint(members []Device) string {
	// Prefer the HID serial queried directly from the device. Some Windows USB
	// instance IDs also contain a serial token; treating both representations as
	// two different serials would incorrectly discard a perfectly strong identity.
	hidSerials := map[string]bool{}
	usbSerials := map[string]bool{}
	for _, d := range members {
		if serial := strings.TrimSpace(d.HIDSerial); serial != "" {
			hidSerials[strings.ToLower(serial)] = true
		}
		for _, id := range []string{d.InstanceID, d.ParentID} {
			if serial := usbSerialToken(id); serial != "" {
				usbSerials[serial] = true
			}
		}
	}
	var serial string
	scheme := "usb"
	if len(hidSerials) == 1 {
		for v := range hidSerials {
			serial = v
		}
		scheme = "hid"
	} else if len(hidSerials) > 1 {
		return ""
	} else if len(usbSerials) == 1 {
		for v := range usbSerials {
			serial = v
		}
	} else {
		return ""
	}
	sum := sha256.Sum256([]byte("logitech-046d|" + scheme + "|" + serial))
	return "usbserial:" + hex.EncodeToString(sum[:16])
}

func preferredStableWheelID(members []Device) string {
	// Prefer the physical USB wheel devnode. HID collection LocationPaths may
	// contain collection-specific topology and SetupAPI enumeration order is not
	// guaranteed. The USB parent's identity is what should survive a mode change.
	for _, d := range members {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(d.InstanceID)), `USB\VID_046D&PID_`) {
			if id := deviceStableID(d); id != "" {
				return id
			}
		}
	}
	// If the USB node itself is missing, prefer an ID derivable from a USB
	// parent/suffix before accepting a HID collection-specific LocationPath.
	for _, d := range members {
		if id := DeriveStableWheelID(d.InstanceID, d.ParentID); id != "" {
			return id
		}
	}
	for _, d := range members {
		if id := deviceStableID(d); id != "" {
			return id
		}
	}
	return ""
}

func BuildWheelDevices(devs []Device, preference string) []WheelDevice {
	return BuildWheelDevicesWithPreferences(devs, preference, nil)
}

func BuildWheelDevicesWithPreferences(devs []Device, preference string, perDevice map[string]string) []WheelDevice {
	// `preference` is retained for source compatibility with older callers only.
	// A global C294 model fallback is unsafe: it can silently identify a newly
	// attached physical wheel as whatever model a previous wheel used. Model
	// confirmation is now valid only when bound to the stable physical Wheel ID.
	_ = preference
	groups := map[string][]Device{}
	order := make([]string, 0)
	for _, d := range devs {
		if !supportedWheelPIDText(d.InstanceID) && !IsSupportedWheelModel(d.Model) {
			continue
		}
		key := deviceSessionGroupID(d)
		if key == "" {
			continue
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], d)
	}

	out := make([]WheelDevice, 0, len(groups))
	for _, key := range order {
		members := groups[key]
		rep := preferredRepresentative(members)
		sessionID := key
		stableID := preferredStableWheelID(members)
		if stableID == "" {
			stableID = sessionID
		}
		devicePref := interfacePreference(perDevice, stableID, sessionID, members)
		pref := ""
		if isManualWheelModel(devicePref) {
			pref = devicePref
		}

		// Prefer authoritative native identity from any interface in the group.
		// A descriptive C294 friendly name may improve the display label, but it
		// never authorizes model-specific writes by itself. Only a native PID or
		// an explicit per-wheel confirmation sets ModelConfirmed.
		model, evidence := logicalWheelModel(rep, pref)
		pnpVerified := false
		for _, d := range members {
			if deviceIsPnPVerified(d) && supportedWheelPIDText(d.InstanceID) {
				pnpVerified = true
			}
			pid := devicePID(d.InstanceID)
			if (pid == pidG25 || pid == pidG27 || pid == pidDFGT) && deviceIsPnPVerified(d) {
				model, evidence = logicalWheelModel(d, pref)
				rep = d
				break
			}
		}
		nativeIdentityConfirmed := false
		for _, d := range members {
			pid := devicePID(d.InstanceID)
			if deviceIsPnPVerified(d) && (pid == pidG25 || pid == pidDFGT || pid == pidG27) {
				nativeIdentityConfirmed = true
				break
			}
		}
		// PnPVerified proves that Windows actually enumerated this physical target;
		// it does not prove which model an ambiguous C294 wheel is. Only a native
		// model PID or an explicit current-device confirmation may authorize model-
		// specific commands.
		modelConfirmed := nativeIdentityConfirmed || isManualWheelModel(devicePref)
		if devicePID(rep.InstanceID) == pidCompat && isManualWheelModel(devicePref) {
			evidence = "C294 + gespeicherte Gerätebestätigung"
		}

		name := strings.TrimSpace(rep.Name)
		for _, d := range members {
			n := strings.TrimSpace(d.Name)
			if strings.Contains(strings.ToLower(n), "g25") || strings.Contains(strings.ToLower(n), "g27") || strings.Contains(strings.ToLower(n), "driving force gt") {
				name = n
				break
			}
		}
		if name == "" {
			name = model
		}

		interfaces := make([]string, 0, len(members))
		for _, d := range members {
			if strings.TrimSpace(d.InstanceID) != "" {
				interfaces = append(interfaces, d.InstanceID)
			}
		}
		hardwareFingerprint := strongWheelHardwareFingerprint(members)
		modeText := groupMode(members)
		out = append(out, WheelDevice{
			ID: stableID, SessionID: sessionID, Name: name, Model: model, Mode: modeText, ModelKind: ClassifyWheelModel(model), ModeKind: ClassifyOperatingMode(modeText), Evidence: evidence,
			InstanceID: rep.InstanceID, Service: rep.Service, INF: rep.INF,
			InterfaceIDs: interfaces, AliasIDs: wheelAliases(stableID, sessionID, members),
			Supported:           !IsExplicitlyUnsupportedWheelModel(model) && (IsSupportedWheelModel(model) || supportedWheelPIDText(rep.InstanceID)),
			ModelConfirmed:      modelConfirmed,
			PnPVerified:         pnpVerified,
			HardwareFingerprint: hardwareFingerprint,
			PersistentIdentity:  hardwareFingerprint != "",
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		ni, nj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if ni == nj {
			return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID)
		}
		return ni < nj
	})
	return out
}

func WheelDeviceLabel(w WheelDevice) string {
	pid := devicePID(w.InstanceID)
	if pid == "" {
		pid = "?"
	}
	id := strings.TrimSpace(w.ID)
	short := id
	if len(short) > 18 {
		short = "…" + short[len(short)-17:]
	}
	return w.Name + " · " + w.Model + " · PID " + pid + " · " + short
}

func wheelModelKind(w WheelDevice) WheelModelKind {
	// Zero-value structs and older persisted state predate the typed identity
	// fields. An empty string is therefore "not populated", not an authoritative
	// model. Fall back to the presentation model just as we do for explicit
	// WheelModelUnknown.
	if strings.TrimSpace(string(w.ModelKind)) != "" && w.ModelKind != WheelModelUnknown {
		return w.ModelKind
	}
	return ClassifyWheelModel(w.Model)
}

func wheelModeKind(w WheelDevice) OperatingModeKind {
	if strings.TrimSpace(string(w.ModeKind)) != "" && w.ModeKind != OperatingModeUnknown {
		return w.ModeKind
	}
	return ClassifyOperatingMode(w.Mode)
}

func WheelCapabilitiesForDevice(w WheelDevice) (WheelCapabilities, bool) {
	d, ok := wheelengine.DescriptorForModel(wheelModelKind(w))
	if !ok || d.Model == wheelengine.ModelUnknown || d.Model == wheelengine.ModelCompatibility {
		return WheelCapabilities{Model: wheelModelKind(w)}, false
	}
	return WheelCapabilities{
		Model: d.Model, NativePID: d.NativePID, NativeModeSelector: d.NativeModeSelector, NativeModeLabel: d.NativeModeLabel,
		HasClutch: d.Controls.Clutch, HasHShifter: d.Controls.HShifter, HasRPMLEDs: d.Controls.RPMLEDs,
		HasNativeFFB: d.Output.NativeFFB, MaxRotationDegrees: d.Output.MaxRotationDeg,
	}, true
}

func SelectedWheel(s State) (WheelDevice, bool) {
	for _, w := range s.Wheels {
		if s.SelectedWheelID != "" && strings.EqualFold(w.ID, s.SelectedWheelID) {
			return w, true
		}
	}
	return WheelDevice{}, false
}

func SelectedWheelDevices(s State) []Device {
	w, ok := SelectedWheel(s)
	if !ok {
		return nil
	}
	var out []Device
	for _, d := range s.Devices {
		if strings.EqualFold(deviceSessionGroupID(d), w.SessionID) || strings.EqualFold(physicalWheelID(d), w.ID) {
			out = append(out, d)
		}
	}
	return out
}

// HasReadableSelectedWheel allows generic live diagnostics even while a C294
// model is still awaiting confirmation. It must never authorize model-specific
// HID writes or destructive driver transitions.
func HasReadableSelectedWheel(s State) bool {
	w, ok := SelectedWheel(s)
	return ok && w.Supported
}

func HasActionableSelectedWheel(s State) bool {
	w, ok := SelectedWheel(s)
	if !ok || !w.Supported || !w.ModelConfirmed || !w.PnPVerified {
		return false
	}
	k := wheelModelKind(w)
	return k == WheelModelG25 || k == WheelModelG27 || k == WheelModelDFGT
}

// SelectedWheelNeedsModelConfirmation is true only for a readable supported
// target whose exact model is not yet trusted for model-specific actions. This
// keeps UI guidance independent from display-friendly model strings.
func SelectedWheelNeedsModelConfirmation(s State) bool {
	w, ok := SelectedWheel(s)
	return ok && w.Supported && !w.ModelConfirmed
}

// CanChangeSelectedWheelMode is intentionally stricter than live diagnostics.
// Logitech driver-store packages can be shared by multiple attached wheels, so
// a destructive Modern/Legacy transition is only safe while exactly one
// supported physical wheel is attached. Multi-wheel selection remains useful
// for diagnostics, but never authorizes package deletion/restoration by itself.
func CanChangeSelectedWheelMode(s State) bool {
	return len(s.Wheels) == 1 && HasActionableSelectedWheel(s)
}
