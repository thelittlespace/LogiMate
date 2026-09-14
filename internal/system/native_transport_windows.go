//go:build windows

package system

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

var (
	kernel32Transport             = syscall.NewLazyDLL("kernel32.dll")
	pCancelIoEx                   = kernel32Transport.NewProc("CancelIoEx")
	pCreateEventW                 = kernel32Transport.NewProc("CreateEventW")
	pGetOverlappedResult          = kernel32Transport.NewProc("GetOverlappedResult")
	hidTransportDLL               = syscall.NewLazyDLL("hid.dll")
	pHidDSetOutputReportTransport = hidTransportDLL.NewProc("HidD_SetOutputReport")
)

const (
	defaultNativeWriteTimeout = 350 * time.Millisecond
	openG27SwitchWriteTimeout = 3 * time.Second
	fileFlagOverlapped        = 0x40000000
	waitFailed                = 0xFFFFFFFF
)

// NativeHIDTransportMetrics aggregates the real Windows HID writer lifecycle.
// D5.9 uses these counters as diagnostic evidence only; they never promote the
// repository Stable gate by themselves.
type NativeHIDTransportMetrics struct {
	Opens            uint64        `json:"opens"`
	OpenFailures     uint64        `json:"openFailures"`
	Writes           uint64        `json:"writes"`
	SuccessfulWrites uint64        `json:"successfulWrites"`
	FailedWrites     uint64        `json:"failedWrites"`
	Timeouts         uint64        `json:"timeouts"`
	PartialWrites    uint64        `json:"partialWrites"`
	Cancels          uint64        `json:"cancels"`
	PoisonEvents     uint64        `json:"poisonEvents"`
	CompatFallbacks  uint64        `json:"compatFallbacks"`
	SharingFallbacks uint64        `json:"sharingFallbacks"`
	Closes           uint64        `json:"closes"`
	LastBackend      string        `json:"lastBackend,omitempty"`
	LastShareMode    string        `json:"lastShareMode,omitempty"`
	LastError        string        `json:"lastError,omitempty"`
	LastWrite        time.Time     `json:"lastWrite,omitempty"`
	LastWriteLatency time.Duration `json:"lastWriteLatency"`
	MaxWriteLatency  time.Duration `json:"maxWriteLatency"`
}

var nativeHIDMetrics = struct {
	sync.Mutex
	value NativeHIDTransportMetrics
}{}

// A unified G27 HID session may be borrowed by short-lived output transports.
// OutputLease serializes motor owners; this mutex additionally serializes the
// actual SetOutputReport calls that share the live input handle.
var g27SharedOutputWriteMu sync.Mutex

func noteNativeHIDOpen(err error) {
	nativeHIDMetrics.Lock()
	defer nativeHIDMetrics.Unlock()
	if err != nil {
		nativeHIDMetrics.value.OpenFailures++
		nativeHIDMetrics.value.LastError = err.Error()
		return
	}
	nativeHIDMetrics.value.Opens++
	nativeHIDMetrics.value.LastError = ""
}

func noteNativeHIDCancel() {
	nativeHIDMetrics.Lock()
	nativeHIDMetrics.value.Cancels++
	nativeHIDMetrics.Unlock()
}

func noteNativeHIDPoison() {
	nativeHIDMetrics.Lock()
	nativeHIDMetrics.value.PoisonEvents++
	nativeHIDMetrics.Unlock()
}

func noteNativeHIDCompatFallback() {
	nativeHIDMetrics.Lock()
	nativeHIDMetrics.value.CompatFallbacks++
	nativeHIDMetrics.Unlock()
}

func noteNativeHIDSharingFallback(mode string) {
	nativeHIDMetrics.Lock()
	nativeHIDMetrics.value.SharingFallbacks++
	nativeHIDMetrics.value.LastShareMode = mode
	nativeHIDMetrics.Unlock()
}

func noteNativeHIDShareMode(mode string) {
	nativeHIDMetrics.Lock()
	nativeHIDMetrics.value.LastShareMode = mode
	nativeHIDMetrics.Unlock()
}

func noteNativeHIDClose() {
	nativeHIDMetrics.Lock()
	nativeHIDMetrics.value.Closes++
	nativeHIDMetrics.Unlock()
}

func noteNativeHIDWrite(backend string, elapsed time.Duration, err error) {
	nativeHIDMetrics.Lock()
	defer nativeHIDMetrics.Unlock()
	m := &nativeHIDMetrics.value
	m.Writes++
	m.LastBackend = backend
	m.LastWrite = time.Now().UTC()
	m.LastWriteLatency = elapsed
	if elapsed > m.MaxWriteLatency {
		m.MaxWriteLatency = elapsed
	}
	if err == nil {
		m.SuccessfulWrites++
		m.LastError = ""
		return
	}
	m.FailedWrites++
	m.LastError = err.Error()
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "deadline") || strings.Contains(lower, "zeitlimit") || strings.Contains(lower, "timeout") {
		m.Timeouts++
	}
	if strings.Contains(lower, "unvollständig") || strings.Contains(lower, "partial") {
		m.PartialWrites++
	}
}

func NativeHIDTransportMetricsSnapshot() NativeHIDTransportMetrics {
	nativeHIDMetrics.Lock()
	defer nativeHIDMetrics.Unlock()
	return nativeHIDMetrics.value
}

func ResetNativeHIDTransportMetrics() {
	nativeHIDMetrics.Lock()
	nativeHIDMetrics.value = NativeHIDTransportMetrics{}
	nativeHIDMetrics.Unlock()
}

func FormatNativeHIDTransportMetrics(m NativeHIDTransportMetrics) string {
	last := "—"
	if !m.LastWrite.IsZero() {
		last = m.LastWrite.Local().Format("2006-01-02 15:04:05")
	}
	errText := strings.TrimSpace(m.LastError)
	if errText == "" {
		errText = "none"
	}
	return fmt.Sprintf("opens=%d openFail=%d writes=%d ok=%d fail=%d timeout=%d partial=%d cancel=%d poison=%d compat=%d shared=%d close=%d backend=%s share=%s last=%s latency=%s max=%s error=%s", m.Opens, m.OpenFailures, m.Writes, m.SuccessfulWrites, m.FailedWrites, m.Timeouts, m.PartialWrites, m.Cancels, m.PoisonEvents, m.CompatFallbacks, m.SharingFallbacks, m.Closes, m.LastBackend, m.LastShareMode, last, m.LastWriteLatency.Round(time.Millisecond), m.MaxWriteLatency.Round(time.Millisecond), errText)
}

func NativeHIDTransportSummary() string {
	return FormatNativeHIDTransportMetrics(NativeHIDTransportMetricsSnapshot())
}

func nativeModeReportsForModel(model string, selector byte) [][]byte {
	// Windows HID report-ID byte at index 0 + Logitech's seven-byte lg4ff payload.
	// OpenG27's real-world G27 lifecycle sends exactly the native switch report
	// F8 09 04 01 and then waits for C29B to re-enumerate. Keep that known-good
	// G27 path byte-for-byte compatible. The older two-step prelude remains for
	// the other classic models until their physical hardware certification says
	// otherwise.
	if IsG27Model(model) {
		return [][]byte{wheelengine.WithReportID(wheelengine.NativeSwitch(selector))}
	}
	return [][]byte{
		{0x00, 0xF8, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x00},
		wheelengine.WithReportID(wheelengine.NativeSwitch(selector)),
	}
}

// nativeModeReports is retained for historical parity tests that exercise the
// generic Logitech sequence directly. Product runtime code must use the
// model-aware helper above.
func nativeModeReports(selector byte) [][]byte {
	return nativeModeReportsForModel("", selector)
}

func hidSetOutputReport(h syscall.Handle, report []byte) error {
	if h == 0 || h == syscall.InvalidHandle {
		return errors.New("ungültiges HID-Handle")
	}
	if len(report) == 0 {
		return errors.New("leerer HID-Output-Report")
	}
	r1, _, callErr := pHidDSetOutputReportTransport.Call(uintptr(h), uintptr(unsafe.Pointer(&report[0])), uintptr(len(report)))
	if r1 == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return errors.New("HidD_SetOutputReport wurde von Windows abgelehnt")
	}
	return nil
}

func createOverlappedEvent() (syscall.Handle, error) {
	r1, _, callErr := pCreateEventW.Call(0, 1, 0, 0) // manual reset, initially non-signaled
	if r1 == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return 0, callErr
		}
		return 0, errors.New("CreateEventW fehlgeschlagen")
	}
	return syscall.Handle(r1), nil
}

func getOverlappedResult(h syscall.Handle, ov *syscall.Overlapped, transferred *uint32) error {
	r1, _, callErr := pGetOverlappedResult.Call(uintptr(h), uintptr(unsafe.Pointer(ov)), uintptr(unsafe.Pointer(transferred)), 0)
	if r1 == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return errors.New("GetOverlappedResult fehlgeschlagen")
	}
	return nil
}

// nativeHIDTransport is D4's single serialized HID writer. The preferred
// backend uses WriteFile + a real OVERLAPPED operation so CancelIoEx can target
// the exact pending write. Some classic HID stacks only accept
// HidD_SetOutputReport; a synchronous, definitely-not-started WriteFile error
// may switch to that compatibility backend. A pending/ambiguous write is never
// retried and always poisons the transport fail-closed.
type nativeHIDTransport struct {
	mu               sync.Mutex
	path             string
	handle           syscall.Handle
	closed           bool
	poisoned         bool
	timeout          time.Duration
	backend          string // writefile-overlapped | hidd-compat
	share            uint32
	shareMode        string // protected-reader | shared-rw | shared-g27-session
	borrowedG27      bool
	sharedGeneration uint64
}

func openNativeHIDHandleWithShare(path string, overlapped bool, share uint32) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		noteNativeHIDOpen(err)
		return syscall.InvalidHandle, err
	}
	var flags uint32
	if overlapped {
		flags = fileFlagOverlapped
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, share, nil, syscall.OPEN_EXISTING, flags, 0)
	if err != nil || h == syscall.InvalidHandle {
		if err == nil {
			err = errors.New("ungültiger HID-Handle")
		}
		wrapped := fmt.Errorf("HID-Output konnte nicht geöffnet werden: %w", err)
		noteNativeHIDOpen(wrapped)
		return syscall.InvalidHandle, wrapped
	}
	noteNativeHIDOpen(nil)
	return h, nil
}

func openNativeHIDHandle(path string, overlapped bool) (syscall.Handle, error) {
	return openNativeHIDHandleWithShare(path, overlapped, syscall.FILE_SHARE_READ)
}

func isNativeHIDSharingViolation(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == syscall.Errno(32) // ERROR_SHARING_VIOLATION
}

func knownNativeHIDWriterProcesses() []string {
	known := []string{"OpenG27.exe", "OpenG27FFB.exe", "LCore.exe"}
	running := make([]string, 0, len(known))
	for _, name := range known {
		if IsProcessRunning(name) {
			running = append(running, name)
		}
	}
	return running
}

// NativeHIDConflictProcesses exposes known external wheel writers for UI and
// diagnostics. The returned slice is detached from internal state. It is only
// advisory: an unknown process can still hold the HID interface exclusively.
func NativeHIDConflictProcesses() []string {
	running := knownNativeHIDWriterProcesses()
	return append([]string(nil), running...)
}

// openNativeHIDHandleAdaptive keeps the C8 single-writer guard as the first
// choice. Some Windows/DirectInput/HID clients, however, keep a cooperative
// GENERIC_READ|GENERIC_WRITE handle open with FILE_SHARE_WRITE. In that case
// our stricter share mask itself causes ERROR_SHARING_VIOLATION even though the
// other client explicitly permits a shared writer. OpenG27/HidSharp uses full
// read/write sharing for these classic Logitech interfaces. D6.1 therefore
// retries exactly once with the same cooperative sharing semantics. The
// central LogiMate OutputLease + OS-wide mutex still serialize every LogiMate
// writer, and the fallback is surfaced in metrics/UI instead of being hidden.
func openNativeHIDHandleAdaptive(path string, overlapped bool) (syscall.Handle, uint32, string, error) {
	strictShare := uint32(syscall.FILE_SHARE_READ)
	h, err := openNativeHIDHandleWithShare(path, overlapped, strictShare)
	if err == nil {
		noteNativeHIDShareMode("protected-reader")
		return h, strictShare, "protected-reader", nil
	}
	if !isNativeHIDSharingViolation(err) {
		return syscall.InvalidHandle, strictShare, "protected-reader", err
	}
	if conflicts := knownNativeHIDWriterProcesses(); len(conflicts) > 0 {
		return syscall.InvalidHandle, strictShare, "protected-reader", fmt.Errorf("HID-Output ist belegt und konkurrierende Wheel-Software läuft (%s). Beende diese Software vor LogiMate-FFB: %w", strings.Join(conflicts, ", "), err)
	}

	shared := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE)
	h, sharedErr := openNativeHIDHandleWithShare(path, overlapped, shared)
	if sharedErr == nil {
		noteNativeHIDSharingFallback("shared-rw")
		return h, shared, "shared-rw", nil
	}
	if isNativeHIDSharingViolation(sharedErr) {
		return syscall.InvalidHandle, shared, "shared-rw", fmt.Errorf("HID-Output ist exklusiv durch einen anderen Prozess belegt. Schließe andere Wheel-/FFB-Tools oder deren Geräteansicht und versuche es erneut: %w", sharedErr)
	}
	return syscall.InvalidHandle, shared, "shared-rw", errors.Join(err, sharedErr)
}

func openOpenG27CompatibleSwitchTransport(path string) (*nativeHIDTransport, error) {
	// HidSharp/OpenG27 opens the G27 read+write with full read/write sharing.
	// Match that lifecycle for the short-lived C294 -> C29B handshake. The
	// process-wide LogiMate output mutex still serializes our own writer, while
	// FILE_SHARE_WRITE avoids a false sharing violation caused by Windows/game
	// handles that OpenG27 itself tolerates.
	h, err := openNativeHIDHandleWithShare(path, true, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE)
	if err != nil {
		return nil, err
	}
	share := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE)
	noteNativeHIDShareMode("shared-rw")
	return &nativeHIDTransport{path: path, handle: h, timeout: openG27SwitchWriteTimeout, backend: "writefile-overlapped", share: share, shareMode: "shared-rw"}, nil
}

func openNativeHIDTransport(path string) (*nativeHIDTransport, error) {
	// G27 C29B: force one unified device session. Before Build 010 the input
	// reader and motor writer could own two independent HID handles; Windows can
	// report the second WriteFile as successful even when the command does not
	// affect the stream currently driving the native G27. OpenG27/HidSharp uses
	// one non-exclusive stream for both directions, so LogiMate now does the same.
	if devicePID(path) == pidG27 {
		ensureG27SharedReader([]string{path})
		if h, generation, ok := borrowG27SharedOutput(path); ok {
			noteNativeHIDShareMode("shared-g27-session")
			return &nativeHIDTransport{
				path: path, handle: h, timeout: defaultNativeWriteTimeout,
				backend: "g27-shared-overlapped", share: syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE,
				shareMode: "shared-g27-session", borrowedG27: true, sharedGeneration: generation,
			}, nil
		}
		return nil, errors.New("G27 Unified-HID-Session konnte nicht hergestellt werden; Direct-HID muss C29B mit Lese- und Schreibzugriff öffnen")
	}

	h, share, shareMode, err := openNativeHIDHandleAdaptive(path, true)
	if err != nil {
		// Opening with FILE_FLAG_OVERLAPPED itself can be unsupported on an old
		// HID stack. Falling back here is safe because no report was attempted.
		h, share, shareMode, err = openNativeHIDHandleAdaptive(path, false)
		if err != nil {
			return nil, err
		}
		return &nativeHIDTransport{path: path, handle: h, timeout: defaultNativeWriteTimeout, backend: "hidd-compat", share: share, shareMode: shareMode}, nil
	}
	return &nativeHIDTransport{path: path, handle: h, timeout: defaultNativeWriteTimeout, backend: "writefile-overlapped", share: share, shareMode: shareMode}, nil
}

func (t *nativeHIDTransport) requestCancelLocked(ov *syscall.Overlapped) {
	if t == nil || t.handle == 0 || t.handle == syscall.InvalidHandle {
		return
	}
	// Never cancel all I/O on a borrowed G27 handle: the same handle owns the
	// live input ReadFile. Ambiguous shared-output failures invalidate/close the
	// whole session through poisonAndCloseLocked instead.
	if t.borrowedG27 && ov == nil {
		return
	}
	var ptr uintptr
	if ov != nil {
		ptr = uintptr(unsafe.Pointer(ov))
	}
	_, _, _ = pCancelIoEx.Call(uintptr(t.handle), ptr)
	noteNativeHIDCancel()
}

func (t *nativeHIDTransport) poisonAndCloseLocked() {
	if t == nil {
		return
	}
	if !t.poisoned {
		noteNativeHIDPoison()
	}
	t.poisoned = true
	if t.borrowedG27 {
		h := t.handle
		t.handle = syscall.InvalidHandle
		t.closed = true
		invalidateG27SharedOutput(t.path, h, t.sharedGeneration, errors.New("ambiguous G27 shared-output I/O"))
		return
	}
	if t.handle != 0 && t.handle != syscall.InvalidHandle {
		t.requestCancelLocked(nil)
		_ = syscall.CloseHandle(t.handle)
		t.handle = syscall.InvalidHandle
	}
	t.closed = true
}

// failClosedAmbiguousLocked is used whenever Windows cannot prove the exact
// completion of an output report. Partial writes and completion-query failures
// are safety-significant: the handle is never reused even if the caller later
// retries the higher-level operation.
func (t *nativeHIDTransport) failClosedAmbiguousLocked(err error) error {
	if err == nil {
		err = errors.New("unbestimmter HID-I/O-Zustand")
	}
	t.poisonAndCloseLocked()
	return err
}

func unsupportedOverlappedWrite(err error) bool {
	if err == nil {
		return false
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	// Only synchronous failures that prove no overlapped request was queued may
	// enter compatibility mode.
	return errno == syscall.Errno(1) || // ERROR_INVALID_FUNCTION
		errno == syscall.Errno(50) || // ERROR_NOT_SUPPORTED
		errno == syscall.Errno(87) // ERROR_INVALID_PARAMETER
}

func (t *nativeHIDTransport) reopenHidDCompatibilityLocked() error {
	if t.handle != 0 && t.handle != syscall.InvalidHandle {
		_ = syscall.CloseHandle(t.handle)
		t.handle = syscall.InvalidHandle
	}
	share := t.share
	if share == 0 {
		share = syscall.FILE_SHARE_READ
	}
	h, err := openNativeHIDHandleWithShare(t.path, false, share)
	if err != nil {
		t.poisoned = true
		t.closed = true
		return err
	}
	t.handle = h
	t.backend = "hidd-compat"
	noteNativeHIDCompatFallback()
	return nil
}

func contextWaitMilliseconds(ctx context.Context, fallback time.Duration) uint32 {
	wait := fallback
	if deadline, ok := ctx.Deadline(); ok {
		wait = time.Until(deadline)
	}
	if wait <= 0 {
		return 0
	}
	ms := wait.Milliseconds()
	if ms <= 0 {
		ms = 1
	}
	if ms > int64(^uint32(0)-1) {
		return ^uint32(0) - 1
	}
	return uint32(ms)
}

func (t *nativeHIDTransport) writeOverlappedLocked(ctx context.Context, payload []byte) error {
	event, err := createOverlappedEvent()
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(event)
	ov := syscall.Overlapped{HEvent: event}
	var written uint32
	err = syscall.WriteFile(t.handle, payload, &written, &ov)
	if err == nil {
		if written != 0 && int(written) != len(payload) {
			return t.failClosedAmbiguousLocked(fmt.Errorf("unvollständiger HID-Write: %d/%d Bytes; Transport gesperrt", written, len(payload)))
		}
		return nil
	}
	if err != syscall.ERROR_IO_PENDING {
		return err
	}

	waitResult, waitErr := syscall.WaitForSingleObject(event, contextWaitMilliseconds(ctx, t.timeout))
	if waitErr != nil || waitResult == waitFailed {
		t.requestCancelLocked(&ov)
		t.poisonAndCloseLocked()
		if waitErr != nil {
			return fmt.Errorf("WaitForSingleObject für HID-Write fehlgeschlagen; Transport gesperrt: %w", waitErr)
		}
		return errors.New("WaitForSingleObject für HID-Write fehlgeschlagen; Transport gesperrt")
	}
	if waitResult == syscall.WAIT_OBJECT_0 {
		if err := getOverlappedResult(t.handle, &ov, &written); err != nil {
			return t.failClosedAmbiguousLocked(fmt.Errorf("HID-Write Completion konnte nicht bestätigt werden; Transport gesperrt: %w", err))
		}
		if int(written) != len(payload) {
			return t.failClosedAmbiguousLocked(fmt.Errorf("unvollständiger HID-Write: %d/%d Bytes; Transport gesperrt", written, len(payload)))
		}
		return nil
	}
	if waitResult != syscall.WAIT_TIMEOUT {
		t.requestCancelLocked(&ov)
		t.poisonAndCloseLocked()
		return fmt.Errorf("unerwarteter HID-Wait-Status 0x%X; Transport gesperrt", waitResult)
	}

	// Deadline: cancel this exact operation. Even if cancellation subsequently
	// reports success we cannot prove whether the command reached hardware, so
	// this handle is never reused.
	t.requestCancelLocked(&ov)
	graceResult, _ := syscall.WaitForSingleObject(event, 250)
	var completionErr error
	if graceResult == syscall.WAIT_OBJECT_0 {
		completionErr = getOverlappedResult(t.handle, &ov, &written)
	}
	t.poisonAndCloseLocked()
	if completionErr != nil {
		return fmt.Errorf("HID-Write überschritt die Deadline; CancelIoEx abgeschlossen, Ergebnis unbestimmt; Transport gesperrt: %w", completionErr)
	}
	cause := ctx.Err()
	if cause == nil {
		cause = context.DeadlineExceeded
	}
	return fmt.Errorf("HID-Write überschritt die Deadline; Transport wurde fail-closed gesperrt: %w", cause)
}

func (t *nativeHIDTransport) writeHidDCompatLocked(ctx context.Context, payload []byte) error {
	done := make(chan error, 1)
	h := t.handle
	go func() { done <- hidSetOutputReport(h, payload) }()
	wait := t.timeout
	if deadline, ok := ctx.Deadline(); ok {
		wait = time.Until(deadline)
	}
	if wait <= 0 {
		wait = time.Millisecond
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.requestCancelLocked(nil)
		t.poisonAndCloseLocked()
		return fmt.Errorf("HidD-Kompatibilitätswrite überschritt die Deadline; Transport gesperrt: %w", ctx.Err())
	case <-timer.C:
		t.requestCancelLocked(nil)
		t.poisonAndCloseLocked()
		return errors.New("HidD-Kompatibilitätswrite überschritt die Deadline; Transport gesperrt")
	}
}

func (t *nativeHIDTransport) WriteReport(report []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), t.timeout)
	defer cancel()
	return t.WriteReportContext(ctx, report)
}

func (t *nativeHIDTransport) WriteReportContext(ctx context.Context, report []byte) error {
	start := time.Now()
	err := t.writeReportContext(ctx, report)
	backend := ""
	if t != nil {
		t.mu.Lock()
		backend = t.backend
		t.mu.Unlock()
	}
	noteNativeHIDWrite(backend, time.Since(start), err)
	return err
}

func (t *nativeHIDTransport) writeReportContext(ctx context.Context, report []byte) error {
	if t == nil {
		return errors.New("nil HID transport")
	}
	if len(report) == 0 {
		return errors.New("leerer HID-Output-Report")
	}
	payload := append([]byte(nil), report...)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.handle == 0 || t.handle == syscall.InvalidHandle {
		if t.poisoned {
			return errors.New("HID-Transport ist nach einem unbestätigten I/O-Abbruch gesperrt")
		}
		return errors.New("HID-Transport ist geschlossen")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if t.borrowedG27 {
		if !g27SharedOutputHealthy(t.path, t.handle, t.sharedGeneration) {
			t.closed = true
			return errors.New("G27 Shared-HID-Session ist nicht mehr aktuell; neu verbinden und erneut testen")
		}
		g27SharedOutputWriteMu.Lock()
		defer g27SharedOutputWriteMu.Unlock()
		if !g27SharedOutputHealthy(t.path, t.handle, t.sharedGeneration) {
			t.closed = true
			return errors.New("G27 Shared-HID-Session wechselte vor dem Output-Write")
		}
		if t.backend == "g27-shared-hidd" {
			return t.writeHidDCompatLocked(ctx, payload)
		}
		err := t.writeOverlappedLocked(ctx, payload)
		if err == nil {
			return nil
		}
		if unsupportedOverlappedWrite(err) && !t.closed && !t.poisoned {
			// The same shared handle remains authoritative; only the transfer
			// mechanism changes to HID SET_REPORT. No second device handle is opened.
			t.backend = "g27-shared-hidd"
			noteNativeHIDCompatFallback()
			return t.writeHidDCompatLocked(ctx, payload)
		}
		return err
	}

	if t.backend == "hidd-compat" {
		// This compatibility backend cannot target a specific OVERLAPPED
		// operation, but it is still deadline-bounded and poisons/closes the
		// handle on an ambiguous timeout.
		return t.writeHidDCompatLocked(ctx, payload)
	}

	err := t.writeOverlappedLocked(ctx, payload)
	if err == nil {
		return nil
	}
	if unsupportedOverlappedWrite(err) && !t.closed && !t.poisoned {
		if reopenErr := t.reopenHidDCompatibilityLocked(); reopenErr != nil {
			return errors.Join(fmt.Errorf("OVERLAPPED WriteFile nicht unterstützt: %w", err), reopenErr)
		}
		return t.writeHidDCompatLocked(ctx, payload)
	}
	return err
}

func (t *nativeHIDTransport) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	if t.borrowedG27 {
		// Borrowed output transports release only their wrapper. The Direct-HID
		// reader owns the physical handle for the lifetime of the C29B session.
		t.closed = true
		t.handle = syscall.InvalidHandle
		return nil
	}
	t.requestCancelLocked(nil)
	t.closed = true
	if t.handle != 0 && t.handle != syscall.InvalidHandle {
		err := syscall.CloseHandle(t.handle)
		t.handle = syscall.InvalidHandle
		noteNativeHIDClose()
		return err
	}
	noteNativeHIDClose()
	return nil
}

func (t *nativeHIDTransport) Healthy() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.poisoned || t.handle == 0 || t.handle == syscall.InvalidHandle {
		return false
	}
	if t.borrowedG27 {
		return g27SharedOutputHealthy(t.path, t.handle, t.sharedGeneration)
	}
	return true
}

func (t *nativeHIDTransport) Backend() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.backend
}

func (t *nativeHIDTransport) ShareMode() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.shareMode
}

func writeNativeReports(path string, reports ...[]byte) error {
	tr, err := openNativeHIDTransport(path)
	if err != nil {
		return err
	}
	defer tr.Close()
	for i, report := range reports {
		if err := tr.WriteReport(report); err != nil {
			return fmt.Errorf("HID-Report %d/%d: %w", i+1, len(reports), err)
		}
		if i+1 < len(reports) {
			time.Sleep(12 * time.Millisecond)
		}
	}
	return nil
}

func transportErrorText(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}
