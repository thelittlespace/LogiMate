package brand

import _ "embed"

// IconICO contains the multi-resolution LogiMate application icon.
// It is embedded in the binary so the UI never depends on an external image file.
//
//go:embed logimate.ico
var IconICO []byte
