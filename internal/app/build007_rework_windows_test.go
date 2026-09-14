//go:build windows

package app

import "testing"

func TestBuild007EffectivePercentPreservesMute(t *testing.T) {
	if got := multPercent(75, 100, 0); got != 0 {
		t.Fatalf("effective gain must preserve explicit 0%% mute, got %d", got)
	}
	if got := multPercent(80, 50, 100); got != 40 {
		t.Fatalf("effective gain chain mismatch: got %d want 40", got)
	}
}

func TestBuild003InlineChoiceCycles(t *testing.T) {
	if got := nextIntChoice([]int{900, 720, 540}, 720); got != 540 {
		t.Fatalf("range cycle: got %d", got)
	}
	if got := nextStringChoice([]string{"linear", "progressiv", "weich"}, "weich"); got != "linear" {
		t.Fatalf("curve cycle wrap: got %q", got)
	}
	if got := nextFloatChoice([]float64{0, .02, .05}, .05); got != 0 {
		t.Fatalf("deadzone cycle wrap: got %v", got)
	}
}
