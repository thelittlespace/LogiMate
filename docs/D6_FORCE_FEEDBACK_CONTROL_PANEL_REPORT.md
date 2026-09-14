# D6 Wheel Control Panel / Force Feedback UI

**Release:** 0.6.1-alpha  
**Purpose:** expose the already-implemented LogiMate Native / D4 force-feedback controls directly on the Wheel page with a modern slider-based control panel instead of dialog-only tuning.

## UI placement

D6 does **not** add a new main navigation destination. The existing `Lenkrad` page now has an in-page sub-navigation directly beneath the page header:

- `Live-Test` — existing live steering/pedal/button/shifter dashboard and Advanced View;
- `Force Feedback` — new visual FFB control panel.

This keeps device identity, live hardware state and force-feedback setup on one physical-wheel page.

## Force Feedback control panel

The panel exposes real per-wheel settings through visual sliders:

### Wheel Engine profile
- Master / overall gain
- Constant Force gain
- Spring gain
- Damper gain
- Friction gain
- operating range / steering rotation (270 / 360 / 540 / 720 / 900°)

Manual edits become the per-wheel `Custom` Native Engine profile. Gain changes only alter configuration. Rotation is applied to hardware only when Native Wheel Output has already been explicitly enabled; otherwise it remains safely persisted for later activation.

### D4 Advanced FFB shaping
- FFB shaping pipeline on/off
- Transient Gain
- Minimum Force
- Deadband
- Response Curve
- Low-Pass filter
- Smoothing
- Pre-Safety Output Limit

These controls edit the existing D4 `advanced-ffb.json` schema. They remain upstream of the hard `NativeFFBConfig` safety mixer and cannot raise the physical motor ceiling.

### Presets
Wheel profile shortcuts:
- Gentle / Sanft
- Balanced / Ausgewogen
- Direct / Direkt

D4 shaping shortcuts:
- Neutral
- Smooth
- Responsive
- Compensated (Experimental)

### Live monitor
The panel surfaces the existing Native Game Output / Native FFB telemetry:
- requested/raw game force
- shaped force
- applied percent
- active effect
- frame count
- pipeline/safety clipping counts
- active HID backend
- average sample latency

### Bounded hardware tests
The panel exposes the already-existing central safe output paths for:
- Constant Force
- Spring
- Damper
- Friction
- Autocenter
- Emergency Stop / Neutralize

Test strength is limited to 1–10%. D6 creates no new motor writer: all tests reuse the central OutputLease, runtime recovery marker, watchdog and Emergency Stop logic.

## Input / accessibility

- slider tracks support mouse dragging;
- keyboard focus includes Wheel subtabs, presets, toggle, all sliders and test controls;
- focused sliders use Left/Right/Up/Down to adjust values;
- D6 controls receive transparent native accessibility peers so Narrator/UI Automation can reach them while the LogiMate custom renderer remains unchanged;
- slider accessibility labels include the current value and keyboard adjustment hint.

## Safety boundary

D6 is an operator UI over existing D4/D5 engine functionality. It does not:
- bypass `NativeFFBConfig` hard caps;
- start FFB automatically when changing a normal gain slider;
- create a second HID/motor output implementation;
- mark physical G25/G27/DFGT certification as passed;
- change D5.8/D5.9 Stable evidence flags.

## Validation required

Before Stable certification:
1. validate all D6 controls on the real G27;
2. verify each slider survives restart and remains bound to the selected physical wheel;
3. verify rotation application and rollback on a real Modern-mode G27;
4. exercise all bounded tests and Emergency Stop;
5. repeat model-specific behavior on G25/DFGT when hardware is available;
6. include the new D6 controls in D5.8 Narrator/UIA, High Contrast and mixed-DPI evidence.
