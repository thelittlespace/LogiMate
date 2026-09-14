# Fusion C6 · Wreckfest 2 Pino / Game Output

C6 ports OpenG27 1.0.4's Wreckfest 2 Pino telemetry path and connects it to LogiMate's common telemetry and safety architecture.

## Telemetry
- Binds only `127.0.0.1:23123`.
- Decodes 1218-byte Main packets with the OpenG27 offsets/signature.
- Publishes RPM, max/redline RPM, game FFB force, physics-running and player-control state into LogiMate `TelemetryFrame`.
- Keeps packet/main/invalid counters, ring buffer and 750 ms stale state.
- Active imported Wreckfest profiles may auto-start the listener; this is network input only.

## Hardware output
- Motor/LED output is still explicit: `Fusion C6 · Game-FFB + RPM-LEDs starten`.
- Phase C live path is G27-only.
- Game force is inverted/gained according to the translated profile, then passed through LogiMate Engine Profile + hard NativeFFB safety caps.
- Physics stopped, AI/non-player control, stale or missing telemetry requests neutral force.
- RPM LEDs use the OpenG27/lg4ff cumulative LED-bar thresholds.
- Generation owner, slew limiter, watchdog, crash marker, Emergency Stop and OpenG27 process interlock stay authoritative.
