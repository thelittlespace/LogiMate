//go:build windows

package system

// D2 canonical runtime names. Historical Fusion-C6 entry points remain as
// compatibility wrappers for old tests/diagnostics, but normal runtime code
// uses the LogiMate-native names below.
type NativeGameOutputStatus = FusionC6Status

func NativeGameOutputSnapshot() NativeGameOutputStatus { return FusionC6Snapshot() }
func StartNativeGameOutput(s State, p GameProfile, process string) error {
	return StartFusionC6GameOutput(s, p, process)
}
func StopNativeGameOutput(reason string) { StopFusionC6GameOutput(reason) }
func NativeGameOutputSummary() string    { return FusionC6Summary() }
