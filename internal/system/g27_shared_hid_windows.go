//go:build windows

package system

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// LogiMate owns one shared native G27 HID session for both input and output.
// OpenG27/HidSharp uses the same connected stream for reads and lg4ff writes;
// Build 010 mirrors that lifecycle so FFB never targets a second, detached
// device handle while the live input reader owns the real C29B session.
var g27Shared = struct {
	sync.RWMutex
	path           string
	handle         syscall.Handle
	generation     uint64
	latest         JoyState
	raw            []byte
	lastRead       time.Time
	lastScan       time.Time
	lastError      string
	lastErrorKind  string
	lastInputError string
	lastMalformed  time.Time
	malformedCount int
	nextOpen       time.Time
	failures       int
	reconnects     int
	everOpened     bool
	rateStart      time.Time
	rateCount      int
	rateHz         float64
}{handle: syscall.InvalidHandle}

var wheelAutoSwitch = struct {
	sync.Mutex
	running     bool
	lastAttempt time.Time
	lastError   string
	phase       string
	detail      string
}{}

func wheelAutoSwitchError() string {
	wheelAutoSwitch.Lock()
	defer wheelAutoSwitch.Unlock()
	return wheelAutoSwitch.lastError
}

func wheelAutoSwitchStatus() (string, string, bool) {
	wheelAutoSwitch.Lock()
	defer wheelAutoSwitch.Unlock()
	return wheelAutoSwitch.phase, wheelAutoSwitch.detail, wheelAutoSwitch.running
}

// NativeActivationStatus exposes the short-lived compatibility->native
// handshake for diagnostics/UI without exposing the writer itself.
func NativeActivationStatus() (phase, detail string, running bool) {
	return wheelAutoSwitchStatus()
}

func setWheelAutoSwitchStatus(phase, detail string) {
	wheelAutoSwitch.Lock()
	wheelAutoSwitch.phase = strings.TrimSpace(phase)
	wheelAutoSwitch.detail = strings.TrimSpace(detail)
	wheelAutoSwitch.Unlock()
}

func rawPathsForPID(paths []string, pid string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if devicePID(p) != pid {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(p))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	// OpenG27 does not trust arbitrary enumeration order: among matching HID
	// interfaces it prefers the device with the largest output report. Do the
	// same here, with input length/usage as deterministic tie breakers.
	return rankHIDPathsOpenG27Style(out, pid)
}

func rawPathForPID(paths []string, pid string) string {
	matches := rawPathsForPID(paths, pid)
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

// rawPathInstanceID converts a Raw Input interface path such as
// \\?\HID#VID_046D&PID_C29B#7&ABC&0&0000#{GUID} back to the PnP
// instance-ID form returned by SetupAPI. This gives LogiMate an exact bridge
// from live HID input to the selected logical wheel, even with two G27s.
func rawPathInstanceID(path string) string {
	v := strings.TrimSpace(path)
	if strings.HasPrefix(v, `\\?\`) {
		v = v[4:]
	}
	if i := strings.Index(strings.ToLower(v), "#{"); i >= 0 {
		v = v[:i]
	}
	v = strings.ReplaceAll(v, "#", `\`)
	return strings.ToUpper(strings.TrimSpace(v))
}

func rawPathsForWheel(w WheelDevice, paths []string, pid string) []string {
	interfaces := map[string]bool{}
	for _, id := range w.InterfaceIDs {
		if id = strings.ToUpper(strings.TrimSpace(id)); id != "" {
			interfaces[id] = true
		}
	}
	if id := strings.ToUpper(strings.TrimSpace(w.InstanceID)); id != "" {
		interfaces[id] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, path := range paths {
		if devicePID(path) != pid || !interfaces[rawPathInstanceID(path)] {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(path))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, path)
	}
	return out
}

func nativeG27RawPath(paths []string) string { return rawPathForPID(paths, pidG27) }
func compatG27RawPath(paths []string) string { return rawPathForPID(paths, pidCompat) }

func g27ReportBase(report []byte) int {
	// Windows HID ReadFile normally includes a report-id byte. The G27 has no
	// meaningful report id, so it is 0. Some alternate readers omit it; accept
	// both layouts so the parser is independently testable.
	if len(report) >= 12 && report[0] == 0 {
		return 1
	}
	return 0
}

func boolBit(v byte, mask byte) bool { return v&mask != 0 }

func setMaskBit(mask *uint32, index int, active bool) {
	if active && index >= 0 && index < 32 {
		*mask |= 1 << uint(index)
	}
}

func g27RawAxisFields(report []byte) (steering uint16, throttle, brake, clutch byte, ok bool) {
	base := g27ReportBase(report)
	if len(report) < base+11 {
		return 0, 0, 0, 0, false
	}
	steerIdx := base + 3
	steering = binary.LittleEndian.Uint16(report[steerIdx : steerIdx+2])
	return steering, report[base+5], report[base+6], report[base+7], true
}

func parseG27NativeReport(report []byte) (JoyState, bool) {
	raw, err := wheelengine.ParseNativeInputReport(wheelengine.ModelG27, report)
	if err != nil {
		return JoyState{}, false
	}
	observeFusionC4RawFields(report, raw.Steering, raw.ThrottleRaw, raw.BrakeRaw, raw.ClutchRaw)

	// Stored LogiMate calibration remains a Windows/user-state concern. The
	// model-specific byte decoding itself is now platform-independent in D2.
	cfg := defaultNativeInputDefaults()
	steerAxis := normSignedToAxis(normalizeNativeSteering(raw.Steering, cfg.Steering, cfg.Deadzone))
	throttle := norm01ToAxis(normalizeNativePedal(raw.ThrottleRaw, cfg.Throttle))
	brake := norm01ToAxis(normalizeNativePedal(raw.BrakeRaw, cfg.Brake))
	clutch := norm01ToAxis(normalizeNativePedal(raw.ClutchRaw, cfg.Clutch))

	j := JoyState{
		Found: true, Name: "Logitech G27 · Direct HID",
		X: steerAxis, Y: throttle, Z: brake, R: clutch,
		XMin: 0, XMax: 65535, YMin: 0, YMax: 65535, ZMin: 0, ZMax: 65535, RMin: 0, RMax: 65535,
		NumAxes: 4, NumButtons: 16, NativeControls: true,
		Buttons: raw.Buttons, Gear: raw.Gear, DPad: raw.DPad,
		PaddleLeft: raw.PaddleLeft, PaddleRight: raw.PaddleRight,
		WheelButtons: raw.WheelButtons, ShifterButtons: raw.ShifterButtons,
		Selection:   "LogiMate Direct HID · native LogiMate-Kalibrierung",
		InputSource: "direct-hid", LayoutID: raw.LayoutID, SampleValid: true, SampleAt: time.Now(),
		POV: 0xFFFF,
	}
	if raw.DPad > 0 {
		j.POV = uint32((raw.DPad - 1) * 4500)
	}
	return j, true
}
func noteG27OpenFailure(err error) {
	g27Shared.Lock()
	g27Shared.failures++
	shift := g27Shared.failures - 1
	if shift > 4 {
		shift = 4
	}
	delay := 250 * time.Millisecond * time.Duration(1<<shift)
	if delay > 5*time.Second {
		delay = 5 * time.Second
	}
	g27Shared.nextOpen = time.Now().Add(delay)
	if err != nil {
		g27Shared.lastError = "CreateFile: " + err.Error()
		g27Shared.lastErrorKind = "open"
		g27Shared.lastInputError = g27Shared.lastError
	}
	g27Shared.Unlock()
}

func ensureG27SharedReader(paths []string) {
	path := nativeG27RawPath(paths)
	if path == "" {
		g27Shared.Lock()
		shouldScan := time.Since(g27Shared.lastScan) >= time.Second
		if shouldScan {
			g27Shared.lastScan = time.Now()
		}
		g27Shared.Unlock()
		if shouldScan {
			path = nativeG27RawPath(EnumerateRawInputLogitechWheels())
		}
	}
	if path == "" {
		return
	}

	g27Shared.RLock()
	alreadyOpen := strings.EqualFold(g27Shared.path, path) && g27Shared.handle != 0 && g27Shared.handle != syscall.InvalidHandle
	g27Shared.RUnlock()
	if alreadyOpen {
		return
	}
	g27Shared.RLock()
	nextOpen := g27Shared.nextOpen
	g27Shared.RUnlock()
	if !nextOpen.IsZero() && time.Now().Before(nextOpen) {
		return
	}

	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		noteG27OpenFailure(err)
		return
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, fileFlagOverlapped, 0)
	if err != nil || h == syscall.InvalidHandle {
		if err == nil {
			err = errors.New("Direct-HID-Gerät lieferte einen ungültigen Handle")
		}
		noteG27OpenFailure(err)
		return
	}

	g27Shared.Lock()
	old := g27Shared.handle
	g27Shared.generation++
	gen := g27Shared.generation
	g27Shared.path = path
	g27Shared.handle = h
	if g27Shared.everOpened {
		g27Shared.reconnects++
	} else {
		g27Shared.everOpened = true
	}
	g27Shared.latest = JoyState{}
	g27Shared.raw = nil
	g27Shared.lastRead = time.Time{}
	g27Shared.rateStart = time.Now()
	g27Shared.rateCount = 0
	g27Shared.rateHz = 0
	g27Shared.lastError = ""
	g27Shared.lastErrorKind = ""
	g27Shared.lastMalformed = time.Time{}
	g27Shared.malformedCount = 0
	g27Shared.nextOpen = time.Time{}
	g27Shared.failures = 0
	g27Shared.Unlock()
	if old != 0 && old != syscall.InvalidHandle {
		_ = syscall.CloseHandle(old)
	}
	go g27SharedReadLoop(h, path, gen)
}

// borrowG27SharedOutput returns the exact live C29B session currently used by
// the Direct-HID input reader. The caller must never close the returned handle;
// it remains owned by g27Shared until reconnect/invalidation.
func borrowG27SharedOutput(path string) (syscall.Handle, uint64, bool) {
	g27Shared.RLock()
	defer g27Shared.RUnlock()
	if g27Shared.handle == 0 || g27Shared.handle == syscall.InvalidHandle || g27Shared.path == "" {
		return syscall.InvalidHandle, 0, false
	}
	if !strings.EqualFold(strings.TrimSpace(g27Shared.path), strings.TrimSpace(path)) {
		return syscall.InvalidHandle, 0, false
	}
	return g27Shared.handle, g27Shared.generation, true
}

func g27SharedOutputHealthy(path string, h syscall.Handle, generation uint64) bool {
	g27Shared.RLock()
	defer g27Shared.RUnlock()
	return generation != 0 && g27Shared.generation == generation && g27Shared.handle == h &&
		h != 0 && h != syscall.InvalidHandle && strings.EqualFold(strings.TrimSpace(g27Shared.path), strings.TrimSpace(path))
}

// invalidateG27SharedOutput fail-closes the unified reader/writer session when
// an output operation becomes ambiguous. Closing the one shared handle also
// aborts the input read, after which normal discovery opens a fresh session.
func invalidateG27SharedOutput(path string, h syscall.Handle, generation uint64, cause error) {
	shouldClose := false
	g27Shared.Lock()
	if generation != 0 && g27Shared.generation == generation && g27Shared.handle == h &&
		strings.EqualFold(strings.TrimSpace(g27Shared.path), strings.TrimSpace(path)) {
		g27Shared.generation++
		g27Shared.handle = syscall.InvalidHandle
		g27Shared.path = ""
		msg := "Unified G27 HID output session invalidated"
		if cause != nil {
			msg += ": " + cause.Error()
		}
		g27Shared.lastError = msg
		g27Shared.lastErrorKind = "write"
		g27Shared.lastInputError = msg
		g27Shared.failures++
		shift := g27Shared.failures - 1
		if shift > 4 {
			shift = 4
		}
		g27Shared.nextOpen = time.Now().Add(250 * time.Millisecond * time.Duration(1<<shift))
		shouldClose = true
	}
	g27Shared.Unlock()
	if shouldClose && h != 0 && h != syscall.InvalidHandle {
		_ = syscall.CloseHandle(h)
	}
}

func g27SharedReadLoop(h syscall.Handle, path string, generation uint64) {
	buf := make([]byte, 64)
	for {
		event, eventErr := createOverlappedEvent()
		if eventErr != nil {
			invalidateG27SharedOutput(path, h, generation, fmt.Errorf("Direct-HID Read-Event: %w", eventErr))
			return
		}
		ov := syscall.Overlapped{HEvent: event}
		var n uint32
		err := syscall.ReadFile(h, buf, &n, &ov)
		if err == syscall.ERROR_IO_PENDING {
			for {
				waitResult, waitErr := syscall.WaitForSingleObject(event, 250)
				if waitErr != nil || waitResult == waitFailed {
					_, _, _ = pCancelIoEx.Call(uintptr(h), uintptr(unsafe.Pointer(&ov)))
					if waitErr != nil {
						err = fmt.Errorf("Direct-HID WaitForSingleObject: %w", waitErr)
					} else {
						err = errors.New("Direct-HID WaitForSingleObject fehlgeschlagen")
					}
					break
				}
				if waitResult == syscall.WAIT_OBJECT_0 {
					err = getOverlappedResult(h, &ov, &n)
					break
				}
				if waitResult == syscall.WAIT_TIMEOUT {
					if !g27SharedOutputHealthy(path, h, generation) {
						_, _, _ = pCancelIoEx.Call(uintptr(h), uintptr(unsafe.Pointer(&ov)))
						_ = syscall.CloseHandle(event)
						return
					}
					continue
				}
				_, _, _ = pCancelIoEx.Call(uintptr(h), uintptr(unsafe.Pointer(&ov)))
				err = fmt.Errorf("unerwarteter Direct-HID Wait-Status 0x%X", waitResult)
				break
			}
		}
		_ = syscall.CloseHandle(event)
		if err != nil || n == 0 {
			if err == nil {
				err = errors.New("Direct-HID-Lesung lieferte 0 Bytes")
			}
			shouldClose := false
			g27Shared.Lock()
			if g27Shared.generation == generation && g27Shared.handle == h {
				g27Shared.generation++
				g27Shared.handle = syscall.InvalidHandle
				g27Shared.path = ""
				g27Shared.lastError = err.Error()
				g27Shared.lastErrorKind = "read"
				g27Shared.lastInputError = "ReadFile: " + err.Error()
				g27Shared.failures++
				shift := g27Shared.failures - 1
				if shift > 4 {
					shift = 4
				}
				g27Shared.nextOpen = time.Now().Add(250 * time.Millisecond * time.Duration(1<<shift))
				shouldClose = true
			}
			g27Shared.Unlock()
			if shouldClose {
				_ = syscall.CloseHandle(h)
			}
			return
		}
		raw := append([]byte(nil), buf[:n]...)
		j, ok := parseG27NativeReport(raw)
		if !ok {
			g27Shared.Lock()
			if g27Shared.generation == generation && g27Shared.handle == h {
				g27Shared.lastMalformed = time.Now()
				g27Shared.malformedCount++
			}
			g27Shared.Unlock()
			continue
		}
		now := time.Now()
		j.Connected = []string{"Direct HID: " + path}
		j.ConnectionState = "Connected"
		j.RawReport = raw
		g27Shared.Lock()
		if g27Shared.generation == generation && g27Shared.handle == h {
			g27Shared.rateCount++
			if d := now.Sub(g27Shared.rateStart); d >= 500*time.Millisecond {
				g27Shared.rateHz = float64(g27Shared.rateCount) / d.Seconds()
				g27Shared.rateStart = now
				g27Shared.rateCount = 0
			}
			j.ReportRateHz = g27Shared.rateHz
			j.Reconnects = g27Shared.reconnects
			j.LastInputError = g27Shared.lastInputError
			g27Shared.latest = j
			g27Shared.raw = raw
			g27Shared.lastRead = now
			g27Shared.lastMalformed = time.Time{}
			g27Shared.malformedCount = 0
		}
		g27Shared.Unlock()
	}
}

func readG27SharedInput(paths []string) JoyState {
	ensureG27SharedReader(paths)
	g27Shared.RLock()
	j := g27Shared.latest
	h := g27Shared.handle
	path := g27Shared.path
	last := g27Shared.lastRead
	lastErr := g27Shared.lastError
	lastErrKind := g27Shared.lastErrorKind
	lastInputErr := g27Shared.lastInputError
	nextOpen := g27Shared.nextOpen
	lastMalformed := g27Shared.lastMalformed
	malformedCount := g27Shared.malformedCount
	reconnects := g27Shared.reconnects
	rateHz := g27Shared.rateHz
	raw := append([]byte(nil), g27Shared.raw...)
	g27Shared.RUnlock()

	// HID input reports are event-driven: an idle wheel is allowed to stop
	// producing reports for an arbitrary amount of time. The open shared HID
	// handle is therefore the connection authority, not "time since movement".
	connected := directHIDHandleConnected(h, path)
	if connected && j.Found {
		j.ConnectionState = "Connected"
		j.RawReport = raw
		j.ReportRateHz = rateHz
		j.Reconnects = reconnects
		j.LastInputError = lastInputErr
		if !last.IsZero() {
			j.LastReportAge = time.Since(last)
		}
		if malformedCount > 0 && !lastMalformed.IsZero() && (last.IsZero() || lastMalformed.After(last)) {
			j.ConnectionState = "Malformed"
			j.Selection += fmt.Sprintf(" · ungültiger HID-Report (%d)", malformedCount)
		} else if !last.IsZero() && time.Since(last) > 3*time.Second {
			j.ConnectionState = "Idle"
			j.Selection += " · verbunden (idle / letzter Zustand)"
		}
		return j
	}
	if connected {
		state := "Connected"
		selection := "LogiMate Direct HID · verbunden · wartet auf ersten Eingabereport"
		if malformedCount > 0 {
			state = "Malformed"
			selection = fmt.Sprintf("LogiMate Direct HID · verbunden · %d ungültige Reports vor erstem gültigen Zustand", malformedCount)
		}
		return JoyState{
			Found: false, Name: "Logitech G27 · Direct HID", ConnectionState: state,
			X: 32768, XMin: 0, XMax: 65535,
			YMin: 0, YMax: 65535, ZMin: 0, ZMax: 65535, RMin: 0, RMax: 65535,
			NumAxes: 4, NumButtons: 16, NativeControls: true, POV: 0xFFFF,
			Selection: selection, Connected: []string{"Direct HID: " + path}, RawReport: raw,
			ReportRateHz: rateHz, Reconnects: reconnects, LastInputError: lastInputErr,
		}
	}
	if lastErr != "" {
		if !nextOpen.IsZero() && time.Now().Before(nextOpen) {
			return JoyState{ConnectionState: "Reconnecting", Error: "Direct HID verbindet erneut: " + lastErr, LastInputError: lastErr, Reconnects: reconnects}
		}
		if lastErrKind == "read" {
			return JoyState{ConnectionState: "Read error", Error: "Direct HID Lesefehler: " + lastErr, LastInputError: lastErr, Reconnects: reconnects}
		}
		if lastErrKind == "switch" {
			return JoyState{ConnectionState: "Reconnecting", Error: lastErr, LastInputError: lastErr, Reconnects: reconnects}
		}
		return JoyState{ConnectionState: "Open error", Error: "Direct HID Öffnungsfehler: " + lastErr, LastInputError: lastErr, Reconnects: reconnects}
	}
	return JoyState{ConnectionState: "Reconnecting", Error: "Direct-HID-Sensorpfad wartet auf einen nativen G27-C29B-Report.", Reconnects: reconnects}
}

// G27DirectHIDConnected reports physical connection state from the shared HID
// handle. It intentionally does not depend on recent input activity.
func directHIDHandleConnected(h syscall.Handle, path string) bool {
	return h != 0 && h != syscall.InvalidHandle && path != ""
}

func G27DirectHIDConnected() bool {
	g27Shared.RLock()
	defer g27Shared.RUnlock()
	return directHIDHandleConnected(g27Shared.handle, g27Shared.path)
}

// nativeModeSpec returns Logitech's EXT_CMD9 selector for the three wheel
// models LogiMate currently manages. C294 is a shared compatibility identity;
// callers must already have an explicitly known/confirmed model before using
// these model-specific writes.
func nativeModeSpec(model string) (pid string, selector byte, label string, ok bool) {
	c, ok := CapabilitiesForWheelModel(model)
	if !ok || c.NativePID == "" || c.NativeModeSelector == 0 {
		return "", 0, "", false
	}
	return c.NativePID, c.NativeModeSelector, c.NativeModeLabel, true
}

func sendNativeModeReports(path, model string, selector byte, label string) error {
	// Native-mode switching is also a hardware write. Serialize it against every
	// other LogiMate output owner, even though C294 has not yet become native.
	hwMutex, err := acquireNamedMutex(`Global\LogiMate.NativeOutput.v1`)
	if err != nil {
		return fmt.Errorf("Native-Mode-Switch zu %s ist blockiert, solange ein anderer Hardware-Writer aktiv ist: %w", label, err)
	}
	defer releaseNamedMutex(hwMutex)
	var tr *nativeHIDTransport
	if IsG27Model(model) {
		// Match OpenG27/HidSharp exactly for the compatibility handshake: shared
		// read/write handle, OVERLAPPED WriteFile and a 3 s write deadline.
		tr, err = openOpenG27CompatibleSwitchTransport(path)
	} else {
		tr, err = openNativeHIDTransport(path)
	}
	if err != nil {
		return fmt.Errorf("C294-HID-Interface konnte für %s nicht geöffnet werden: %w", label, err)
	}
	defer tr.Close()

	reports := nativeModeReportsForModel(model, selector)
	for i, report := range reports {
		if err := tr.WriteReport(report); err != nil {
			return fmt.Errorf("Native-Mode-Switch zu %s fehlgeschlagen (Set_Report %d/%d): %w", label, i+1, len(reports), err)
		}
		if i+1 < len(reports) {
			time.Sleep(35 * time.Millisecond)
		}
	}
	return nil
}

// PrepareSupportedWheelNativeMode performs Logitech's standard non-firmware
// C294 -> native-mode switch for the integrated G25, DFGT and G27. It is never
// called for an unconfirmed C294 device. Driver/migration callers additionally
// enforce that exactly one physical supported wheel is attached, so picking the
// single C294 Raw-Input interface cannot target a sibling wheel by accident.
func PrepareSupportedWheelNativeMode(model string, paths []string) error {
	expectedPID, selector, label, ok := nativeModeSpec(model)
	if !ok || !IsKnownWheelModel(model) {
		return fmt.Errorf("kein sicher bestätigtes unterstütztes Modell für Native-Mode-Switch: %s", model)
	}
	if rawPathForPID(paths, expectedPID) != "" {
		if expectedPID == pidG27 {
			setWheelAutoSwitchStatus("native", "G27 bereits als C29B sichtbar")
		}
		if expectedPID == pidG27 {
			ensureG27SharedReader(paths)
		}
		return nil
	}

	compatPaths := rawPathsForPID(paths, pidCompat)
	if len(compatPaths) == 0 {
		// Driver removal and USB re-enumeration are asynchronous. Wait briefly for
		// either C294 or the requested native PID to appear.
		discoveryDeadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(discoveryDeadline) {
			time.Sleep(300 * time.Millisecond)
			if discovered, err := discoverLogitechHIDPathsOpenG27Style(); err == nil {
				paths = discovered
			}
			if rawPathForPID(paths, expectedPID) != "" {
				if expectedPID == pidG27 {
					ensureG27SharedReader(paths)
				}
				return nil
			}
			if compatPaths = rawPathsForPID(paths, pidCompat); len(compatPaths) > 0 {
				break
			}
		}
	}
	if len(compatPaths) == 0 {
		return fmt.Errorf("kein C294-Kompatibilitätsgerät für den Native-Mode-Switch zu %s gefunden", label)
	}

	if expectedPID == pidG27 {
		setWheelAutoSwitchStatus("compat-found", "G27 als C294 gefunden; bereite OpenG27-kompatiblen Native-Switch vor")
	}
	ClosePreferredInput()
	// One physical wheel can expose more than one HID collection. Raw Input does
	// not tell us which collection accepts Logitech output reports, so do not
	// blindly trust the first path. Try each C294 collection until one accepts the
	// complete Set_Report sequence. Destructive callers already require exactly
	// one physical wheel, so every candidate belongs to that same target session.
	var sendErrors []string
	sent := false
	for _, compat := range compatPaths {
		if expectedPID == pidG27 {
			setWheelAutoSwitchStatus("sending", "Sende 00 F8 09 04 01 00 00 00 an den bevorzugten C294-HID-Pfad")
		}
		if err := sendNativeModeReports(compat, model, selector, label); err != nil {
			sendErrors = append(sendErrors, err.Error())
			continue
		}
		sent = true
		break
	}
	if !sent {
		return fmt.Errorf("kein C294-HID-Interface akzeptierte den Native-Mode-Switch zu %s: %s", label, strings.Join(sendErrors, "; "))
	}

	if expectedPID == pidG27 {
		setWheelAutoSwitchStatus("waiting", "Native-Switch wurde geschrieben; warte auf USB-Neuanmeldung als C29B")
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(250 * time.Millisecond)
		if discovered, err := discoverLogitechHIDPathsOpenG27Style(); err == nil {
			paths = discovered
		}
		if rawPathForPID(paths, expectedPID) != "" {
			if expectedPID == pidG27 {
				setWheelAutoSwitchStatus("native", "G27 wurde als C29B neu erkannt und gebunden")
				ensureG27SharedReader(paths)
			}
			return nil
		}
	}
	return fmt.Errorf("Lenkrad wurde nach dem Native-Mode-Switch nicht als %s neu erkannt", label)
}

// ActivateSelectedWheelNativeMode is the explicit equivalent of OpenG27's
// Connect action. It is intentionally safe and narrow: exactly one actionable
// wheel must be selected, the live Windows binding must not be Legacy, and the
// same model-aware C294 -> native handshake used by the automatic path runs.
func ActivateSelectedWheelNativeMode(s State) error {
	if len(s.Wheels) != 1 || !HasActionableSelectedWheel(s) {
		return errors.New("genau ein bestätigtes unterstütztes Lenkrad muss ausgewählt sein")
	}
	if strings.Contains(strings.ToLower(s.ActiveMode), "legacy") {
		return errors.New("aktiver Logitech-Legacy-Treiber blockiert die Native-Aktivierung")
	}
	paths := append([]string(nil), s.RawInputDevices...)
	if discovered, err := discoverLogitechHIDPathsOpenG27Style(); err == nil && len(discovered) > 0 {
		paths = discovered
	}
	return PrepareSupportedWheelNativeMode(s.WheelModel, paths)
}

// PrepareG27DirectHID remains the compatibility entry point used by older
// LogiMate code; the actual switch is now shared with G25 and DFGT.
func PrepareG27DirectHID(paths []string) error {
	return PrepareSupportedWheelNativeMode(modelG27, paths)
}

func shouldAutoPrepareNativeWheel(s State) bool {
	if len(s.Wheels) != 1 || !HasActionableSelectedWheel(s) {
		return false
	}
	// The live driver binding is authoritative. A stale saved "legacy"
	// preference must not strand a physically generic-HID G27 in C294. Only an
	// actually active Logitech legacy binding suppresses the automatic switch.
	if strings.Contains(strings.ToLower(s.ActiveMode), "legacy") {
		return false
	}
	expectedPID, _, _, ok := nativeModeSpec(s.WheelModel)
	return ok && rawPathForPID(s.RawInputDevices, expectedPID) == "" && rawPathForPID(s.RawInputDevices, pidCompat) != ""
}

func maybePrepareRememberedModernWheel(s State) {
	// OpenG27 switches an unambiguous C294 G27 immediately. LogiMate supports
	// multiple classic Logitech models, so automatic switching still requires
	// a fully actionable/confirmed single wheel. An explicit Legacy preference
	// is the only opt-out. With no saved preference, a confirmed Generic-HID
	// wheel is treated as the normal LogiMate Native path and may be promoted
	// to its native PID automatically. This fixes first-run C294 wheels getting
	// stuck in compatibility mode even though Direct HID already proved G25/G27.
	if !shouldAutoPrepareNativeWheel(s) {
		return
	}
	wheelAutoSwitch.Lock()
	if wheelAutoSwitch.running || time.Since(wheelAutoSwitch.lastAttempt) < 8*time.Second {
		wheelAutoSwitch.Unlock()
		return
	}
	wheelAutoSwitch.running = true
	wheelAutoSwitch.lastAttempt = time.Now()
	wheelAutoSwitch.phase = "queued"
	wheelAutoSwitch.detail = "Automatische Native-Aktivierung geplant"
	wheelAutoSwitch.Unlock()
	go func(model string, paths []string) {
		err := PrepareSupportedWheelNativeMode(model, paths)
		wheelAutoSwitch.Lock()
		wheelAutoSwitch.running = false
		if err != nil {
			wheelAutoSwitch.lastError = err.Error()
			wheelAutoSwitch.phase = "failed"
			wheelAutoSwitch.detail = err.Error()
		} else {
			wheelAutoSwitch.lastError = ""
			if wheelAutoSwitch.phase == "" || wheelAutoSwitch.phase == "queued" {
				wheelAutoSwitch.phase = "native"
				wheelAutoSwitch.detail = "Native-Aktivierung abgeschlossen"
			}
		}
		wheelAutoSwitch.Unlock()
	}(s.WheelModel, append([]string(nil), s.RawInputDevices...))
}

// ReadPreferredWheelInput chooses the strongest live source without requiring
// external software. A confirmed G27 always tries LogiMate's shared, read-only C29B HID
// reader first. WinMM remains the compatibility fallback for legacy/non-G27
// paths and for a G27 that has not yet been explicitly switched to native HID.
func ReadPreferredWheelInput(s State) JoyState {
	if !HasReadableSelectedWheel(s) {
		return JoyState{ConnectionState: "Selection required", Error: "Kein eindeutig ausgewähltes unterstütztes Lenkrad für Live-Eingaben."}
	}
	if !HasActionableSelectedWheel(s) {
		j := ReadJoystickForModel(s.WheelModel)
		if j.Selection != "" {
			j.Selection += " · C294-Modellbestätigung ausstehend"
		}
		ApplyControlProfile(s.DataDir, s.SelectedWheelID, &j)
		return j
	}

	maybePrepareRememberedModernWheel(s)

	w, _ := SelectedWheel(s)
	modelKind := wheelModelKind(w)
	sameFamily := 0
	for _, candidate := range s.Wheels {
		if wheelModelKind(candidate) == modelKind {
			sameFamily++
		}
	}
	var out JoyState
	switch modelKind {
	case WheelModelG27:
		selectedNative := rawPathsForWheel(w, s.RawInputDevices, pidG27)
		if len(selectedNative) == 1 {
			out = readG27SharedInput(selectedNative)
		} else if sameFamily <= 1 {
			nativePaths := rawPathsForPID(s.RawInputDevices, pidG27)
			if len(nativePaths) <= 1 {
				out = readG27SharedInput(nativePaths)
			}
		}
		if !out.Found && sameFamily > 1 {
			return JoyState{ConnectionState: "Selection ambiguous", Error: "Mehrere G27 sind verbunden, aber der aktuelle Raw-Input-Pfad konnte nicht eindeutig dem ausgewählten Windows-Gerät zugeordnet werden. LogiMate rät nicht."}
		}
	case WheelModelG25, WheelModelDFGT:
		caps, ok := WheelCapabilitiesForDevice(w)
		if !ok || caps.NativePID == "" {
			return JoyState{ConnectionState: "Read error", Error: "Für das ausgewählte Wheel fehlt eine native Input-Capability."}
		}
		selectedNative := rawPathsForWheel(w, s.RawInputDevices, caps.NativePID)
		if len(selectedNative) == 1 {
			out = readClassicSharedInput(w.Model, selectedNative)
		} else if sameFamily <= 1 {
			paths := rawPathsForPID(s.RawInputDevices, caps.NativePID)
			if len(paths) <= 1 {
				out = readClassicSharedInput(w.Model, paths)
			}
		}
		if !out.Found && sameFamily > 1 {
			return JoyState{ConnectionState: "Selection ambiguous", Error: "Mehrere Lenkräder desselben Modells sind verbunden, aber der aktuelle Raw-Input-Pfad konnte nicht eindeutig dem ausgewählten Windows-Gerät zugeordnet werden. LogiMate rät nicht."}
		}
	default:
		if sameFamily > 1 {
			return JoyState{ConnectionState: "Selection ambiguous", Error: "Mehrere Lenkräder desselben Modells sind verbunden. WinMM liefert für dieses Modell keine sichere Zuordnung zur ausgewählten PnP-Instanz; Live-Eingaben werden deshalb nicht geraten."}
		}
	}
	if !out.Found {
		hidErr := strings.TrimSpace(out.Error)
		fallback := ReadJoystickForModel(s.WheelModel)
		if hidErr != "" {
			fallback.LastInputError = hidErr
			if fallback.Selection != "" {
				fallback.Selection += " · Direct-HID-Fallback"
			}
		}
		out = fallback
	}
	if out.Found {
		out.WheelID = s.SelectedWheelID
		if w, ok := SelectedWheel(s); ok {
			out.SessionID = w.SessionID
		}
		if out.InputSource == "" {
			out.InputSource = "winmm"
			out.LayoutID = "winmm-joyinfoex-v1"
		}
		if out.SampleAt.IsZero() {
			out.SampleAt = time.Now()
		}
		out.SampleValid = true
	}
	ApplyControlProfile(s.DataDir, s.SelectedWheelID, &out)
	updateNativeWheelInput(s, out)
	return out
}

func ClosePreferredInput() {
	closeClassicSharedInput()
	g27Shared.Lock()
	h := g27Shared.handle
	g27Shared.generation++
	g27Shared.handle = syscall.InvalidHandle
	g27Shared.path = ""
	g27Shared.latest = JoyState{}
	g27Shared.raw = nil
	g27Shared.lastRead = time.Time{}
	g27Shared.rateStart = time.Time{}
	g27Shared.rateCount = 0
	g27Shared.rateHz = 0
	g27Shared.lastError = ""
	g27Shared.lastErrorKind = ""
	g27Shared.lastMalformed = time.Time{}
	g27Shared.malformedCount = 0
	g27Shared.nextOpen = time.Time{}
	g27Shared.failures = 0
	g27Shared.everOpened = false
	g27Shared.Unlock()
	if h != 0 && h != syscall.InvalidHandle {
		_ = syscall.CloseHandle(h)
	}
}
