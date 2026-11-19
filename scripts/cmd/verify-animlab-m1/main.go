package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

type checkResult struct {
	ID     string
	Passed bool
	Detail string
}

func main() {
	cdpEndpoint := os.Getenv("ANIMLAB_CDP")
	if cdpEndpoint == "" {
		cdpEndpoint = "http://127.0.0.1:9222"
	}
	bindIP := os.Getenv("ANIMLAB_BIND_IP")
	if bindIP == "" {
		bindIP = "192.168.100.47"
	}
	evidenceDir := "/tmp/m1-evidence"
	_ = os.MkdirAll(evidenceDir, 0755)

	var (
		mu    sync.RWMutex
		pages = map[string]struct {
			ctype string
			body  string
		}{}
	)
	ln, err := net.Listen("tcp", bindIP+":0")
	if err != nil {
		log.Fatalf("listen %s: %v", bindIP, err)
	}
	defer ln.Close()
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := strings.TrimPrefix(r.URL.Path, "/")
			mu.RLock()
			p, ok := pages[name]
			mu.RUnlock()
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", p.ctype)
			_, _ = w.Write([]byte(p.body))
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()
	baseURL := fmt.Sprintf("http://%s/", ln.Addr().String())

	allocCtx, cancelAlloc := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer cancelAlloc()
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[string](`'ok'`)); err != nil {
		log.Fatalf("connect CDP %s: %v", cdpEndpoint, err)
	}

	setPage := func(name, ctype, body string) string {
		mu.Lock()
		pages[name] = struct {
			ctype string
			body  string
		}{ctype: ctype, body: body}
		mu.Unlock()
		return baseURL + name
	}

	var results []checkResult
	record := func(id string, ok bool, detail string) {
		status := "PASS"
		if !ok {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s: %s\n", status, id, detail)
		results = append(results, checkResult{ID: id, Passed: ok, Detail: detail})
	}

	// =========================================================================
	// AT-M1-01 & VIS-M1-01: Standalone SVG playback directly in Chromium
	// =========================================================================
	{
		allOK := true
		var notes []string
		for _, method := range []string{"css", "smil", "javascript-raf"} {
			inst := animlab.InstanceParams{
				ID: "standalone-" + method, Family: "perimeter-tracer", Method: method,
				Shape: "rect", Width: 80, Height: 80, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
			}
			svg := animlab.BuildStandaloneSampleSVG(inst)
			u := setPage("at01-"+method+".svg", "image/svg+xml; charset=utf-8", svg)
			if _, err := chromedp.Run(ctx, chromedp.Navigate(u)); err != nil {
				allOK = false
				notes = append(notes, method+":nav-err")
				continue
			}
			_, _ = chromedp.Run(ctx, chromedp.Sleep(250*time.Millisecond))
			spec := animlab.TestRunSpec{
				RunID:     "at01-" + method,
				Family:    "perimeter-tracer",
				Method:    method,
				Count:     1,
				Instances: []animlab.InstanceParams{inst},
			}
			corr := animlab.EvaluateCorrectness(ctx, spec)
			if !corr.Passed {
				allOK = false
				notes = append(notes, fmt.Sprintf("%s:FAIL(%s:%s)", method, corr.FaultCode, corr.Reason))
			} else {
				notes = append(notes, method+":OK")
			}
			if method == "javascript-raf" {
				buf, _ := chromedp.Run(ctx, chromedp.CaptureScreenshot())
				_ = os.WriteFile(filepath.Join(evidenceDir, "vis-m1-01-standalone-raf.png"), buf, 0644)
			}
		}
		record("AT-M1-01", allOK, strings.Join(notes, ", "))
	}

	runSpec := func(spec animlab.TestRunSpec) animlab.CorrectnessResult {
		html := animlab.BuildTestPageHTML(spec)
		u := setPage(spec.RunID+".html", "text/html; charset=utf-8", html)
		if _, err := chromedp.Run(ctx, chromedp.Navigate(u)); err != nil {
			return animlab.CorrectnessResult{Passed: false, Reason: err.Error()}
		}
		_, _ = chromedp.Run(ctx, chromedp.Sleep(250*time.Millisecond))
		return animlab.EvaluateCorrectness(ctx, spec)
	}

	makeSpec := func(id, fam, method string, insts []animlab.InstanceParams) animlab.TestRunSpec {
		return animlab.TestRunSpec{
			RunID:     id,
			Family:    fam,
			Method:    method,
			Count:     len(insts),
			Isolation: "independent-svg-roots",
			Instances: insts,
		}
	}

	// =========================================================================
	// AT-M1-02: Tracer cw vs ccw across css, smil, javascript-raf
	// =========================================================================
	{
		allOK := true
		var notes []string
		for _, method := range []string{"css", "smil", "javascript-raf"} {
			for _, dir := range []string{"cw", "ccw"} {
				inst := animlab.InstanceParams{
					ID: fmt.Sprintf("tr-%s-%s", method, dir), Family: "perimeter-tracer", Method: method,
					Shape: "rect", Width: 60, Height: 60, Speed: 1.6, Segments: 2, LineGapRatio: "25:75", Direction: dir, Color: "#38bdf8",
				}
				c := runSpec(makeSpec("at02-"+method+"-"+dir, "perimeter-tracer", method, []animlab.InstanceParams{inst}))
				if !c.Passed {
					allOK = false
					notes = append(notes, fmt.Sprintf("%s/%s:%s", method, dir, c.FaultCode))
				}
			}
		}
		record("AT-M1-02", allOK, fmt.Sprintf("cw/ccw across css/smil/raf failures=%v", notes))
	}

	// =========================================================================
	// AT-M1-03 & AT-M1-04: Tracer speed & continuous loop continuity
	// =========================================================================
	{
		instFast := animlab.InstanceParams{
			ID: "tr-fast", Family: "perimeter-tracer", Method: "css",
			Shape: "rect", Width: 60, Height: 60, Speed: 1.0, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}
		instSlow := animlab.InstanceParams{
			ID: "tr-slow", Family: "perimeter-tracer", Method: "css",
			Shape: "rect", Width: 60, Height: 60, Speed: 3.0, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}
		c1 := runSpec(makeSpec("at03-fast", "perimeter-tracer", "css", []animlab.InstanceParams{instFast}))
		c2 := runSpec(makeSpec("at03-slow", "perimeter-tracer", "css", []animlab.InstanceParams{instSlow}))
		record("AT-M1-03", c1.Passed && c2.Passed, fmt.Sprintf("fast=%v slow=%v", c1.Passed, c2.Passed))
		record("AT-M1-04", c1.Passed && c1.LevelBPassed, c1.Reason)
	}

	// =========================================================================
	// AT-M1-05: Pulse 3 modes across css, smil, javascript-raf
	// =========================================================================
	{
		allOK := true
		var notes []string
		for _, method := range []string{"css", "smil", "javascript-raf"} {
			for _, mode := range []string{"opacity-only", "scale-only", "scale-opacity"} {
				inst := animlab.InstanceParams{
					ID: fmt.Sprintf("pl-%s-%s", method, mode), Family: "pulse", Method: method,
					Shape: "circle", Width: 40, Height: 40, Speed: 1.2, PulseMode: mode, PulseAmplitude: 0.25, Color: "#a855f7",
				}
				c := runSpec(makeSpec("at05-"+method+"-"+mode, "pulse", method, []animlab.InstanceParams{inst}))
				if !c.Passed {
					allOK = false
					notes = append(notes, fmt.Sprintf("%s/%s:%s(%s)", method, mode, c.FaultCode, c.Reason))
				}
			}
		}
		record("AT-M1-05", allOK, fmt.Sprintf("pulse 3x3 failures=%v", notes))
	}

	// =========================================================================
	// AT-M1-06: Flash 120ms pulse across css, smil, javascript-raf
	// =========================================================================
	{
		allOK := true
		var notes []string
		for _, method := range []string{"css", "smil", "javascript-raf"} {
			inst := animlab.InstanceParams{
				ID: "fl-" + method, Family: "flash", Method: method,
				Shape: "circle", Width: 36, Height: 36, FlashMode: "repeating", FlashDurMs: 120, FlashIntervalS: 1.0, Color: "#f59e0b",
			}
			c := runSpec(makeSpec("at06-"+method, "flash", method, []animlab.InstanceParams{inst}))
			if !c.Passed {
				allOK = false
				notes = append(notes, fmt.Sprintf("%s:%s(%s)", method, c.FaultCode, c.Reason))
			}
		}
		record("AT-M1-06", allOK, fmt.Sprintf("flash 120ms failures=%v", notes))
	}

	// =========================================================================
	// AT-M1-07: flow-line vs flow-particle across css, smil, javascript-raf
	// =========================================================================
	{
		allOK := true
		var notes []string
		for _, fam := range []string{"flow-line", "flow-particle"} {
			for _, method := range []string{"css", "smil", "javascript-raf"} {
				inst := animlab.InstanceParams{
					ID: fmt.Sprintf("%s-%s", fam, method), Family: fam, Method: method,
					Shape: "curved", Width: 90, Height: 40, Speed: 1.6, Direction: "cw", Color: "#22c55e",
				}
				c := runSpec(makeSpec("at07-"+fam+"-"+method, fam, method, []animlab.InstanceParams{inst}))
				if !c.Passed {
					allOK = false
					notes = append(notes, fmt.Sprintf("%s/%s:%s(%s)", fam, method, c.FaultCode, c.Reason))
				}
			}
		}
		record("AT-M1-07", allOK, fmt.Sprintf("flow-line & flow-particle failures=%v", notes))
	}

	// =========================================================================
	// AT-M1-08 & VIS-M1-02: Large circle tracer non-clipping
	// =========================================================================
	{
		inst := animlab.InstanceParams{
			ID: "large-circle", Family: "perimeter-tracer", Method: "css",
			Shape: "circle", Width: 120, Height: 120, Speed: 1.8, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}
		c := runSpec(makeSpec("at08-large-circle", "perimeter-tracer", "css", []animlab.InstanceParams{inst}))
		clipped, _ := chromedp.Run(ctx, chromedp.Evaluate[bool](`(() => {
			const svg = document.querySelector('.lab-svg-root');
			const el = document.querySelector('.lab-payload');
			const sr = svg.getBoundingClientRect();
			const er = el.getBoundingClientRect();
			return er.left < sr.left || er.top < sr.top || er.right > sr.right || er.bottom > sr.bottom;
		})()`))
		buf, _ := chromedp.Run(ctx, chromedp.CaptureScreenshot())
		_ = os.WriteFile(filepath.Join(evidenceDir, "vis-m1-02-large-circle-tracer.png"), buf, 0644)
		record("AT-M1-08", c.Passed && !clipped, fmt.Sprintf("passed=%v clipped=%v", c.Passed, clipped))
	}

	// =========================================================================
	// AT-M1-09 & VIS-M1-03: Star shape flash
	// =========================================================================
	{
		inst := animlab.InstanceParams{
			ID: "star-flash", Family: "flash", Method: "css",
			Shape: "star", Width: 64, Height: 64, FlashMode: "repeating", FlashDurMs: 180, FlashIntervalS: 0.9, Color: "#f59e0b",
		}
		c := runSpec(makeSpec("at09-star-flash", "flash", "css", []animlab.InstanceParams{inst}))
		_, _ = chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			const el = document.querySelector('.lab-payload');
			if (el) { el.style.animation = 'none'; el.style.opacity = '1'; el.style.transform = 'scale(1.15)'; }
			return 'ok';
		})()`))
		buf, _ := chromedp.Run(ctx, chromedp.CaptureScreenshot())
		_ = os.WriteFile(filepath.Join(evidenceDir, "vis-m1-03-star-flash.png"), buf, 0644)
		record("AT-M1-09", c.Passed, c.Reason)
	}

	// =========================================================================
	// AT-M1-10: Shared vs per-instance RAF scheduling
	// =========================================================================
	{
		instS := animlab.InstanceParams{
			ID: "raf-shared", Family: "perimeter-tracer", Method: "javascript-raf", RAFScheduling: "shared",
			Shape: "rect", Width: 50, Height: 50, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}
		instP := animlab.InstanceParams{
			ID: "raf-per", Family: "perimeter-tracer", Method: "javascript-raf", RAFScheduling: "per-instance",
			Shape: "rect", Width: 50, Height: 50, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}
		cs := runSpec(makeSpec("at10-shared", "perimeter-tracer", "javascript-raf", []animlab.InstanceParams{instS}))
		cp := runSpec(makeSpec("at10-per", "perimeter-tracer", "javascript-raf", []animlab.InstanceParams{instP}))
		record("AT-M1-10", cs.Passed && cp.Passed, fmt.Sprintf("shared=%v per-instance=%v", cs.Passed, cp.Passed))
	}

	// =========================================================================
	// AT-M1-11: data-driven static-gauge genuinely updates DOM
	// =========================================================================
	{
		inst := animlab.InstanceParams{
			ID: "gauge-dd", Family: "static-gauge", Method: "data-driven",
			Shape: "circle", Width: 60, Height: 60, Color: "#38bdf8",
		}
		c := runSpec(makeSpec("at11-gauge-dd", "static-gauge", "data-driven", []animlab.InstanceParams{inst}))
		record("AT-M1-11", c.Passed, c.Reason)
	}

	// =========================================================================
	// AT-M1-12: clip-reveal rejected as UNSUPPORTED_OR_NON_EQUIVALENT
	// =========================================================================
	{
		inst := animlab.InstanceParams{
			ID: "clip-rev", Family: "perimeter-tracer", Method: "clip-reveal",
			Shape: "rect", Width: 60, Height: 60, Speed: 1.5, Color: "#38bdf8",
		}
		spec := makeSpec("at12-clip-reveal", "perimeter-tracer", "clip-reveal", []animlab.InstanceParams{inst})
		spec.NonEquivalent = true
		spec.Limitations = []string{"clip-reveal is not equivalent to stroke-dashoffset perimeter tracer"}
		c := runSpec(spec)
		ok := !c.Passed && c.FaultCode == string(animlab.FaultUnsupportedOrNonEquivalent)
		record("AT-M1-12", ok, fmt.Sprintf("passed=%v fault=%s", c.Passed, c.FaultCode))
	}

	// =========================================================================
	// AT-M1-13: Offscreen suspension & scroll-into-view resumption
	// =========================================================================
	{
		var insts []animlab.InstanceParams
		for i := 0; i < 24; i++ {
			insts = append(insts, animlab.InstanceParams{
				ID: fmt.Sprintf("susp-%d", i), Family: "perimeter-tracer", Method: "javascript-raf",
				Shape: "rect", Width: 60, Height: 60, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
			})
		}
		spec := makeSpec("at13-suspend", "perimeter-tracer", "javascript-raf", insts)
		spec.VisibleCount = 6
		spec.SuspendOffscreen = true
		c := runSpec(spec)
		record("AT-M1-13", c.Passed && c.SuspendedCount > 0, fmt.Sprintf("passed=%v susp=%d reason=%s", c.Passed, c.SuspendedCount, c.Reason))
	}

	// =========================================================================
	// AT-M1-14: Whole-population check on 100 instances
	// =========================================================================
	{
		var insts []animlab.InstanceParams
		for i := 0; i < 100; i++ {
			insts = append(insts, animlab.InstanceParams{
				ID: fmt.Sprintf("pop100-%d", i), Family: "perimeter-tracer", Method: "css",
				Shape: "rect", Width: 40, Height: 40, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
			})
		}
		c := runSpec(makeSpec("at14-pop100", "perimeter-tracer", "css", insts))
		record("AT-M1-14", c.Passed && strings.Contains(c.Reason, "100 instances"), c.Reason)
	}

	// =========================================================================
	// NEGATIVE CONTROLS NC-01 .. NC-08
	// =========================================================================

	// NC-01: STATIONARY
	{
		inst := animlab.InstanceParams{
			ID: "nc01", Family: "perimeter-tracer", Method: "css",
			Shape: "rect", Width: 60, Height: 60, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}
		spec := makeSpec("nc01", "perimeter-tracer", "css", []animlab.InstanceParams{inst})
		u := setPage("nc01.html", "text/html; charset=utf-8", animlab.BuildTestPageHTML(spec))
		_, _ = chromedp.Run(ctx, chromedp.Navigate(u))
		_, _ = chromedp.Run(ctx, chromedp.Evaluate[string](`document.querySelector('.lab-payload').style.animationPlayState='paused';'ok'`))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
		c := animlab.EvaluateCorrectness(ctx, spec)
		record("NC-01", !c.Passed && c.FaultCode == string(animlab.FaultStationary), fmt.Sprintf("fault=%s reason=%s", c.FaultCode, c.Reason))
	}

	// NC-02: WRONG_DIRECTION (spec expects cw, page runs ccw)
	{
		actual := animlab.InstanceParams{
			ID: "nc02", Family: "perimeter-tracer", Method: "css",
			Shape: "rect", Width: 60, Height: 60, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "ccw", Color: "#38bdf8",
		}
		expectedSpec := makeSpec("nc02", "perimeter-tracer", "css", []animlab.InstanceParams{{
			ID: "nc02", Family: "perimeter-tracer", Method: "css",
			Shape: "rect", Width: 60, Height: 60, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}})
		u := setPage("nc02.html", "text/html; charset=utf-8", animlab.BuildTestPageHTML(makeSpec("nc02", "perimeter-tracer", "css", []animlab.InstanceParams{actual})))
		_, _ = chromedp.Run(ctx, chromedp.Navigate(u))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(150*time.Millisecond))
		c := animlab.EvaluateCorrectness(ctx, expectedSpec)
		record("NC-02", !c.Passed && c.FaultCode == string(animlab.FaultWrongDirection), fmt.Sprintf("fault=%s reason=%s", c.FaultCode, c.Reason))
	}

	// NC-03: WRONG_SPEED (spec expects 1.0s, page runs 5.0s)
	{
		actual := animlab.InstanceParams{
			ID: "nc03", Family: "perimeter-tracer", Method: "css",
			Shape: "rect", Width: 60, Height: 60, Speed: 5.0, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}
		expectedSpec := makeSpec("nc03", "perimeter-tracer", "css", []animlab.InstanceParams{{
			ID: "nc03", Family: "perimeter-tracer", Method: "css",
			Shape: "rect", Width: 60, Height: 60, Speed: 1.0, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}})
		u := setPage("nc03.html", "text/html; charset=utf-8", animlab.BuildTestPageHTML(makeSpec("nc03", "perimeter-tracer", "css", []animlab.InstanceParams{actual})))
		_, _ = chromedp.Run(ctx, chromedp.Navigate(u))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(150*time.Millisecond))
		c := animlab.EvaluateCorrectness(ctx, expectedSpec)
		record("NC-03", !c.Passed && c.FaultCode == string(animlab.FaultWrongSpeed), fmt.Sprintf("fault=%s reason=%s", c.FaultCode, c.Reason))
	}

	// NC-04: LOOP_DISCONTINUITY (inject mid-cycle jump -100 -> -550 on javascript-raf driver)
	{
		inst := animlab.InstanceParams{
			ID: "nc04", Family: "perimeter-tracer", Method: "javascript-raf",
			Shape: "rect", Width: 60, Height: 60, Speed: 2.0, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
		}
		spec := makeSpec("nc04", "perimeter-tracer", "javascript-raf", []animlab.InstanceParams{inst})
		u := setPage("nc04.html", "text/html; charset=utf-8", animlab.BuildTestPageHTML(spec))
		_, _ = chromedp.Run(ctx, chromedp.Navigate(u))
		_, _ = chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			const el = document.querySelector('.lab-payload');
			let flip = false;
			setInterval(() => {
				flip = !flip;
				el.style.strokeDashoffset = flip ? '-100' : '-550';
			}, 120);
			return 'ok';
		})()`))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
		c := animlab.EvaluateCorrectness(ctx, spec)
		record("NC-04", !c.Passed && c.FaultCode == string(animlab.FaultLoopDiscontinuity), fmt.Sprintf("fault=%s reason=%s", c.FaultCode, c.Reason))
	}

	// NC-05: MODE_MISMATCH (spec expects opacity-only, page runs scale-opacity)
	{
		actual := animlab.InstanceParams{
			ID: "nc05", Family: "pulse", Method: "css",
			Shape: "circle", Width: 40, Height: 40, Speed: 1.2, PulseMode: "scale-opacity", PulseAmplitude: 0.25, Color: "#a855f7",
		}
		expectedSpec := makeSpec("nc05", "pulse", "css", []animlab.InstanceParams{{
			ID: "nc05", Family: "pulse", Method: "css",
			Shape: "circle", Width: 40, Height: 40, Speed: 1.2, PulseMode: "opacity-only", PulseAmplitude: 0.25, Color: "#a855f7",
		}})
		u := setPage("nc05.html", "text/html; charset=utf-8", animlab.BuildTestPageHTML(makeSpec("nc05", "pulse", "css", []animlab.InstanceParams{actual})))
		_, _ = chromedp.Run(ctx, chromedp.Navigate(u))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(150*time.Millisecond))
		c := animlab.EvaluateCorrectness(ctx, expectedSpec)
		record("NC-05", !c.Passed && c.FaultCode == string(animlab.FaultModeMismatch), fmt.Sprintf("fault=%s reason=%s", c.FaultCode, c.Reason))
	}

	// NC-06: MISSED_FLASH & STUCK_ACTIVE
	{
		inst := animlab.InstanceParams{
			ID: "nc06", Family: "flash", Method: "javascript-raf",
			Shape: "circle", Width: 36, Height: 36, FlashMode: "repeating", FlashDurMs: 120, FlashIntervalS: 1.0, Color: "#f59e0b",
		}
		spec := makeSpec("nc06", "flash", "javascript-raf", []animlab.InstanceParams{inst})
		u := setPage("nc06a.html", "text/html; charset=utf-8", animlab.BuildTestPageHTML(spec))
		_, _ = chromedp.Run(ctx, chromedp.Navigate(u))
		_, _ = chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			window.__labRafDrivers = [{ id: 'nc06', kind: 'none' }];
			const el = document.querySelector('.lab-payload');
			el.style.opacity = '0';
			el.style.transform = 'scale(1)';
			return 'ok';
		})()`))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
		ca := animlab.EvaluateCorrectness(ctx, spec)

		_, _ = chromedp.Run(ctx, chromedp.Navigate(u))
		_, _ = chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			window.__labRafDrivers = [{ id: 'nc06', kind: 'none' }];
			const el = document.querySelector('.lab-payload');
			el.style.opacity = '1';
			el.style.transform = 'scale(1)';
			return 'ok';
		})()`))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
		cb := animlab.EvaluateCorrectness(ctx, spec)
		ok := !ca.Passed && ca.FaultCode == string(animlab.FaultMissedFlash) && !cb.Passed && cb.FaultCode == string(animlab.FaultStuckActive)
		record("NC-06", ok, fmt.Sprintf("missed=%s stuck=%s", ca.FaultCode, cb.FaultCode))
	}

	// NC-07: PARTIAL_POPULATION_FAILURE (instance 73 of 100 frozen)
	{
		var insts []animlab.InstanceParams
		for i := 0; i < 100; i++ {
			insts = append(insts, animlab.InstanceParams{
				ID: fmt.Sprintf("nc07-%d", i), Family: "perimeter-tracer", Method: "css",
				Shape: "rect", Width: 40, Height: 40, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
			})
		}
		spec := makeSpec("nc07", "perimeter-tracer", "css", insts)
		u := setPage("nc07.html", "text/html; charset=utf-8", animlab.BuildTestPageHTML(spec))
		_, _ = chromedp.Run(ctx, chromedp.Navigate(u))
		_, _ = chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			const el = document.querySelector('#nc07-73 .lab-payload');
			el.style.animationPlayState = 'paused';
			return 'ok';
		})()`))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
		c := animlab.EvaluateCorrectness(ctx, spec)
		has73 := false
		for _, fi := range c.FailingInstances {
			if strings.Contains(fi, "73") {
				has73 = true
			}
		}
		record("NC-07", !c.Passed && c.FaultCode == string(animlab.FaultPartialPopulationFailure) && has73, fmt.Sprintf("fault=%s failing=%v", c.FaultCode, c.FailingInstances))
	}

	// NC-08: SUSPENSION_LEAK (instance in __labSuspendedSet continues animating)
	{
		var insts []animlab.InstanceParams
		for i := 0; i < 12; i++ {
			insts = append(insts, animlab.InstanceParams{
				ID: fmt.Sprintf("nc08-%d", i), Family: "perimeter-tracer", Method: "css",
				Shape: "rect", Width: 60, Height: 60, Speed: 1.5, Segments: 2, LineGapRatio: "25:75", Direction: "cw", Color: "#38bdf8",
			})
		}
		spec := makeSpec("nc08", "perimeter-tracer", "css", insts)
		spec.VisibleCount = 4
		spec.SuspendOffscreen = true
		u := setPage("nc08.html", "text/html; charset=utf-8", animlab.BuildTestPageHTML(spec))
		_, _ = chromedp.Run(ctx, chromedp.Navigate(u))
		_, _ = chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			window.__labSuspendedSet = window.__labSuspendedSet || new Set();
			window.__labSuspendedSet.add('nc08-10');
			window.__labSuspendedCount = window.__labSuspendedSet.size;
			const root = document.getElementById('nc08-10');
			if (root) root.classList.remove('lab-suspended');
			const el = document.querySelector('#nc08-10 .lab-payload');
			if (el) el.style.setProperty('animation-play-state', 'running', 'important');
			return 'ok';
		})()`))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(100*time.Millisecond))
		c := animlab.EvaluateCorrectness(ctx, spec)
		record("NC-08", !c.Passed && c.FaultCode == string(animlab.FaultSuspensionLeak), fmt.Sprintf("fault=%s reason=%s", c.FaultCode, c.Reason))
	}

	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	fmt.Printf("\nSUMMARY: %d/%d checks passed (%d failed)\n", len(results)-failed, len(results), failed)
	if failed > 0 {
		os.Exit(1)
	}
}
