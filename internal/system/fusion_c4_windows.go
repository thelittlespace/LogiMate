//go:build windows

package system

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// fusionC4Parity is a read-only shadow observer. It never changes the input
// state consumed by LogiMate; it only checks the historical parser reference against
// LogiMate's current raw-axis extraction on the exact same HID report.
var fusionC4Parity = struct {
	sync.RWMutex
	Reports      uint64
	Matches      uint64
	Mismatches   uint64
	ParseErrors  uint64
	LastAt       time.Time
	LastMatch    bool
	LastDetail   string
	LastUpstream wheelengine.NativeInputReport
}{LastMatch: true}

func observeFusionC4RawFields(report []byte, steering uint16, throttle, brake, clutch byte) {
	upstream, err := wheelengine.ParseNativeInputReport(wheelengine.ModelG27, report)
	fusionC4Parity.Lock()
	defer fusionC4Parity.Unlock()
	fusionC4Parity.Reports++
	fusionC4Parity.LastAt = time.Now()
	if err != nil {
		fusionC4Parity.ParseErrors++
		fusionC4Parity.LastMatch = false
		fusionC4Parity.LastDetail = "Native parser: " + err.Error()
		return
	}
	fusionC4Parity.LastUpstream = upstream
	match := upstream.Steering == steering && upstream.ThrottleRaw == throttle && upstream.BrakeRaw == brake && upstream.ClutchRaw == clutch
	fusionC4Parity.LastMatch = match
	if match {
		fusionC4Parity.Matches++
		fusionC4Parity.LastDetail = fmt.Sprintf("MATCH steer=%04X throttle=%02X brake=%02X clutch=%02X buttons=%016X", upstream.Steering, upstream.ThrottleRaw, upstream.BrakeRaw, upstream.ClutchRaw, upstream.Buttons)
	} else {
		fusionC4Parity.Mismatches++
		fusionC4Parity.LastDetail = fmt.Sprintf("DIFF Reference=%04X/%02X/%02X/%02X LogiMate=%04X/%02X/%02X/%02X", upstream.Steering, upstream.ThrottleRaw, upstream.BrakeRaw, upstream.ClutchRaw, steering, throttle, brake, clutch)
	}
}

func FusionC4ParserSummary() string {
	fusionC4Parity.RLock()
	p := struct {
		Reports, Matches, Mismatches, ParseErrors uint64
		LastAt                                    time.Time
		LastMatch                                 bool
		LastDetail                                string
	}{fusionC4Parity.Reports, fusionC4Parity.Matches, fusionC4Parity.Mismatches, fusionC4Parity.ParseErrors, fusionC4Parity.LastAt, fusionC4Parity.LastMatch, fusionC4Parity.LastDetail}
	fusionC4Parity.RUnlock()
	if p.Reports == 0 {
		return "Noch kein nativer G27-HID-Report im C4-Shadow beobachtet."
	}
	age := time.Since(p.LastAt).Round(time.Millisecond)
	return fmt.Sprintf("Reports: %d · MATCH: %d · DIFF: %d · Parse errors: %d\r\nLetzter Report vor: %s · Match: %v\r\n%s", p.Reports, p.Matches, p.Mismatches, p.ParseErrors, age, p.LastMatch, p.LastDetail)
}

// FusionC4DeviceLifecycleSummary compares the historical G27 lifecycle contract with
// LogiMate's selected-wheel lifecycle. LogiMate keeps its stable physical ID as
// the authority and never adopts legacy first-device selection behavior.
func FusionC4DeviceLifecycleSummary(s State) string {
	w, ok := SelectedWheel(s)
	if !ok || !IsG27Model(w.Model) {
		return "Kein bestätigtes G27 ausgewählt; C4 Device-Lifecycle bleibt inaktiv."
	}
	native := false
	compat := false
	for _, id := range append(append([]string{}, w.InterfaceIDs...), w.InstanceID) {
		u := strings.ToUpper(id)
		native = native || strings.Contains(u, "PID_C29B")
		compat = compat || strings.Contains(u, "PID_C294")
	}
	pathState := "kein Direct-HID-Handle"
	if G27DirectHIDConnected() {
		pathState = "shared Direct-HID verbunden"
	}
	switch {
	case native:
		return fmt.Sprintf("NATIVE: bereits C29B -> direkt öffnen, kein USB-Reset. StableID=%s · %s.", redactIdentifier(redactIdentifier(w.ID)), pathState)
	case compat:
		return fmt.Sprintf("NATIVE: C294 -> modellbestätigter Native-Switch erforderlich, danach C29B per StableID=%s neu korrelieren.", redactIdentifier(w.ID))
	default:
		return fmt.Sprintf("G27 ausgewählt (%s), aber aktuelle PID nicht aus Interface-IDs ableitbar · %s.", redactIdentifier(redactIdentifier(w.ID)), pathState)
	}
}
