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
				"flow":             20,
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
