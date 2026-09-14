//go:build windows

package system

import "testing"

func TestD510StableVersionPolicy(t *testing.T) {
	cases := []struct {
		version string
		stable  bool
	}{
		{"0.5.10-alpha", false},
		{"1.0.0-beta.1", false},
		{"1.0.0-rc.1", false},
		{"dev", false},
		{"", false},
		{"1.0.0", true},
		{"2.3.4", true},
	}
	for _, tc := range cases {
		if got := isStableVersion(tc.version); got != tc.stable {
			t.Fatalf("isStableVersion(%q)=%v want %v", tc.version, got, tc.stable)
		}
	}
}
