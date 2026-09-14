//go:build windows

package system

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// NativeFFBConfig is deliberately conservative. 0.1.7 exposes bounded constant
// and condition-effect hardware tests, while persistent game-driven FFB remains
// gated until physical G25/DFGT/G27 validation is recorded.
type NativeFFBConfig struct {
	MasterGainPercent int `json:"masterGainPercent"`
	ConstantLimit     int `json:"constantTestLimitPercent"`
	SpringGain        int `json:"springGainPercent"`
	DamperGain        int `json:"damperGainPercent"`
	FrictionGain      int `json:"frictionGainPercent"`
	SlewPerTick       int `json:"slewPercentPerTick"`
	WatchdogMS        int `json:"watchdogMs"`
}

const nativeFFBConfigSchemaVersion = 1

// Manual hardware tests are intentionally separate from the persistent/game
// safety mixer. The UI value is the actual requested hardware-test strength,
// capped at 30% and automatically neutralized after a short bounded window.
// Normal game FFB keeps the stricter NativeFFBConfig caps unchanged.
const (
	NativeManualFFBTestMaxPercent     = 30
	NativeManualFFBTestDefaultPercent = 15
	nativeManualFFBTestDuration       = 3 * time.Second
)

type nativeFFBConfigFile struct {
	Version int                        `json:"version"`
	Wheels  map[string]NativeFFBConfig `json:"wheels"`
}

type NativeFFBStatus struct {
	Active            bool
	Stopping          bool
	Faulted           bool
	WheelID           string
	Model             string
	Effect            string
	Requested         int
	Applied           int
	SpringApplied     int
	DamperApplied     int
	FrictionApplied   int
	LastError         string
	Frames            uint64
	SlewLimited       uint64
	ClipEvents        uint64
	EffectTransitions uint64
	WatchdogStops     int
	EmergencyStops    int
	Generation        uint64
	ProfileName       string
	LastHeartbeat     time.Time
	StartedAt         time.Time
	Config            NativeFFBConfig
}

var nativeFFB = struct {
	sync.Mutex
	status     NativeFFBStatus
	cancel     chan struct{}
	done       chan struct{}
	lease      *nativeOutputLeaseToken
	generation uint64
	dataDir    string
}{}

func defaultNativeFFBConfig() NativeFFBConfig {
	return NativeFFBConfig{
		MasterGainPercent: 100,
		ConstantLimit:     10,
		SpringGain:        20,
		DamperGain:        20,
		FrictionGain:      20,
		SlewPerTick:       2,
		WatchdogMS:        1200,
	}
}

func nativeFFBConfigPath(dataDir string) string {
	return filepath.Join(dataDir, "native-wheel-engine.json")
}

func normalizeNativeFFBConfig(c NativeFFBConfig) NativeFFBConfig {
	d := defaultNativeFFBConfig()
	if c.MasterGainPercent <= 0 {
		c.MasterGainPercent = d.MasterGainPercent
	}
	if c.ConstantLimit <= 0 {
		c.ConstantLimit = d.ConstantLimit
	}
	if c.SpringGain <= 0 {
		c.SpringGain = d.SpringGain
	}
	if c.DamperGain <= 0 {
		c.DamperGain = d.DamperGain
	}
	if c.FrictionGain <= 0 {
		c.FrictionGain = d.FrictionGain
	}
	if c.SlewPerTick <= 0 {
		c.SlewPerTick = d.SlewPerTick
	}
	if c.WatchdogMS <= 0 {
		c.WatchdogMS = d.WatchdogMS
	}
	if c.MasterGainPercent > 100 {
		c.MasterGainPercent = 100
	}
	if c.ConstantLimit > 10 {
		c.ConstantLimit = 10
	}
	if c.SpringGain > 30 {
		c.SpringGain = 30
	}
	if c.DamperGain > 30 {
		c.DamperGain = 30
	}
	if c.FrictionGain > 30 {
		c.FrictionGain = 30
	}
	if c.SlewPerTick > 5 {
		c.SlewPerTick = 5
	}
	if c.WatchdogMS < 500 {
		c.WatchdogMS = 500
	}
	if c.WatchdogMS > 2000 {
		c.WatchdogMS = 2000
	}
	return c
}

func ReadNativeFFBConfig(dataDir, wheelID string) NativeFFBConfig {
	d := defaultNativeFFBConfig()
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(wheelID) == "" {
		return d
	}
	var f nativeFFBConfigFile
	if _, err := ReadJSONConfigStrict(nativeFFBConfigPath(dataDir), &f); err != nil || f.Wheels == nil {
		return d
	}
	c, ok := f.Wheels[inputProfileKey(wheelID)]
	if !ok {
		return d
	}
	return normalizeNativeFFBConfig(c)
}

func SaveNativeFFBConfig(dataDir, wheelID string, c NativeFFBConfig) error {
	if strings.TrimSpace(wheelID) == "" {
		return errors.New("keine stabile Wheel-ID")
	}
	c = normalizeNativeFFBConfig(c)
	f := nativeFFBConfigFile{Version: nativeFFBConfigSchemaVersion, Wheels: map[string]NativeFFBConfig{}}
	exists, err := ReadJSONConfigStrict(nativeFFBConfigPath(dataDir), &f)
	if err != nil {
		return err
	}
	if exists && f.Version > nativeFFBConfigSchemaVersion {
		return fmt.Errorf("native-ffb.json verwendet Schema %d; unterstützt wird maximal %d", f.Version, nativeFFBConfigSchemaVersion)
	}
	if f.Wheels == nil {
		f.Wheels = map[string]NativeFFBConfig{}
	}
	f.Version = nativeFFBConfigSchemaVersion
	f.Wheels[inputProfileKey(wheelID)] = c
	return WriteJSONConfigStrict(nativeFFBConfigPath(dataDir), f, 0644)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func translateClassicForcePercent(percent int) byte {
	percent = clampInt(percent, -100, 100)
	level := percent * 32767 / 100
	v := (level + 0x8000) >> 8
	return byte(clampInt(v, 0, 255))
}

func classicEffectOpcode(slot int, update bool) (byte, error) {
	if slot < 0 || slot > 3 {
		return 0, fmt.Errorf("ungültiger Effekt-Slot %d", slot)
	}
	base := byte(0x10 << uint(slot))
	if update {
		return base + 0x0c, nil
	}
	return base + 0x01, nil
}

// Slot 0 is the classic constant-force slot. 0x11 starts, 0x1c updates and
// 0x13 stops it; slots 1–3 follow the same classic Logitech slot encoding.
func BuildClassicConstantForceSlotReport(slot, percent int, update bool) ([]byte, error) {
	if percent < -NativeManualFFBTestMaxPercent || percent > NativeManualFFBTestMaxPercent {
		return nil, fmt.Errorf("Konstantkraft-Hardwaretest ist auf ±%d %% begrenzt", NativeManualFFBTestMaxPercent)
	}
	op, err := classicEffectOpcode(slot, update)
	if err != nil {
		return nil, err
	}
	return outputReport(op, 0x00, translateClassicForcePercent(percent), 0x00, 0x00, 0x00, 0x00), nil
}

func BuildClassicConstantForceReport(percent int, update bool) ([]byte, error) {
	return BuildClassicConstantForceSlotReport(0, percent, update)
}

// BuildNativeConstantForceReport is the productive OpenG27/lg4ff-compatible
// constant-force packet used by the D6.2 live test. The older slot builder is
// retained only for historical protocol diagnostics/tests.
func BuildNativeConstantForceReport(percent int) ([]byte, error) {
	if percent < -100 || percent > 100 {
		return nil, fmt.Errorf("Konstantkraft außerhalb -100..100 %%: %d", percent)
	}
	return wheelengine.WithReportID(wheelengine.ConstantForce(wheelengine.SliderToForceByte(percent))), nil
}

func BuildNativeStopAllEffectsReport() []byte {
	return wheelengine.WithReportID(wheelengine.StopAllEffects())
}

// BuildClassicFixedLoopReport selects the lg4ff slot scheduler mode. new-lg4ff
// explicitly sends 0x0D/0 before initialising its effect slots. Keeping this
// preamble in the manual hardware tests makes the Windows test sequence match
// the real lg4ff slot protocol instead of relying on whatever mode firmware
// happened to retain from a previous process.
func BuildClassicFixedLoopReport(enabled bool) []byte {
	v := byte(0)
	if enabled {
		v = 1
	}
	return outputReport(0x0D, v, 0, 0, 0, 0, 0)
}

func buildClassicConditionSlotReport(slot int, effect byte, payload [5]byte, update bool) ([]byte, error) {
	if slot < 1 || slot > 3 {
		return nil, fmt.Errorf("Condition-Effekte verwenden Slot 1–3; erhalten: %d", slot)
	}
	op, err := classicEffectOpcode(slot, update)
	if err != nil {
		return nil, err
	}
	return outputReport(op, effect, payload[0], payload[1], payload[2], payload[3], payload[4]), nil
}

func BuildClassicEffectStopReport(slot int) ([]byte, error) {
	if slot < 0 || slot > 3 {
		return nil, fmt.Errorf("ungültiger Effekt-Slot %d", slot)
	}
	cmd := byte((0x10 << uint(slot)) + 0x03)
	return outputReport(cmd, 0, 0, 0, 0, 0, 0), nil
}

func scaleU16(v, bits int) byte {
	v = clampInt(v, 0, 0xffff)
	return byte(v >> uint(16-bits))
}
func scaleCoeff(v, bits int) byte {
	if v < 0 {
		v = -v
	}
	return scaleU16(v*2, bits)
}

// The following condition reports mirror lg4ff's packet encoding. They are
// pure/testable and used only by short watchdog-bounded hardware tests; persistent
// game effects remain gated.
func BuildClassicSpringSlotReport(slot, strength int, update bool) ([]byte, error) {
	if strength < 0 || strength > 30 {
		return nil, fmt.Errorf("Spring-Testbasis ist auf 0–30 %% begrenzt")
	}
	d1, d2 := 1024, 1024
	k := -(strength * 32767 / 100)
	ak := absInt(k)
	if ak < 2048 {
		d1 = 0
		d2 = 2047
		ak = 0
	} else {
		ak -= 2048
	}
	sign := byte(1)
	coeff := scaleCoeff(ak, 4)
	packed := byte(((d2 & 7) << 5) | ((d1 & 7) << 1) | int(sign<<4) | int(sign))
	clip := scaleU16(strength*65535/100, 8)
	return buildClassicConditionSlotReport(slot, 0x0b, [5]byte{byte(d1 >> 3), byte(d2 >> 3), (coeff << 4) | coeff, packed, clip}, update)
}

func BuildClassicDamperSlotReport(slot, strength int, update bool) ([]byte, error) {
	if strength < 0 || strength > 30 {
		return nil, fmt.Errorf("Damper-Testbasis ist auf 0–30 %% begrenzt")
	}
	k := strength * 32767 / 100
	coeff := scaleCoeff(k, 4)
	clip := scaleU16(strength*65535/100, 8)
	return buildClassicConditionSlotReport(slot, 0x0c, [5]byte{coeff, 0, coeff, 0, clip}, update)
}

func BuildClassicFrictionSlotReport(slot, strength int, update bool) ([]byte, error) {
	if strength < 0 || strength > 30 {
		return nil, fmt.Errorf("Friction-Testbasis ist auf 0–30 %% begrenzt")
	}
	k := strength * 32767 / 100
	coeff := scaleCoeff(k, 8)
	clip := scaleU16(strength*65535/100, 8)
	return buildClassicConditionSlotReport(slot, 0x0e, [5]byte{coeff, coeff, clip, 0, 0}, update)
}

func BuildClassicSpringReport(strength int, update bool) ([]byte, error) {
	return BuildClassicSpringSlotReport(1, strength, update)
}
func BuildClassicDamperReport(strength int, update bool) ([]byte, error) {
	return BuildClassicDamperSlotReport(2, strength, update)
}
func BuildClassicFrictionReport(strength int, update bool) ([]byte, error) {
	return BuildClassicFrictionSlotReport(3, strength, update)
}

func applyFFBSlew(current, target, step int) (int, bool) {
	if step < 1 {
		step = 1
	}
	d := target - current
	if d > step {
		return current + step, true
	}
	if d < -step {
		return current - step, true
	}
	return target, false
}

func updateNativeFFBStatus(fn func(*NativeFFBStatus)) {
	nativeFFB.Lock()
	defer nativeFFB.Unlock()
	fn(&nativeFFB.status)
}

func updateNativeFFBStatusGeneration(generation uint64, fn func(*NativeFFBStatus)) bool {
	nativeFFB.Lock()
	defer nativeFFB.Unlock()
	if nativeFFB.status.Generation != generation || nativeFFB.generation != generation {
		return false
	}
	fn(&nativeFFB.status)
	return true
}

func stopNativeFFBSession(countEmergency bool) error {
	nativeFFB.Lock()
	ch := nativeFFB.cancel
	done := nativeFFB.done
	lease := nativeFFB.lease
	if ch != nil {
		close(ch)
		nativeFFB.cancel = nil
		nativeFFB.status.Stopping = true
	}
	if countEmergency && nativeFFB.status.Active {
		nativeFFB.status.EmergencyStops++
	}
	nativeFFB.Unlock()
	if done != nil {
		select {
		case <-done:
			if lease != nil {
				if cur, ok := currentNativeOutputLease(); ok && cur.Generation == lease.Generation {
					updateNativeFFBStatus(func(st *NativeFFBStatus) {
						st.Active = true // hardware state is still unproven
						st.Stopping = false
						st.Faulted = true
						if strings.TrimSpace(st.LastError) == "" {
							st.LastError = "Output-Worker endete, aber Neutralisierung/Lease-Freigabe wurde nicht bestätigt"
						}
					})
					return errors.New("Output-Worker endete ohne bestätigte Neutralisierung; Lease bleibt gesperrt")
				}
			}
			updateNativeFFBStatus(func(st *NativeFFBStatus) { st.Stopping = false })
			return nil
		case <-time.After(2 * time.Second):
			updateNativeFFBStatus(func(st *NativeFFBStatus) {
				st.Active = true // conservative: output may still be active
				st.Stopping = false
				st.Faulted = true
				st.LastError = "Output-Worker antwortet nach 2 s nicht; Lease und Recovery-Marker bleiben gesperrt"
			})
			return errors.New("Output-Worker konnte nicht innerhalb von 2 s bestätigt beendet werden")
		}
	}
	return nil
}

// completeNativeMotorStop releases ownership only after both the hardware
// neutralization and the durable recovery-marker transition are confirmed.
// A marker-clear failure is intentionally fail-closed: the lease remains held.
func completeNativeMotorStop(lease nativeOutputLeaseToken, dataDir string, stopErr error) error {
	if stopErr != nil {
		return stopErr
	}
	if strings.TrimSpace(dataDir) != "" {
		if err := ClearRuntimeOutputMarker(dataDir); err != nil {
			return fmt.Errorf("Motor ist neutralisiert, aber Recovery-Marker konnte nicht bestätigt gelöscht werden: %w", err)
		}
	}
	if lease.Generation != 0 && !releaseNativeOutputLease(lease) {
		return errors.New("Neutralisierung bestätigt, aber Output-Lease konnte nicht eindeutig freigegeben werden")
	}
	return nil
}

func StartNativeConstantForceTest(s State, requestedPercent int) error {
	// Manual hardware tests intentionally bypass wheel/game profile gains so the
	// slider has one unambiguous meaning: 15% means a 15% protocol request. The
	// dedicated 30% manual-test ceiling, OutputLease, recovery marker and bounded
	// duration still apply. Persistent/game FFB keeps its stricter mixer caps.
	if err := NativeOutputEmergencyStop(s); err != nil {
		return fmt.Errorf("vorherige Motor-Ausgabe konnte nicht sicher neutralisiert werden: %w", err)
	}
	if requestedPercent <= 0 || requestedPercent > NativeManualFFBTestMaxPercent {
		return fmt.Errorf("Konstantkraft-Hardwaretest muss 1–%d %% sein", NativeManualFFBTestMaxPercent)
	}
	lease, err := acquireNativeOutputLease(s, "constant-force", true)
	if err != nil {
		return err
	}
	w, _ := SelectedWheel(s)
	cfg := ReadNativeFFBConfig(s.DataDir, w.ID)
	target := requestedPercent

	tr, err := openNativeHIDTransport(lease.Path)
	if err != nil {
		releaseNativeOutputLease(lease)
		return err
	}
	if err := MarkRuntimeOutputActiveTarget(s.DataDir, w, "constant-force"); err != nil {
		_ = tr.Close()
		releaseNativeOutputLease(lease)
		return fmt.Errorf("Recovery-Marker konnte nicht geschrieben werden; Motor-Ausgabe verweigert: %w", err)
	}
	first, _ := applyFFBSlew(0, target, cfg.SlewPerTick)
	firstReport, buildErr := BuildClassicConstantForceSlotReport(0, first, false)
	if buildErr != nil {
		_ = ClearRuntimeOutputMarker(s.DataDir)
		_ = tr.Close()
		releaseNativeOutputLease(lease)
		return buildErr
	}
	// new-lg4ff starts by selecting dynamic/non-fixed loop mode before slot
	// activation. This is motorless and makes the slot state deterministic.
	if writeErr := tr.WriteReport(BuildClassicFixedLoopReport(false)); writeErr != nil {
		_ = tr.Close()
		return abortMotorStartAfterPossibleWrite(lease, fmt.Errorf("FFB-Slotmodus konnte nicht initialisiert werden: %w", writeErr))
	}
	if writeErr := tr.WriteReport(firstReport); writeErr != nil {
		_ = tr.Close()
		return abortMotorStartAfterPossibleWrite(lease, fmt.Errorf("initialer FFB-HID-Write fehlgeschlagen: %w", writeErr))
	}

	cancel := make(chan struct{})
	done := make(chan struct{})
	nativeFFB.Lock()
	prev := nativeFFB.status
	nativeFFB.generation++
	generation := nativeFFB.generation
	nativeFFB.cancel, nativeFFB.done = cancel, done
	leaseCopy := lease
	nativeFFB.lease = &leaseCopy
	nativeFFB.dataDir = s.DataDir
	nativeFFB.status = NativeFFBStatus{Active: true, WheelID: w.ID, Model: w.Model, Effect: "constant-test", Requested: requestedPercent, Applied: first, Config: cfg, ProfileName: "Hardware-Test direkt", Generation: generation, StartedAt: time.Now(), LastHeartbeat: time.Now(), ClipEvents: prev.ClipEvents, EffectTransitions: prev.EffectTransitions + 1, WatchdogStops: prev.WatchdogStops, EmergencyStops: prev.EmergencyStops}
	nativeFFB.Unlock()

	go func() {
		defer close(done)
		defer tr.Close()
		defer func() {
			nativeFFB.Lock()
			if nativeFFB.lease != nil && nativeFFB.lease.Generation == lease.Generation {
				if cur, ok := currentNativeOutputLease(); !ok || cur.Generation != lease.Generation {
					nativeFFB.lease = nil
				}
			}
			if nativeFFB.done == done {
				nativeFFB.done = nil
			}
			if nativeFFB.cancel == cancel {
				nativeFFB.cancel = nil
			}
			nativeFFB.Unlock()
		}()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		duration := time.NewTimer(nativeManualFFBTestDuration)
		defer duration.Stop()
		current := first
		stopSlot, _ := BuildClassicEffectStopReport(0)
		safeStop := func(timed bool) error {
			stopErr := tr.WriteReport(stopSlot)
			stopErr = errors.Join(stopErr, tr.WriteReport(BuildNativeStopAllEffectsReport()))
			stopErr = completeNativeMotorStop(lease, s.DataDir, stopErr)
			updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) {
				st.Stopping = false
				st.Faulted = stopErr != nil
				st.Active = stopErr != nil
				if stopErr == nil {
					st.Applied = 0
				}
				st.LastHeartbeat = time.Now()
				st.Frames++
				if timed {
					st.WatchdogStops++
				}
				if stopErr != nil {
					st.LastError = stopErr.Error()
				} else {
					st.LastError = ""
				}
			})
			return stopErr
		}
		for {
			select {
			case <-cancel:
				_ = safeStop(false)
				return
			case <-duration.C:
				_ = safeStop(true)
				return
			case <-ticker.C:
				next, limited := applyFFBSlew(current, target, cfg.SlewPerTick)
				report, buildErr := BuildClassicConstantForceSlotReport(0, next, true)
				if buildErr != nil {
					updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) { st.LastError = buildErr.Error() })
					_ = safeStop(false)
					return
				}
				if writeErr := tr.WriteReport(report); writeErr != nil {
					updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) { st.LastError = writeErr.Error() })
					_ = safeStop(false)
					return
				}
				current = next
				updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) {
					st.Applied = current
					st.LastHeartbeat = time.Now()
					st.Frames++
					if limited {
						st.SlewLimited++
					}
				})
			}
		}
	}()
	return nil
}

func NativeFFBEmergencyStop(s State) error {
	// Keep this compatibility entry point, but route every emergency stop through
	// the single fail-closed output coordinator. The former duplicate path could
	// clear a recovery marker without proving lease release.
	return NativeOutputEmergencyStop(s)
}

func NativeFFBRelease() error {
	return stopNativeFFBSession(false)
}

func NativeFFBSnapshot() NativeFFBStatus {
	nativeFFB.Lock()
	defer nativeFFB.Unlock()
	return nativeFFB.status
}

// NativeFFBHeartbeatHealthy is intentionally pure enough for UI/diagnostics.
func NativeFFBHeartbeatHealthy(now time.Time) bool {
	st := NativeFFBSnapshot()
	if !st.Active {
		return true
	}
	if st.LastHeartbeat.IsZero() {
		return false
	}
	limit := time.Duration(st.Config.WatchdogMS) * time.Millisecond
	if limit <= 0 {
		limit = 1200 * time.Millisecond
	}
	return now.Sub(st.LastHeartbeat) <= limit
}

func NativeFFBConfigSummary(c NativeFFBConfig) string {
	c = normalizeNativeFFBConfig(c)
	return fmt.Sprintf("Master %d%% · Constant cap ±%d%% · Spring/Damper/Friction %d/%d/%d%% · Slew %d%%/20ms · Watchdog %dms", c.MasterGainPercent, c.ConstantLimit, c.SpringGain, c.DamperGain, c.FrictionGain, c.SlewPerTick, c.WatchdogMS)
}
