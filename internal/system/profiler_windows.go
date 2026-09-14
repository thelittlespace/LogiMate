//go:build windows

package system

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	legacyLGSSupportURL  = "https://support.logi.com/hc/de/articles/360025120194-Logitech-Gaming-Software"
	legacyLGSDownloadURL = "https://download01.logi.com/web/ftp/pub/techsupport/joystick/lgs510.exe"
)

func LegacyLGSSupportURL() string { return legacyLGSSupportURL }

// DownloadAndInstallOfficialLGS provides a fresh-machine Legacy path without
// redistributing Logitech's proprietary installer. The binary is downloaded
// directly from Logitech and must carry a valid Logitech Authenticode
// signature before LogiMate will execute it.
func DownloadAndInstallOfficialLGS(dataDir string) (string, error) {
	dir := filepath.Join(dataDir, "Downloads", "Legacy")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, "lgs510.exe")
	tmp := target + ".download"
	_ = os.Remove(tmp)

	client := &http.Client{Timeout: 3 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, legacyLGSDownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "LogiMate/LegacySetup")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("offizieller Logitech-Download fehlgeschlagen: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Logitech-Download antwortete mit HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > 64*1024*1024 {
		return "", errors.New("Logitech-Installer ist unerwartet groß; Download aus Sicherheitsgründen abgebrochen")
	}
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	n, cpErr := io.Copy(f, io.LimitReader(resp.Body, 64*1024*1024+1))
	closeErr := f.Close()
	if cpErr != nil {
		_ = os.Remove(tmp)
		return "", cpErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}
	if n <= 0 || n > 64*1024*1024 {
		_ = os.Remove(tmp)
		return "", errors.New("ungültige Größe des Logitech-Installers")
	}

	quoted := strings.ReplaceAll(tmp, "'", "''")
	ps := fmt.Sprintf("$s=Get-AuthenticodeSignature -LiteralPath '%s'; if($s.SignerCertificate){[Console]::WriteLine(($s.Status.ToString()+'|'+$s.SignerCertificate.Subject))}else{[Console]::WriteLine(($s.Status.ToString()+'|'))}", quoted)
	out, sigErr := RunPowerShellTimeout(ps, 20*time.Second)
	if sigErr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("Authenticode-Prüfung fehlgeschlagen: %w", sigErr)
	}
	parts := strings.SplitN(strings.TrimSpace(out), "|", 2)
	status, subject := "", ""
	if len(parts) > 0 {
		status = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		subject = strings.TrimSpace(parts[1])
	}
	if !strings.EqualFold(status, "Valid") || !strings.Contains(strings.ToLower(subject), "logitech") {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("Logitech-Installer wurde nicht ausgeführt: Authenticode=%q, Signer=%q", status, subject)
	}
	// A previous verified/downloaded copy may already exist. Windows' Rename
	// does not replace an existing destination reliably, so remove only the old
	// cached installer after the new download has passed Authenticode validation.
	_ = os.Remove(target)
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}

	cmd := exec.Command(target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: false}
	if err := cmd.Run(); err != nil {
		return target, fmt.Errorf("Logitech-LGS-Installer meldete einen Fehler: %w", err)
	}
	invalidateProfilerCache()
	return target, nil
}

type ProfilerInfo struct {
	Installed            bool   `json:"installed"`
	DisplayName          string `json:"displayName"`
	DisplayVersion       string `json:"displayVersion"`
	InstallLocation      string `json:"installLocation"`
	InstallSource        string `json:"installSource"`
	UninstallString      string `json:"uninstallString"`
	QuietUninstallString string `json:"quietUninstallString"`
}

type profilerBackupManifest struct {
	Created          string            `json:"created"`
	Complete         bool              `json:"complete"`
	Profiler         ProfilerInfo      `json:"profiler"`
	CopiedPaths      []string          `json:"copiedPaths"`
	RegistryExported bool              `json:"registryExported"`
	InstallerFile    string            `json:"installerFile,omitempty"`
	SHA256           map[string]string `json:"sha256,omitempty"`
}

var profilerCache = struct {
	sync.Mutex
	at   time.Time
	info ProfilerInfo
	err  error
}{}

func DetectProfilerStrict() (ProfilerInfo, error) {
	// LGS generations use different installer technologies, so query both
	// 32-bit and 64-bit uninstall views. A PowerShell failure is reported and
	// never silently reinterpreted as "not installed".
	ps := `$keys=@('HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*','HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*');` +
		`$x=Get-ItemProperty $keys -ErrorAction Stop | Where-Object {$_.DisplayName -match 'Logitech (Gaming Software|Profiler)' -or $_.DisplayName -match '^Logitech Gaming Software'} | Sort-Object DisplayVersion -Descending | Select-Object -First 1 DisplayName,DisplayVersion,InstallLocation,InstallSource,UninstallString,QuietUninstallString;` +
		`if($x){$x|ConvertTo-Json -Compress}`
	out, err := RunPowerShellTimeout(ps, 5*time.Second)
	if err != nil {
		if IsProcessRunning("LCore.exe") {
			return ProfilerInfo{Installed: true, DisplayName: "Logitech Gaming Software / Profiler"}, fmt.Errorf("Profiler-Metadaten konnten nicht gelesen werden: %w", err)
		}
		return ProfilerInfo{}, fmt.Errorf("Profiler-Metadaten konnten nicht gelesen werden: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		if IsProcessRunning("LCore.exe") {
			return ProfilerInfo{Installed: true, DisplayName: "Logitech Gaming Software / Profiler"}, nil
		}
		return ProfilerInfo{}, nil
	}
	var p ProfilerInfo
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		return ProfilerInfo{}, fmt.Errorf("Profiler-Metadaten konnten nicht ausgewertet werden: %w", err)
	}
	p.Installed = p.DisplayName != ""
	return p, nil
}

func detectProfilerCached(maxAge time.Duration) (ProfilerInfo, error) {
	profilerCache.Lock()
	if !profilerCache.at.IsZero() && time.Since(profilerCache.at) < maxAge {
		p, err := profilerCache.info, profilerCache.err
		profilerCache.Unlock()
		return p, err
	}
	profilerCache.Unlock()
	p, err := DetectProfilerStrict()
	profilerCache.Lock()
	profilerCache.at, profilerCache.info, profilerCache.err = time.Now(), p, err
	profilerCache.Unlock()
	return p, err
}

func invalidateProfilerCache() {
	profilerCache.Lock()
	profilerCache.at = time.Time{}
	profilerCache.Unlock()
}

func DetectProfiler() ProfilerInfo {
	p, _ := detectProfilerCached(30 * time.Second)
	return p
}

func profilerSummary(info ProfilerInfo, err error) string {
	if err != nil && !info.Installed {
		return "Status konnte nicht zuverlässig gelesen werden"
	}
	if !info.Installed {
		return "Nicht installiert"
	}
	if info.DisplayVersion != "" {
		return fmt.Sprintf("%s %s", info.DisplayName, info.DisplayVersion)
	}
	return info.DisplayName
}

func profilerBackupRoot(dataDir string) string { return filepath.Join(dataDir, "ProfilerBackups") }

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(out, in)
	closeErr := out.Close()
	if cpErr != nil {
		return cpErr
	}
	return closeErr
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func findProfilerInstallerCandidate(info ProfilerInfo) string {
	roots := []string{info.InstallSource}
	for _, root := range roots {
		root = strings.TrimSpace(strings.Trim(root, `"`))
		if root == "" {
			continue
		}
		st, err := os.Stat(root)
		if err == nil && !st.IsDir() {
			ext := strings.ToLower(filepath.Ext(root))
			if ext == ".exe" || ext == ".msi" {
				return root
			}
			continue
		}
		if err != nil || !st.IsDir() {
			continue
		}
		var best string
		_ = filepath.Walk(root, func(path string, fi os.FileInfo, e error) error {
			if e != nil || fi == nil || fi.IsDir() {
				return nil
			}
			n := strings.ToLower(fi.Name())
			ext := strings.ToLower(filepath.Ext(n))
			if (ext == ".exe" || ext == ".msi") && (strings.Contains(n, "lgs") || strings.Contains(n, "logitech")) {
				best = path
				return filepath.SkipDir
			}
			return nil
		})
		if best != "" {
			return best
		}
	}
	return ""
}

func BackupProfiler(dataDir string) (string, error) {
	stamp := time.Now().Format("2006-01-02_150405")
	dst := filepath.Join(profilerBackupRoot(dataDir), stamp)
	if err := os.MkdirAll(dst, 0755); err != nil {
		return "", err
	}

	info, detectErr := DetectProfilerStrict()
	if detectErr != nil {
		return "", detectErr
	}
	m := profilerBackupManifest{Created: time.Now().Format(time.RFC3339), Profiler: info, SHA256: map[string]string{}}
	candidates := []struct{ src, name string }{
		{filepath.Join(os.Getenv("LOCALAPPDATA"), "Logitech", "Gaming Software"), "LocalAppData_GamingSoftware"},
		{filepath.Join(os.Getenv("LOCALAPPDATA"), "Logitech", "Logitech Gaming Software"), "LocalAppData_LogitechGamingSoftware"},
		{filepath.Join(os.Getenv("APPDATA"), "Logitech", "Gaming Software"), "RoamingAppData_GamingSoftware"},
		{filepath.Join(os.Getenv("APPDATA"), "Logitech", "Logitech Gaming Software"), "RoamingAppData_LogitechGamingSoftware"},
	}
	for _, c := range candidates {
		if c.src == "" {
			continue
		}
		if _, err := os.Stat(c.src); err == nil {
			if err := copyTree(c.src, filepath.Join(dst, "files", c.name)); err != nil {
				return "", err
			}
			m.CopiedPaths = append(m.CopiedPaths, c.src)
		}
	}

	regFile := filepath.Join(dst, "Logitech-Gaming-Software-HKCU.reg")
	if _, err := RunHiddenTimeout(10*time.Second, "reg.exe", "export", `HKCU\Software\Logitech\Gaming Software`, regFile, "/y"); err == nil {
		if _, statErr := os.Stat(regFile); statErr == nil {
			m.RegistryExported = true
		}
	}

	if installer := findProfilerInstallerCandidate(info); installer != "" {
		out := filepath.Join(dst, "installer", filepath.Base(installer))
		if copyFile(installer, out) == nil {
			m.InstallerFile = filepath.Base(installer)
		}
	}

	// Hash every payload file before the manifest is marked complete. The
	// manifest itself is excluded because it contains the hashes.
	if err := filepath.Walk(dst, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi == nil || fi.IsDir() || strings.EqualFold(fi.Name(), "profiler-backup.json") {
			return nil
		}
		rel, err := filepath.Rel(dst, path)
		if err != nil {
			return err
		}
		sum, err := fileSHA256(path)
		if err != nil {
			return err
		}
		m.SHA256[filepath.ToSlash(rel)] = sum
		return nil
	}); err != nil {
		return "", fmt.Errorf("Profiler-Backup konnte nicht vollständig gehasht werden: %w", err)
	}
	m.Complete = true
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	if err := AtomicWriteFile(filepath.Join(dst, "profiler-backup.json"), b, 0644); err != nil {
		return "", err
	}
	return dst, nil
}

func latestProfilerBackup(dataDir string) (string, profilerBackupManifest, error) {
	entries, err := os.ReadDir(profilerBackupRoot(dataDir))
	if err != nil {
		return "", profilerBackupManifest{}, err
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			continue
		}
		dir := filepath.Join(profilerBackupRoot(dataDir), entries[i].Name())
		b, err := os.ReadFile(filepath.Join(dir, "profiler-backup.json"))
		if err != nil {
			continue
		}
		var m profilerBackupManifest
		if json.Unmarshal(b, &m) == nil {
			return dir, m, nil
		}
	}
	return "", profilerBackupManifest{}, errors.New("kein vollständiges Profiler-Backup gefunden")
}

func validateProfilerBackup(dir string, m profilerBackupManifest) error {
	// Pre-0.0.5 backups did not have Complete/SHA256 fields. Keep them usable
	// for rollback, but all new backups are fully verified.
	if !m.Complete && len(m.SHA256) > 0 {
		return errors.New("Profiler-Backup ist nicht als vollständig markiert")
	}
	for rel, want := range m.SHA256 {
		clean := filepath.Clean(filepath.FromSlash(rel))
		if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("ungültiger Pfad im Profiler-Backup: %s", rel)
		}
		got, err := fileSHA256(filepath.Join(dir, clean))
		if err != nil {
			return fmt.Errorf("Profiler-Backup-Datei fehlt/beschädigt (%s): %w", rel, err)
		}
		if !strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(want)) {
			return fmt.Errorf("Profiler-Backup SHA-256 stimmt nicht: %s", rel)
		}
	}
	return nil
}

func UninstallProfiler(info ProfilerInfo) error {
	if !info.Installed {
		return nil
	}
	cmdline := strings.TrimSpace(info.QuietUninstallString)
	if cmdline == "" {
		cmdline = strings.TrimSpace(info.UninstallString)
	}
	if cmdline == "" {
		return errors.New("Profiler erkannt, aber kein Deinstallationsbefehl wurde gefunden")
	}
	lower := strings.ToLower(cmdline)
	if strings.Contains(lower, "msiexec") && strings.Contains(lower, " /i") {
		cmdline = strings.Replace(cmdline, " /I", " /X", 1)
		cmdline = strings.Replace(cmdline, " /i", " /X", 1)
	}
	_, err := RunHiddenTimeout(5*time.Minute, "cmd.exe", "/d", "/s", "/c", cmdline)
	return err
}

func RestoreProfilerBackup(dataDir string, tryInstall bool) (string, error) {
	dir, m, err := latestProfilerBackup(dataDir)
	if err != nil {
		return "", err
	}
	if err := validateProfilerBackup(dir, m); err != nil {
		return "", fmt.Errorf("Profiler-Backup-Integritätsprüfung fehlgeschlagen: %w", err)
	}
	var notes []string

	if current, _ := DetectProfilerStrict(); tryInstall && !current.Installed && m.InstallerFile != "" {
		installer := filepath.Join(dir, "installer", m.InstallerFile)
		ext := strings.ToLower(filepath.Ext(installer))
		if ext == ".msi" {
			if _, err := RunHiddenTimeout(10*time.Minute, "msiexec.exe", "/i", installer); err != nil {
				notes = append(notes, "Profiler-Installer fehlgeschlagen: "+err.Error())
			}
		} else {
			if _, err := RunHiddenTimeout(10*time.Minute, installer); err != nil {
				notes = append(notes, "Profiler-Installer fehlgeschlagen: "+err.Error())
			}
		}
	}

	restore := []struct{ src, dst string }{
		{filepath.Join(dir, "files", "LocalAppData_GamingSoftware"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Logitech", "Gaming Software")},
		{filepath.Join(dir, "files", "LocalAppData_LogitechGamingSoftware"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Logitech", "Logitech Gaming Software")},
		{filepath.Join(dir, "files", "RoamingAppData_GamingSoftware"), filepath.Join(os.Getenv("APPDATA"), "Logitech", "Gaming Software")},
		{filepath.Join(dir, "files", "RoamingAppData_LogitechGamingSoftware"), filepath.Join(os.Getenv("APPDATA"), "Logitech", "Logitech Gaming Software")},
	}
	for _, r := range restore {
		if _, err := os.Stat(r.src); err == nil {
			if err := copyTree(r.src, r.dst); err != nil {
				notes = append(notes, "Settings konnten nicht vollständig kopiert werden: "+err.Error())
			}
		}
	}
	regFile := filepath.Join(dir, "Logitech-Gaming-Software-HKCU.reg")
	if _, err := os.Stat(regFile); err == nil {
		if _, err := RunHiddenTimeout(10*time.Second, "reg.exe", "import", regFile); err != nil {
			notes = append(notes, "Registry-Import fehlgeschlagen: "+err.Error())
		}
	}

	if current, _ := DetectProfilerStrict(); tryInstall && !current.Installed && m.InstallerFile == "" {
		notes = append(notes, "Kein vollständiger LGS/Profiler-Installer war im ursprünglichen InstallSource verfügbar. Treiber und Einstellungen wurden trotzdem wiederhergestellt; die Profiler-Anwendung muss bei Bedarf separat installiert werden.")
	}
	if len(notes) == 0 {
		return "Profiler-Einstellungen wurden wiederhergestellt.", nil
	}
	return strings.Join(notes, "\r\n"), nil
}

// ProfilerSummary is intentionally cache-only. UI paint/update paths must never
// be able to trigger PowerShell. CollectState performs the real probe on its
// worker goroutine and stores the result in this cache.
func ProfilerSummary() string {
	profilerCache.Lock()
	defer profilerCache.Unlock()
	if profilerCache.at.IsZero() {
		return "Noch nicht geprüft"
	}
	return profilerSummary(profilerCache.info, profilerCache.err)
}
