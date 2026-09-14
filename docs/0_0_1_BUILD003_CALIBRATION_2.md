# LogiMate 0.0.1-alpha · Build 003 — Calibration 2.0

## Goal

Replace the normal MessageBox chain with a visible, resumable calibration workflow inside the selected wheel context.

## Implemented

- Four calibration cards: steering, pedals, buttons/paddles and H-shifter.
- Clicking a card opens an inline wizard in the same Wheel/Calibration tab.
- Steering captures left/center/right and the selected 270/360/540/720/900° range before one final save.
- Pedals capture one stable rest sample, then gas/brake/clutch full travel; source/session/layout continuity is verified before persistence.
- Deadzone and curve choices are changed inline rather than via nested dialogs.
- Button learning captures a baseline and exactly one new pressed bit before saving.
- H-shifter captures Neutral, 1–6 and Reverse and rejects duplicate/overlapping signatures before commit.
- Invalid/malformed/no-sample states remain fail-closed and are shown inline.
- Only the compact live-input rows repaint at 4 Hz while the wizard is active.

## Persistence rule

Nothing is saved until the required capture sequence is complete and validates against the current wheel/session/source.
