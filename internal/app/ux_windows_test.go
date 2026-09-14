//go:build windows

package app

import (
	"strings"
	"testing"

	"github.com/thelittlespace/LogiMate/internal/system"
)

func selectedG27State() system.State {
	w := system.WheelDevice{ID: "USB\\VID_046D&PID_C29B\\TEST-A", Name: "Logitech G27", Model: "Logitech G27", Mode: "Generic HID", Supported: true, ModelConfirmed: true, PnPVerified: true}
	return system.State{
		Wheels:          []system.WheelDevice{w},
		SelectedWheelID: w.ID,
		WheelModel:      "Logitech G27",
		ActiveMode:      "Generic HID",
	}
}

func TestOverviewPrimaryActionGuidesSafely(t *testing.T) {
	cases := []struct {
		name  string
		state system.State
		want  string
	}{
		{"detection error", system.State{DeviceDetectionError: "boom"}, "Erkennung neu starten"},
		{"no wheel", system.State{}, "Geräte verwalten"},
		{"needs selection", system.State{Wheels: []system.WheelDevice{{ID: "A", Name: "G27", Model: "Logitech G27", Supported: true}}}, "Geräte verwalten"},
		{"legacy needs backup", func() system.State {
			s := selectedG27State()
			s.ActiveMode = "Logitech Legacy"
			s.LegacyDrivers = []system.LegacyDriver{{PublishedName: "oem1.inf"}}
			return s
		}(), "Treiber sichern"},
		{"modern native no external dependency", selectedG27State(), "Hardware testen"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := overviewPrimaryAction(tc.state); got != tc.want {
				t.Fatalf("overviewPrimaryAction()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestAboutPageContainsIdentityProjectsAndSupport(t *testing.T) {
	text := aboutPageText()
	for _, want := range []string{"Markus Kleine", "LogiMate", "Indicana Tools", "StromPilot", "Green-ITea", "TheLittleCyclist", aboutPayPalEmail, "Sharing is caring"} {
		if !strings.Contains(text, want) {
			t.Fatalf("about page missing %q", want)
		}
	}
	if !strings.HasPrefix(aboutPayPalURL, "https://") {
		t.Fatalf("PayPal URL must use https: %q", aboutPayPalURL)
	}
}

func TestSettingsAndAboutUseTallContentArea(t *testing.T) {
	old := currentPage
	defer func() { currentPage = old }()
	currentPage = pageSystem
	if got := contentTopForPage(); got != 101 {
		t.Fatalf("system content top=%d want 101", got)
	}
	currentPage = pageSettings
	if got := contentTopForPage(); got != 101 {
		t.Fatalf("settings content top=%d want 101", got)
	}
	currentPage = pageAbout
	if got := contentTopForPage(); got != 101 {
		t.Fatalf("about content top=%d want 101", got)
	}
}

func TestModeChangeGuidanceExplainsWhyBlocked(t *testing.T) {
	s := selectedG27State()
	s.Wheels = append(s.Wheels, system.WheelDevice{ID: "B", Name: "Logitech G27", Model: "Logitech G27", Supported: true})
	if got := modeChangeGuidance(s); !strings.Contains(got, "Mehrere") {
		t.Fatalf("expected multi-wheel explanation, got %q", got)
	}
}

func TestWheelAttentionKeepsMultiDeviceGuidanceSeparateFromDashboard(t *testing.T) {
	a := system.WheelDevice{ID: "A", Name: "Logitech G27", Model: "Logitech G27", Mode: "Generic HID", Supported: true}
	b := system.WheelDevice{ID: "B", Name: "Logitech G25", Model: "Logitech G25", Mode: "Generic HID", Supported: true}

	s := system.State{Wheels: []system.WheelDevice{a, b}}
	title, detail, _, _, visible := wheelAttentionCopy(s)
	if !visible || !strings.Contains(title, "Mehrere") || !strings.Contains(title, "Auswahl") {
		t.Fatalf("unexpected unselected multi-wheel attention: visible=%v title=%q detail=%q", visible, title, detail)
	}
	if !strings.Contains(detail, "Instrumente bleiben sichtbar") {
		t.Fatalf("multi-wheel guidance must promise persistent instruments, got %q", detail)
	}

	s.SelectedWheelID = a.ID
	title, detail, _, _, visible = wheelAttentionCopy(s)
	if !visible || title != "Mehrere Lenkräder erkannt" || !strings.Contains(detail, "Logitech G27") {
		t.Fatalf("selected multi-wheel attention missing active target: title=%q detail=%q", title, detail)
	}

	s.Wheels = []system.WheelDevice{a}
	title, _, _, _, visible = wheelAttentionCopy(s)
	if visible || title != "" {
		t.Fatalf("single selected wheel should not show attention panel: visible=%v title=%q", visible, title)
	}
}

func TestWheelAttentionRequestsC294ModelConfirmation(t *testing.T) {
	w := system.WheelDevice{ID: "usbloc:test", Name: "USB-Eingabegerät", Model: "Logitech C294 (Kompatibilitätsmodus)", Mode: "Generic HID / Modern", Supported: true}
	s := system.State{Wheels: []system.WheelDevice{w}, SelectedWheelID: w.ID, WheelModel: w.Model, ActiveMode: w.Mode}
	title, detail, _, _, visible := wheelAttentionCopy(s)
	if !visible || !strings.Contains(title, "C294") || !strings.Contains(strings.ToLower(detail), "modell") {
		t.Fatalf("C294 confirmation guidance missing: visible=%v title=%q detail=%q", visible, title, detail)
	}
	if got := overviewPrimaryAction(s); got != "Modell bestätigen" {
		t.Fatalf("C294 primary action=%q want Modell bestätigen", got)
	}
}
