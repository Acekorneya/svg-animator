package animlab

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// FaultCode classifies animation correctness failures deterministically.
type FaultCode string

const (
	FaultNone                       FaultCode = ""
	FaultMissingElements            FaultCode = "MISSING_ELEMENTS"
	FaultStationary                 FaultCode = "STATIONARY"
	FaultWrongDirection             FaultCode = "WRONG_DIRECTION"
	FaultWrongSpeed                 FaultCode = "WRONG_SPEED"
	FaultLoopDiscontinuity          FaultCode = "LOOP_DISCONTINUITY"
	FaultWrongAmplitude             FaultCode = "WRONG_AMPLITUDE"
	FaultModeMismatch               FaultCode = "MODE_MISMATCH"
	FaultMissedFlash                FaultCode = "MISSED_FLASH"
	FaultStuckActive                FaultCode = "STUCK_ACTIVE"
	FaultPartialPopulationFailure   FaultCode = "PARTIAL_POPULATION_FAILURE"
	FaultSuspensionLeak             FaultCode = "SUSPENSION_LEAK"
	FaultResumptionFailed           FaultCode = "RESUMPTION_FAILED"
	FaultUnsupportedOrNonEquivalent FaultCode = "UNSUPPORTED_OR_NON_EQUIVALENT"
)

// CorrectnessResult captures two-level (whole-population + detailed temporal) verification.
type CorrectnessResult struct {
	Passed           bool     `json:"passed"`
	FaultCode        string   `json:"faultCode,omitempty"`
	Reason           string   `json:"reason"`
	LevelAPassed     bool     `json:"levelAPassed"`
	LevelBPassed     bool     `json:"levelBPassed"`
	SampledDelta     float64  `json:"sampledDelta,omitempty"`
	SuspendedCount   int      `json:"suspendedCount,omitempty"`
	VisibleCount     int      `json:"visibleCount,omitempty"`
	CheckedIndices   []int    `json:"checkedIndices,omitempty"`
	FailingInstances []string `json:"failingInstances,omitempty"`
}

type popNodeState struct {
	Idx        int     `json:"idx"`
	ID         string  `json:"id"`
	Family     string  `json:"family"`
	Method     string  `json:"method"`
	Suspended  bool    `json:"suspended"`
	HasAnim    bool    `json:"hasAnim"`
	Off        float64 `json:"off"`
	Op         float64 `json:"op"`
	Scale      float64 `json:"scale"`
	DashVal    float64 `json:"dashVal"`
	Cx         float64 `json:"cx"`
	Cy         float64 `json:"cy"`
	Tr         string  `json:"tr"`
	AttrTr     string  `json:"attrTr"`
}

type popProbe struct {
	Count int            `json:"count"`
	Vis   int            `json:"vis"`
	Susp  int            `json:"susp"`
	Items []popNodeState `json:"items"`
}

const jsEvalPopulationProbe = `(() => {
	const payloads = Array.from(document.querySelectorAll('.lab-payload'));
	const suspendedSet = window.__labSuspendedSet || new Set();
	const rafIds = new Set((window.__labRafDrivers || []).map(d => d.id));
	const dataIds = new Set((window.__labDataDrivers || []).map(d => d.id));
	const parseScale = (csTr, attrTr) => {
		if (csTr && csTr !== 'none') {
			const m = csTr.match(/matrix\(([^)]+)\)/);
			if (m) {
				const parts = m[1].split(',').map(Number);
				if (parts.length >= 4 && !Number.isNaN(parts[0]) && !Number.isNaN(parts[1])) {
					return Math.hypot(parts[0], parts[1]);
				}
			}
		}
		if (attrTr) {
			const sm = attrTr.match(/scale\(\s*([0-9.eE+-]+)/);
			if (sm) return parseFloat(sm[1]) || 1;
		}
		return 1;
	};
	const items = payloads.map((el, i) => {
		const inst = el.closest('.lab-instance, svg');
		const id = (inst && inst.id) ? inst.id : (el.id || ('idx-' + i));
		const idxAttr = inst ? inst.getAttribute('data-lab-idx') : null;
		const idx = idxAttr !== null ? parseInt(idxAttr, 10) : i;
		const fam = el.getAttribute('data-lab-family') || '';
		const method = el.getAttribute('data-lab-method') || '';
		const cs = getComputedStyle(el);
		const rect = el.getBoundingClientRect();
		const dashStr = cs.strokeDasharray || el.getAttribute('stroke-dasharray') || '';
		const dashVal = parseFloat(dashStr) || 0;
		const hasSmil = !!el.querySelector('animate, animateTransform, animateMotion');
		const hasCssAnim = !!(cs.animationName && cs.animationName !== 'none');
		const hasDriver = rafIds.has(id) || dataIds.has(id);
		let hasAnim = true;
		if (method === 'smil') hasAnim = hasSmil;
		else if (method === 'css') hasAnim = hasCssAnim;
		else if (method === 'javascript-raf' || method === 'data-driven') hasAnim = hasDriver;
		return {
			idx,
			id,
			family: fam,
			method,
			suspended: suspendedSet.has(id),
			hasAnim,
			off: parseFloat(cs.strokeDashoffset) || 0,
			op: parseFloat(cs.opacity) || 0,
			scale: parseScale(cs.transform, el.getAttribute('transform') || ''),
			dashVal,
			cx: rect.left + rect.width * 0.5,
			cy: rect.top + rect.height * 0.5,
			tr: cs.transform || '',
			attrTr: el.getAttribute('transform') || ''
		};
	});
	return JSON.stringify({
		count: payloads.length,
		vis: (typeof window.__labVisibleCount === 'number') ? window.__labVisibleCount : payloads.length,
		susp: (typeof window.__labSuspendedCount === 'number') ? window.__labSuspendedCount : suspendedSet.size,
		items
	});
})()`

// EvaluateCorrectness runs Level A (whole-population) and Level B (temporal) correctness checks.
func EvaluateCorrectness(ctx context.Context, s TestRunSpec) CorrectnessResult {
	if s.Unsupported || s.NonEquivalent {
		reason := "unsupported or non-equivalent rendering method"
		if len(s.Limitations) > 0 {
			reason = strings.Join(s.Limitations, "; ")
		}
		return CorrectnessResult{
			Passed:    false,
			FaultCode: string(FaultUnsupportedOrNonEquivalent),
			Reason:    reason,
		}
	}
	if s.Method == "static" {
		return CorrectnessResult{
			Passed:       true,
			LevelAPassed: true,
			LevelBPassed: true,
			Reason:       "declared static baseline",
		}
	}

	// Level A: Whole-population 3-point incommensurate probe at T0, T1 (+130ms), T2 (+220ms; total 350ms).
	// Because gcd(130, 220, 350) = 10ms and minimum supported cycle duration is 200ms (0.2s),
	// no valid periodic animation can complete an integer number of cycles across all three intervals.
	p0, err := runPopulationProbe(ctx)
	if err != nil {
		return CorrectnessResult{
			Passed:    false,
			FaultCode: string(FaultMissingElements),
			Reason:    fmt.Sprintf("population probe T0 failed: %v", err),
		}
	}
	if p0.Count == 0 || (len(s.Instances) > 0 && p0.Count != len(s.Instances)) {
		return CorrectnessResult{
			Passed:    false,
			FaultCode: string(FaultMissingElements),
			Reason:    fmt.Sprintf("expected %d .lab-payload elements, found %d", len(s.Instances), p0.Count),
		}
	}

	_, _ = chromedp.Run(ctx, chromedp.Sleep(130*time.Millisecond))
	p1, err := runPopulationProbe(ctx)
	if err != nil || len(p1.Items) != len(p0.Items) {
		return CorrectnessResult{
			Passed:    false,
			FaultCode: string(FaultMissingElements),
			Reason:    "population probe T1 failed or element count changed",
		}
	}

	_, _ = chromedp.Run(ctx, chromedp.Sleep(220*time.Millisecond))
	p2, err := runPopulationProbe(ctx)
	if err != nil || len(p2.Items) != len(p0.Items) {
		return CorrectnessResult{
			Passed:    false,
			FaultCode: string(FaultMissingElements),
			Reason:    "population probe T2 failed or element count changed",
		}
	}

	if s.SuspendOffscreen && s.VisibleCount > 0 && s.Count > s.VisibleCount && p2.Susp == 0 {
		return CorrectnessResult{
			Passed:         false,
			FaultCode:      string(FaultSuspensionLeak),
			Reason:         "suspend_offscreen=true but 0 instances suspended",
			VisibleCount:   p2.Vis,
			SuspendedCount: p2.Susp,
		}
	}

	// Verify unique IDs and per-instance motion across ALL N instances
	seenIDs := make(map[string]bool, len(p0.Items))
	var activeIndices []int
	var suspendedIndices []int
	var frozenActive []string
	var leakedSuspended []string
	maxDelta := 0.0

	for i := 0; i < len(p0.Items); i++ {
		a := p0.Items[i]
		b := p1.Items[i]
		c := p2.Items[i]
		if seenIDs[a.ID] {
			return CorrectnessResult{
				Passed:           false,
				FaultCode:        string(FaultMissingElements),
				Reason:           fmt.Sprintf("duplicate instance id %q", a.ID),
				FailingInstances: []string{a.ID},
			}
		}
		seenIDs[a.ID] = true

		if a.Method == "static" || a.Family == "static-gauge" && a.Method != "data-driven" {
			continue
		}

		delta := math.Max(instanceDelta(a, b), math.Max(instanceDelta(a, c), instanceDelta(b, c)))
		if delta > maxDelta {
			maxDelta = delta
		}

		if a.Suspended || b.Suspended || c.Suspended {
			suspendedIndices = append(suspendedIndices, i)
			if delta > 0.05 {
				leakedSuspended = append(leakedSuspended, fmt.Sprintf("%s(idx=%d,delta=%.2f)", a.ID, a.Idx, delta))
			}
			continue
		}

		activeIndices = append(activeIndices, i)
		// Flash may be in its off-period during a 350ms window; Level B checks flash pulses exhaustively.
		if a.Family == "flash" {
			if !a.HasAnim {
				frozenActive = append(frozenActive, fmt.Sprintf("%s(idx=%d,no-anim)", a.ID, a.Idx))
			}
			continue
		}

		if !a.HasAnim || delta <= 0.005 {
			frozenActive = append(frozenActive, fmt.Sprintf("%s(idx=%d)", a.ID, a.Idx))
		}
	}

	if len(leakedSuspended) > 0 {
		return CorrectnessResult{
			Passed:           false,
			FaultCode:        string(FaultSuspensionLeak),
			Reason:           fmt.Sprintf("%d suspended instance(s) continued animating: %s", len(leakedSuspended), strings.Join(limitStrings(leakedSuspended, 5), ", ")),
			VisibleCount:     p2.Vis,
			SuspendedCount:   p2.Susp,
			FailingInstances: limitStrings(leakedSuspended, 10),
		}
	}

	if len(frozenActive) > 0 {
		fc := FaultPartialPopulationFailure
		if len(frozenActive) == len(activeIndices) {
			fc = FaultStationary
		}
		return CorrectnessResult{
			Passed:           false,
			FaultCode:        string(fc),
			Reason:           fmt.Sprintf("%d/%d active instances stationary or missing animation (%s)", len(frozenActive), len(activeIndices), strings.Join(limitStrings(frozenActive, 5), ", ")),
			SampledDelta:     maxDelta,
			VisibleCount:     p2.Vis,
			SuspendedCount:   p2.Susp,
			FailingInstances: limitStrings(frozenActive, 10),
		}
	}

	// Level B: Detailed temporal sampling on deterministic subset
	subset := selectDeterministicSubset(activeIndices, p0.Items, s.IsMixture)
	if len(subset) == 0 {
		return CorrectnessResult{
			Passed:       false,
			FaultCode:    string(FaultStationary),
			Reason:       "no active non-suspended instances available for Level B sampling",
			LevelAPassed: true,
		}
	}

	hasFlash := s.Family == "flash"
	maxFlashInterval := 1.0
	minContDur := 3.0
	for _, idx := range subset {
		if idx < len(s.Instances) {
			inst := s.Instances[idx]
			if inst.Family == "flash" {
				hasFlash = true
				if inst.FlashIntervalS > maxFlashInterval {
					maxFlashInterval = inst.FlashIntervalS
				}
			} else {
				d := math.Max(0.2, inst.Speed)
				if d < minContDur {
					minContDur = d
				}
			}
		}
	}

	cadence := 60 * time.Millisecond
	totalWindow := time.Duration(math.Min(2.4, math.Max(1.2, 1.2*minContDur)) * float64(time.Second))
	if hasFlash {
		cadence = 30 * time.Millisecond
		flashWin := math.Max(1.4, 1.35*maxFlashInterval)
		if flashWin > 4.2 {
			flashWin = 4.2
		}
		totalWindow = time.Duration(flashWin * float64(time.Second))
	}

	type timedSample struct {
		tSec  float64
		items []popNodeState
	}
	var series []timedSample
	start := time.Now()
	for {
		subProbe, err := runSubsetProbe(ctx, subset)
		if err != nil {
			return CorrectnessResult{
				Passed:         false,
				FaultCode:      string(FaultMissingElements),
				Reason:         fmt.Sprintf("Level B probe failed: %v", err),
				LevelAPassed:   true,
				CheckedIndices: subset,
			}
		}
		series = append(series, timedSample{
			tSec:  time.Since(start).Seconds(),
			items: subProbe,
		})
		if time.Since(start) >= totalWindow {
			break
		}
		_, _ = chromedp.Run(ctx, chromedp.Sleep(cadence))
	}

	// Validate temporal behavior for each subset instance
	for subPos, instIdx := range subset {
		var p InstanceParams
		if instIdx < len(s.Instances) {
			p = s.Instances[instIdx]
		} else {
			p = InstanceParams{
				ID:     p0.Items[instIdx].ID,
				Family: p0.Items[instIdx].Family,
				Method: p0.Items[instIdx].Method,
				Speed:  1.0,
			}
		}
		if p.Method == "static" || (p.Family == "static-gauge" && p.Method != "data-driven") {
			continue
		}

		// Collect time series for this subset slot
		ts := make([]float64, len(series))
		vals := make([]popNodeState, len(series))
		for k := range series {
			ts[k] = series[k].tSec
			if subPos < len(series[k].items) {
				vals[k] = series[k].items[subPos]
			}
		}

		if fc, reason := validateInstanceSeries(p, ts, vals); fc != FaultNone {
			return CorrectnessResult{
				Passed:           false,
				FaultCode:        string(fc),
				Reason:           reason,
				LevelAPassed:     true,
				LevelBPassed:     false,
				SampledDelta:     maxDelta,
				VisibleCount:     p1.Vis,
				SuspendedCount:   p1.Susp,
				CheckedIndices:   subset,
				FailingInstances: []string{p.ID},
			}
		}
	}

	// If suspend_offscreen is active and we have suspended instances, test scroll-into-view resumption (NC-08)
	if s.SuspendOffscreen && len(suspendedIndices) > 0 {
		targetIdx := suspendedIndices[0]
		targetID := p0.Items[targetIdx].ID
		_, _ = chromedp.Run(ctx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
			const el = document.getElementById(%q);
			if (el) el.scrollIntoView({ behavior: 'instant', block: 'center' });
			return 'ok';
		})()`, targetID)))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(250*time.Millisecond))
		r0, _ := runSubsetProbe(ctx, []int{targetIdx})
		_, _ = chromedp.Run(ctx, chromedp.Sleep(350*time.Millisecond))
		r1, _ := runSubsetProbe(ctx, []int{targetIdx})
		if len(r0) == 0 || len(r1) == 0 || r1[0].Suspended || instanceDelta(r0[0], r1[0]) <= 0.005 {
			return CorrectnessResult{
				Passed:           false,
				FaultCode:        string(FaultResumptionFailed),
				Reason:           fmt.Sprintf("offscreen instance %s failed to resume after scrollIntoView", targetID),
				LevelAPassed:     true,
				LevelBPassed:     false,
				VisibleCount:     p1.Vis,
				SuspendedCount:   p1.Susp,
				CheckedIndices:   subset,
				FailingInstances: []string{targetID},
			}
		}
	}

	return CorrectnessResult{
		Passed:         true,
		LevelAPassed:   true,
		LevelBPassed:   true,
		Reason:         fmt.Sprintf("Level A (%d instances) & Level B (%d sampled) verified (vis=%d, susp=%d)", len(p0.Items), len(subset), p1.Vis, p1.Susp),
		SampledDelta:   maxDelta,
		VisibleCount:   p1.Vis,
		SuspendedCount: p1.Susp,
		CheckedIndices: subset,
	}
}

func validateInstanceSeries(p InstanceParams, ts []float64, vals []popNodeState) (FaultCode, string) {
	if len(vals) < 3 {
		return FaultStationary, "insufficient temporal samples"
	}
	elapsed := ts[len(ts)-1] - ts[0]
	if elapsed <= 0.1 {
		return FaultStationary, "elapsed sampling window too short"
	}

	switch p.Family {
	case "perimeter-tracer", "flow-line":
		// Unwrap strokeDashoffset modulo 1000
		unwrapped := make([]float64, len(vals))
		unwrapped[0] = vals[0].Off
		for i := 1; i < len(vals); i++ {
			step := vals[i].Off - vals[i-1].Off
			if step > 500 {
				step -= 1000
			} else if step < -500 {
				step += 1000
			}
			// Check for non-wrap discontinuity (e.g. jump > 380 units in 60ms at normal speed)
			dur := math.Max(0.2, p.Speed)
			if p.Family == "flow-line" {
				dur = math.Max(0.3, p.Speed)
			}
			expectedStepMax := (1000.0 / dur) * (ts[i] - ts[i-1]) * 3.0
			if expectedStepMax < 250 {
				expectedStepMax = 250
			}
			if math.Abs(step) > expectedStepMax {
				return FaultLoopDiscontinuity, fmt.Sprintf("%s loop discontinuity at step %d: raw %.1f->%.1f (unwrapped step %.1f)", p.ID, i, vals[i-1].Off, vals[i].Off, step)
			}
			unwrapped[i] = unwrapped[i-1] + step
		}
		totalDelta := unwrapped[len(unwrapped)-1] - unwrapped[0]
		if math.Abs(totalDelta) < 20.0 {
			return FaultStationary, fmt.Sprintf("%s dashoffset stationary (delta=%.2f)", p.ID, totalDelta)
		}
		// Direction: cw => negative dashoffset delta; ccw => positive dashoffset delta
		dir := p.Direction
		if dir == "" {
			dir = "cw"
		}
		if dir == "cw" && totalDelta > 15.0 {
			return FaultWrongDirection, fmt.Sprintf("%s expected cw (negative delta) but moved +%.1f", p.ID, totalDelta)
		}
		if dir == "ccw" && totalDelta < -15.0 {
			return FaultWrongDirection, fmt.Sprintf("%s expected ccw (positive delta) but moved %.1f", p.ID, totalDelta)
		}
		// Speed check
		dur := math.Max(0.2, p.Speed)
		if p.Family == "flow-line" {
			dur = math.Max(0.3, p.Speed)
		}
		expectedRate := 1000.0 / dur
		obsRate := math.Abs(totalDelta) / elapsed
		if obsRate < expectedRate*0.45 || obsRate > expectedRate*2.2 {
			return FaultWrongSpeed, fmt.Sprintf("%s speed mismatch: observed %.1f u/s, expected %.1f u/s", p.ID, obsRate, expectedRate)
		}

	case "flow-particle":
		totalDist := 0.0
		for i := 1; i < len(vals); i++ {
			dx := vals[i].Cx - vals[i-1].Cx
			dy := vals[i].Cy - vals[i-1].Cy
			d := math.Hypot(dx, dy)
			// Ignore wrap-around jumps from end of path back to start
			if d < math.Max(float64(p.Width), 60)*0.75 {
				totalDist += d
			}
		}
		if totalDist < 8.0 {
			return FaultStationary, fmt.Sprintf("%s particle stationary (dist=%.2fpx)", p.ID, totalDist)
		}
		dur := math.Max(0.3, p.Speed)
		pathLen := float64(p.Width) - 20
		if pathLen < 30 {
			pathLen = 30
		}
		expectedRate := pathLen / dur
		obsRate := totalDist / elapsed
		if obsRate < expectedRate*0.30 || obsRate > expectedRate*2.8 {
			return FaultWrongSpeed, fmt.Sprintf("%s particle speed mismatch: observed %.1f px/s, expected %.1f px/s", p.ID, obsRate, expectedRate)
		}

	case "pulse":
		minOp, maxOp := vals[0].Op, vals[0].Op
		minSc, maxSc := vals[0].Scale, vals[0].Scale
		for _, v := range vals[1:] {
			if v.Op < minOp {
				minOp = v.Op
			}
			if v.Op > maxOp {
				maxOp = v.Op
			}
			if v.Scale < minSc {
				minSc = v.Scale
			}
			if v.Scale > maxSc {
				maxSc = v.Scale
			}
		}
		opRange := maxOp - minOp
		scRange := maxSc - minSc
		mode := p.PulseMode
		if mode == "" {
			mode = "scale-opacity"
		}
		switch mode {
		case "opacity-only":
			if scRange > 0.03 {
				return FaultModeMismatch, fmt.Sprintf("%s pulse opacity-only mode unexpectedly scaled (range=%.3f)", p.ID, scRange)
			}
			if opRange < 0.15 {
				return FaultWrongAmplitude, fmt.Sprintf("%s pulse opacity range too small (%.3f)", p.ID, opRange)
			}
		case "scale-only":
			if opRange > 0.03 {
				return FaultModeMismatch, fmt.Sprintf("%s pulse scale-only mode unexpectedly changed opacity (range=%.3f)", p.ID, opRange)
			}
			if scRange < 0.06 {
				return FaultWrongAmplitude, fmt.Sprintf("%s pulse scale range too small (%.3f)", p.ID, scRange)
			}
		default: // scale-opacity
			if opRange < 0.15 || scRange < 0.06 {
				return FaultWrongAmplitude, fmt.Sprintf("%s pulse scale-opacity amplitude insufficient (opRange=%.3f, scRange=%.3f)", p.ID, opRange, scRange)
			}
		}

	case "flash":
		minOp, maxOp := vals[0].Op, vals[0].Op
		maxSc := vals[0].Scale
		for _, v := range vals[1:] {
			if v.Op < minOp {
				minOp = v.Op
			}
			if v.Op > maxOp {
				maxOp = v.Op
			}
			if v.Scale > maxSc {
				maxSc = v.Scale
			}
		}
		if maxOp < 0.35 && maxSc < 1.05 {
			return FaultMissedFlash, fmt.Sprintf("%s no active flash pulse observed (maxOp=%.2f, maxScale=%.2f)", p.ID, maxOp, maxSc)
		}
		if minOp > 0.25 {
			return FaultStuckActive, fmt.Sprintf("%s flash stuck active (minOp=%.2f)", p.ID, minOp)
		}

	case "static-gauge":
		if p.Method == "data-driven" {
			minD, maxD := vals[0].DashVal, vals[0].DashVal
			for _, v := range vals[1:] {
				if v.DashVal < minD {
					minD = v.DashVal
				}
				if v.DashVal > maxD {
					maxD = v.DashVal
				}
			}
			if maxD-minD < 1.0 {
				return FaultStationary, fmt.Sprintf("%s data-driven gauge stationary (dashRange=%.2f)", p.ID, maxD-minD)
			}
		}
	}

	return FaultNone, ""
}

func runPopulationProbe(ctx context.Context) (popProbe, error) {
	var p popProbe
	raw, err := chromedp.Run(ctx, chromedp.Evaluate[string](jsEvalPopulationProbe))
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, err
	}
	return p, nil
}

func runSubsetProbe(ctx context.Context, indices []int) ([]popNodeState, error) {
	idxJSON, _ := json.Marshal(indices)
	script := fmt.Sprintf(`(() => {
		const idxs = %s;
		const payloads = Array.from(document.querySelectorAll('.lab-payload'));
		const suspendedSet = window.__labSuspendedSet || new Set();
		const parseScale = (csTr, attrTr) => {
			if (csTr && csTr !== 'none') {
				const m = csTr.match(/matrix\(([^)]+)\)/);
				if (m) {
					const parts = m[1].split(',').map(Number);
					if (parts.length >= 4 && !Number.isNaN(parts[0]) && !Number.isNaN(parts[1])) {
						return Math.hypot(parts[0], parts[1]);
					}
				}
			}
			if (attrTr) {
				const sm = attrTr.match(/scale\(\s*([0-9.eE+-]+)/);
				if (sm) return parseFloat(sm[1]) || 1;
			}
			return 1;
		};
		return JSON.stringify(idxs.map(i => {
			const el = payloads[i];
			if (!el) return {};
			const inst = el.closest('.lab-instance, svg');
			const id = (inst && inst.id) ? inst.id : (el.id || ('idx-' + i));
			const cs = getComputedStyle(el);
			const rect = el.getBoundingClientRect();
			const dashStr = cs.strokeDasharray || el.getAttribute('stroke-dasharray') || '';
			return {
				idx: i,
				id,
				family: el.getAttribute('data-lab-family') || '',
				method: el.getAttribute('data-lab-method') || '',
				suspended: suspendedSet.has(id),
				off: parseFloat(cs.strokeDashoffset) || 0,
				op: parseFloat(cs.opacity) || 0,
				scale: parseScale(cs.transform, el.getAttribute('transform') || ''),
				dashVal: parseFloat(dashStr) || 0,
				cx: rect.left + rect.width * 0.5,
				cy: rect.top + rect.height * 0.5,
				tr: cs.transform || '',
				attrTr: el.getAttribute('transform') || ''
			};
		}));
	})()`, string(idxJSON))
	raw, err := chromedp.Run(ctx, chromedp.Evaluate[string](script))
	if err != nil {
		return nil, err
	}
	var res []popNodeState
	if err := json.Unmarshal([]byte(raw), &res); err != nil {
		return nil, err
	}
	return res, nil
}

func instanceDelta(a, b popNodeState) float64 {
	d := math.Abs(b.Off - a.Off)
	d += math.Abs(b.Op-a.Op) * 100.0
	d += math.Abs(b.Scale-a.Scale) * 100.0
	d += math.Abs(b.DashVal - a.DashVal)
	d += math.Hypot(b.Cx-a.Cx, b.Cy-a.Cy)
	if a.Tr != b.Tr || a.AttrTr != b.AttrTr {
		d += 1.0
	}
	return d
}

func selectDeterministicSubset(active []int, items []popNodeState, isMixture bool) []int {
	if len(active) == 0 {
		return nil
	}
	n := len(active)
	candPos := []int{0, n / 4, n / 2, (3 * n) / 4, n - 1}
	seen := map[int]bool{}
	var out []int
	for _, pos := range candPos {
		if pos >= 0 && pos < n {
			idx := active[pos]
			if !seen[idx] {
				seen[idx] = true
				out = append(out, idx)
			}
		}
	}
	if isMixture {
		seenFam := map[string]bool{}
		for _, idx := range out {
			seenFam[items[idx].Family] = true
		}
		for _, idx := range active {
			fam := items[idx].Family
			if !seenFam[fam] {
				seenFam[fam] = true
				if !seen[idx] {
					seen[idx] = true
					out = append(out, idx)
				}
			}
		}
	}
	return out
}

func limitStrings(in []string, max int) []string {
	if len(in) <= max {
		return in
	}
	return in[:max]
}
