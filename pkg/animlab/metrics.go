package animlab

import (
	"math"
	"sort"
)

const MeasurementSpecVersion = "2.0.0"

// TrialStatus classifies the integrity of a single experimental repetition.
type TrialStatus string

const (
	TrialValid        TrialStatus = "valid"
	TrialPartial      TrialStatus = "partial"
	TrialFailed       TrialStatus = "failed"
	TrialContaminated TrialStatus = "contaminated"
)

// MetricDefinition documents the exact semantics of each reported benchmark metric.
type MetricDefinition struct {
	Name                 string `json:"name"`
	SourceAPI            string `json:"sourceApi"`
	Unit                 string `json:"unit"`
	CollectionInterval   string `json:"collectionInterval"`
	Scope                string `json:"scope"`
	Aggregation          string `json:"aggregation"`
	Limitations          string `json:"limitations"`
	UnavailableCondition string `json:"unavailableCondition"`
}

// DistributionStats summarises repeated-trial spread without hiding variance behind means alone.
type DistributionStats struct {
	N      int     `json:"n"`
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"stdDev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Median float64 `json:"median"`
}

// FrameStats holds rAF callback interval measurements (explicitly NOT presented/compositor FPS).
type FrameStats struct {
	Frames             int     `json:"frames"`
	RafHz              float64 `json:"rafHz"`
	AvgRafIntervalMs   float64 `json:"avgRafIntervalMs"`
	P95RafIntervalMs   float64 `json:"p95RafIntervalMs"`
	MaxRafIntervalMs   float64 `json:"maxRafIntervalMs"`
	LongFramesOver16Ms int     `json:"longFramesOver16ms"` // > 16.67 ms (60 Hz budget)
	LongFramesOver20Ms int     `json:"longFramesOver20ms"` // > 20.0 ms
	LongFramesOver50Ms int     `json:"longFramesOver50ms"` // > 50.0 ms
}

// DefaultMetricDefinitions returns the authoritative M2 metric catalog.
func DefaultMetricDefinitions() map[string]MetricDefinition {
	return map[string]MetricDefinition{
		"processCpuSec": {
			Name:                 "processCpuSec",
			SourceAPI:            "Linux /proc/<pid>/stat (utime+stime) scoped to owned Chromium PID tree via CLK_TCK",
			Unit:                 "seconds",
			CollectionInterval:   "Delta across measurement window (T_end - T_start)",
			Scope:                "Owned browser root PID and its recursive subprocesses only (browser, zygote, renderer, gpu, utility)",
			Aggregation:          "Mean, StdDev, Min, Max across valid repetitions",
			Limitations:          "Requires /proc access on browser host; reflects CPU time across all threads in the owned browser tree",
			UnavailableCondition: "Reported as null / N/A if browser PID ownership cannot be established or ProcessCPUHost is unreachable",
		},
		"rafHz": {
			Name:                 "rafHz",
			SourceAPI:            "Browser requestAnimationFrame(performance.now()) callback delta",
			Unit:                 "Hz (callbacks/sec)",
			CollectionInterval:   "Every rAF callback during measurement window (first 2 warmup callbacks discarded)",
			Scope:                "Page main-thread rAF loop",
			Aggregation:          "1000.0 / mean(intervalMs) per trial; Mean/StdDev across repetitions",
			Limitations:          "Measures main-thread rAF scheduling cadence only; MUST NOT be interpreted as visually presented or compositor FPS",
			UnavailableCondition: "Reported as null / N/A if fewer than 2 rAF samples are captured",
		},
		"presentedFps": {
			Name:                 "presentedFps",
			SourceAPI:            "Compositor frame presentation counter (unavailable in standard headless SwiftShader CDP without tracing)",
			Unit:                 "frames/sec",
			CollectionInterval:   "Measurement window",
			Scope:                "Display compositor",
			Aggregation:          "Mean across valid repetitions",
			Limitations:          "Not fabricated from rAF; requires compositor frame-sink instrumentation",
			UnavailableCondition: "Always null / N/A in standard headless/SwiftShader CDP runs unless explicit compositor frame telemetry is available",
		},
		"avgRafIntervalMs": {
			Name:                 "avgRafIntervalMs",
			SourceAPI:            "Browser requestAnimationFrame(performance.now()) delta",
			Unit:                 "ms",
			CollectionInterval:   "Every rAF callback during measurement window",
			Scope:                "Page main-thread",
			Aggregation:          "Mean per trial; Mean/StdDev across repetitions",
			Limitations:          "Reflects main-thread callback pacing, not GPU rasterization",
			UnavailableCondition: "null / N/A if frame_timing disabled or no callbacks observed",
		},
		"longFramesOver16ms": {
			Name:                 "longFramesOver16ms",
			SourceAPI:            "Browser rAF interval > 16.67 ms",
			Unit:                 "count",
			CollectionInterval:   "Measurement window",
			Scope:                "Page main-thread rAF callbacks",
			Aggregation:          "Sum or mean count per trial",
			Limitations:          "Threshold 16.67 ms (60 Hz budget); distinct from > 20.0 ms",
			UnavailableCondition: "0 if no rAF intervals exceed 16.67 ms",
		},
		"longFramesOver20ms": {
			Name:                 "longFramesOver20ms",
			SourceAPI:            "Browser rAF interval > 20.0 ms",
			Unit:                 "count",
			CollectionInterval:   "Measurement window",
			Scope:                "Page main-thread rAF callbacks",
			Aggregation:          "Mean count per trial",
			Limitations:          "Explicitly separates >20 ms stalls from >16.67 ms budget misses",
			UnavailableCondition: "0 if no rAF intervals exceed 20.0 ms",
		},
		"longTasks": {
			Name:                 "longTasks",
			SourceAPI:            "PerformanceObserver({ entryTypes: ['longtask'] })",
			Unit:                 "count / ms",
			CollectionInterval:   "Measurement window",
			Scope:                "Page main-thread tasks > 50 ms",
			Aggregation:          "Count and cumulative duration (ms)",
			Limitations:          "Only captures tasks >= 50 ms on the main thread",
			UnavailableCondition: "null / N/A if PerformanceObserver longtask is unsupported",
		},
		"taskDeltaMs": {
			Name:                 "taskDeltaMs",
			SourceAPI:            "CDP Performance.getMetrics (TaskDuration)",
			Unit:                 "ms",
			CollectionInterval:   "Delta across measurement window",
			Scope:                "Target renderer main thread",
			Aggregation:          "Mean/StdDev across repetitions",
			Limitations:          "Renderer thread only; excludes compositor/GPU processes",
			UnavailableCondition: "null / N/A if CDP Performance.getMetrics fails",
		},
		"jsHeapMB": {
			Name:                 "jsHeapMB",
			SourceAPI:            "CDP Performance.getMetrics (JSHeapUsedSize)",
			Unit:                 "MiB",
			CollectionInterval:   "End of measurement window",
			Scope:                "Target V8 isolate",
			Aggregation:          "Mean across repetitions; interpreted with GC tolerance",
			Limitations:          "Subject to normal V8 GC timing fluctuations",
			UnavailableCondition: "null / N/A if CDP Performance.getMetrics fails",
		},
		"domNodes": {
			Name:                 "domNodes",
			SourceAPI:            "CDP Performance.getMetrics (Nodes) and document.querySelectorAll('*').length",
			Unit:                 "count",
			CollectionInterval:   "End of measurement window",
			Scope:                "Isolated per-trial target document",
			Aggregation:          "Observed count per trial",
			Limitations:          "Should remain stable across identical isolated trials",
			UnavailableCondition: "null / N/A if CDP metrics unavailable",
		},
	}
}

func computeFrameStats(frames []float64) FrameStats {
	if len(frames) == 0 {
		return FrameStats{}
	}
	sorted := append([]float64(nil), frames...)
	sort.Float64s(sorted)
	sum, maxF := 0.0, 0.0
	over16, over20, over50 := 0, 0, 0
	for _, f := range frames {
		sum += f
		if f > maxF {
			maxF = f
		}
		if f > 16.67 {
			over16++
		}
		if f > 20.0 {
			over20++
		}
		if f > 50.0 {
			over50++
		}
	}
	mean := sum / float64(len(frames))
	p95 := sorted[int(math.Floor(0.95*float64(len(sorted)-1)))]
	hz := 0.0
	if mean > 0 {
		hz = 1000.0 / mean
	}
	return FrameStats{
		Frames:             len(frames),
		RafHz:              hz,
		AvgRafIntervalMs:   mean,
		P95RafIntervalMs:   p95,
		MaxRafIntervalMs:   maxF,
		LongFramesOver16Ms: over16,
		LongFramesOver20Ms: over20,
		LongFramesOver50Ms: over50,
	}
}

func computeDistribution(vals []float64) *DistributionStats {
	if len(vals) == 0 {
		return nil
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(len(sorted))
	varSq := 0.0
	for _, v := range sorted {
		d := v - mean
		varSq += d * d
	}
	stdDev := 0.0
	if len(sorted) > 1 {
		stdDev = math.Sqrt(varSq / float64(len(sorted)-1))
	}
	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = 0.5 * (sorted[len(sorted)/2-1] + sorted[len(sorted)/2])
	}
	return &DistributionStats{
		N:      len(sorted),
		Mean:   mean,
		StdDev: stdDev,
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		Median: median,
	}
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }
