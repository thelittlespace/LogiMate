//go:build windows

package system

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ReleaseTrustState is the runtime view of the Authenticode trust attached to
// the exact executable that is currently running. Pre-release builds may be
// unsigned; Stable builds are never allowed to treat an unsigned executable as
// release-ready.
type ReleaseTrustState struct {
	Version        string
	Executable     string
	StableRequired bool
	CheckedAt      time.Time
	Status         string
	Subject        string
	Thumbprint     string
	Valid          bool
	Error          string
}

var releaseTrustCache struct {
	sync.Mutex
	exe     string
	version string
	at      time.Time
	state   ReleaseTrustState
}

func isStableVersion(version string) bool {
	version = strings.TrimSpace(version)
	return version != "" && !strings.EqualFold(version, "dev") && !strings.Contains(version, "-")
}

func inspectReleaseTrustPath(path, version string) ReleaseTrustState {
	st := ReleaseTrustState{
		Version:        strings.TrimSpace(version),
		Executable:     path,
		StableRequired: isStableVersion(version),
		CheckedAt:      time.Now(),
		Status:         "Unknown",
	}
	id, err := ReadAuthenticodeIdentity(path)
	st.Status = strings.TrimSpace(id.Status)
	st.Subject = strings.TrimSpace(id.Subject)
	st.Thumbprint = strings.TrimSpace(id.Thumbprint)
	if err == nil {
		st.Valid = true
		return st
	}
	st.Error = err.Error()
	return st
}

// InspectReleaseTrust inspects the current executable. The result is cached for
// a short period because Engine Health refreshes several times per second and
// PowerShell Authenticode validation is intentionally much heavier than normal
// live UI diagnostics.
func InspectReleaseTrust(version string) ReleaseTrustState {
	exe, err := os.Executable()
	if err != nil {
		return ReleaseTrustState{Version: version, StableRequired: isStableVersion(version), CheckedAt: time.Now(), Status: "Unavailable", Error: err.Error()}
	}
	now := time.Now()
	releaseTrustCache.Lock()
	defer releaseTrustCache.Unlock()
	if releaseTrustCache.exe == exe && releaseTrustCache.version == version && now.Sub(releaseTrustCache.at) < 30*time.Second {
		return releaseTrustCache.state
	}
	st := inspectReleaseTrustPath(exe, version)
	releaseTrustCache.exe, releaseTrustCache.version, releaseTrustCache.at, releaseTrustCache.state = exe, version, now, st
	return st
}

func ReleaseTrustSummary(version string) string {
	st := InspectReleaseTrust(version)
	mode := "Pre-release: unsigned binaries are permitted, but can never satisfy the Stable signing gate."
	if st.StableRequired {
		mode = "Stable: a valid Authenticode signature is mandatory."
	}
	signer := st.Subject
	if signer == "" {
		signer = "—"
	}
	thumb := st.Thumbprint
	if thumb == "" {
		thumb = "—"
	}
	status := st.Status
	if status == "" {
		status = "Unknown"
	}
	verdict := "PASS"
	if st.StableRequired && !st.Valid {
		verdict = "BLOCKED"
	} else if !st.Valid {
		verdict = "INFO"
	}
	detail := ""
	if st.Error != "" {
		detail = "\r\nDetail: " + st.Error
	}
	return fmt.Sprintf("Release Trust: %s\r\nVersion: %s\r\nAuthenticode: valid=%v status=%s\r\nSigner: %s\r\nThumbprint: %s\r\nPolicy: %s%s", verdict, st.Version, st.Valid, status, signer, thumb, mode, detail)
}

// StableAuthenticodeGate is deliberately strict and evaluates the exact
// running binary. It is useful for diagnostics and tests; the build pipeline
// independently verifies both LogiMate.exe and LogiMate-Setup-x64.exe so a
// signed app cannot mask an unsigned installer.
func StableAuthenticodeGate(version string) error {
	st := InspectReleaseTrust(version)
	if !st.StableRequired {
		return nil
	}
	if !st.Valid {
		return fmt.Errorf("Stable Authenticode gate blocked: %s", st.Error)
	}
	return nil
}
