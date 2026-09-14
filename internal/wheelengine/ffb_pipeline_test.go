package wheelengine

import (
	"math"
	"testing"
	"time"
)

func TestFFBPipelineDeadbandRescalesAndSafetyNeutral(t *testing.T) {
	p, err := NewFFBPipeline(FFBTuning{Enabled: true, ConstantGain: 1, TransientGain: 1, Deadband: .10, ResponseExponent: 1, OutputLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	if got := p.ProcessFrame(ForceFrame{Constant: .05}, 1, now, now); got != 0 {
		t.Fatalf("deadband got %f", got)
	}
	got := p.ProcessFrame(ForceFrame{Constant: .55}, 1, now, now.Add(6*time.Millisecond))
	want := (.55 - .10) / .90
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("rescale got=%f want=%f", got, want)
	}
}

func TestFFBPipelineMinimumForceCannotExceedConfiguredOutputLimit(t *testing.T) {
	p, err := NewFFBPipeline(FFBTuning{Enabled: true, ConstantGain: 1, TransientGain: 1, MinimumForce: .20, ResponseExponent: 1, OutputLimit: .30})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	got := p.ProcessFrame(ForceFrame{Constant: 1}, 1, now, now)
	if got > .3000001 {
		t.Fatalf("limit bypass: %f", got)
	}
	if p.Snapshot().OutputClipEvents == 0 {
		t.Fatal("expected output clip metric")
	}
}

func TestFFBPipelineResponseCurveAndLatency(t *testing.T) {
	p, err := NewFFBPipeline(FFBTuning{Enabled: true, ConstantGain: 1, TransientGain: 1, ResponseExponent: 2, OutputLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	got := p.ProcessFrame(ForceFrame{Constant: .5}, 1, now.Add(-12*time.Millisecond), now)
	if math.Abs(got-.25) > 1e-6 {
		t.Fatalf("curve got %f", got)
	}
	if p.Snapshot().LastSampleLatency != 12*time.Millisecond {
		t.Fatalf("latency=%v", p.Snapshot().LastSampleLatency)
	}
}

func TestFFBPipelineLowPassAndSmoothingReduceStep(t *testing.T) {
	p, err := NewFFBPipeline(FFBTuning{Enabled: true, ConstantGain: 1, TransientGain: 1, ResponseExponent: 1, LowPassHz: 20, Smoothing: .25, OutputLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	_ = p.ProcessFrame(ForceFrame{}, 1, now, now)
	got := p.ProcessFrame(ForceFrame{Constant: 1}, 1, now.Add(6*time.Millisecond), now.Add(6*time.Millisecond))
	if got <= 0 || got >= 1 {
		t.Fatalf("expected filtered step 0<got<1, got=%f", got)
	}
}

func TestFFBPipelineInvalidNumbersRejected(t *testing.T) {
	_, err := NewFFBPipeline(FFBTuning{Enabled: true, ConstantGain: math.NaN(), TransientGain: 1, ResponseExponent: 1, OutputLimit: 1})
	if err == nil {
		t.Fatal("NaN tuning must be rejected")
	}
}

func TestFFBPipelineAllowsZeroPerChannelGain(t *testing.T) {
	p, err := NewFFBPipeline(FFBTuning{Enabled: true, ConstantGain: 0, TransientGain: 0, ResponseExponent: 1, OutputLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(200, 0)
	if got := p.ProcessFrame(ForceFrame{Constant: .8, Transient: .2}, 1, now, now); got != 0 {
		t.Fatalf("zero gains must mute their channels, got %f", got)
	}
}
