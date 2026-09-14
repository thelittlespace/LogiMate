# Fusion C5 · OpenG27 Game Profiles

C5 ports OpenG27 1.0.4 GameProfile matching and profile persistence semantics into an isolated compatibility layer, then translates profiles into LogiMate rather than replacing LogiMate's broader schema.

- Case-insensitive substring `ProcessMatch`; first matching profile in source order wins in the parity core.
- Imports `%APPDATA%\OpenG27\game-profiles.json`, with the old `%APPDATA%\OpenG27FFB` path as fallback.
- Legacy OpenG27 JSON missing newer fields keeps the upstream constructor defaults, including GameFfbGain=100.
- Creates LogiMate Game-Profile records plus a per-StableWheelID Wheel-Engine profile when a wheel is selected.
- WreckfestPino maps to `openg27-pino`; RPM LEDs map to telemetry LED policy.
- Hard FFB safety configuration is never imported or modified.
- Import never starts force feedback.
