//go:build windows

package app

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thelittlespace/LogiMate/internal/system"
)

const uiSettingsSchemaVersion = 1

type uiSettings struct {
	SchemaVersion              int    `json:"schemaVersion,omitempty"`
	ThemeMode                  string `json:"themeMode"`
	Acrylic                    bool   `json:"acrylic"`
	Animations                 bool   `json:"animations"`
	SidebarAutoExpand          bool   `json:"sidebarAutoExpand"`
	AutoRefresh                bool   `json:"autoRefresh"`
	NativeWheelOutput          bool   `json:"nativeWheelOutputExperimental"`
	NativeOutputRiskVersion    int    `json:"nativeOutputRiskVersion,omitempty"`
	NativeOutputRiskAcceptedAt string `json:"nativeOutputRiskAcceptedAt,omitempty"`
	CheckUpdates               bool   `json:"checkUpdates"`
	AutoDownloadUpdate         bool   `json:"autoDownloadUpdate"`
	PreviewUpdates             bool   `json:"previewUpdates"`
	ShowWhatsNew               bool   `json:"showWhatsNew"`
	StartWithWindows           bool   `json:"startWithWindows"`
	OfferSetup                 bool   `json:"offerSetupOnStartup"`
	SetupCompleted             bool   `json:"setupCompleted"`
	SetupDeferredUntil         string `json:"setupDeferredUntil,omitempty"`
	SetupResumeStep            int    `json:"setupResumeStep,omitempty"`
	Language                   string `json:"language"`
	LastSeenVersion            string `json:"lastSeenVersion,omitempty"`
}

var (
	uiSettingsMu           sync.RWMutex
	uiPrefs                = defaultUISettings()
	uiPrefsPath            string
	uiSettingsLoadWarning  string
	uiSettingsWriteBlocked string
)

func defaultUISettings() uiSettings {
	return uiSettings{
		SchemaVersion:      uiSettingsSchemaVersion,
		ThemeMode:          "dark",
		Acrylic:            true,
		Animations:         true,
		SidebarAutoExpand:  true,
		AutoRefresh:        true,
		NativeWheelOutput:  false,
		CheckUpdates:       true,
		AutoDownloadUpdate: !strings.Contains(strings.ToLower(strings.TrimPrefix(Version, "v")), "-"),
		PreviewUpdates:     strings.Contains(strings.ToLower(strings.TrimPrefix(Version, "v")), "-"),
		ShowWhatsNew:       true,
		StartWithWindows:   false,
		OfferSetup:         true,
		SetupCompleted:     false,
		Language:           "de",
	}
}

func loadUISettings(dataDir string) uiSettings {
	p := defaultUISettings()
	uiPrefsPath = filepath.Join(dataDir, "settings.json")
	var raw map[string]json.RawMessage
	if exists, err := system.ReadJSONConfigStrict(uiPrefsPath, &raw); err != nil {
		uiSettingsLoadWarning = err.Error()
	} else if exists {
		uiSettingsLoadWarning = ""
		uiSettingsWriteBlocked = ""
		if v, ok := raw["schemaVersion"]; ok {
			_ = json.Unmarshal(v, &p.SchemaVersion)
		}
		if p.SchemaVersion <= 0 {
			p.SchemaVersion = uiSettingsSchemaVersion
		}
		if p.SchemaVersion > uiSettingsSchemaVersion {
			uiSettingsWriteBlocked = fmt.Sprintf("settings.json verwendet Schema %d; unterstützt wird maximal %d", p.SchemaVersion, uiSettingsSchemaVersion)
			uiSettingsLoadWarning = uiSettingsWriteBlocked + ". Einstellungen bleiben lesbar, Änderungen sind bis zu einem bewussten Reset gesperrt."
		}
		readBool := func(key string, dst *bool) {
			if v, ok := raw[key]; ok {
				var x bool
				if json.Unmarshal(v, &x) == nil {
					*dst = x
				}
			}
		}
		if v, ok := raw["themeMode"]; ok {
			_ = json.Unmarshal(v, &p.ThemeMode)
		}
		p.ThemeMode = normalizeThemeMode(p.ThemeMode)
		readBool("acrylic", &p.Acrylic)
		readBool("animations", &p.Animations)
		readBool("sidebarAutoExpand", &p.SidebarAutoExpand)
		readBool("autoRefresh", &p.AutoRefresh)
		readBool("nativeWheelOutputExperimental", &p.NativeWheelOutput)
		readBool("checkUpdates", &p.CheckUpdates)
		readBool("autoDownloadUpdate", &p.AutoDownloadUpdate)
		readBool("previewUpdates", &p.PreviewUpdates)
		readBool("showWhatsNew", &p.ShowWhatsNew)
		readBool("startWithWindows", &p.StartWithWindows)
		readBool("offerSetupOnStartup", &p.OfferSetup)
		readBool("setupCompleted", &p.SetupCompleted)
		if v, ok := raw["setupDeferredUntil"]; ok {
			_ = json.Unmarshal(v, &p.SetupDeferredUntil)
		}
		if v, ok := raw["setupResumeStep"]; ok {
			_ = json.Unmarshal(v, &p.SetupResumeStep)
		}
		if p.SetupResumeStep < 0 || p.SetupResumeStep >= setupPageCount {
			p.SetupResumeStep = 0
		}
		if v, ok := raw["language"]; ok {
			_ = json.Unmarshal(v, &p.Language)
		}
		if p.Language != "en" {
			p.Language = "de"
		}
		if v, ok := raw["lastSeenVersion"]; ok {
			_ = json.Unmarshal(v, &p.LastSeenVersion)
		}
		if v, ok := raw["nativeOutputRiskVersion"]; ok {
			_ = json.Unmarshal(v, &p.NativeOutputRiskVersion)
		}
		if v, ok := raw["nativeOutputRiskAcceptedAt"]; ok {
			_ = json.Unmarshal(v, &p.NativeOutputRiskAcceptedAt)
		}
	}
	// Registry is source of truth for actual Windows autostart state.
	p.StartWithWindows = isStartupEnabled()
	return p
}

func saveUISettings(p uiSettings) error {
	if uiPrefsPath == "" {
		return fmt.Errorf("Einstellungspfad ist noch nicht initialisiert")
	}
	if uiSettingsWriteBlocked != "" {
		return fmt.Errorf("Einstellungen sind schreibgeschützt: %s", uiSettingsWriteBlocked)
	}
	p.SchemaVersion = uiSettingsSchemaVersion
	return system.WriteJSONConfigStrict(uiPrefsPath, p, 0644)
}

func resetUISettingsStorage(p uiSettings) error {
	if uiPrefsPath == "" {
		return fmt.Errorf("Einstellungspfad ist noch nicht initialisiert")
	}
	p.SchemaVersion = uiSettingsSchemaVersion
	if err := system.ResetJSONConfig(uiPrefsPath, p, 0644); err != nil {
		return err
	}
	uiSettingsMu.Lock()
	uiPrefs = p
	uiSettingsLoadWarning = ""
	uiSettingsWriteBlocked = ""
	uiSettingsMu.Unlock()
	return nil
}

const nativeOutputRiskTextVersion = 1

func getUISettings() uiSettings {
	uiSettingsMu.RLock()
	defer uiSettingsMu.RUnlock()
	return uiPrefs
}

func replaceUISettings(p uiSettings) error {
	if err := saveUISettings(p); err != nil {
		return err
	}
	uiSettingsMu.Lock()
	uiPrefs = p
	uiSettingsMu.Unlock()
	return nil
}

func setStartupEnabled(enabled bool) error {
	return system.SetUserStartup(currentExe(), enabled)
}

func isStartupEnabled() bool {
	return system.UserStartupMatches(currentExe())
}
