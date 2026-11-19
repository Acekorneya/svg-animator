package animlab

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level YAML configuration for the SVG Animation Test Lab.
type Config struct {
	Version       int                   `yaml:"version" json:"version"`
	Inherits      string                `yaml:"inherits,omitempty" json:"inherits,omitempty"`
	Presets       map[string]EffectSpec `yaml:"presets,omitempty" json:"presets,omitempty"`
	Test          TestMeta              `yaml:"test" json:"test"`
	Browser       BrowserConfig         `yaml:"browser" json:"browser"`
	Execution     ExecutionConfig       `yaml:"execution" json:"execution"`
	Instances     InstancesConfig       `yaml:"instances" json:"instances"`
	Scene         *SceneConfig          `yaml:"scene,omitempty" json:"scene,omitempty"`
	WidgetRuntime *WidgetRuntimeConfig  `yaml:"widget_runtime,omitempty" json:"widget_runtime,omitempty"`
	Effects       []EffectSpec          `yaml:"effects" json:"effects"`
	Mixture       *MixtureConfig        `yaml:"mixture,omitempty" json:"mixture,omitempty"`
	Workloads     []WorkloadSpec        `yaml:"workloads" json:"workloads"`
	Measurements  []string              `yaml:"measurements" json:"measurements"`
	Output        OutputConfig          `yaml:"output" json:"output"`
	ConfigDir     string                `yaml:"-" json:"-"`
}

type SceneConfig struct {
	Width      int               `yaml:"width,omitempty" json:"width,omitempty"`
	Height     int               `yaml:"height,omitempty" json:"height,omitempty"`
	Background *BackgroundConfig `yaml:"background,omitempty" json:"background,omitempty"`
	Positions  [][2]int          `yaml:"positions,omitempty" json:"positions,omitempty"`
}

type BackgroundConfig struct {
	File   string `yaml:"file,omitempty" json:"file,omitempty"`
	Fit    string `yaml:"fit,omitempty" json:"fit,omitempty"` // "contain" | "cover" | "fill"
	Static bool   `yaml:"static,omitempty" json:"static,omitempty"`
}

type WidgetRuntimeConfig struct {
	PackageDir string               `yaml:"package_dir" json:"package_dir"`
	Updates    *WidgetUpdatesConfig `yaml:"updates,omitempty" json:"updates,omitempty"`
}

type WidgetUpdatesConfig struct {
	Enabled    bool                   `yaml:"enabled" json:"enabled"`
	IntervalMs int                    `yaml:"interval_ms,omitempty" json:"interval_ms,omitempty"`
	Properties []WidgetPropertyUpdate `yaml:"properties,omitempty" json:"properties,omitempty"`
}

type WidgetPropertyUpdate struct {
	Name     string `yaml:"name" json:"name"`
	Values   []any  `yaml:"values" json:"values"`
	Optional bool   `yaml:"optional,omitempty" json:"optional,omitempty"`
}

type TestMeta struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
}

type BrowserConfig struct {
	Engine            string  `yaml:"engine" json:"engine"`
	Headless          bool    `yaml:"headless" json:"headless"`
	CDPEndpoint       string  `yaml:"cdp_endpoint,omitempty" json:"cdp_endpoint,omitempty"`
	HostBindIP        string  `yaml:"host_bind_ip,omitempty" json:"host_bind_ip,omitempty"`
	ProcessCPUHost    string  `yaml:"process_cpu_host,omitempty" json:"process_cpu_host,omitempty"`
	ViewportWidth     int     `yaml:"viewport_width,omitempty" json:"viewport_width,omitempty"`
	ViewportHeight    int     `yaml:"viewport_height,omitempty" json:"viewport_height,omitempty"`
	DeviceScaleFactor float64 `yaml:"device_scale_factor,omitempty" json:"device_scale_factor,omitempty"`
}

type ExecutionConfig struct {
	WarmupSeconds        float64 `yaml:"warmup_seconds" json:"warmup_seconds"`
	DurationSeconds      float64 `yaml:"duration_seconds" json:"duration_seconds"`
	Repetitions          int     `yaml:"repetitions" json:"repetitions"`
	RandomSeed           int64   `yaml:"random_seed" json:"random_seed"`
	AlternateMethodOrder bool    `yaml:"alternate_method_order" json:"alternate_method_order"`
	OrderStrategy        string  `yaml:"order_strategy,omitempty" json:"order_strategy,omitempty"` // "alternating" | "randomized" | "sequential"
}

type InstancesConfig struct {
	Counts    []int  `yaml:"counts" json:"counts"`
	Isolation string `yaml:"isolation" json:"isolation"` // "independent-svg-roots" | "shared-scene"
}

type EffectSpec struct {
	Preset            string             `yaml:"preset,omitempty" json:"preset,omitempty"`
	Family            string             `yaml:"family" json:"family"`
	Methods           []string           `yaml:"methods" json:"methods"`
	VariationStrategy string             `yaml:"variation_strategy,omitempty" json:"variation_strategy,omitempty"` // "representative" (default) | "factorial" | "isolated"
	Shapes            []string           `yaml:"shapes,omitempty" json:"shapes,omitempty"`
	Sizes             map[string][2]int  `yaml:"sizes,omitempty" json:"sizes,omitempty"`
	Speeds            map[string]float64 `yaml:"speeds,omitempty" json:"speeds,omitempty"`
	SegmentCounts     []int              `yaml:"segment_counts,omitempty" json:"segment_counts,omitempty"`
	LineGapRatios     []string           `yaml:"line_gap_ratios,omitempty" json:"line_gap_ratios,omitempty"`
	Directions        []string           `yaml:"directions,omitempty" json:"directions,omitempty"`               // ["cw"] | ["ccw"] | ["cw", "ccw"]
	PulseModes        []string           `yaml:"pulse_modes,omitempty" json:"pulse_modes,omitempty"`           // opacity-only | scale-only | scale-opacity
	PulseAmplitudes   []float64          `yaml:"pulse_amplitudes,omitempty" json:"pulse_amplitudes,omitempty"` // e.g. [0.2, 0.35]
	FlashModes        []string           `yaml:"flash_modes,omitempty" json:"flash_modes,omitempty"`           // single-shot | repeating | concurrent | staggered
	FlashDurationMs   int                `yaml:"flash_duration_ms,omitempty" json:"flash_duration_ms,omitempty"`
	FlashIntervalS    float64            `yaml:"flash_interval_s,omitempty" json:"flash_interval_s,omitempty"`
	RAFScheduling     string             `yaml:"raf_scheduling,omitempty" json:"raf_scheduling,omitempty"`     // "shared" (default) | "per-instance"
}

type MixtureConfig struct {
	Name    string             `yaml:"name,omitempty" json:"name,omitempty"`
	Methods []string           `yaml:"methods" json:"methods"`
	Weights map[string]float64 `yaml:"weights" json:"weights"`
}

type WorkloadSpec struct {
	Name             string  `yaml:"name" json:"name"`
	Visibility       string  `yaml:"visibility,omitempty" json:"visibility,omitempty"` // "all" | "dashboard" | "offscreen"
	VisibleCount     int     `yaml:"visible_count,omitempty" json:"visible_count,omitempty"`
	SuspendOffscreen bool    `yaml:"suspend_offscreen,omitempty" json:"suspend_offscreen,omitempty"`
	DisplayScale     float64 `yaml:"display_scale,omitempty" json:"display_scale,omitempty"` // e.g. 0.5, 1.0, 2.0, 4.0
	ScalingMode      string  `yaml:"scaling_mode,omitempty" json:"scaling_mode,omitempty"`   // "native" | "viewbox" | "css-transform" | "authored-size"
}

type OutputConfig struct {
	Dir            string `yaml:"dir,omitempty" json:"dir,omitempty"`
	Samples        bool   `yaml:"samples" json:"samples"`
	Screenshots    bool   `yaml:"screenshots" json:"screenshots"`
	RawResults     bool   `yaml:"raw_results" json:"raw_results"`
	HTMLReport     bool   `yaml:"html_report" json:"html_report"`
	MarkdownReport bool   `yaml:"markdown_report" json:"markdown_report"`
	DecisionReport bool   `yaml:"decision_report" json:"decision_report"`
}

var ValidMeasurements = map[string]bool{
	"process_cpu":           true,
	"frame_timing":          true,
	"long_tasks":            true,
	"rendering_activity":    true,
	"memory":                true,
	"animation_correctness": true,
}

// LoadConfig reads, resolves inheritance/presets, applies defaults, and validates a YAML config.
func LoadConfig(path string) (*Config, error) {
	cfg, err := loadConfigRaw(path, map[string]bool{})
	if err != nil {
		return nil, err
	}
	if err := resolvePresets(cfg); err != nil {
		return nil, err
	}
	applyDefaults(cfg)
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadConfigRaw(path string, visited map[string]bool) (*Config, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	if visited[absPath] {
		return nil, fmt.Errorf("YAML inheritance cycle detected involving %s", path)
	}
	if len(visited) > 10 {
		return nil, errors.New("YAML inheritance depth exceeded")
	}
	visited[absPath] = true
	defer delete(visited, absPath)

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML %s: %w", path, err)
	}
	var rawMap map[string]any
	_ = yaml.Unmarshal(raw, &rawMap)

	if cfg.Inherits != "" {
		basePath := cfg.Inherits
		if !filepath.IsAbs(basePath) {
			basePath = filepath.Join(filepath.Dir(path), basePath)
		}
		baseCfg, err := loadConfigRaw(basePath, visited)
		if err != nil {
			return nil, fmt.Errorf("loading inherited base %s: %w", basePath, err)
		}
		cfg = mergeConfigs(*baseCfg, cfg, rawMap)
	}
	cfg.ConfigDir = filepath.Dir(absPath)
	return &cfg, nil
}

func rawHasKey(m map[string]any, section, key string) bool {
	if m == nil {
		return false
	}
	sub, ok := m[section].(map[string]any)
	if !ok {
		return false
	}
	_, exists := sub[key]
	return exists
}

func mergeConfigs(base, over Config, overRaw map[string]any) Config {
	out := base
	if over.Version != 0 {
		out.Version = over.Version
	}
	out.Inherits = over.Inherits
	if len(over.Presets) > 0 {
		if out.Presets == nil {
			out.Presets = map[string]EffectSpec{}
		}
		for k, v := range over.Presets {
			out.Presets[k] = v
		}
	}
	if over.Test.Name != "" {
		out.Test.Name = over.Test.Name
	}
	if over.Test.Description != "" {
		out.Test.Description = over.Test.Description
	}
	if over.Browser.Engine != "" {
		out.Browser.Engine = over.Browser.Engine
	}
	if rawHasKey(overRaw, "browser", "headless") {
		out.Browser.Headless = over.Browser.Headless
	}
	if over.Browser.CDPEndpoint != "" {
		out.Browser.CDPEndpoint = over.Browser.CDPEndpoint
	}
	if over.Browser.HostBindIP != "" {
		out.Browser.HostBindIP = over.Browser.HostBindIP
	}
	if over.Browser.ProcessCPUHost != "" {
		out.Browser.ProcessCPUHost = over.Browser.ProcessCPUHost
	}
	if over.Browser.ViewportWidth > 0 {
		out.Browser.ViewportWidth = over.Browser.ViewportWidth
	}
	if over.Browser.ViewportHeight > 0 {
		out.Browser.ViewportHeight = over.Browser.ViewportHeight
	}
	if over.Browser.DeviceScaleFactor > 0 {
		out.Browser.DeviceScaleFactor = over.Browser.DeviceScaleFactor
	}
	if over.Execution.WarmupSeconds > 0 || rawHasKey(overRaw, "execution", "warmup_seconds") {
		out.Execution.WarmupSeconds = over.Execution.WarmupSeconds
	}
	if over.Execution.DurationSeconds > 0 {
		out.Execution.DurationSeconds = over.Execution.DurationSeconds
	}
	if over.Execution.Repetitions > 0 {
		out.Execution.Repetitions = over.Execution.Repetitions
	}
	if over.Execution.RandomSeed != 0 || rawHasKey(overRaw, "execution", "random_seed") {
		out.Execution.RandomSeed = over.Execution.RandomSeed
	}
	if over.Execution.AlternateMethodOrder || rawHasKey(overRaw, "execution", "alternate_method_order") {
		out.Execution.AlternateMethodOrder = over.Execution.AlternateMethodOrder
	}
	if over.Execution.OrderStrategy != "" {
		out.Execution.OrderStrategy = over.Execution.OrderStrategy
	}
	if len(over.Instances.Counts) > 0 {
		out.Instances.Counts = over.Instances.Counts
	}
	if over.Instances.Isolation != "" {
		out.Instances.Isolation = over.Instances.Isolation
	}
	if over.Scene != nil {
		out.Scene = over.Scene
	}
	if over.WidgetRuntime != nil {
		out.WidgetRuntime = over.WidgetRuntime
	}
	if len(over.Effects) > 0 {
		out.Effects = over.Effects
	}
	if over.Mixture != nil {
		out.Mixture = over.Mixture
	}
	if len(over.Workloads) > 0 {
		out.Workloads = over.Workloads
	}
	if len(over.Measurements) > 0 {
		out.Measurements = over.Measurements
	}
	if over.Output.Dir != "" {
		out.Output.Dir = over.Output.Dir
	}
	if rawHasKey(overRaw, "output", "samples") {
		out.Output.Samples = over.Output.Samples
	}
	if rawHasKey(overRaw, "output", "screenshots") {
		out.Output.Screenshots = over.Output.Screenshots
	}
	if rawHasKey(overRaw, "output", "raw_results") {
		out.Output.RawResults = over.Output.RawResults
	}
	if rawHasKey(overRaw, "output", "html_report") {
		out.Output.HTMLReport = over.Output.HTMLReport
	}
	if rawHasKey(overRaw, "output", "markdown_report") {
		out.Output.MarkdownReport = over.Output.MarkdownReport
	}
	if rawHasKey(overRaw, "output", "decision_report") {
		out.Output.DecisionReport = over.Output.DecisionReport
	}
	return out
}

func applyDefaults(cfg *Config) {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Browser.Engine == "" {
		cfg.Browser.Engine = "chromium"
	}
	if cfg.Browser.CDPEndpoint == "" {
		cfg.Browser.CDPEndpoint = "http://127.0.0.1:9222"
	}
	if cfg.Browser.ViewportWidth <= 0 {
		cfg.Browser.ViewportWidth = 1366
	}
	if cfg.Browser.ViewportHeight <= 0 {
		cfg.Browser.ViewportHeight = 900
	}
	if cfg.Execution.DurationSeconds <= 0 {
		cfg.Execution.DurationSeconds = 5
	}
	if cfg.Execution.Repetitions <= 0 {
		cfg.Execution.Repetitions = 1
	}
	if cfg.Execution.RandomSeed == 0 {
		cfg.Execution.RandomSeed = 12345
	}
	if cfg.Browser.DeviceScaleFactor <= 0 {
		cfg.Browser.DeviceScaleFactor = 1.0
	}
	if cfg.Execution.OrderStrategy == "" {
		if cfg.Execution.AlternateMethodOrder {
			cfg.Execution.OrderStrategy = "alternating"
		} else {
			cfg.Execution.OrderStrategy = "sequential"
		}
	}
	if cfg.Instances.Isolation == "" {
		cfg.Instances.Isolation = "independent-svg-roots"
	}
	if len(cfg.Workloads) == 0 {
		cfg.Workloads = []WorkloadSpec{{Name: "all-active", Visibility: "all"}}
	}
	if len(cfg.Measurements) == 0 {
		cfg.Measurements = []string{"process_cpu", "frame_timing", "long_tasks", "rendering_activity", "memory", "animation_correctness"}
	}
	for i := range cfg.Effects {
		e := &cfg.Effects[i]
		if len(e.Shapes) == 0 {
			e.Shapes = defaultShapesForFamily(e.Family)
		}
		if len(e.Sizes) == 0 {
			e.Sizes = map[string][2]int{"medium": {120, 80}}
		}
		if len(e.Speeds) == 0 {
			e.Speeds = map[string]float64{"medium": 3.0}
		}
		if e.Family == "perimeter-tracer" {
			if len(e.SegmentCounts) == 0 {
				e.SegmentCounts = []int{2}
			}
			if len(e.LineGapRatios) == 0 {
				e.LineGapRatios = []string{"1:3"}
			}
		}
		if e.Family == "pulse" {
			if len(e.PulseModes) == 0 {
				e.PulseModes = []string{"scale-opacity"}
			}
			if len(e.PulseAmplitudes) == 0 {
				e.PulseAmplitudes = []float64{0.2}
			}
		}
		if e.Family == "flash" {
			if len(e.FlashModes) == 0 {
				e.FlashModes = []string{"repeating"}
			}
			if e.FlashDurationMs <= 0 {
				e.FlashDurationMs = 120
			}
			if e.FlashIntervalS <= 0 {
				e.FlashIntervalS = 1.2
			}
		}
	}
}

func defaultShapesForFamily(fam string) []string {
	switch fam {
	case "perimeter-tracer":
		return []string{"rectangle", "rounded-rectangle", "circle", "ellipse"}
	case "pulse":
		return []string{"circle", "ellipse", "rectangle", "diamond"}
	case "flash":
		return []string{"diamond", "cross", "star"}
	case "flow-line", "flow-particle":
		return []string{"straight", "curved"}
	default:
		return []string{"circle"}
	}
}

func resolvePresets(cfg *Config) error {
	for i := range cfg.Effects {
		e := &cfg.Effects[i]
		if e.Preset == "" {
			continue
		}
		p, ok := cfg.Presets[e.Preset]
		if !ok {
			return fmt.Errorf("effect[%d] references unknown preset %q", i, e.Preset)
		}
		if e.Family == "" {
			e.Family = p.Family
		}
		if len(e.Methods) == 0 {
			e.Methods = append([]string(nil), p.Methods...)
		}
		if len(e.Shapes) == 0 {
			e.Shapes = append([]string(nil), p.Shapes...)
		}
		if len(e.Sizes) == 0 && len(p.Sizes) > 0 {
			e.Sizes = map[string][2]int{}
			for k, v := range p.Sizes {
				e.Sizes[k] = v
			}
		}
		if len(e.Speeds) == 0 && len(p.Speeds) > 0 {
			e.Speeds = map[string]float64{}
			for k, v := range p.Speeds {
				e.Speeds[k] = v
			}
		}
		if len(e.SegmentCounts) == 0 {
			e.SegmentCounts = append([]int(nil), p.SegmentCounts...)
		}
		if len(e.LineGapRatios) == 0 {
			e.LineGapRatios = append([]string(nil), p.LineGapRatios...)
		}
		if len(e.Directions) == 0 {
			e.Directions = append([]string(nil), p.Directions...)
		}
		if len(e.PulseModes) == 0 {
			e.PulseModes = append([]string(nil), p.PulseModes...)
		}
		if len(e.PulseAmplitudes) == 0 {
			e.PulseAmplitudes = append([]float64(nil), p.PulseAmplitudes...)
		}
		if len(e.FlashModes) == 0 {
			e.FlashModes = append([]string(nil), p.FlashModes...)
		}
		if e.FlashDurationMs <= 0 && p.FlashDurationMs > 0 {
			e.FlashDurationMs = p.FlashDurationMs
		}
		if e.FlashIntervalS <= 0 && p.FlashIntervalS > 0 {
			e.FlashIntervalS = p.FlashIntervalS
		}
		if e.RAFScheduling == "" {
			e.RAFScheduling = p.RAFScheduling
		}
		if e.VariationStrategy == "" {
			e.VariationStrategy = p.VariationStrategy
		}
	}
	return nil
}

// ValidateConfig checks schema invariants and returns clear error messages.
func ValidateConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("nil configuration")
	}
	if cfg.Version != 1 {
		return fmt.Errorf("unsupported config version %d (expected 1)", cfg.Version)
	}
	if strings.TrimSpace(cfg.Test.Name) == "" {
		return errors.New("test.name is required")
	}
	if strings.ToLower(cfg.Browser.Engine) != "chromium" {
		return fmt.Errorf("unsupported browser.engine %q (supported: chromium)", cfg.Browser.Engine)
	}
	if cfg.Execution.WarmupSeconds < 0 {
		return errors.New("execution.warmup_seconds cannot be negative")
	}
	if cfg.Execution.DurationSeconds <= 0 {
		return errors.New("execution.duration_seconds must be > 0")
	}
	if cfg.Execution.Repetitions < 1 {
		return errors.New("execution.repetitions must be >= 1")
	}
	if cfg.Execution.OrderStrategy != "" && cfg.Execution.OrderStrategy != "sequential" && cfg.Execution.OrderStrategy != "alternating" && cfg.Execution.OrderStrategy != "randomized" {
		return fmt.Errorf("execution.order_strategy must be 'sequential', 'alternating', or 'randomized', got %q", cfg.Execution.OrderStrategy)
	}
	if len(cfg.Instances.Counts) == 0 {
		return errors.New("instances.counts must specify at least one positive integer count")
	}
	for _, c := range cfg.Instances.Counts {
		if c <= 0 {
			return fmt.Errorf("instances.counts entries must be positive integers, got %d", c)
		}
	}
	if cfg.Instances.Isolation != "independent-svg-roots" &&
		cfg.Instances.Isolation != "shared-scene" &&
		cfg.Instances.Isolation != "shared-svg-scene" &&
		cfg.Instances.Isolation != "shared-dt-scene" &&
		cfg.Instances.Isolation != "stage16c-widgets" {
		return fmt.Errorf("instances.isolation must be 'independent-svg-roots', 'shared-scene', 'shared-svg-scene', 'shared-dt-scene', or 'stage16c-widgets', got %q", cfg.Instances.Isolation)
	}
	if cfg.Instances.Isolation == "stage16c-widgets" {
		if cfg.WidgetRuntime == nil || strings.TrimSpace(cfg.WidgetRuntime.PackageDir) == "" {
			return errors.New("widget_runtime.package_dir is required when instances.isolation is 'stage16c-widgets'")
		}
		pkgDir := resolveAssetPath(cfg.ConfigDir, cfg.WidgetRuntime.PackageDir)
		pkg, err := LoadStage16CPackage(pkgDir)
		if err != nil {
			return fmt.Errorf("widget_runtime: %w", err)
		}
		if cfg.WidgetRuntime.Updates != nil && len(cfg.WidgetRuntime.Updates.Properties) > 0 {
			if _, _, err := ValidateWidgetProperties(pkg, cfg.WidgetRuntime.Updates.Properties); err != nil {
				return fmt.Errorf("widget_runtime.updates: %w", err)
			}
		}
	} else {
		if len(cfg.Effects) == 0 && cfg.Mixture == nil {
			return errors.New("at least one entry in effects or a mixture section is required")
		}
	}
	if cfg.Scene != nil && cfg.Scene.Background != nil {
		if cfg.Scene.Background.Fit != "" && cfg.Scene.Background.Fit != "contain" && cfg.Scene.Background.Fit != "cover" && cfg.Scene.Background.Fit != "fill" {
			return fmt.Errorf("scene.background.fit must be 'contain', 'cover', or 'fill', got %q", cfg.Scene.Background.Fit)
		}
		if cfg.Scene.Background.File != "" {
			bgPath := resolveAssetPath(cfg.ConfigDir, cfg.Scene.Background.File)
			if _, err := os.Stat(bgPath); err != nil {
				return fmt.Errorf("scene.background.file %q not found (resolved: %s): %w", cfg.Scene.Background.File, bgPath, err)
			}
		}
	}

	reg := DefaultRegistry()
	for i, e := range cfg.Effects {
		if e.Family == "flow" {
			return fmt.Errorf("effects[%d]: family 'flow' has been split into 'flow-line' and 'flow-particle' for visual equivalence; please update your YAML", i)
		}
		gen, ok := reg[e.Family]
		if !ok {
			return fmt.Errorf("effects[%d]: unknown family %q", i, e.Family)
		}
		if e.RAFScheduling != "" && e.RAFScheduling != "shared" && e.RAFScheduling != "per-instance" {
			return fmt.Errorf("effects[%d] (%s): raf_scheduling must be 'shared' or 'per-instance', got %q", i, e.Family, e.RAFScheduling)
		}
		if e.VariationStrategy != "" && e.VariationStrategy != "representative" && e.VariationStrategy != "factorial" && e.VariationStrategy != "isolated" {
			return fmt.Errorf("effects[%d] (%s): variation_strategy must be 'representative', 'factorial', or 'isolated', got %q", i, e.Family, e.VariationStrategy)
		}
		for _, d := range e.Directions {
			if d != "cw" && d != "ccw" {
				return fmt.Errorf("effects[%d] (%s): direction must be 'cw' or 'ccw', got %q", i, e.Family, d)
			}
		}
		if len(e.Methods) == 0 {
			return fmt.Errorf("effects[%d] (%s): at least one rendering method is required", i, e.Family)
		}
		for _, m := range e.Methods {
			if !gen.SupportsMethod(m) {
				return fmt.Errorf("effects[%d] (%s): unsupported method %q (supported: %v)", i, e.Family, m, gen.SupportedMethods())
			}
		}
		for szName, dims := range e.Sizes {
			if dims[0] <= 0 || dims[1] <= 0 {
				return fmt.Errorf("effects[%d].sizes[%s]: width and height must be positive, got %v", i, szName, dims)
			}
		}
		for spName, sec := range e.Speeds {
			if sec <= 0 {
				return fmt.Errorf("effects[%d].speeds[%s]: duration must be > 0, got %v", i, spName, sec)
			}
		}
	}

	if cfg.Mixture != nil {
		if len(cfg.Mixture.Weights) == 0 {
			return errors.New("mixture.weights must specify at least one effect family weight")
		}
		totalW := 0.0
		for fam, w := range cfg.Mixture.Weights {
			if fam == "flow" {
				return errors.New("mixture.weights: family 'flow' has been split into 'flow-line' and 'flow-particle'; please update your YAML")
			}
			if _, ok := reg[fam]; !ok {
				return fmt.Errorf("mixture.weights: unknown effect family %q", fam)
			}
			if w <= 0 {
				return fmt.Errorf("mixture.weights[%s]: weight must be > 0, got %v", fam, w)
			}
			totalW += w
		}
		if totalW <= 0 {
			return errors.New("mixture.weights total must be > 0")
		}
		if len(cfg.Mixture.Methods) == 0 {
			return errors.New("mixture.methods must specify at least one method")
		}
		for fam := range cfg.Mixture.Weights {
			if fam == "static-gauge" {
				continue
			}
			gen := reg[fam]
			for _, m := range cfg.Mixture.Methods {
				if !gen.SupportsMethod(m) {
					return fmt.Errorf("mixture: family %q does not support method %q (supported: %v); silent substitution is disallowed", fam, m, gen.SupportedMethods())
				}
			}
		}
	}

	for i, wl := range cfg.Workloads {
		if strings.TrimSpace(wl.Name) == "" {
			return fmt.Errorf("workloads[%d].name is required", i)
		}
		if wl.Visibility != "" && wl.Visibility != "all" && wl.Visibility != "dashboard" && wl.Visibility != "offscreen" {
			return fmt.Errorf("workloads[%d].visibility must be 'all', 'dashboard', or 'offscreen', got %q", i, wl.Visibility)
		}
		if wl.VisibleCount < 0 {
			return fmt.Errorf("workloads[%d].visible_count cannot be negative, got %d", i, wl.VisibleCount)
		}
		if wl.DisplayScale < 0 {
			return fmt.Errorf("workloads[%d].display_scale cannot be negative, got %v", i, wl.DisplayScale)
		}
		if wl.ScalingMode != "" && wl.ScalingMode != "native" && wl.ScalingMode != "viewbox" && wl.ScalingMode != "css-transform" && wl.ScalingMode != "authored-size" {
			return fmt.Errorf("workloads[%d].scaling_mode must be 'native', 'viewbox', 'css-transform', or 'authored-size', got %q", i, wl.ScalingMode)
		}
	}

	for _, m := range cfg.Measurements {
		if !ValidMeasurements[m] {
			return fmt.Errorf("unknown measurement %q", m)
		}
	}
	return nil
}
