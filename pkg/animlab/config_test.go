package animlab

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLInheritancePresetsAndValidation(t *testing.T) {
	dir := t.TempDir()
	baseYAML := `version: 1
presets:
  tracer_std:
    family: perimeter-tracer
    methods: [css, smil]
    shapes: [rectangle, circle]
    sizes:
      small: [40, 40]
    speeds:
      fast: 0.8
    segment_counts: [2]
    line_gap_ratios: ["1:3"]
test:
  name: base-test
  description: Base
browser:
  engine: chromium
  headless: true
execution:
  warmup_seconds: 0.5
  duration_seconds: 1.0
  repetitions: 1
  random_seed: 42
instances:
  counts: [50]
  isolation: independent-svg-roots
effects:
  - preset: tracer_std
workloads:
  - name: all-active
    visibility: all
`
	childYAML := `inherits: base.yaml
test:
  name: child-test
instances:
  counts: [100, 250]
  isolation: independent-svg-roots
`
	if err := os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(baseYAML), 0644); err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(dir, "child.yaml")
	if err := os.WriteFile(childPath, []byte(childYAML), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(childPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Test.Name != "child-test" {
		t.Fatalf("expected child-test, got %q", cfg.Test.Name)
	}
	if len(cfg.Instances.Counts) != 2 || cfg.Instances.Counts[0] != 100 || cfg.Instances.Counts[1] != 250 {
		t.Fatalf("unexpected counts: %v", cfg.Instances.Counts)
	}
	specs := ExpandScenarios(cfg)
	if len(specs) != 4 { // 2 counts * 2 methods (css, smil)
		t.Fatalf("expected 4 scenarios, got %d", len(specs))
	}
	if specs[0].Count != 100 || specs[2].Count != 250 {
		t.Fatalf("unexpected scenario counts: %d, %d", specs[0].Count, specs[2].Count)
	}
	html := BuildTestPageHTML(specs[0])
	if !strings.Contains(html, `class="lab-svg-root"`) {
		t.Fatalf("expected independent SVG roots in HTML")
	}

	// Invalid config check
	badPath := filepath.Join(dir, "bad.yaml")
	_ = os.WriteFile(badPath, []byte("version: 1\ntest:\n  name: bad\ninstances:\n  counts: [0]\neffects:\n  - family: unknown\n    methods: [css]\n"), 0644)
	if _, err := LoadConfig(badPath); err == nil {
		t.Fatalf("expected validation error for invalid config")
	}
}

func TestMixedAndSharedSceneWorkloads(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Test:    TestMeta{Name: "mix"},
		Browser: BrowserConfig{Engine: "chromium"},
		Execution: ExecutionConfig{
			DurationSeconds: 1,
			Repetitions:     1,
			RandomSeed:      99,
		},
		Instances: InstancesConfig{
			Counts:    []int{50},
			Isolation: "shared-scene",
		},
		Mixture: &MixtureConfig{
			Name:    "mixed-dash",
			Methods: []string{"css", "smil"},
			Weights: map[string]float64{
				"perimeter-tracer": 35,
				"pulse":            25,
				"flow-line":        12,
				"flow-particle":    8,
				"static-gauge":     20,
			},
		},
		Workloads: []WorkloadSpec{{Name: "all-active", Visibility: "all"}},
	}
	applyDefaults(cfg)
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	specs := ExpandScenarios(cfg)
	if len(specs) != 2 {
		t.Fatalf("expected 2 mixed scenarios, got %d", len(specs))
	}
	html := BuildTestPageHTML(specs[0])
	if !strings.Contains(html, `class="lab-shared-wrap"`) {
		t.Fatalf("expected shared-scene wrapper")
	}
}


func TestM1ConfigAndEffectInvariants(t *testing.T) {
	// 1. Legacy 'flow' is rejected with clear migration error
	legacyCfg := &Config{
		Version:   1,
		Test:      TestMeta{Name: "legacy"},
		Browser:   BrowserConfig{Engine: "chromium"},
		Execution: ExecutionConfig{DurationSeconds: 1, Repetitions: 1},
		Instances: InstancesConfig{Counts: []int{5}, Isolation: "independent-svg-roots"},
		Effects:   []EffectSpec{{Family: "flow", Methods: []string{"css"}}},
		Workloads: []WorkloadSpec{{Name: "all", Visibility: "all"}},
	}
	applyDefaults(legacyCfg)
	if err := ValidateConfig(legacyCfg); err == nil || !strings.Contains(err.Error(), "flow-line") {
		t.Fatalf("expected legacy 'flow' rejection mentioning flow-line, got %v", err)
	}

	reg := DefaultRegistry()

	// 2. Standalone SVG embeds script for javascript-raf and not for css
	rafSample := BuildStandaloneSampleSVG(InstanceParams{
		ID: "s-raf", Family: "perimeter-tracer", Method: "javascript-raf",
		Shape: "rect", Width: 60, Height: 60, Speed: 1.0, Segments: 2, LineGapRatio: "25:75", Direction: "cw",
	})
	if !strings.Contains(rafSample, "<script") || !strings.Contains(rafSample, "__labRafStarted") {
		t.Fatalf("expected embedded script with __labRafStarted in javascript-raf standalone sample")
	}
	cssSample := BuildStandaloneSampleSVG(InstanceParams{
		ID: "s-css", Family: "perimeter-tracer", Method: "css",
		Shape: "rect", Width: 60, Height: 60, Speed: 1.0, Segments: 2, LineGapRatio: "25:75", Direction: "cw",
	})
	if strings.Contains(cssSample, "<script") {
		t.Fatalf("did not expect <script> in pure CSS standalone sample")
	}

	// 3. Star flash shape uses genuine 8-point star polygon (16 vertices), not diamond (4 vertices)
	star := reg["flash"].Render(InstanceParams{
		ID: "f-star", Family: "flash", Method: "css", Shape: "star",
		Width: 40, Height: 40, FlashMode: "repeating", FlashDurMs: 120, FlashIntervalS: 1.0,
	}, 100, 100)
	if !strings.Contains(star.SVGInner, "<polygon") || !strings.Contains(star.CSS, "scale(") {
		t.Fatalf("expected star flash to render <polygon> and animate scale(), got markup=%s css=%s", star.SVGInner, star.CSS)
	}

	// 4. Pulse mode exclusivity
	opOnly := reg["pulse"].Render(InstanceParams{
		ID: "p-op", Family: "pulse", Method: "css", Shape: "circle",
		Width: 40, Height: 40, Speed: 1.0, PulseMode: "opacity-only", PulseAmplitude: 0.25,
	}, 100, 100)
	if strings.Contains(opOnly.CSS, "scale(") {
		t.Fatalf("opacity-only pulse should not animate scale(), got %s", opOnly.CSS)
	}
	scOnly := reg["pulse"].Render(InstanceParams{
		ID: "p-sc", Family: "pulse", Method: "css", Shape: "circle",
		Width: 40, Height: 40, Speed: 1.0, PulseMode: "scale-only", PulseAmplitude: 0.25,
	}, 100, 100)
	if strings.Contains(scOnly.CSS, "opacity:") {
		t.Fatalf("scale-only pulse should not animate opacity, got %s", scOnly.CSS)
	}

	// 5. clip-reveal marked NonEquivalent
	cr := reg["perimeter-tracer"].Render(InstanceParams{
		ID: "t-cr", Family: "perimeter-tracer", Method: "clip-reveal",
		Shape: "rect", Width: 60, Height: 60, Speed: 1.0,
	}, 100, 100)
	if !cr.NonEquivalent {
		t.Fatalf("expected clip-reveal to be marked NonEquivalent=true")
	}

	// 6. Large shape cell sizing prevents clipping
	cw, ch := computeCellDimensions([]InstanceParams{
		{Family: "perimeter-tracer", Shape: "circle", Width: 120, Height: 120},
	})
	if cw < 140 || ch < 140 {
		t.Fatalf("expected cell dimensions >= 140 for 120x120 shape, got %dx%d", cw, ch)
	}
}


func TestM2MeasurementStatsOrderingAndRanking(t *testing.T) {
	// 1. FrameStats: 16.67ms vs 20.0ms threshold separation and RafHz naming
	fs := computeFrameStats([]float64{16.0, 17.5, 22.0, 55.0})
	if fs.LongFramesOver16Ms != 3 || fs.LongFramesOver20Ms != 2 || fs.LongFramesOver50Ms != 1 {
		t.Fatalf("expected over16=3, over20=2, over50=1; got %+v", fs)
	}
	if fs.RafHz <= 0 || fs.AvgRafIntervalMs <= 0 {
		t.Fatalf("expected positive RafHz and AvgRafIntervalMs, got %+v", fs)
	}

	// 2. DistributionStats
	d := computeDistribution([]float64{10, 20, 30})
	if d == nil || d.Mean != 20 || d.Median != 20 || d.Min != 10 || d.Max != 30 || d.StdDev <= 0 {
		t.Fatalf("unexpected distribution: %+v", d)
	}

	// 3. Deterministic randomized & alternating ordering
	specs := []TestRunSpec{{RunID: "a"}, {RunID: "b"}, {RunID: "c"}, {RunID: "d"}}
	r0 := orderTrialsForRepetition(specs, ExecutionConfig{OrderStrategy: "randomized", RandomSeed: 42}, 0)
	r0b := orderTrialsForRepetition(specs, ExecutionConfig{OrderStrategy: "randomized", RandomSeed: 42}, 0)
	for i := range r0 {
		if r0[i].RunID != r0b[i].RunID {
			t.Fatalf("randomized order not deterministic for same seed")
		}
	}
	a0 := orderTrialsForRepetition(specs, ExecutionConfig{OrderStrategy: "alternating"}, 0)
	a1 := orderTrialsForRepetition(specs, ExecutionConfig{OrderStrategy: "alternating"}, 1)
	if a0[0].RunID == a1[0].RunID {
		t.Fatalf("alternating order did not rotate/reverse across reps")
	}

	// 4. Decision report ranking uses MEANS, not sums (F-C14)
	tmpDir := t.TempDir()
	res := &LabResults{
		TestName: "rank-mean-test",
		Scenarios: []ScenarioReport{
			// Method "css" has 1 scenario with 10ms task (mean=10)
			{RunID: "css-1", Family: "pulse", Method: "css", CorrectnessPassed: true, ValidRepetitions: 1, TotalRepetitions: 1, AvgRafHz: ptrFloat64(60), AvgTaskMs: ptrFloat64(10), AvgProcessCPUSec: ptrFloat64(0.10)},
			// Method "smil" has 3 scenarios with 30ms task each (sum=90, mean=30)
			{RunID: "smil-1", Family: "pulse", Method: "smil", CorrectnessPassed: true, ValidRepetitions: 1, TotalRepetitions: 1, AvgRafHz: ptrFloat64(60), AvgTaskMs: ptrFloat64(30), AvgProcessCPUSec: ptrFloat64(0.30)},
			{RunID: "smil-2", Family: "pulse", Method: "smil", CorrectnessPassed: true, ValidRepetitions: 1, TotalRepetitions: 1, AvgRafHz: ptrFloat64(60), AvgTaskMs: ptrFloat64(30), AvgProcessCPUSec: ptrFloat64(0.30)},
			{RunID: "smil-3", Family: "pulse", Method: "smil", CorrectnessPassed: true, ValidRepetitions: 1, TotalRepetitions: 1, AvgRafHz: ptrFloat64(60), AvgTaskMs: ptrFloat64(30), AvgProcessCPUSec: ptrFloat64(0.30)},
		},
	}
	if err := WriteDecisionReports(&Config{}, res, tmpDir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(tmpDir, "decision-pulse.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "**Selected Recommendation:** `css`") {
		t.Fatalf("expected css (lower mean task/CPU) to win over smil (3 scenarios), got:\n%s", string(b))
	}
}
