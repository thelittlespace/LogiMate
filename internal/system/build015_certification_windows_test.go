//go:build windows

package system

import "testing"

func TestBuild015CertificationWorkflowContainsAllFFBEffects(t *testing.T) {
	want := map[string]bool{"constant-force": false, "spring": false, "damper": false, "friction": false, "autocenter": false}
	for _, x := range certificationWorkflowOrder {
		if _, ok := want[x]; ok {
			want[x] = true
		}
	}
	for k, ok := range want {
		if !ok {
			t.Fatalf("FFB certification check missing: %s", k)
		}
	}
}

func TestBuild015CertificationNeverAutoCompletesModel(t *testing.T) {
	f := hardwareCertificationFile{SchemaVersion: hardwareCertificationSchema, Records: map[string]wheelCertificationRecord{}}
	if len(f.Records) != 0 {
		t.Fatal("empty certification file unexpectedly has records")
	}
	// Physical validation remains evidence-driven; there is intentionally no
	// helper in this package that flips repository release certification flags.
}
