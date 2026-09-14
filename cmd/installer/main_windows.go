//go:build windows && installer

package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

//go:embed payload/LogiMate.exe
var payload []byte

var version = "0.0.1-alpha"
var buildID = "dev"

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	pShellExecuteW       = shell32.NewProc("ShellExecuteW")
	pIsUserAnAdmin       = shell32.NewProc("IsUserAnAdmin")
	pOpenProcess         = kernel32.NewProc("OpenProcess")
	pWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	pCloseHandle         = kernel32.NewProc("CloseHandle")
	pRegCreateKeyExW     = advapi32.NewProc("RegCreateKeyExW")
	pRegSetValueExW      = advapi32.NewProc("RegSetValueExW")
	pRegCloseKey         = advapi32.NewProc("RegCloseKey")
)

const (
	SW_SHOWNORMAL      = 1
	SYNCHRONIZE        = 0x00100000
	REG_SZ             = 1
	KEY_SET_VALUE      = 0x0002
	KEY_CREATE_SUB_KEY = 0x0004
	HKEY_LOCAL_MACHINE = uintptr(0x80000002)
)

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func admin() bool          { r, _, _ := pIsUserAnAdmin.Call(); return r != 0 }

func parseArgs() (update bool, waitPID int) {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--update":
			update = true
		case "--wait-pid":
			if i+1 < len(args) {
				waitPID, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
	}
	return
}

func elevate(update bool, waitPID int) bool {
	exe, _ := os.Executable()
	args := "--elevated"
	if update {
		args = "--update --elevated"
	}
	if waitPID > 0 {
		args += fmt.Sprintf(" --wait-pid %d", waitPID)
	}
	r, _, _ := pShellExecuteW.Call(0, uintptr(unsafe.Pointer(u16("runas"))), uintptr(unsafe.Pointer(u16(exe))), uintptr(unsafe.Pointer(u16(args))), 0, SW_SHOWNORMAL)
	return r > 32
}

func waitForProcess(pid int) bool {
	if pid <= 0 {
		return true
	}
	h, _, _ := pOpenProcess.Call(SYNCHRONIZE, 0, uintptr(pid))
	if h == 0 {
		// The process may already have exited between launch and this check.
		return true
	}
	defer pCloseHandle.Call(h)
	r, _, _ := pWaitForSingleObject.Call(h, 45000)
	return r == 0 // WAIT_OBJECT_0
}

func setRegString(path, name, value string) error {
	var h uintptr
	var disp uint32
	r, _, _ := pRegCreateKeyExW.Call(HKEY_LOCAL_MACHINE, uintptr(unsafe.Pointer(u16(path))), 0, 0, 0, KEY_SET_VALUE|KEY_CREATE_SUB_KEY, 0, uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(&disp)))
	if r != 0 || h == 0 {
		if r != 0 {
			return syscall.Errno(r)
		}
		return fmt.Errorf("Registry-Schlüssel konnte nicht geöffnet werden")
	}
	defer pRegCloseKey.Call(h)
	u, _ := syscall.UTF16FromString(value)
	r, _, _ = pRegSetValueExW.Call(h, uintptr(unsafe.Pointer(u16(name))), 0, REG_SZ, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)*2))
	if r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

func main() {
	updateMode, waitPID := parseArgs()
	if !admin() {
		// UAC itself is the only system-owned dialog left in the installer. If the
		// user cancels it, exit quietly instead of falling back to a classic
		// MessageBox that visually breaks the LogiMate setup experience.
		_ = elevate(updateMode, waitPID)
		return
	}
	runInstallerWindow(updateMode, func(report installReportFunc) (string, error) {
		return performInstall(updateMode, waitPID, report)
	})
}

func performInstall(updateMode bool, waitPID int, report installReportFunc) (string, error) {
	if report == nil {
		report = func(int, string, string, string) {}
	}
	var warnings []string
	if updateMode {
		report(4, "Laufende Instanz prüfen", "LogiMate wird erst ersetzt, nachdem die alte Instanz beendet wurde.", fmt.Sprintf("WaitForSingleObject(pid=%d, timeout=45000)", waitPID))
		if !waitForProcess(waitPID) {
			return "", fmt.Errorf("die laufende LogiMate-Instanz wurde innerhalb von 45 Sekunden nicht beendet; das Update wurde sicher abgebrochen")
		}
	}

	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	dir := filepath.Join(pf, "LogiMate")
	report(12, "Installationsordner vorbereiten", dir, `mkdir "`+dir+`"`)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	exe := filepath.Join(dir, "LogiMate.exe")
	tmpExe := exe + ".new"
	previousExe := exe + ".previous"
	_ = os.Remove(tmpExe)
	report(28, "Neue Programmdatei schreiben", "Die eingebettete LogiMate.exe wird zunächst als .new geschrieben.", `write "`+tmpExe+`"`)
	if err := os.WriteFile(tmpExe, payload, 0755); err != nil {
		return "", err
	}

	_ = os.Remove(previousExe)
	hadPrevious := false
	if _, err := os.Stat(exe); err == nil {
		report(40, "Rückfallebene anlegen", "Die bisherige EXE bleibt bis zur erfolgreichen Aktivierung als .previous erhalten.", `move "`+exe+`" "`+previousExe+`"`)
		if err := os.Rename(exe, previousExe); err != nil {
			_ = os.Remove(tmpExe)
			return "", fmt.Errorf("die bestehende LogiMate.exe konnte nicht sicher als Rückfallebene verschoben werden: %w", err)
		}
		hadPrevious = true
	}

	report(54, "Neue Version aktivieren", exe, `move "`+tmpExe+`" "`+exe+`"`)
	if err := os.Rename(tmpExe, exe); err != nil {
		if hadPrevious {
			_ = os.Rename(previousExe, exe)
		}
		_ = os.Remove(tmpExe)
		return "", fmt.Errorf("die neue LogiMate.exe konnte nicht aktiviert werden; die vorherige Version wurde soweit möglich wiederhergestellt: %w", err)
	}

	desktop := filepath.Join(os.Getenv("PUBLIC"), "Desktop", "LogiMate.lnk")
	startDir := filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "Start Menu", "Programs", "LogiMate")
	_ = os.MkdirAll(startDir, 0755)
	start := filepath.Join(startDir, "LogiMate.lnk")
	safeStart := filepath.Join(startDir, "LogiMate - Safe UI.lnk")
	uninstall := filepath.Join(startDir, "LogiMate deinstallieren.lnk")
	ps := fmt.Sprintf(`$w=New-Object -ComObject WScript.Shell; foreach($x in @(@('%s',''),@('%s',''),@('%s','--safe-ui'),@('%s','--uninstall'))){$s=$w.CreateShortcut($x[0]);$s.TargetPath='%s';$s.Arguments=$x[1];$s.WorkingDirectory='%s';$s.Description='LogiMate';$s.Save()}`,
		esc(desktop), esc(start), esc(safeStart), esc(uninstall), esc(exe), esc(dir))
	report(70, "Verknüpfungen erstellen", "Desktop, Startmenü, Safe UI und Deinstallation werden registriert.", `powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command <CreateShortcut>`)
	if err := run("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", ps); err != nil {
		warnings = append(warnings, "Windows-Verknüpfungen konnten nicht vollständig erstellt werden: "+err.Error())
		report(73, "Verknüpfungen", "Warnung: "+err.Error(), "")
	}

	regPath := `Software\Microsoft\Windows\CurrentVersion\Uninstall\LogiMate`
	regValues := [][2]string{
		{"DisplayName", "LogiMate"},
		{"DisplayVersion", strings.TrimPrefix(version, "v")},
		{"Publisher", "LogiMate contributors"},
		{"InstallLocation", dir},
		{"DisplayIcon", exe},
		{"UninstallString", `"` + exe + `" --uninstall`},
	}
	report(84, "Windows-Deinstallation registrieren", "Apps & Features erhält den LogiMate-Eintrag.", `HKLM\`+regPath)
	for _, item := range regValues {
		if err := setRegString(regPath, item[0], item[1]); err != nil {
			warnings = append(warnings, "Windows-Deinstallations-Eintrag konnte nicht vollständig geschrieben werden: "+err.Error())
			report(87, "Registry", "Warnung: "+err.Error(), "")
			break
		}
	}

	report(96, "Installation prüfen", "Programmdatei und Installationspfad sind aktiv.", `verify "`+exe+`"`)
	if _, err := os.Stat(exe); err != nil {
		if hadPrevious {
			_ = os.Remove(exe)
			_ = os.Rename(previousExe, exe)
		}
		return "", fmt.Errorf("die installierte Programmdatei konnte nach der Aktivierung nicht bestätigt werden: %w", err)
	}

	msg := "LogiMate " + strings.TrimPrefix(version, "v") + " (Build " + buildID + ") wurde erfolgreich installiert."
	if updateMode {
		msg = "LogiMate wurde auf " + strings.TrimPrefix(version, "v") + " · Build " + buildID + " aktualisiert. Beim nächsten Start erscheint ‚Was ist neu?‘."
	}
	if len(warnings) > 0 {
		msg += " Hinweise: " + strings.Join(warnings, " | ")
	}
	report(100, "Abgeschlossen", msg, "")
	return msg, nil
}

func run(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return c.Run()
}
func esc(s string) string { return strings.ReplaceAll(s, "'", "''") }
