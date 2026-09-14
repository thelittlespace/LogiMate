package openg27port

const (
	LogitechVID  = 0x046D
	G27NativePID = 0xC29B
	G27CompatPID = 0xC294
)

type G27LifecycleAction string

const (
	G27LifecycleNotFound     G27LifecycleAction = "not-found"
	G27LifecycleOpenNative   G27LifecycleAction = "open-native"
	G27LifecycleSwitchCompat G27LifecycleAction = "switch-compat-to-native"
)

// PlanG27Lifecycle mirrors OpenG27.G27Device.Connect's selection contract:
// prefer an already-native C29B device to avoid USB re-enumeration; only when
// native is absent and C294 exists should the native-mode switch be sent.
func PlanG27Lifecycle(hasNativeC29B, hasCompatC294 bool) G27LifecycleAction {
	if hasNativeC29B {
		return G27LifecycleOpenNative
	}
	if hasCompatC294 {
		return G27LifecycleSwitchCompat
	}
	return G27LifecycleNotFound
}
