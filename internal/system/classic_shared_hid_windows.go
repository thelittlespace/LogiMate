//go:build windows

package system

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// Read-only Direct-HID reader for the G25 and Driving Force GT. G27 keeps its
// dedicated parser because its detailed shifter map has already been validated
// against the archived parser reference. The classic family shares 16-bit steering and inverted 8-bit
// pedal channels; model-specific extras are decoded conservatively.
var classicShared = struct {
	sync.RWMutex
	model, pid, path string
	handle           syscall.Handle
	generation       uint64
	latest           JoyState
	raw              []byte
	lastRead         time.Time
	lastError        string
	lastErrorKind    string
	lastInputError   string
	lastMalformed    time.Time
	malformedCount   int
	nextOpen         time.Time
	failures         int
	reconnects       int
	everOpened       bool
	rateStart        time.Time
	rateCount        int
	rateHz           float64
}{handle: syscall.InvalidHandle}

func parseClassicNativeReport(model string, report []byte) (JoyState, bool) {
	modelID := ClassifyWheelModel(model)
	raw, err := wheelengine.ParseNativeInputReport(modelID, report)
	if err != nil {
		return JoyState{}, false
	}
	pedal := func(v byte) uint32 { return uint32(255-v) * 257 }
	j := JoyState{
		Found: true, X: uint32(raw.Steering), XMin: 0, XMax: 65535,
		Y: pedal(raw.ThrottleRaw), Z: pedal(raw.BrakeRaw), YMin: 0, YMax: 65535, ZMin: 0, ZMax: 65535,
		Buttons: raw.Buttons, POV: 0xFFFF, DPad: raw.DPad, Gear: raw.Gear,
		PaddleLeft: raw.PaddleLeft, PaddleRight: raw.PaddleRight,
		WheelButtons: raw.WheelButtons, ShifterButtons: raw.ShifterButtons,
		ConnectionState: "Connected", Selection: "LogiMate Direct HID · Generic HID",
		InputSource: "direct-hid", LayoutID: raw.LayoutID, SampleValid: true, SampleAt: time.Now(),
	}
	if raw.DPad > 0 {
		j.POV = uint32((raw.DPad - 1) * 4500)
	}
	switch modelID {
	case WheelModelG25:
		j.Name = "Logitech G25 · Direct HID"
		j.NumAxes, j.NumButtons = 4, 20
		j.R, j.RMin, j.RMax = pedal(raw.ClutchRaw), 0, 65535
		j.NativeControls = true
	case WheelModelDFGT:
		j.Name = "Logitech Driving Force GT · Direct HID"
		j.NumAxes, j.NumButtons = 3, 24
		j.RMin, j.RMax = 0, 65535
	default:
		return JoyState{}, false
	}
	return j, true
}
func noteClassicOpenFailure(err error) {
	classicShared.Lock()
	defer classicShared.Unlock()
	classicShared.failures++
	shift := classicShared.failures - 1
	if shift > 4 {
		shift = 4
	}
	classicShared.nextOpen = time.Now().Add(250 * time.Millisecond * time.Duration(1<<shift))
	if err != nil {
		classicShared.lastError = "CreateFile: " + err.Error()
		classicShared.lastErrorKind = "open"
		classicShared.lastInputError = classicShared.lastError
	}
}

func ensureClassicSharedReader(model string, paths []string) {
	pid, _, _, ok := nativeModeSpec(model)
	if !ok || pid == pidG27 {
		return
	}
	matches := rawPathsForPID(paths, pid)
	if len(matches) == 0 {
		matches = rawPathsForPID(EnumerateRawInputLogitechWheels(), pid)
	}
	if len(matches) != 1 {
		return
	}
	path := matches[0]
	classicShared.RLock()
	already := strings.EqualFold(classicShared.path, path) && strings.EqualFold(classicShared.model, model) && directHIDHandleConnected(classicShared.handle, classicShared.path)
	next := classicShared.nextOpen
	classicShared.RUnlock()
	if already || (!next.IsZero() && time.Now().Before(next)) {
		return
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		noteClassicOpenFailure(err)
		return
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil || h == syscall.InvalidHandle {
		if err == nil {
			err = errors.New("ungültiger Direct-HID-Handle")
		}
		noteClassicOpenFailure(err)
		return
	}
	classicShared.Lock()
	old := classicShared.handle
	if classicShared.everOpened {
		classicShared.reconnects++
	} else {
		classicShared.everOpened = true
	}
	classicShared.generation++
	gen := classicShared.generation
	classicShared.model, classicShared.pid, classicShared.path = model, pid, path
	classicShared.handle = h
	classicShared.latest = JoyState{}
	classicShared.raw = nil
	classicShared.lastRead = time.Time{}
	classicShared.lastError = ""
	classicShared.lastErrorKind = ""
	classicShared.lastMalformed = time.Time{}
	classicShared.malformedCount = 0
	classicShared.nextOpen = time.Time{}
	classicShared.failures = 0
	classicShared.rateStart = time.Now()
	classicShared.rateCount = 0
	classicShared.rateHz = 0
	classicShared.Unlock()
	if old != 0 && old != syscall.InvalidHandle {
		_ = syscall.CloseHandle(old)
	}
	go classicSharedReadLoop(h, path, model, gen)
}

func classicSharedReadLoop(h syscall.Handle, path, model string, generation uint64) {
	buf := make([]byte, 64)
	for {
		var n uint32
		err := syscall.ReadFile(h, buf, &n, nil)
		if err != nil || n == 0 {
			if err == nil {
				err = errors.New("Direct-HID-Lesung lieferte 0 Bytes")
			}
			classicShared.Lock()
			if classicShared.generation == generation && classicShared.handle == h {
				classicShared.handle = syscall.InvalidHandle
				classicShared.path = ""
				classicShared.lastError = err.Error()
				classicShared.lastErrorKind = "read"
				classicShared.lastInputError = "ReadFile: " + err.Error()
				classicShared.failures++
				shift := classicShared.failures - 1
				if shift > 4 {
					shift = 4
				}
				classicShared.nextOpen = time.Now().Add(250 * time.Millisecond * time.Duration(1<<shift))
			}
			classicShared.Unlock()
			_ = syscall.CloseHandle(h)
			return
		}
		raw := append([]byte(nil), buf[:n]...)
		j, ok := parseClassicNativeReport(model, raw)
		if !ok {
			classicShared.Lock()
			if classicShared.generation == generation && classicShared.handle == h {
				classicShared.malformedCount++
				classicShared.lastMalformed = time.Now()
				classicShared.lastInputError = fmt.Sprintf("ungültiger Direct-HID-Report (%d Bytes)", len(raw))
			}
			classicShared.Unlock()
			continue
		}
		now := time.Now()
		j.Connected = []string{"Direct HID: " + path}
		j.ConnectionState = "Connected"
		j.RawReport = raw
		classicShared.Lock()
		if classicShared.generation == generation && classicShared.handle == h {
			classicShared.rateCount++
			if d := now.Sub(classicShared.rateStart); d >= 500*time.Millisecond {
				classicShared.rateHz = float64(classicShared.rateCount) / d.Seconds()
				classicShared.rateStart = now
				classicShared.rateCount = 0
			}
			j.ReportRateHz = classicShared.rateHz
			j.LastReportAge = 0
			j.Reconnects = classicShared.reconnects
			j.LastInputError = classicShared.lastInputError
			classicShared.latest = j
			classicShared.raw = raw
			classicShared.lastRead = now
			classicShared.lastMalformed = time.Time{}
			classicShared.malformedCount = 0
		}
		classicShared.Unlock()
	}
}

func readClassicSharedInput(model string, paths []string) JoyState {
	ensureClassicSharedReader(model, paths)
	classicShared.RLock()
	j := classicShared.latest
	h := classicShared.handle
	path := classicShared.path
	last := classicShared.lastRead
	errtxt := classicShared.lastError
	kind := classicShared.lastErrorKind
	lastInputErr := classicShared.lastInputError
	lastMalformed := classicShared.lastMalformed
	malformedCount := classicShared.malformedCount
	next := classicShared.nextOpen
	rec := classicShared.reconnects
	hz := classicShared.rateHz
	raw := append([]byte(nil), classicShared.raw...)
	classicShared.RUnlock()
	connected := directHIDHandleConnected(h, path)
	if connected && j.Found {
		j.ReportRateHz = hz
		j.Reconnects = rec
		j.RawReport = raw
		if !last.IsZero() {
			j.LastReportAge = time.Since(last)
		}
		if malformedCount > 0 && !lastMalformed.IsZero() && (last.IsZero() || lastMalformed.After(last)) {
			j.ConnectionState = "Malformed"
			j.Selection += fmt.Sprintf(" · ungültiger HID-Report (%d)", malformedCount)
		} else if j.LastReportAge > 3*time.Second {
			j.ConnectionState = "Idle"
		}
		return j
	}
	if connected {
		state := "Connected"
		selection := "LogiMate Direct HID · wartet auf ersten Report"
		if malformedCount > 0 {
			state = "Malformed"
			selection = fmt.Sprintf("LogiMate Direct HID · %d ungültige Reports vor erstem gültigen Zustand", malformedCount)
		}
		return JoyState{Found: false, Name: fmt.Sprintf("%s · Direct HID", compactInputModelName(model)), ConnectionState: state, Selection: selection, X: 32768, XMin: 0, XMax: 65535, YMin: 0, YMax: 65535, ZMin: 0, ZMax: 65535, RMin: 0, RMax: 65535, POV: 0xFFFF, Reconnects: rec, ReportRateHz: hz, LastInputError: lastInputErr}
	}
	if errtxt != "" {
		state := "Open error"
		if kind == "read" {
			state = "Read error"
		}
		if !next.IsZero() && time.Now().Before(next) {
			state = "Reconnecting"
		}
		return JoyState{ConnectionState: state, Error: "Direct HID: " + errtxt, LastInputError: lastInputErr, Reconnects: rec}
	}
	return JoyState{ConnectionState: "Reconnecting", Error: "Direct HID wartet auf den nativen Logitech-HID-Pfad.", Reconnects: rec}
}

func compactInputModelName(model string) string {
	switch {
	case IsG25Model(model):
		return "Logitech G25"
	case IsDFGTModel(model):
		return "Logitech Driving Force GT"
	case IsG27Model(model):
		return "Logitech G27"
	}
	return "Logitech Lenkrad"
}

func closeClassicSharedInput() {
	classicShared.Lock()
	h := classicShared.handle
	classicShared.generation++
	classicShared.handle = syscall.InvalidHandle
	classicShared.path = ""
	classicShared.model = ""
	classicShared.pid = ""
	classicShared.latest = JoyState{}
	classicShared.raw = nil
	classicShared.lastRead = time.Time{}
	classicShared.lastError = ""
	classicShared.lastErrorKind = ""
	classicShared.lastMalformed = time.Time{}
	classicShared.malformedCount = 0
	classicShared.nextOpen = time.Time{}
	classicShared.failures = 0
	classicShared.everOpened = false
	classicShared.Unlock()
	if h != 0 && h != syscall.InvalidHandle {
		_ = syscall.CloseHandle(h)
	}
}
