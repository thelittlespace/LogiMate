package openg27port

import "testing"

func TestPlanG27LifecyclePrefersNative(t *testing.T) {
	if got := PlanG27Lifecycle(true, true); got != G27LifecycleOpenNative {
		t.Fatalf("got %s", got)
	}
}
func TestPlanG27LifecycleSwitchesCompatOnlyWhenNeeded(t *testing.T) {
	if got := PlanG27Lifecycle(false, true); got != G27LifecycleSwitchCompat {
		t.Fatalf("got %s", got)
	}
}
func TestPlanG27LifecycleNotFound(t *testing.T) {
	if got := PlanG27Lifecycle(false, false); got != G27LifecycleNotFound {
		t.Fatalf("got %s", got)
	}
}
