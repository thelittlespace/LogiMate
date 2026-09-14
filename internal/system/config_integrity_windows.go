//go:build windows

package system

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func configCorruptMarker(path string) string { return path + ".corrupt" }

// PreserveCorruptConfig keeps the exact unreadable bytes before any caller can
// fall back to defaults. The marker deliberately blocks normal writes until a
// user explicitly resets/repairs the file.
func PreserveCorruptConfig(path string, data []byte, cause error) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("leerer Konfigurationspfad")
	}
	dir := filepath.Join(filepath.Dir(path), "Corrupt")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	backup := filepath.Join(dir, fmt.Sprintf("%s.%s.%s.corrupt", filepath.Base(path), stamp, hex.EncodeToString(sum[:6])))
	if err := AtomicWriteFile(backup, data, 0600); err != nil {
		return err
	}
	detail := "invalid JSON"
	if cause != nil {
		detail = cause.Error()
	}
	marker := fmt.Sprintf("LogiMate preserved a corrupt configuration.\nsource=%s\nbackup=%s\nerror=%s\nsha256=%x\n", filepath.Base(path), backup, detail, sum)
	return AtomicWriteFile(configCorruptMarker(path), []byte(marker), 0600)
}

func ConfigWriteAllowed(path string) error {
	if _, err := os.Stat(configCorruptMarker(path)); err == nil {
		return fmt.Errorf("%s ist als beschädigt markiert; Original wurde gesichert. Bitte die Einstellungen ausdrücklich zurücksetzen/reparieren, bevor LogiMate diese Datei überschreibt", filepath.Base(path))
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func ClearConfigCorruption(path string) error {
	err := os.Remove(configCorruptMarker(path))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func ReadJSONConfigStrict(path string, dst any) (bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, dst); err != nil {
		preserveErr := PreserveCorruptConfig(path, b, err)
		if preserveErr != nil {
			return true, errors.Join(fmt.Errorf("%s ist beschädigt: %w", filepath.Base(path), err), fmt.Errorf("Sicherung fehlgeschlagen: %w", preserveErr))
		}
		return true, fmt.Errorf("%s ist beschädigt; Original wurde gesichert: %w", filepath.Base(path), err)
	}
	return true, nil
}

func WriteJSONConfigStrict(path string, value any, perm os.FileMode) error {
	if err := ConfigWriteAllowed(path); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(path, b, perm)
}

func ResetJSONConfig(path string, value any, perm os.FileMode) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := AtomicWriteFile(path, b, perm); err != nil {
		return err
	}
	return ClearConfigCorruption(path)
}
