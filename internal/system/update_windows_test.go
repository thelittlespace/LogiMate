//go:build windows

package system

import "testing"

func TestCompareSemVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.3.3-alpha", "0.3.2-alpha", 1},
		{"v0.3.3-alpha", "0.3.3-alpha", 0},
		{"0.3.3", "0.3.3-rc", 1},
		{"0.3.3-beta", "0.3.3-alpha", 1},
		{"0.4.0-alpha", "0.3.99", 1},
		{"0.3.2", "0.3.3", -1},
	}
	for _, c := range cases {
		got := compareSemVersion(c.a, c.b)
		if got < 0 {
			got = -1
		} else if got > 0 {
			got = 1
		}
		if got != c.want {
			t.Fatalf("compareSemVersion(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSelectLogiMateUpdateAsset(t *testing.T) {
	info := &UpdateInfo{Assets: []UpdateAsset{{Name: "LogiMate.exe", BrowserDownloadURL: "exe"}, {Name: "LogiMate-Setup-x64.exe", BrowserDownloadURL: "setup"}, {Name: "LogiMate-Portable-x64.zip", BrowserDownloadURL: "zip"}}}
	a, err := SelectLogiMateUpdateAsset(info, UpdateInstalled)
	if err != nil || a.Name != "LogiMate-Setup-x64.exe" {
		t.Fatalf("installed: %#v %v", a, err)
	}
	a, err = SelectLogiMateUpdateAsset(info, UpdatePortable)
	if err != nil || a.Name != "LogiMate-Portable-x64.zip" {
		t.Fatalf("portable: %#v %v", a, err)
	}
	a, err = SelectLogiMateUpdateAsset(info, UpdateStandalone)
	if err != nil || a.Name != "LogiMate.exe" {
		t.Fatalf("standalone: %#v %v", a, err)
	}
}

func TestPinnedAlphaLine(t *testing.T) {
	cur, err := parseSemVersion("0.0.1-alpha.1")
	if err != nil || !isPinnedAlphaLine(cur) {
		t.Fatalf("current pinned line parse failed: %#v %v", cur, err)
	}
	oldInternal, _ := parseSemVersion("0.6.2-alpha")
	if samePinnedAlphaLine(cur, oldInternal) {
		t.Fatal("historical 0.6.x preview must not be in pinned 0.0.1-alpha line")
	}
	next, _ := parseSemVersion("0.0.1-alpha.2")
	if !samePinnedAlphaLine(cur, next) || compareSemVersion("0.0.1-alpha.2", "0.0.1-alpha.1") <= 0 {
		t.Fatal("next pinned build should be accepted and compare newer")
	}
}
