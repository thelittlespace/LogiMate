# Third-party notices

## OpenG27

LogiMate is an independent Windows application and does **not** require the external OpenG27 executable at runtime.

OpenG27 remains an important MIT-licensed reference implementation used during protocol research, parity testing, provenance review and historical validation:

- Project: `Jabelius/OpenG27`
- Purpose in LogiMate: protocol/reference oracle and provenance source; **not** a production runtime dependency
- License: MIT
- Copyright: (c) 2026 jabelius

Selected behavior was independently translated or derived into Go during LogiMate's native-engine work. File-level provenance is documented in `docs/OPENG27_PROVENANCE.md`. The production `cmd/logimate` dependency gate intentionally rejects a runtime dependency on `internal/openg27port`.

OpenG27's MIT notice applies to OpenG27-derived portions included in LogiMate:

> MIT License
>
> Copyright (c) 2026 jabelius
>
> Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:
>
> The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.
>
> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

## lg4ff protocol references

Logitech G25/G27/Driving Force GT protocol behavior is informed by public protocol knowledge and the open-source Logitech wheel work in `berarma/new-lg4ff`. LogiMate does not distribute that Linux kernel module. Protocol facts are implemented independently in Go. GPL-covered implementation code must not be copied into LogiMate without a separate license review.

Build 011 hardware testing on a physical G27 confirmed the production condition-effect slot map used by LogiMate: Constant=0, Spring=1, Damper=2 and Friction=3.

## nightmode/logitech-g27 HID map

The semantic G27 input mapping (buttons, paddles, shifter, D-pad and H-pattern selector) is informed by the public-domain `nightmode/logitech-g27` project.

- Project: `nightmode/logitech-g27`
- License: CC0 1.0 Universal / public-domain dedication

LogiMate independently implements its parser in Go.
