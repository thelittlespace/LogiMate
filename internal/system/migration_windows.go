//go:build windows

package system

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// MigrationProgress receives durable milestones from privileged mode changes.
// status is one of running/done/warning/failed/rollback.
type MigrationProgress func(name, status, detail string) error

type migrationJournalAbort struct{ err error }

func migrationProgress(p MigrationProgress, name, status, detail string) {
	if p == nil {
		return
	}
	if err := p(name, status, detail); err != nil {
		// Journal durability is a safety prerequisite. Abort immediately before
		// any following driver/profile/HVCI step can run without an audit trail.
		panic(migrationJournalAbort{err: err})
	}
}

func requireMigrationWheel(expectG27 bool) (State, error) {
	s := CollectState()
	if len(s.Wheels) > 1 {
		return s, errors.New("Sicherheitsabbruch: mehrere unterstützte Lenkräder sind gleichzeitig verbunden. Logitech-Treiberpakete können mehrere Geräte betreffen; für einen Modern/Legacy-Wechsel bitte alle anderen Räder vorübergehend trennen")
	}
	if !CanChangeSelectedWheelMode(s) {
		return s, errors.New("Sicherheitsabbruch: kein eindeutig ausgewähltes unterstütztes Logitech-Lenkrad ist aktuell verbunden")
	}
	if expectG27 && !IsG27Model(s.WheelModel) {
		return s, fmt.Errorf("Sicherheitsabbruch: der G27-Modern-Pfad wurde angefordert, erkannt wurde aber %s", s.WheelModel)
	}
	return s, nil
}

func restoreOperatingPreference(dataDir, old string) error {
	switch old {
	case "modern", "legacy":
		return SaveOperatingPreference(dataDir, old)
	default:
		return ClearOperatingPreference(dataDir)
	}
}

func verifyMigrationTerminalState(dataDir, target string, timeout time.Duration) (State, error) {
	target = strings.ToLower(strings.TrimSpace(target))
	deadline := time.Now().Add(timeout)
	var last State
	var lastDetail string
	for {
		InvalidateSystemCaches()
		last = CollectState()
		pref := ReadOperatingPreference(dataDir)
		mode := strings.ToLower(last.ActiveMode)
		modeOK := false
		switch target {
		case "modern":
			modeOK = strings.Contains(mode, "generic hid")
		case "legacy":
			modeOK = strings.Contains(mode, "legacy")
		case "":
			// An empty original preference cannot prove a mode. It is only used
			// by rollback callers that have a concrete originalMode below.
			modeOK = true
		default:
			return last, fmt.Errorf("unbekannter Zielmodus %q", target)
		}
		if last.DeviceDetectionError == "" && HasActionableSelectedWheel(last) && modeOK && pref == target {
			return last, nil
		}
		lastDetail = fmt.Sprintf("mode=%s pref=%s detection=%s selection=%s", last.ActiveMode, pref, last.DeviceDetectionError, last.SelectionStatus)
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(750 * time.Millisecond)
	}
	return last, fmt.Errorf("Zielzustand %s wurde nicht verifiziert (%s)", target, lastDetail)
}

func verifyRollbackTerminalState(dataDir, originalPreference, originalMode string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		InvalidateSystemCaches()
		s := CollectState()
		pref := ReadOperatingPreference(dataDir)
		prefOK := pref == originalPreference
		if originalPreference == "" {
			prefOK = pref == ""
		}
		modeOK := originalMode == "" || strings.EqualFold(strings.TrimSpace(s.ActiveMode), strings.TrimSpace(originalMode))
		if s.DeviceDetectionError == "" && HasActionableSelectedWheel(s) && prefOK && modeOK {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("Rollback-Endzustand nicht verifiziert (mode=%s expected=%s pref=%s expectedPref=%s detection=%s)", s.ActiveMode, originalMode, pref, originalPreference, s.DeviceDetectionError)
		}
		time.Sleep(750 * time.Millisecond)
	}
}

func rollbackModern(dataDir string, originalPreference, originalMode string, driversMayHaveChanged, profilerWasRemoved bool, progress MigrationProgress) (string, error) {
	var notes []string
	migrationProgress(progress, "Rollback", "running", "Vorherigen Zustand möglichst vollständig wiederherstellen")
	if driversMayHaveChanged {
		msg, err := RestoreDrivers(dataDir)
		if err != nil {
			notes = append(notes, "Treiber-Rollback fehlgeschlagen: "+err.Error())
			migrationProgress(progress, "Treiber-Rollback", "failed", err.Error())
		} else {
			notes = append(notes, msg)
			migrationProgress(progress, "Treiber-Rollback", "rollback", msg)
		}
	}
	if profilerWasRemoved {
		msg, err := RestoreProfilerBackup(dataDir, true)
		if err != nil {
			notes = append(notes, "Profiler-Rollback fehlgeschlagen: "+err.Error())
			migrationProgress(progress, "Profiler-Rollback", "failed", err.Error())
		} else {
			notes = append(notes, msg)
			migrationProgress(progress, "Profiler-Rollback", "rollback", msg)
		}
	}
	if err := restoreOperatingPreference(dataDir, originalPreference); err != nil {
		notes = append(notes, "Betriebspräferenz-Rollback fehlgeschlagen: "+err.Error())
		migrationProgress(progress, "Betriebsmodus-Rollback", "failed", err.Error())
	} else {
		migrationProgress(progress, "Betriebsmodus-Rollback", "rollback", originalPreference)
	}
	InvalidateSystemCaches()
	verifyErr := verifyRollbackTerminalState(dataDir, originalPreference, originalMode, 20*time.Second)
	if verifyErr != nil {
		notes = append(notes, "Rollback-Verifikation fehlgeschlagen: "+verifyErr.Error())
		migrationProgress(progress, "Rollback verifizieren", "failed", verifyErr.Error())
	} else {
		migrationProgress(progress, "Rollback verifizieren", "rollback", "Ursprünglicher Zustand bestätigt")
	}
	if len(notes) == 0 {
		notes = append(notes, "Kein Daten-Rollback war erforderlich; ursprünglicher Zustand wurde verifiziert.")
	}
	return strings.Join(notes, "\r\n"), verifyErr
}

// SetupModernG27 performs the explicitly requested G27 migration. Since Fusion
// Since C7, LogiMate Native is the normal Modern engine. D5 has no external Modern runtime
// fallback only and is never downloaded/launched as a migration dependency.
func SetupModernG27(dataDir string, uninstallProfiler bool) (string, error) {
	return SetupModernG27Tracked(dataDir, uninstallProfiler, nil)
}

func SetupModernG27Tracked(dataDir string, uninstallProfiler bool, progress MigrationProgress) (string, error) {
	state, err := requireMigrationWheel(true)
	if err != nil {
		migrationProgress(progress, "Zielgerät prüfen", "failed", err.Error())
		return "", err
	}
	migrationProgress(progress, "G27-Kompatibilität", "done", state.WheelModel+" · gemeinsamer nativer Migrationspfad")
	return SetupModernGenericTracked(dataDir, uninstallProfiler, progress)
}

func SetupLegacy(dataDir string, withProfiler bool) (string, error) {
	return SetupLegacyTracked(dataDir, withProfiler, nil)
}

func SetupLegacyTracked(dataDir string, withProfiler bool, progress MigrationProgress) (string, error) {
	var log []string
	state, err := requireMigrationWheel(false)
	if err != nil {
		migrationProgress(progress, "Zielgerät prüfen", "failed", err.Error())
		return "", err
	}
	migrationProgress(progress, "Zielgerät prüfen", "done", state.WheelModel+" · "+state.ActiveMode)

	installed, err := ListLegacyDriversStrict()
	if err != nil {
		migrationProgress(progress, "Treiber inventarisieren", "failed", err.Error())
		return "", err
	}
	if len(installed) > 0 {
		// First-run users may already be in a perfectly working Legacy setup.
		// Do not unnecessarily remove/reinstall it just because they chose
		// "Original / Legacy" in the guide. Create a safety backup instead.
		if latestBackupDir(dataDir) == "" {
			migrationProgress(progress, "Treiber sichern", "running", "Bestehenden Legacy-Zustand sichern")
			msg, err := BackupDrivers(dataDir)
			if err != nil {
				migrationProgress(progress, "Treiber sichern", "failed", err.Error())
				return msg, fmt.Errorf("Legacy-Treiber sind aktiv, aber das Sicherheits-Backup ist fehlgeschlagen: %w", err)
			}
			log = append(log, msg)
			migrationProgress(progress, "Treiber sichern", "done", msg)
		}
		log = append(log, "Logitech-Legacy-Treiber sind bereits installiert; sie wurden unverändert beibehalten.")
		migrationProgress(progress, "Legacy-Treiber", "done", "Bereits aktiv; keine Neuinstallation")
	} else {
		migrationProgress(progress, "Legacy-Treiber wiederherstellen", "running", "Backup prüfen und installieren")
		msg, restoreErr := RestoreDrivers(dataDir)
		if restoreErr != nil {
			// Fresh-machine path: there is no LogiMate backup yet. When the user
			// explicitly selected Legacy + Profiler, download Logitech's own
			// LGS 5.10.127 installer, verify its Authenticode signer and let the
			// official installer provision both Profiler and legacy wheel stack.
			if !withProfiler {
				migrationProgress(progress, "Legacy-Treiber wiederherstellen", "failed", restoreErr.Error())
				return msg, fmt.Errorf("kein Legacy-Backup vorhanden. Für einen frischen Legacy-Aufbau 'Profiler installieren' wählen oder zuerst einen vorhandenen Treiber sichern: %w", restoreErr)
			}
			migrationProgress(progress, "Logitech LGS herunterladen", "running", "Offizielle Logitech Gaming Software 5.10.127")
			installer, installErr := DownloadAndInstallOfficialLGS(dataDir)
			if installErr != nil {
				migrationProgress(progress, "Logitech LGS herunterladen", "failed", installErr.Error())
				return strings.Join(log, "\r\n"), fmt.Errorf("Legacy konnte weder aus Backup noch über den offiziellen Logitech-Installer aufgebaut werden: %w", installErr)
			}
			log = append(log, "Offizielle Logitech Gaming Software 5.10.127 wurde gestartet/installiert: "+installer)
			migrationProgress(progress, "Logitech LGS herunterladen", "done", "Authenticode: Logitech · Installer abgeschlossen")
			// Some LGS installer generations return before Windows has finished PnP /
			// Driver Store publication. Do not turn that normal propagation delay into
			// a false setup failure; poll a bounded window and preserve the last real
			// inventory error if Windows never converges.
			var rescanned []LegacyDriver
			var scanErr error
			deadline := time.Now().Add(45 * time.Second)
			for {
				InvalidateSystemCaches()
				rescanned, scanErr = ListLegacyDriversStrict()
				if scanErr == nil && len(rescanned) > 0 {
					break
				}
				if time.Now().After(deadline) {
					break
				}
				time.Sleep(2 * time.Second)
			}
			if scanErr != nil {
				return strings.Join(log, "\r\n"), fmt.Errorf("LGS wurde installiert, aber die Treiberinventur danach schlug fehl: %w", scanErr)
			}
			if len(rescanned) == 0 {
				return strings.Join(log, "\r\n"), errors.New("Logitech LGS wurde ausgeführt, danach wurden innerhalb des Prüfzeitfensters jedoch keine Legacy-Wheel-Treiber erkannt")
			}
			migrationProgress(progress, "Legacy-Treiber wiederherstellen", "done", fmt.Sprintf("%d Logitech-Legacy-Paket(e) erkannt", len(rescanned)))
		} else {
			log = append(log, msg)
			migrationProgress(progress, "Legacy-Treiber wiederherstellen", "done", msg)
		}
	}

	if withProfiler {
		p, pErr := DetectProfilerStrict()
		if pErr != nil {
			migrationProgress(progress, "Profiler prüfen", "warning", pErr.Error())
			log = append(log, "Profiler-Status konnte nicht zuverlässig gelesen werden: "+pErr.Error())
		} else if p.Installed {
			log = append(log, "Logitech Gaming Software / Profiler ist bereits installiert und bleibt erhalten.")
			migrationProgress(progress, "Profiler wiederherstellen", "done", "Bereits installiert")
		} else {
			migrationProgress(progress, "Profiler wiederherstellen", "running", "Gesicherten Installer/Settings verwenden")
			pmsg, perr := RestoreProfilerBackup(dataDir, true)
			if perr != nil {
				log = append(log, "Profiler konnte nicht automatisch wiederhergestellt werden: "+perr.Error())
				migrationProgress(progress, "Profiler wiederherstellen", "warning", perr.Error())
			} else if pmsg != "" {
				log = append(log, pmsg)
				migrationProgress(progress, "Profiler wiederherstellen", "done", pmsg)
			}
		}
	} else {
		log = append(log, "Legacy-Modus wird ohne Profiler-Wiederherstellung verwendet.")
		migrationProgress(progress, "Profiler wiederherstellen", "done", "Vom Nutzer nicht ausgewählt")
	}
	if err := SaveOperatingPreference(dataDir, "legacy"); err != nil {
		migrationProgress(progress, "Betriebsmodus speichern", "failed", err.Error())
		return strings.Join(log, "\r\n"), fmt.Errorf("Legacy-Zustand wurde vorbereitet, aber die Betriebspräferenz konnte nicht sicher gespeichert werden: %w", err)
	}
	migrationProgress(progress, "Betriebsmodus speichern", "done", "legacy")
	if _, err := verifyMigrationTerminalState(dataDir, "legacy", 45*time.Second); err != nil {
		migrationProgress(progress, "Zielzustand verifizieren", "failed", err.Error())
		return strings.Join(log, "\r\n"), err
	}
	migrationProgress(progress, "Zielzustand verifizieren", "done", "Logitech Legacy bestätigt")
	InvalidateSystemCaches()
	return strings.Join(log, "\r\n"), nil
}

func SetupModernGeneric(dataDir string, uninstallProfiler bool) (string, error) {
	return SetupModernGenericTracked(dataDir, uninstallProfiler, nil)
}

func SetupModernGenericTracked(dataDir string, uninstallProfiler bool, progress MigrationProgress) (string, error) {
	var log []string
	state, err := requireMigrationWheel(false)
	if err != nil {
		migrationProgress(progress, "Zielgerät prüfen", "failed", err.Error())
		return "", err
	}
	migrationProgress(progress, "Zielgerät prüfen", "done", state.WheelModel+" · "+state.ActiveMode)
	originalPreference := ReadOperatingPreference(dataDir)

	migrationProgress(progress, "LogiMate Native Engine prüfen", "running", "Gemeinsamen G25/G27/DFGT-Core und Modell-Descriptoren validieren")
	if err := NativeEngineCoreGate(); err != nil {
		migrationProgress(progress, "LogiMate Native Engine prüfen", "failed", err.Error())
		return "", fmt.Errorf("Native Engine Gate fehlgeschlagen; es wurden keine Treiber verändert: %w", err)
	}
	log = append(log, "LogiMate Native Engine: gemeinsamer Core-Gate PASS. Der Modern-Pfad ist vollständig eigenständig.")
	migrationProgress(progress, "LogiMate Native Engine prüfen", "done", "G25/G27/DFGT Core bereit · keine externe Runtime erforderlich")

	profilerInfo, profilerDetectErr := DetectProfilerStrict()
	if profilerDetectErr != nil {
		migrationProgress(progress, "Profiler prüfen", "failed", profilerDetectErr.Error())
		return "", profilerDetectErr
	}
	migrationProgress(progress, "Profiler sichern", "running", "Einstellungen sichern")
	profilerDir, err := BackupProfiler(dataDir)
	if err != nil {
		migrationProgress(progress, "Profiler sichern", "failed", err.Error())
		return "", fmt.Errorf("Profiler-Backup fehlgeschlagen: %w", err)
	}
	log = append(log, "Profiler-Einstellungen gesichert: "+profilerDir)
	migrationProgress(progress, "Profiler sichern", "done", profilerDir)

	drivers, err := ListLegacyDriversStrict()
	if err != nil {
		migrationProgress(progress, "Treiber inventarisieren", "failed", err.Error())
		return strings.Join(log, "\r\n"), err
	}
	driverBackupMade := false
	if len(drivers) > 0 {
		migrationProgress(progress, "Treiber sichern", "running", "Driver Store exportieren")
		msg, err := BackupDrivers(dataDir)
		if err != nil {
			migrationProgress(progress, "Treiber sichern", "failed", err.Error())
			return strings.Join(append(log, msg), "\r\n"), fmt.Errorf("Treiber-Backup fehlgeschlagen: %w", err)
		}
		driverBackupMade = true
		log = append(log, msg)
		migrationProgress(progress, "Treiber sichern", "done", msg)
	} else {
		log = append(log, "Keine Legacy-Treiberpakete vorhanden; Treiber-Backup nicht erforderlich.")
		migrationProgress(progress, "Treiber sichern", "done", "Nicht erforderlich")
	}

	profilerRemoved := false
	if uninstallProfiler && profilerInfo.Installed {
		migrationProgress(progress, "Profiler deinstallieren", "running", profilerInfo.DisplayName)
		if err := UninstallProfiler(profilerInfo); err != nil {
			migrationProgress(progress, "Profiler deinstallieren", "failed", err.Error())
			return strings.Join(log, "\r\n"), fmt.Errorf("Profiler-Deinstallation fehlgeschlagen; Treiber wurden noch nicht entfernt: %w", err)
		}
		profilerRemoved = true
		InvalidateSystemCaches()
		log = append(log, "Logitech Gaming Software / Profiler deinstalliert.")
		migrationProgress(progress, "Profiler deinstallieren", "done", "Profiler wurde entfernt")
	} else if !uninstallProfiler {
		log = append(log, "Profiler bleibt installiert (LCore wird für Generic HID beendet).")
		migrationProgress(progress, "Profiler deinstallieren", "done", "Vom Nutzer nicht ausgewählt")
	}

	if len(drivers) > 0 && state.ActiveMode == "Logitech Legacy" {
		if err := ValidateLegacyDriverBinding(drivers, SelectedWheelDevices(state)); err != nil {
			migrationProgress(progress, "Legacy-Treiber zuordnen", "failed", err.Error())
			rollback, rollbackErr := rollbackModern(dataDir, originalPreference, state.ActiveMode, false, profilerRemoved, progress)
			return strings.Join(append(log, "Rollback:\r\n"+rollback), "\r\n"), errors.Join(fmt.Errorf("Sicherheitsabbruch vor Treiberentfernung: %w", err), rollbackErr)
		}
		migrationProgress(progress, "Legacy-Treiber entfernen", "running", "Generic HID vorbereiten")
		msg, err := removeLegacyDriverPackages(drivers)
		if err != nil {
			migrationProgress(progress, "Legacy-Treiber entfernen", "failed", err.Error())
			rollback, rollbackErr := rollbackModern(dataDir, originalPreference, state.ActiveMode, driverBackupMade, profilerRemoved, progress)
			return strings.Join(append(log, msg, "Rollback:\r\n"+rollback), "\r\n"), errors.Join(err, rollbackErr)
		}
		log = append(log, msg)
		migrationProgress(progress, "Legacy-Treiber entfernen", "done", msg)
	} else if len(drivers) > 0 {
		msg := "Legacy-Pakete liegen noch im Driver Store, sind aber nicht an das aktuelle Lenkrad gebunden. Sie wurden aus Sicherheitsgründen nicht gelöscht."
		log = append(log, msg)
		migrationProgress(progress, "Legacy-Treiber entfernen", "warning", msg)
	} else {
		msg, err := removeLegacyDriverPackages(nil)
		if err != nil {
			migrationProgress(progress, "Windows-Gerätescan", "failed", err.Error())
			rollback, rollbackErr := rollbackModern(dataDir, originalPreference, state.ActiveMode, false, profilerRemoved, progress)
			return strings.Join(append(log, "Rollback:\r\n"+rollback), "\r\n"), errors.Join(err, rollbackErr)
		}
		log = append(log, msg)
	}

	// G25 and Driving Force GT are multimode wheels just like G27. Once their
	// exact C294 identity has been explicitly confirmed, switch them to their
	// native PID as part of the Modern path instead of leaving them in the
	// reduced compatibility identity indefinitely.
	migrationProgress(progress, "Native Wheel Mode", "running", "C294 bei Bedarf auf native Logitech-PID umschalten")
	if err := PrepareSupportedWheelNativeMode(state.WheelModel, EnumerateRawInputLogitechWheels()); err != nil {
		warning := "Native Logitech-PID konnte noch nicht aktiviert werden: " + err.Error()
		log = append(log, "Hinweis: "+warning)
		migrationProgress(progress, "Native Wheel Mode", "warning", warning)
	} else {
		log = append(log, "Nativer Logitech-Modus für "+state.WheelModel+" ist vorbereitet.")
		migrationProgress(progress, "Native Wheel Mode", "done", state.WheelModel)
	}
	if err := SaveOperatingPreference(dataDir, "modern"); err != nil {
		migrationProgress(progress, "Betriebsmodus speichern", "failed", err.Error())
		rollback, rollbackErr := rollbackModern(dataDir, originalPreference, state.ActiveMode, driverBackupMade, profilerRemoved, progress)
		return strings.Join(append(log, "Rollback:\r\n"+rollback), "\r\n"), errors.Join(fmt.Errorf("Betriebspräferenz modern konnte nicht gespeichert werden: %w", err), rollbackErr)
	}
	migrationProgress(progress, "Betriebsmodus speichern", "done", "modern")
	if _, err := verifyMigrationTerminalState(dataDir, "modern", 20*time.Second); err != nil {
		migrationProgress(progress, "Zielzustand verifizieren", "failed", err.Error())
		rollback, rollbackErr := rollbackModern(dataDir, originalPreference, state.ActiveMode, driverBackupMade, profilerRemoved, progress)
		return strings.Join(append(log, "Rollback:\r\n"+rollback), "\r\n"), errors.Join(err, rollbackErr)
	}
	migrationProgress(progress, "Zielzustand verifizieren", "done", "Modern/Generic HID bestätigt")
	InvalidateSystemCaches()
	return strings.Join(log, "\r\n"), nil
}
