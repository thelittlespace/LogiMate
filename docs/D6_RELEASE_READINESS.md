# D6 Release Readiness — 0.6.1-alpha

## Software gates

- Existing Lenkrad page retained; no new main navigation page: PASS
- Live-Test / Force Feedback in-page tabs: PASS
- Native Engine profile sliders persisted per wheel: PASS
- D4 Advanced FFB sliders persisted per wheel: PASS
- Built-in Wheel/D4 presets integrated: PASS
- Live Raw/Shaped/Applied FFB telemetry integrated: PASS
- Bounded Constant/Spring/Damper/Friction/Autocenter tests reuse central OutputLease: PASS
- Emergency Stop exposed on the same panel: PASS
- No productive `internal/openg27port` dependency reintroduced: PASS
- Keyboard focus/slider adjustment added: PASS
- D6 controls added to native accessibility overlay: PASS (software); physical/UIA validation pending
- Windows amd64 vet: PASS
- Windows amd64 app/system test compile: PASS
- Windows amd64 GUI build: PASS
- Windows arm64 compile validation: PASS
- wheelengine/gameadapter/OpenG27 provenance tests: PASS

## Physical / release gates still open

D6 does not change these repository certification truths:
- G25 physical certification: PENDING
- G27 full physical certification: PENDING
- DFGT physical certification: PENDING
- crash/recovery certification: PENDING
- migration certification: PENDING
- UI accessibility validation: PENDING
- HID stress validation: PENDING
- Stable Authenticode signing: external release-host gate

0.6.1-alpha is therefore a development/validation build, not Stable.
