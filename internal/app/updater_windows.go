//go:build windows

package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thelittlespace/LogiMate/internal/system"
)

var (
	updateMu                   sync.RWMutex
	latestLogiMateUpdate       *system.UpdateInfo
	downloadedLogiMateUpdate   string
	updateStatus               = "Noch nicht geprüft"
	updateChecking             bool
	updateDownloading          bool
	updateInstallPromptPending bool
	forceWhatsNew              bool
)

func initUpdateStartupFlags() {
	for _, a := range os.Args[1:] {
		if a == "--post-update" {
			forceWhatsNew = true
			cleanupOldUpdaterHelper()
		}
	}
}

func getUpdateStatus() string {
	updateMu.RLock()
	defer updateMu.RUnlock()
	return updateStatus
}

func setUpdateStatus(s string) {
	updateMu.Lock()
	updateStatus = s
	updateMu.Unlock()
	if mainWnd != 0 {
		invalidate(mainWnd)
	}
}

func updateSnapshot() (*system.UpdateInfo, string, bool) {
	updateMu.RLock()
	defer updateMu.RUnlock()
	return latestLogiMateUpdate, downloadedLogiMateUpdate, updateChecking || updateDownloading
}

func onStartupStable() {
	if smokeTest {
		return
	}
	// The optional setup guide owns the foreground first. Changelog/update prompts
	// continue only after the guide has been closed so first-run users never get
	// two overlapping welcome windows.
	if maybeOfferFirstRunSetup() {
		return
	}
	continueStartupAfterSetup()
}

func continueStartupAfterSetup() {
	if smokeTest {
		return
	}
	prefs := getUISettings()
	current := releaseIdentity()
	shouldShow := prefs.ShowWhatsNew && (forceWhatsNew || strings.TrimSpace(prefs.LastSeenVersion) == "" || !strings.EqualFold(strings.TrimSpace(prefs.LastSeenVersion), current))
	if shouldShow {
		if showWhatsNewWindow(currentReleaseNotes) {
			prefs.LastSeenVersion = current
			_ = replaceUISettings(prefs)
		}
	} else if !prefs.ShowWhatsNew && !strings.EqualFold(strings.TrimSpace(prefs.LastSeenVersion), current) {
		prefs.LastSeenVersion = current
		_ = replaceUISettings(prefs)
	}
	if prefs.CheckUpdates {
		checkForUpdatesAsync(false)
	}
}

func showCurrentWhatsNew() {
	if showWhatsNewWindow(currentReleaseNotes) {
		p := getUISettings()
		p.LastSeenVersion = releaseIdentity()
		_ = replaceUISettings(p)
	}
}

func checkForUpdatesAsync(manual bool) {
	updateMu.Lock()
	if updateChecking {
		updateMu.Unlock()
		if manual {
			queueNotice("Eine Update-Prüfung läuft bereits.", "LogiMate Update", MB_OK|MB_ICONINFORMATION)
		}
		return
	}
	updateChecking = true
	updateStatus = "GitHub wird geprüft …"
	updateMu.Unlock()
	invalidate(mainWnd)

	prefs := getUISettings()
	go func() {
		info, err := system.CheckLatestLogiMate(releaseIdentity(), prefs.PreviewUpdates)
		updateMu.Lock()
		updateChecking = false
		updateMu.Unlock()
		if err != nil {
			setUpdateStatus("Update-Prüfung fehlgeschlagen: " + err.Error())
			logStartup("Update check failed: %v", err)
			if manual {
				queueNotice(err.Error(), "LogiMate Update", MB_OK|MB_ICONERROR)
			}
			return
		}
		if info == nil {
			setUpdateStatus("Aktuell · " + displayVersion(Version))
			if manual {
				queueNotice("Du verwendest bereits die neueste passende LogiMate-Version.", "LogiMate Update", MB_OK|MB_ICONINFORMATION)
			}
			return
		}
		updateMu.Lock()
		cachedPath := downloadedLogiMateUpdate
		cachedSame := latestLogiMateUpdate != nil && strings.EqualFold(latestLogiMateUpdate.Version, info.Version) && cachedPath != ""
		latestLogiMateUpdate = info
		if !cachedSame {
			downloadedLogiMateUpdate = ""
		}
		updateMu.Unlock()
		if cachedSame {
			if _, statErr := os.Stat(cachedPath); statErr == nil {
				setUpdateStatus("Bereit zur Installation · " + info.Version)
				if manual {
					postMessage(mainWnd, msgUpdateDownloaded, 0, 0)
				}
				return
			}
		}
		setUpdateStatus("Update verfügbar · " + info.Version)
		logStartup("Update available: %s prerelease=%v", info.Version, info.Prerelease)
		if prefs.AutoDownloadUpdate && currentBuildHasTrustedPublisher() {
			downloadUpdateAsync(info, manual)
		} else if prefs.AutoDownloadUpdate {
			setUpdateStatus("Update verfügbar · manuell installieren (Pre-Release unsigned)")
			if manual {
				postMessage(mainWnd, msgUpdateFound, 0, 0)
			}
		} else if manual {
			postMessage(mainWnd, msgUpdateFound, 0, 0)
		}
	}()
}

func currentBuildHasTrustedPublisher() bool {
	_, err := system.ReadAuthenticodeIdentity(currentExe())
	return err == nil
}

func releasePageURL(info *system.UpdateInfo) string {
	if info == nil || strings.TrimSpace(info.Repository) == "" || strings.TrimSpace(info.TagName) == "" {
		return ""
	}
	return "https://github.com/" + strings.Trim(strings.TrimSpace(info.Repository), "/") + "/releases/tag/" + url.PathEscape(strings.TrimSpace(info.TagName))
}

func handleUpdateFound() {
	info, _, _ := updateSnapshot()
	if info == nil {
		return
	}
	if !currentBuildHasTrustedPublisher() {
		text := fmt.Sprintf("LogiMate %s ist verfügbar.\r\n\r\nDieser Alpha-Build ist nicht Authenticode-signiert. LogiMate ersetzt eine nicht verifizierbare laufende EXE absichtlich nicht automatisch.\r\n\r\nRelease-Seite im Browser öffnen?", info.Version)
		if messageBox(mainWnd, text, "LogiMate Update · Pre-Release", MB_YESNO|MB_ICONINFORMATION|MB_DEFBUTTON2) == IDYES {
			if u := releasePageURL(info); u != "" {
				if err := openExternalURL(u); err != nil {
					queueNotice(err.Error(), "LogiMate Update", MB_OK|MB_ICONERROR)
				}
			}
		}
		return
	}
	text := fmt.Sprintf("LogiMate %s ist verfügbar.\r\n\r\nJetzt herunterladen?", info.Version)
	if strings.TrimSpace(info.Body) != "" {
		text += "\r\n\r\nDie Änderungen werden nach dem Download vor der Installation angezeigt."
	}
	if messageBox(mainWnd, text, "LogiMate Update", MB_YESNO|MB_ICONINFORMATION) == IDYES {
		downloadUpdateAsync(info, true)
	}
}

func downloadUpdateAsync(info *system.UpdateInfo, manual bool) {
	if info == nil {
		return
	}
	if !currentBuildHasTrustedPublisher() {
		setUpdateStatus("Update verfügbar · manuell installieren (Pre-Release unsigned)")
		if manual {
			postMessage(mainWnd, msgUpdateFound, 0, 0)
		}
		return
	}
	updateMu.Lock()
	if updateDownloading {
		updateMu.Unlock()
		if manual {
			queueNotice("Ein Update-Download läuft bereits.", "LogiMate Update", MB_OK|MB_ICONINFORMATION)
		}
		return
	}
	updateDownloading = true
	updateMu.Unlock()
	dataDir, _ := system.GetDataDir()
	target := currentUpdateTarget()
	setUpdateStatus("Update " + info.Version + " wird vorbereitet …")
	go func() {
		path, err := system.DownloadLogiMateUpdate(info, target, dataDir, func(t string) { setUpdateStatus(t) })
		if err != nil {
			updateMu.Lock()
			updateDownloading = false
			updateMu.Unlock()
			setUpdateStatus("Download fehlgeschlagen: " + err.Error())
			logStartup("Update download failed: %v", err)
			if manual || getUISettings().AutoDownloadUpdate {
				queueNotice(err.Error(), "LogiMate Update", MB_OK|MB_ICONERROR)
			}
			return
		}
		if target != system.UpdatePortable {
			if sigErr := system.VerifySameAuthenticodePublisher(currentExe(), path); sigErr != nil {
				_ = os.Remove(path)
				updateMu.Lock()
				updateDownloading = false
				updateMu.Unlock()
				setUpdateStatus("Update blockiert: Publisher nicht verifiziert")
				logStartup("Update publisher verification failed: %v", sigErr)
				if manual || getUISettings().AutoDownloadUpdate {
					queueNotice(sigErr.Error(), "LogiMate Update · Sicherheit", MB_OK|MB_ICONERROR)
				}
				return
			}
		}
		updateMu.Lock()
		latestLogiMateUpdate = info
		downloadedLogiMateUpdate = path
		updateDownloading = false
		updateMu.Unlock()
		setUpdateStatus("Bereit zur Installation · " + info.Version)
		postMessage(mainWnd, msgUpdateDownloaded, 0, 0)
	}()
}

func handleUpdateDownloaded() {
	if changelogWnd != 0 {
		updateInstallPromptPending = true
		return
	}
	info, path, _ := updateSnapshot()
	if info == nil || path == "" {
		return
	}
	notes := strings.TrimSpace(info.Body)
	text := fmt.Sprintf("LogiMate %s wurde vollständig heruntergeladen und geprüft.\r\n\r\nJetzt anwenden?", info.Version)
	if notes != "" {
		// Keep the prompt compact; detailed notes are shown after restart.
		text += "\r\n\r\nNach dem Update zeigt LogiMate automatisch ‚Was ist neu?‘."
	}
	if messageBox(mainWnd, text, "Update bereit", MB_YESNO|MB_ICONINFORMATION|MB_DEFBUTTON2) != IDYES {
		return
	}
	applyDownloadedUpdate(info, path)
}

func applyDownloadedUpdate(info *system.UpdateInfo, path string) {
	dataDir, _ := system.GetDataDir()
	if err := system.NativeOutputEmergencyStop(appStateSnapshot()); err != nil {
		messageBox(mainWnd, "Update wurde nicht gestartet, weil der Wheel-Output nicht sicher neutralisiert werden konnte:\r\n\r\n"+err.Error(), "LogiMate Update · Sicherheitsabbruch", MB_OK|MB_ICONERROR)
		return
	}
	if err := system.NativeOutputRelease(); err != nil {
		messageBox(mainWnd, "Update wurde nicht gestartet, weil der Native-Output-Lease nicht sicher freigegeben werden konnte:\r\n\r\n"+err.Error(), "LogiMate Update · Sicherheitsabbruch", MB_OK|MB_ICONERROR)
		return
	}
	if need, detail := system.RuntimeOutputRecoveryNeeded(dataDir); need {
		messageBox(mainWnd, "Update wurde blockiert, weil noch ein ungeklärter Motor-Recovery-Zustand existiert:\r\n\r\n"+detail, "LogiMate Update · Sicherheitsabbruch", MB_OK|MB_ICONERROR)
		return
	}
	switch currentUpdateTarget() {
	case system.UpdatePortable:
		if err := applyPortableUpdate(info, path, dataDir); err != nil {
			messageBox(mainWnd, err.Error(), "Portable Update", MB_OK|MB_ICONERROR)
		}
	case system.UpdateStandalone:
		if err := applyStandaloneUpdate(info, path, dataDir); err != nil {
			messageBox(mainWnd, err.Error(), "Standalone Update", MB_OK|MB_ICONERROR)
		}
	default:
		// Installed builds use the release installer. This keeps Program Files,
		// shortcuts and uninstall metadata in sync and delegates UAC to Windows.
		if !shellRunAs(path, fmt.Sprintf("--update --wait-pid %d", os.Getpid())) {
			messageBox(mainWnd, "Der Update-Installer konnte nicht mit Administratorrechten gestartet werden.", "LogiMate Update", MB_OK|MB_ICONERROR)
			return
		}
		logStartup("Update installer launched: %s", path)
		postMessage(mainWnd, WM_CLOSE, 0, 0)
	}
}

func currentUpdateTarget() system.UpdateTarget {
	_, portable := system.GetDataDir()
	if portable {
		return system.UpdatePortable
	}
	exe := strings.ToLower(filepath.Clean(currentExe()))
	for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		base = strings.TrimSpace(base)
		if base == "" {
			continue
		}
		b := strings.ToLower(filepath.Clean(base))
		if exe == b || strings.HasPrefix(exe, b+string(os.PathSeparator)) {
			return system.UpdateInstalled
		}
	}
	return system.UpdateStandalone
}

func applyStandaloneUpdate(info *system.UpdateInfo, newExe, dataDir string) error {
	target := currentExe()
	if _, err := os.Stat(newExe); err != nil {
		return err
	}
	logPath := filepath.Join(dataDir, "update.log")
	rotateLogFile(logPath, 1<<20, 3)
	token, err := createUpdateAuthorization(dataDir, updateAuthorization{Mode: "standalone", PID: uint32(os.Getpid()), NewExe: newExe, Target: target, Version: info.Version, LogPath: logPath})
	if err != nil {
		return fmt.Errorf("Update-Autorisierung konnte nicht erstellt werden: %w", err)
	}
	if err := launchNativeUpdateHelper(dataDir, "--apply-standalone", token); err != nil {
		return err
	}
	logStartup("Standalone native updater helper started for %s", info.Version)
	postMessage(mainWnd, WM_CLOSE, 0, 0)
	return nil
}

func applyPortableUpdate(info *system.UpdateInfo, zipPath, dataDir string) error {
	targetDir := filepath.Dir(currentExe())
	stageRoot := filepath.Join(dataDir, "Updates", "apply-"+sanitizeUpdateName(info.Version))
	stage := filepath.Join(stageRoot, "stage")
	_ = os.RemoveAll(stageRoot)
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	if err := system.ExtractUpdateZip(zipPath, stage); err != nil {
		return fmt.Errorf("Update-ZIP konnte nicht entpackt werden: %w", err)
	}
	stageExe := filepath.Join(stage, "LogiMate.exe")
	if _, err := os.Stat(stageExe); err != nil {
		return fmt.Errorf("Update enthält keine LogiMate.exe")
	}
	if err := system.VerifySameAuthenticodePublisher(currentExe(), stageExe); err != nil {
		return fmt.Errorf("Portable Update wird nicht angewendet: %w", err)
	}
	logPath := filepath.Join(dataDir, "update.log")
	rotateLogFile(logPath, 1<<20, 3)
	token, err := createUpdateAuthorization(dataDir, updateAuthorization{Mode: "portable", PID: uint32(os.Getpid()), Stage: stage, Target: currentExe(), TargetDir: targetDir, Version: info.Version, StageRoot: stageRoot, LogPath: logPath})
	if err != nil {
		return fmt.Errorf("Update-Autorisierung konnte nicht erstellt werden: %w", err)
	}
	if err := launchNativeUpdateHelper(dataDir, "--apply-portable", token); err != nil {
		return err
	}
	logStartup("Portable native updater helper started for %s", info.Version)
	postMessage(mainWnd, WM_CLOSE, 0, 0)
	return nil
}

func sanitizeUpdateName(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return r.Replace(s)
}
