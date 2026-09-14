package wheelengine

import "sort"

// Hardware certification is evidence-based. Compilation never promotes a
// model to validated. D1 exposes the exact evidence set required by release.
var requiredHardwareChecks = []string{
	"native-mode-switch", "steering", "pedals", "buttons", "shifter",
	"range", "constant-force", "spring", "damper", "friction", "autocenter",
	"game-ffb", "reconnect", "usb-removal", "process-kill", "suspend-resume",
	"modern-legacy-rollback",
}

type CertificationEvidence struct {
	Check    string `json:"check"`
	Passed   bool   `json:"passed"`
	Evidence string `json:"evidence,omitempty"`
}
type ModelCertification struct {
	Model    ModelID                 `json:"model"`
	Evidence []CertificationEvidence `json:"evidence"`
}

func RequiredHardwareChecks(model ModelID) []string {
	out := append([]string(nil), requiredHardwareChecks...)
	d, ok := DescriptorForModel(model)
	if ok && !d.Controls.HShifter {
		filtered := out[:0]
		for _, x := range out {
			if x != "shifter" {
				filtered = append(filtered, x)
			}
		}
		out = filtered
	}
	sort.Strings(out)
	return out
}
func (c ModelCertification) Complete() bool {
	required := RequiredHardwareChecks(c.Model)
	passed := map[string]bool{}
	for _, e := range c.Evidence {
		if e.Passed && e.Evidence != "" {
			passed[e.Check] = true
		}
	}
	for _, check := range required {
		if !passed[check] {
			return false
		}
	}
	return len(required) > 0
}
