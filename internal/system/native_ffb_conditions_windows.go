//go:build windows

package system

import (
	"errors"
	"fmt"
	"time"
)

type nativeConditionKind struct{ name string }

var (
	nativeSpringKind   = nativeConditionKind{name: "spring-test"}
	nativeDamperKind   = nativeConditionKind{name: "damper-test"}
	nativeFrictionKind = nativeConditionKind{name: "friction-test"}
)

func percentToByte(percent int) byte {
	percent = clampInt(percent, 0, 100)
	if percent == 0 {
		return 0
	}
	v := (percent*255 + 50) / 100
	if v < 1 {
		v = 1
	}
	return byte(v)
}

func percentToNibble(percent int) byte {
	percent = clampInt(percent, 0, 100)
	if percent == 0 {
		return 0
	}
	v := (percent*15 + 50) / 100
	if v < 1 {
		v = 1
	}
	if v > 15 {
		v = 15
	}
	return byte(v)
}

func percentToSpringSlope(percent int) byte {
	percent = clampInt(percent, 0, 100)
	if percent == 0 {
		return 0
	}
	// The OpenG27/lg4ff spring slope is a small 0..7 field. Keep the live test
	// proportional to the already safety-limited percentage instead of mapping a
	// 30% Alpha cap back to 100% protocol strength.
	v := (percent*7 + 50) / 100
	if v < 1 {
		v = 1
	}
	if v > 7 {
		v = 7
	}
	return byte(v)
}

// nativeConditionReports builds the real new-lg4ff slot protocol used by the
// G27 firmware: Spring=slot 1, Damper=slot 2, Friction=slot 3. Build 009 used
// the simplified OpenG27 helper packets for every condition test; notably its
// Damper/Friction helpers both addressed slot 1. Real-hardware Build 010 proved
// HID output but only the special FE/0D spring transaction was perceptible.
// Build 011 therefore uses the authoritative slot map from new-lg4ff.
func nativeConditionReports(kind nativeConditionKind, applied int) (start [][]byte, refresh []byte, stop []byte, err error) {
	if applied <= 0 || applied > NativeManualFFBTestMaxPercent {
		return nil, nil, nil, fmt.Errorf("Condition-Live-Test außerhalb 1–%d %%: %d", NativeManualFFBTestMaxPercent, applied)
	}
	var first, update []byte
	var slot int
	switch kind.name {
	case "spring-test":
		slot = 1
		first, err = BuildClassicSpringSlotReport(slot, applied, false)
		if err == nil {
			update, err = BuildClassicSpringSlotReport(slot, applied, true)
		}
	case "damper-test":
		slot = 2
		first, err = BuildClassicDamperSlotReport(slot, applied, false)
		if err == nil {
			update, err = BuildClassicDamperSlotReport(slot, applied, true)
		}
	case "friction-test":
		slot = 3
		first, err = BuildClassicFrictionSlotReport(slot, applied, false)
		if err == nil {
			update, err = BuildClassicFrictionSlotReport(slot, applied, true)
		}
	default:
		return nil, nil, nil, fmt.Errorf("unbekannter Condition-Live-Test %q", kind.name)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	stop, err = BuildClassicEffectStopReport(slot)
	if err != nil {
		return nil, nil, nil, err
	}
	return [][]byte{BuildClassicFixedLoopReport(false), first}, update, stop, nil
}

func startNativeConditionTest(s State, kind nativeConditionKind, requestedPercent int) error {
	// All manual motor tests are replace-current-test actions. This handles an
	// active autocenter lease as well as a previous nativeFFB worker.
	if err := NativeOutputEmergencyStop(s); err != nil {
		return fmt.Errorf("vorherige Motor-Ausgabe konnte nicht sicher neutralisiert werden: %w", err)
	}
	lease, err := acquireNativeOutputLease(s, kind.name, true)
	if err != nil {
		return err
	}
	w, _ := SelectedWheel(s)
	if requestedPercent <= 0 || requestedPercent > NativeManualFFBTestMaxPercent {
		releaseNativeOutputLease(lease)
		return fmt.Errorf("Condition-Hardwaretest ist auf 1–%d %% begrenzt", NativeManualFFBTestMaxPercent)
	}
	cfg := ReadNativeFFBConfig(s.DataDir, w.ID)
	// Hardware-Teststärke is direct by design; wheel/game profile gains must not
	// make a diagnostic button appear dead. Normal game FFB remains mixed and
	// safety-capped elsewhere.
	applied := requestedPercent
	startReports, updateReport, stopReport, err := nativeConditionReports(kind, applied)
	if err != nil {
		releaseNativeOutputLease(lease)
		return err
	}
	tr, err := openNativeHIDTransport(lease.Path)
	if err != nil {
		releaseNativeOutputLease(lease)
		return err
	}
	if err := MarkRuntimeOutputActiveTarget(s.DataDir, w, kind.name); err != nil {
		_ = tr.Close()
		releaseNativeOutputLease(lease)
		return fmt.Errorf("Recovery-Marker konnte nicht geschrieben werden; Motor-Ausgabe verweigert: %w", err)
	}
	for _, report := range startReports {
		if writeErr := tr.WriteReport(report); writeErr != nil {
			_ = tr.Close()
			return abortMotorStartAfterPossibleWrite(lease, fmt.Errorf("initialer %s-HID-Write fehlgeschlagen: %w", kind.name, writeErr))
		}
	}

	cancel := make(chan struct{})
	done := make(chan struct{})
	nativeFFB.Lock()
	prev := nativeFFB.status
	nativeFFB.generation++
	generation := nativeFFB.generation
	nativeFFB.cancel, nativeFFB.done = cancel, done
	leaseCopy := lease
	nativeFFB.lease = &leaseCopy
	nativeFFB.dataDir = s.DataDir
	st := NativeFFBStatus{Active: true, WheelID: w.ID, Model: w.Model, Effect: kind.name, Requested: requestedPercent, Config: cfg, ProfileName: "Hardware-Test direkt", Generation: generation, StartedAt: time.Now(), LastHeartbeat: time.Now(), ClipEvents: prev.ClipEvents, EffectTransitions: prev.EffectTransitions + 1, WatchdogStops: prev.WatchdogStops, EmergencyStops: prev.EmergencyStops}
	switch kind.name {
	case "spring-test":
		st.SpringApplied = applied
	case "damper-test":
		st.DamperApplied = applied
	case "friction-test":
		st.FrictionApplied = applied
	}
	nativeFFB.status = st
	nativeFFB.Unlock()

	go func() {
		defer close(done)
		defer tr.Close()
		defer func() {
			nativeFFB.Lock()
			if nativeFFB.lease != nil && nativeFFB.lease.Generation == lease.Generation {
				if cur, ok := currentNativeOutputLease(); !ok || cur.Generation != lease.Generation {
					nativeFFB.lease = nil
				}
			}
			if nativeFFB.cancel == cancel {
				nativeFFB.cancel = nil
			}
			if nativeFFB.done == done {
				nativeFFB.done = nil
			}
			nativeFFB.Unlock()
		}()
		updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) { st.Frames++; st.LastHeartbeat = time.Now() })
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.NewTimer(nativeManualFFBTestDuration)
		defer deadline.Stop()
		ticks := 0
		safeStop := func(watchdog bool) error {
			// Specific off first, then the global stop as a belt-and-suspenders
			// neutralization for classic Logitech firmware.
			stopErr := tr.WriteReport(stopReport)
			stopErr = errors.Join(stopErr, tr.WriteReport(BuildNativeStopAllEffectsReport()))
			stopErr = completeNativeMotorStop(lease, s.DataDir, stopErr)
			updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) {
				st.Stopping = false
				st.Faulted = stopErr != nil
				st.Active = stopErr != nil
				if stopErr == nil {
					st.Applied, st.SpringApplied, st.DamperApplied, st.FrictionApplied = 0, 0, 0, 0
				}
				st.LastHeartbeat = time.Now()
				st.Frames++
				if watchdog {
					st.WatchdogStops++
				}
				if stopErr != nil {
					st.LastError = stopErr.Error()
				} else {
					st.LastError = ""
				}
			})
			return stopErr
		}
		for {
			select {
			case <-cancel:
				_ = safeStop(false)
				return
			case <-deadline.C:
				_ = safeStop(true)
				return
			case <-ticker.C:
				ticks++
				if ticks%5 == 0 {
					if writeErr := tr.WriteReport(updateReport); writeErr != nil {
						updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) { st.LastError = writeErr.Error() })
						_ = safeStop(false)
						return
					}
					updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) { st.Frames++ })
				}
				updateNativeFFBStatusGeneration(generation, func(st *NativeFFBStatus) { st.LastHeartbeat = time.Now() })
			}
		}
	}()
	return nil
}

func StartNativeSpringTest(s State, requestedPercent int) error {
	return startNativeConditionTest(s, nativeSpringKind, requestedPercent)
}
func StartNativeDamperTest(s State, requestedPercent int) error {
	return startNativeConditionTest(s, nativeDamperKind, requestedPercent)
}
func StartNativeFrictionTest(s State, requestedPercent int) error {
	return startNativeConditionTest(s, nativeFrictionKind, requestedPercent)
}
