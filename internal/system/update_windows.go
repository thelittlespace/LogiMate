//go:build windows

package system

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const DefaultLogiMateRepository = "thelittlespace/LogiMate"

type UpdateAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
	Size               int64  `json:"size"`
}

type UpdateInfo struct {
	TagName     string
	Version     string
	Name        string
	Body        string
	Prerelease  bool
	PublishedAt time.Time
	Assets      []UpdateAsset
	Repository  string
}

type UpdateTarget int

const (
	UpdateInstalled UpdateTarget = iota
	UpdatePortable
	UpdateStandalone
)

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	PublishedAt time.Time     `json:"published_at"`
	Assets      []UpdateAsset `json:"assets"`
}

type semVersion struct {
	major, minor, patch int
	pre                 string
}

func parseSemVersion(raw string) (semVersion, error) {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "v"))
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	pre := ""
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = strings.ToLower(strings.TrimSpace(s[i+1:]))
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return semVersion{}, fmt.Errorf("ungültige Version %q", raw)
	}
	nums := []int{0, 0, 0}
	for i := range parts {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return semVersion{}, fmt.Errorf("ungültige Version %q", raw)
		}
		nums[i] = n
	}
	return semVersion{major: nums[0], minor: nums[1], patch: nums[2], pre: pre}, nil
}

func compareSemVersion(a, b string) int {
	av, ae := parseSemVersion(a)
	bv, be := parseSemVersion(b)
	if ae != nil || be != nil {
		return strings.Compare(strings.TrimPrefix(strings.ToLower(a), "v"), strings.TrimPrefix(strings.ToLower(b), "v"))
	}
	if av.major != bv.major {
		if av.major < bv.major {
			return -1
		}
		return 1
	}
	if av.minor != bv.minor {
		if av.minor < bv.minor {
			return -1
		}
		return 1
	}
	if av.patch != bv.patch {
		if av.patch < bv.patch {
			return -1
		}
		return 1
	}
	if av.pre == bv.pre {
		return 0
	}
	if av.pre == "" {
		return 1
	}
	if bv.pre == "" {
		return -1
	}
	return comparePrerelease(av.pre, bv.pre)
}

func comparePrerelease(a, b string) int {
	aa := strings.FieldsFunc(strings.ToLower(a), func(r rune) bool { return r == '.' || r == '-' || r == '_' })
	bb := strings.FieldsFunc(strings.ToLower(b), func(r rune) bool { return r == '.' || r == '-' || r == '_' })
	rank := func(s string) int {
		switch {
		case strings.HasPrefix(s, "alpha"), s == "a":
			return 10
		case strings.HasPrefix(s, "beta"), s == "b":
			return 20
		case strings.HasPrefix(s, "rc"):
			return 30
		default:
			return 15
		}
	}
	max := len(aa)
	if len(bb) > max {
		max = len(bb)
	}
	for i := 0; i < max; i++ {
		if i >= len(aa) {
			return -1
		}
		if i >= len(bb) {
			return 1
		}
		ai, aerr := strconv.Atoi(aa[i])
		bi, berr := strconv.Atoi(bb[i])
		if aerr == nil && berr == nil {
			if ai < bi {
				return -1
			}
			if ai > bi {
				return 1
			}
			continue
		}
		ar, br := rank(aa[i]), rank(bb[i])
		if ar != br {
			if ar < br {
				return -1
			}
			return 1
		}
		if aa[i] < bb[i] {
			return -1
		}
		if aa[i] > bb[i] {
			return 1
		}
	}
	return 0
}

func isPinnedAlphaLine(v semVersion) bool {
	return v.major == 0 && v.minor == 0 && v.patch == 1 && (v.pre == "alpha" || strings.HasPrefix(v.pre, "alpha."))
}

func samePinnedAlphaLine(a, b semVersion) bool {
	return isPinnedAlphaLine(a) && isPinnedAlphaLine(b)
}

func CheckLatestLogiMate(currentVersion string, includePrerelease bool) (*UpdateInfo, error) {
	return CheckLatestLogiMateFromRepo(DefaultLogiMateRepository, currentVersion, includePrerelease)
}

func CheckLatestLogiMateFromRepo(repository, currentVersion string, includePrerelease bool) (*UpdateInfo, error) {
	currentSem, err := parseSemVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("lokale LogiMate-Version ist nicht SemVer-kompatibel: %w", err)
	}
	repository = strings.Trim(strings.TrimSpace(repository), "/")
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(repository, "\\?#") {
		return nil, errors.New("LogiMate Update-Repository ist nicht sicher konfiguriert")
	}
	url := "https://api.github.com/repos/" + repository + "/releases?per_page=20"
	client := &http.Client{Timeout: 25 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "LogiMate/"+strings.TrimPrefix(currentVersion, "v"))
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("GitHub-Repository %s wurde noch nicht veröffentlicht oder ist nicht erreichbar", repository)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("GitHub API: HTTP %d", resp.StatusCode)
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	var best *UpdateInfo
	for _, r := range releases {
		if r.Draft || strings.TrimSpace(r.TagName) == "" {
			continue
		}
		candidateSem, err := parseSemVersion(r.TagName)
		if err != nil {
			continue
		}
		// While the public version is pinned at 0.0.1-alpha, only build
		// revisions from that exact line are eligible. Historical internal
		// 0.5/0.6 tags must never be offered as upgrades.
		if isPinnedAlphaLine(currentSem) && !samePinnedAlphaLine(currentSem, candidateSem) {
			continue
		}
		if r.Prerelease && !includePrerelease {
			continue
		}
		if compareSemVersion(r.TagName, currentVersion) <= 0 {
			continue
		}
		info := &UpdateInfo{TagName: r.TagName, Version: strings.TrimPrefix(r.TagName, "v"), Name: r.Name, Body: r.Body, Prerelease: r.Prerelease, PublishedAt: r.PublishedAt, Assets: r.Assets, Repository: repository}
		if best == nil || compareSemVersion(info.Version, best.Version) > 0 {
			best = info
		}
	}
	return best, nil
}

func SelectLogiMateUpdateAsset(info *UpdateInfo, target UpdateTarget) (UpdateAsset, error) {
	if info == nil {
		return UpdateAsset{}, errors.New("Updateinformationen fehlen")
	}
	preferred := []string{"LogiMate-Setup-x64.exe", "LogiMate.exe"}
	label := "Installation"
	switch target {
	case UpdatePortable:
		preferred = []string{"LogiMate-Portable-x64.zip"}
		label = "Portable"
	case UpdateStandalone:
		preferred = []string{"LogiMate.exe"}
		label = "Standalone"
	}
	for _, name := range preferred {
		for _, a := range info.Assets {
			if strings.EqualFold(a.Name, name) && a.BrowserDownloadURL != "" {
				return a, nil
			}
		}
	}
	return UpdateAsset{}, fmt.Errorf("passendes Update-Asset für %s nicht gefunden", label)
}

func DownloadLogiMateUpdate(info *UpdateInfo, target UpdateTarget, dataDir string, progress func(string)) (string, error) {
	asset, err := SelectLogiMateUpdateAsset(info, target)
	if err != nil {
		return "", err
	}
	const maxUpdateBytes int64 = 256 << 20
	if asset.Size > maxUpdateBytes {
		return "", fmt.Errorf("Update-Asset ist unerwartet groß (%d Bytes)", asset.Size)
	}
	root := filepath.Join(dataDir, "Updates", sanitize(info.Version))
	if err := os.MkdirAll(root, 0755); err != nil {
		return "", err
	}
	finalPath := filepath.Join(root, asset.Name)
	partial := finalPath + ".partial"
	_ = os.Remove(partial)

	if progress != nil {
		progress("LogiMate " + info.Version + " wird heruntergeladen …")
	}
	if err := validateGitHubReleaseAssetURL(asset.BrowserDownloadURL); err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 7 * time.Minute}
	req, _ := http.NewRequest("GET", asset.BrowserDownloadURL, nil)
	req.Header.Set("User-Agent", "LogiMate-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("Update-Download: HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(partial)
	if err != nil {
		return "", err
	}
	written, cpErr := io.Copy(f, io.LimitReader(resp.Body, maxUpdateBytes+1))
	closeErr := f.Close()
	if cpErr != nil {
		_ = os.Remove(partial)
		return "", cpErr
	}
	if written > maxUpdateBytes {
		_ = os.Remove(partial)
		return "", errors.New("Update-Download überschreitet das Sicherheitslimit")
	}
	if asset.Size > 0 && written != asset.Size {
		_ = os.Remove(partial)
		return "", fmt.Errorf("Update-Download ist unvollständig (erwartet %d Bytes, erhalten %d)", asset.Size, written)
	}
	if closeErr != nil {
		_ = os.Remove(partial)
		return "", closeErr
	}

	if progress != nil {
		progress("Download wird per SHA-256 geprüft …")
	}
	want := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(asset.Digest), "sha256:"))
	if want == "" {
		want, _ = fetchReleaseSHA256(info, asset.Name)
	}
	if want == "" {
		_ = os.Remove(partial)
		return "", errors.New("Sicherheitsabbruch: GitHub-Release enthält keine SHA-256-Prüfsumme für " + asset.Name)
	}
	got, err := fileSHA256(partial)
	if err != nil {
		_ = os.Remove(partial)
		return "", err
	}
	if !strings.EqualFold(got, want) {
		_ = os.Remove(partial)
		return "", fmt.Errorf("SHA-256 stimmt nicht überein (erwartet %s, erhalten %s)", want, got)
	}
	_ = os.Remove(finalPath)
	if err := os.Rename(partial, finalPath); err != nil {
		return "", err
	}
	if progress != nil {
		progress("Update " + info.Version + " ist vollständig heruntergeladen und geprüft.")
	}
	return finalPath, nil
}

func fetchReleaseSHA256(info *UpdateInfo, assetName string) (string, error) {
	var sums UpdateAsset
	found := false
	for _, a := range info.Assets {
		if strings.EqualFold(a.Name, "SHA256SUMS.txt") {
			sums = a
			found = true
			break
		}
	}
	if !found || sums.BrowserDownloadURL == "" {
		return "", errors.New("SHA256SUMS.txt fehlt")
	}
	if err := validateGitHubReleaseAssetURL(sums.BrowserDownloadURL); err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	req, _ := http.NewRequest("GET", sums.BrowserDownloadURL, nil)
	req.Header.Set("User-Agent", "LogiMate-Updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("SHA256SUMS HTTP %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 2<<20))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(strings.TrimPrefix(fields[len(fields)-1], "*"), assetName) {
			sum := strings.ToLower(strings.TrimSpace(fields[0]))
			if len(sum) == 64 {
				return sum, nil
			}
		}
	}
	return "", sc.Err()
}

func validateGitHubReleaseAssetURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
		return errors.New("Sicherheitsabbruch: Update-Asset verwendet keine gültige HTTPS-Adresse")
	}
	host := strings.ToLower(u.Hostname())
	allowed := host == "github.com" || host == "objects.githubusercontent.com" || strings.HasSuffix(host, ".githubusercontent.com")
	if !allowed {
		return fmt.Errorf("Sicherheitsabbruch: unerwarteter Update-Downloadhost %q", host)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ExtractUpdateZip(src, dst string) error { return unzip(src, dst) }

// AuthenticodeIdentity returns the trusted signer identity of an executable.
// A hash alone proves integrity only against release metadata; this establishes
// an independent Windows publisher trust chain.
type AuthenticodeIdentity struct {
	Status     string
	Thumbprint string
	Subject    string
}

func ReadAuthenticodeIdentity(path string) (AuthenticodeIdentity, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return AuthenticodeIdentity{}, errors.New("leerer Authenticode-Pfad")
	}
	if _, err := os.Stat(path); err != nil {
		return AuthenticodeIdentity{}, err
	}
	quoted := strings.ReplaceAll(path, "'", "''")
	ps := fmt.Sprintf("$s=Get-AuthenticodeSignature -LiteralPath '%s'; $tp='';$sub=''; if($s.SignerCertificate){$tp=$s.SignerCertificate.Thumbprint;$sub=$s.SignerCertificate.Subject}; [Console]::Write(($s.Status.ToString()+'|'+$tp+'|'+$sub))", quoted)
	out, err := RunPowerShellTimeout(ps, 20*time.Second)
	if err != nil {
		return AuthenticodeIdentity{}, err
	}
	parts := strings.SplitN(strings.TrimSpace(out), "|", 3)
	id := AuthenticodeIdentity{}
	if len(parts) > 0 {
		id.Status = strings.TrimSpace(parts[0])
	}
	if len(parts) > 1 {
		id.Thumbprint = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		id.Subject = strings.TrimSpace(parts[2])
	}
	if !strings.EqualFold(id.Status, "Valid") || id.Thumbprint == "" {
		return id, fmt.Errorf("Authenticode ist nicht gültig (Status=%s, Signer=%s)", id.Status, id.Subject)
	}
	return id, nil
}

func VerifySameAuthenticodePublisher(currentExe, candidateExe string) error {
	current, err := ReadAuthenticodeIdentity(currentExe)
	if err != nil {
		return fmt.Errorf("laufende LogiMate-Version besitzt keine verifizierbare Publisher-Identität; automatisches Ersetzen ist gesperrt: %w", err)
	}
	candidate, err := ReadAuthenticodeIdentity(candidateExe)
	if err != nil {
		return fmt.Errorf("Update besitzt keine gültige Publisher-Signatur: %w", err)
	}
	if !strings.EqualFold(current.Thumbprint, candidate.Thumbprint) {
		return fmt.Errorf("Publisher-Zertifikat stimmt nicht überein (aktuell=%s, Update=%s)", current.Subject, candidate.Subject)
	}
	return nil
}
