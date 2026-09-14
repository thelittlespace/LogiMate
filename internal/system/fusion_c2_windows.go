//go:build windows

package system

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/thelittlespace/LogiMate/internal/wheelengine"
)

// ProtocolShadowComparison compares LogiMate's current production builder with
// the archived C2 protocol reference. C2 is intentionally read-only/shadow mode:
// these results never choose which bytes are sent to hardware.
type ProtocolShadowComparison struct {
	Name      string
	Match     bool
	Current   []byte
	Reference []byte
	Note      string
}

func cloneBytes(b []byte) []byte { return append([]byte(nil), b...) }

func protocolCompare(name string, current, reference []byte, note string) ProtocolShadowComparison {
	return ProtocolShadowComparison{
		Name:      name,
		Match:     bytes.Equal(current, reference),
		Current:   cloneBytes(current),
		Reference: cloneBytes(reference),
		Note:      note,
	}
}

// FusionC2ProtocolComparisons returns deterministic, hardware-free protocol
// comparisons. Known differences are information, not test failures: the whole
// point of C2 is to expose them before C3 is allowed to route the ported bytes
// to a live wheel.
func FusionC2ProtocolComparisons() []ProtocolShadowComparison {
	var out []ProtocolShadowComparison

	rangeCurrent, _ := BuildClassicRangeReport(900)
	out = append(out, protocolCompare(
		"Rotation 900°",
		rangeCurrent,
		wheelengine.WithReportID(wheelengine.SetRange(900)),
		"Soll bereits bytegleich sein.",
	))

	// LogiMate uses a two-report guarded compatibility->native sequence. The archived reference
	// defines the model-specific switch report itself; compare that second report
	// only and keep LogiMate's extra preamble as an intentional implementation
	// detail until the physical C294 matrix is revalidated.
	nativeReports := nativeModeReports(0x04)
	var nativeCurrent []byte
	if len(nativeReports) > 1 {
		nativeCurrent = nativeReports[1]
	}
	out = append(out, protocolCompare(
		"G27 native switch",
		nativeCurrent,
		wheelengine.WithReportID(wheelengine.NativeSwitch(0x04)),
		"LogiMate sendet davor zusätzlich seinen bestehenden F8/0A-Preamble; C2 ändert diesen Live-Pfad nicht.",
	))

	ledCurrent, _ := BuildG27LEDReport(0x1F)
	out = append(out, protocolCompare(
		"G27 LEDs 0x1F",
		ledCurrent,
		wheelengine.WithReportID(wheelengine.SetLeds(0x1F)),
		"Kanonischer lg4ff/OpenG27 LED-Report; Build 010 wurde am echten G27 über den sichtbaren LED-Ping bestätigt.",
	))

	constantCurrent, _ := BuildNativeConstantForceReport(0)
	out = append(out, protocolCompare(
		"Constant Force neutral/start",
		constantCurrent,
		wheelengine.WithReportID(wheelengine.ConstantForce(0x80)),
		"D6.2 nutzt auch im manuellen Live-Test den kanonischen wheelengine/OpenG27-Konstantkraftreport.",
	))

	// Compare the autocenter setup packet at a byte-equivalent slope/clip. The
	// C2 reference additionally requires SpringEnable as a second report; that
	// sequence difference is listed explicitly below.
	autoCurrent, _ := BuildClassicAutocenterReport(30, 0)
	if len(autoCurrent) >= 8 {
		out = append(out, protocolCompare(
			"Autocenter SpringSet",
			autoCurrent,
			wheelengine.WithReportID(wheelengine.SpringSet(autoCurrent[3], autoCurrent[5])),
			"Byte-Layout des Set-Pakets; D6.2 sendet anschließend ebenfalls das kanonische SpringEnable-Paket.",
		))
	}
	autoEnable := BuildClassicAutocenterEnableReport()
	out = append(out, protocolCompare(
		"Autocenter SpringEnable sequence",
		autoEnable,
		wheelengine.WithReportID(wheelengine.SpringEnable()),
		"D6.2 schließt die bekannte Live-Test-Lücke: SpringSet wird jetzt von SpringEnable gefolgt.",
	))

	// Condition effects use the hardware-confirmed lg4ff slot map:
	// Spring=1, Damper=2, Friction=3. Build 011 confirmed these paths on a real G27.
	damperCurrent, _ := BuildClassicDamperReport(20, false)
	if len(damperCurrent) == 8 {
		out = append(out, protocolCompare(
			"Damper slot-2 layout",
			damperCurrent,
			wheelengine.WithReportID(wheelengine.Damper(damperCurrent[3], damperCurrent[7])),
			"Hardwarebestätigter Slot 2; Prozent→Koeffizient bleibt LogiMate-Safety-Policy.",
		))
	}
	frictionCurrent, _ := BuildClassicFrictionReport(20, false)
	if len(frictionCurrent) == 8 {
		out = append(out, protocolCompare(
			"Friction slot-3 layout",
			frictionCurrent,
			wheelengine.WithReportID(wheelengine.Friction(frictionCurrent[3], frictionCurrent[5])),
			"Hardwarebestätigter Slot 3; Friction nutzt den vollen Koeffizienten-Bytebereich.",
		))
	}

	damperStop, _ := BuildClassicEffectStopReport(2)
	out = append(out, protocolCompare(
		"Damper slot-2 stop",
		damperStop,
		wheelengine.WithReportID(wheelengine.DamperOff()),
		"Hardwarebestätigter Slot-2-Stop.",
	))
	frictionStop, _ := BuildClassicEffectStopReport(3)
	out = append(out, protocolCompare(
		"Friction slot-3 stop",
		frictionStop,
		wheelengine.WithReportID(wheelengine.FrictionOff()),
		"Hardwarebestätigter Slot-3-Stop.",
	))

	return out
}

func hexReport(b []byte) string {
	if len(b) == 0 {
		return "—"
	}
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = fmt.Sprintf("%02X", v)
	}
	return strings.Join(parts, " ")
}

func FusionC2ProtocolShadowSummary() string {
	var b strings.Builder
	comparisons := FusionC2ProtocolComparisons()
	matches := 0
	for _, c := range comparisons {
		if c.Match {
			matches++
		}
		state := "DIFF"
		if c.Match {
			state = "MATCH"
		}
		fmt.Fprintf(&b, "- %s: %s\r\n  LogiMate: %s\r\n  Referenz: %s\r\n", c.Name, state, hexReport(c.Current), hexReport(c.Reference))
		if strings.TrimSpace(c.Note) != "" {
			fmt.Fprintf(&b, "  Hinweis: %s\r\n", c.Note)
		}
	}
	fmt.Fprintf(&b, "C2 Shadow: %d/%d byte-/layoutgleich. Unterschiede werden in C2 NICHT automatisch an Hardware geroutet.", matches, len(comparisons))
	return b.String()
}
