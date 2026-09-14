# D5.6 Release Readiness

Version: **0.5.6-alpha**

## Software gates

- wheelengine host tests: PASS
- gameadapter host tests: PASS
- OpenG27 parity/provenance tests: PASS
- Windows x64 system test compile: PASS
- Windows x64 app test compile: PASS
- Windows x64 system/app vet (`-unsafeptr=false`): PASS
- Windows x64 application build: PASS
- Windows ARM64 compile validation: PASS
- D5 standalone dependency gate (`cmd/logimate` does not depend on `internal/openg27port`): PASS

## UI acceptance

- advanced raw data is embedded in the existing Wheel page: PASS
- compact Wheel page remains the default: PASS
- expanded view is live and scrollable: PASS by source/build validation; physical visual check on Windows remains recommended
- HID metadata is background-captured, not probed from WM_PAINT: PASS
- expanded view sends no hardware output: PASS by code path

## Physical validation
The expanded data surface is intended to make the next real-G27 validation more transparent. Physical G25/G27/DFGT certification remains separate from this UI release.


## Real G27 UI note

The user accepted the integrated Advanced Wheel View layout in real use. This validates the in-page layout/scroll concept, but does not by itself certify native-mode automation, motor output or recovery behavior.
