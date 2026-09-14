package wheelengine

import (
	"testing"
	"time"
)

func TestDescriptorRegistry(t *testing.T) {
	d, ok := DescriptorForModel(ModelG27)
	if !ok || d.NativePID != "C29B" || !d.Controls.RPMLEDs || !d.Output.NativeFFB {
		t.Fatalf("unexpected G27 descriptor: %+v ok=%v", d, ok)
	}
	if got := ModelFromPID("c299"); got != ModelG25 {
		t.Fatalf("C299 => %s", got)
	}
	if got := ModelFromDisplayName("Logitech G27 (manuell bestätigt / C294)"); got != ModelG27 {
		t.Fatalf("display classification => %s", got)
	}
}

func TestManagerSessionChangeInvalidatesInput(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Reconcile([]Device{{StableID: "wheel", SessionID: "A", Model: ModelG27, Supported: true}}, "wheel", now)
	if !m.UpdateInput(InputState{WheelID: "wheel", SessionID: "A", Model: ModelG27, SampleValid: true, SampleGeneration: 3, SampleAt: now}, now) {
		t.Fatal("expected input update")
	}
	m.Reconcile([]Device{{StableID: "wheel", SessionID: "B", Model: ModelG27, Supported: true}}, "wheel", now.Add(time.Second))
	r, ok := m.Selected()
	if !ok || r.Input.SampleValid {
		t.Fatalf("session change must invalidate old input: %+v", r)
	}
}

func TestManagerRejectsCrossSessionInput(t *testing.T) {
	m := NewManager()
	now := time.Now()
	m.Reconcile([]Device{{StableID: "wheel", SessionID: "A", Model: ModelG27}}, "wheel", now)
	if m.UpdateInput(InputState{WheelID: "wheel", SessionID: "B", SampleValid: true, SampleAt: now}, now) {
		t.Fatal("cross-session input must be rejected")
	}
}

func TestAllIntegratedDescriptorsAreNativeCapable(t *testing.T) {
	want := map[ModelID]string{ModelG25: "C299", ModelG27: "C29B", ModelDFGT: "C29A"}
	for model, pid := range want {
		d, ok := DescriptorForModel(model)
		if !ok {
			t.Fatalf("descriptor missing: %s", model)
		}
		if d.NativePID != pid || !d.Output.NativeFFB || !d.Output.Range || d.NativeModeSelector == 0 {
			t.Fatalf("descriptor incomplete: %+v", d)
		}
	}
}
