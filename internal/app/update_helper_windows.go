//go:build windows

package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thelittlespace/LogiMate/internal/system"
)

type updateAuthorization struct {
	Token     string    `json:"token"`
	Mode      string    `json:"mode"`
	PID       uint32    `json:"pid"`
	NewExe    string    `json:"newExe,omitempty"`
	Target    string    `json:"target,omitempty"`
	Stage     string    `json:"stage,omitempty"`
	TargetDir string    `json:"targetDir,omitempty"`
	Version   string    `json:"version"`
	StageRoot string    `json:"stageRoot,omitempty"`
	LogPath   string    `json:"logPath"`
	CreatedAt time.Time `json:"createdAt"`
}

func pathWithin(child, parent string) bool {
	c, err1 := filepath.Abs(child)
	p, err2 := filepath.Abs(parent)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(p, c)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func createUpdateAuthorization(dataDir string, a updateAuthorization) (string, error) {
	if a.Mode != "standalone" && a.Mode != "portable" {
		return "", errors.New("ungültiger Update-Modus")
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	a.Token = hex.EncodeToString(buf)
	a.CreatedAt = time.Now().UTC()
	root := filepath.Join(dataDir, "Updates")
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	if a.Mode == "standalone" {
		if !pathWithin(a.NewExe, root) || !strings.EqualFold(filepath.Base(a.Target), "LogiMate.exe") {
			return "", errors.New("unsichere Standalone-Updatepfade")
		}
	} else {
		if !pathWithin(a.Stage, root) || !pathWithin(a.StageRoot, root) || strings.TrimSpace(a.TargetDir) == "" || strings.TrimSpace(a.Target) == "" {
			return "", errors.New("unsichere Portable-Updatepfade")
		}
		expected := filepath.Join(filepath.Clean(a.TargetDir), "LogiMate.exe")
		if !strings.EqualFold(filepath.Clean(a.Target), filepath.Clean(expected)) {
			return "", errors.New("Portable-Ziel-EXE stimmt nicht exakt mit dem autorisierten Zielordner überein")
		}
		if strings.EqualFold(filepath.Clean(a.TargetDir), filepath.Clean(root)) || pathWithin(a.TargetDir, root) {
			return "", errors.New("Portable-Ziel darf nicht im Update-Arbeitsbereich liegen")
		}
	}
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, "authorization-"+a.Token+".json")
	if err := system.AtomicWriteFile(path, b, 0600); err != nil {
		return "", err
	}
	return a.Token, nil
}

func consumeUpdateAuthorization(mode, token string) (updateAuthorization, error) {
	var a updateAuthorization
	if len(token) != 64 {
		return a, errors.New("ungültiger Update-Autorisierungstoken")
	}
	dataDir, _ := systemDataDirForHelper()
	root := filepath.Join(dataDir, "Updates")
	path := filepath.Join(root, "authorization-"+token+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return a, err
	}
	_ = os.Remove(path) // one-shot even on later validation failure
	if err := json.Unmarshal(b, &a); err != nil {
		return a, err
	}
	if a.Token != token || a.Mode != mode {
		return a, errors.New("Update-Autorisierung passt nicht zur Anfrage")
	}
	if a.CreatedAt.IsZero() || time.Since(a.CreatedAt) < -time.Minute || time.Since(a.CreatedAt) > 10*time.Minute {
		return a, errors.New("Update-Autorisierung ist abgelaufen")
	}
	if a.PID == 0 {
		return a, errors.New("Update-Autorisierung enthält keine Quell-PID")
	}
	if a.Mode == "standalone" {
		if !pathWithin(a.NewExe, root) || !strings.EqualFold(filepath.Base(a.Target), "LogiMate.exe") {
			return a, errors.New("Stand-alone Updatepfad außerhalb des autorisierten Bereichs")
		}
	} else {
		if !pathWithin(a.Stage, root) || !pathWithin(a.StageRoot, root) {
			return a, errors.New("Portable Stage außerhalb des autorisierten Bereichs")
		}
		expected := filepath.Join(filepath.Clean(a.TargetDir), "LogiMate.exe")
		if strings.TrimSpace(a.TargetDir) == "" || strings.TrimSpace(a.Target) == "" || !strings.EqualFold(filepath.Clean(a.Target), filepath.Clean(expected)) {
			return a, errors.New("ungültiges oder nicht exakt autorisiertes Portable-Ziel")
		}
		if strings.EqualFold(filepath.Clean(a.TargetDir), filepath.Clean(root)) || pathWithin(a.TargetDir, root) {
			return a, errors.New("Portable-Ziel liegt im Update-Arbeitsbereich")
		}
	}
	return a, nil
}

func rotateLogFile(path string, maxBytes int64, keep int) {
	st, err := os.Stat(path)
	if err != nil || st.Size() < maxBytes {
		return
	}
	if keep < 1 {
		keep = 1
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", path, keep))
	for i := keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1))
	}
	_ = os.Rename(path, path+".1")
}

func appendUpdateLog(path, text string) {
	if path == "" {
		return
	}
	rotateLogFile(path, 1<<20, 3)
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err == nil {
		_, _ = fmt.Fprintf(f, "[%s] %s\r\n", time.Now().Format(time.RFC3339), text)
		_ = f.Close()
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err = os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, cp := io.Copy(out, in)
	cerr := out.Close()
	if cp != nil {
		return cp
	}
	return cerr
}

func replaceExeTransactional(newExe, target string) (string, error) {
	tmp := target + ".new"
	previous := target + ".previous"
	_ = os.Remove(tmp)
	if err := copyFile(newExe, tmp); err != nil {
		return "", err
	}
	_ = os.Remove(previous)
	hadTarget := false
	if _, err := os.Stat(target); err == nil {
		if err := os.Rename(target, previous); err != nil {
			_ = os.Remove(tmp)
			return "", fmt.Errorf("alte EXE konnte nicht als Rückfallebene gesichert werden: %w", err)
		}
		hadTarget = true
	}
	if err := os.Rename(tmp, target); err != nil {
		if hadTarget {
			_ = os.Rename(previous, target)
		}
		_ = os.Remove(tmp)
		return "", fmt.Errorf("neue EXE konnte nicht aktiviert werden: %w", err)
	}
	if !hadTarget {
		return "", nil
	}
	return previous, nil
}

func rollbackExe(target, previous string) error {
	if previous == "" {
		return nil
	}
	if _, err := os.Stat(previous); err != nil {
		return err
	}
	failed := target + ".failed"
	_ = os.Remove(failed)
	if _, err := os.Stat(target); err == nil {
		_ = os.Rename(target, failed)
	}
	if err := os.Rename(previous, target); err != nil {
		return err
	}
	_ = os.Remove(failed)
	return nil
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, e := filepath.Rel(src, path)
		if e != nil {
			return e
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func runUpdateHelperIfRequested() (bool, int) {
	if len(os.Args) < 2 {
		return false, 0
	}
	modeArg := os.Args[1]
	if modeArg != "--apply-standalone" && modeArg != "--apply-portable" {
		return false, 0
	}
	if len(os.Args) != 3 {
		return true, 2
	}
	mode := "standalone"
	if modeArg == "--apply-portable" {
		mode = "portable"
	}
	a, err := consumeUpdateAuthorization(mode, os.Args[2])
	if err != nil {
		return true, 2
	}
	waitForProcessExit(a.PID)
	time.Sleep(350 * time.Millisecond)
	dataDir, _ := systemDataDirForHelper()
	if need, detail := system.RuntimeOutputRecoveryNeeded(dataDir); need {
		appendUpdateLog(a.LogPath, "ERROR: unresolved output recovery before update: "+detail)
		return true, 5
	}
	if mode == "standalone" {
		appendUpdateLog(a.LogPath, "Applying standalone update "+a.Version)
		previous, err := replaceExeTransactional(a.NewExe, a.Target)
		if err != nil {
			appendUpdateLog(a.LogPath, "ERROR: "+err.Error())
			return true, 3
		}
		if err := exec.Command(a.Target, "--post-update").Start(); err != nil {
			appendUpdateLog(a.LogPath, "ERROR restart: "+err.Error())
			if rbErr := rollbackExe(a.Target, previous); rbErr != nil {
				appendUpdateLog(a.LogPath, "ERROR rollback: "+rbErr.Error())
			} else if previous != "" {
				_ = exec.Command(a.Target).Start()
				appendUpdateLog(a.LogPath, "Previous version restored")
			}
			return true, 4
		}
		appendUpdateLog(a.LogPath, "Standalone update applied; previous EXE kept at "+previous)
		return true, 0
	}
	appendUpdateLog(a.LogPath, "Applying portable update "+a.Version)
	target := a.Target
	newExe := filepath.Join(a.Stage, "LogiMate.exe")
	previous, err := replaceExeTransactional(newExe, target)
	if err != nil {
		appendUpdateLog(a.LogPath, "ERROR EXE replace: "+err.Error())
		return true, 3
	}
	entries, _ := os.ReadDir(a.Stage)
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), "LogiMate.exe") {
			continue
		}
		src := filepath.Join(a.Stage, entry.Name())
		dst := filepath.Join(a.TargetDir, entry.Name())
		var copyErr error
		if entry.IsDir() {
			copyErr = copyTree(src, dst)
		} else {
			copyErr = copyFile(src, dst)
		}
		if copyErr != nil {
			appendUpdateLog(a.LogPath, "WARNING support file "+entry.Name()+": "+copyErr.Error())
		}
	}
	if err := exec.Command(target, "--post-update").Start(); err != nil {
		appendUpdateLog(a.LogPath, "ERROR restart: "+err.Error())
		if rbErr := rollbackExe(target, previous); rbErr != nil {
			appendUpdateLog(a.LogPath, "ERROR rollback: "+rbErr.Error())
		} else if previous != "" {
			_ = exec.Command(target).Start()
		}
		return true, 4
	}
	_ = os.RemoveAll(a.StageRoot)
	appendUpdateLog(a.LogPath, "Portable update applied")
	return true, 0
}

func launchNativeUpdateHelper(dataDir string, args ...string) error {
	dir := filepath.Join(dataDir, "Updates")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	helper := filepath.Join(dir, "LogiMate-Updater.exe")
	_ = os.Remove(helper)
	if err := copyFile(currentExe(), helper); err != nil {
		return err
	}
	cmd := exec.Command(helper, args...)
	return cmd.Start()
}

func cleanupOldUpdaterHelper() {
	dataDir, _ := systemDataDirForHelper()
	helper := filepath.Join(dataDir, "Updates", "LogiMate-Updater.exe")
	go func() { time.Sleep(2 * time.Second); _ = os.Remove(helper) }()
}

func systemDataDirForHelper() (string, bool) {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	// The native update helper is always copied to <DataDir>\Updates.
	// Resolve the data directory from that trusted placement first; this also
	// makes portable updates independent from LOCALAPPDATA.
	if strings.EqualFold(filepath.Base(exe), "LogiMate-Updater.exe") && strings.EqualFold(filepath.Base(dir), "Updates") {
		return filepath.Dir(dir), false
	}
	if _, err := os.Stat(filepath.Join(dir, "portable.flag")); err == nil {
		d := filepath.Join(dir, "LogiMateData")
		return d, true
	}
	base := os.Getenv("LOCALAPPDATA")
	if strings.TrimSpace(base) == "" {
		base = dir
	}
	return filepath.Join(base, "LogiMate"), false
}
