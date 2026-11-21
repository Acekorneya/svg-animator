package main

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

type Point struct {
	X, Y float64
}

// buildMultiPointPath builds a 4-waypoint path with straight and quadratic Bezier curved sections,
// plus optional lead-in and lead-out extensions of length r along the start and end tangents.
func buildMultiPointPath(pts []Point, bends []float64, leadIn, leadOut float64) (string, float64, Point, Point, Point, Point) {
	n := len(pts)
	// Compute start tangent (from segment 0) and end tangent (from last segment)
	var t0, tL Point
	totalLen := 0.0

	type sectionGeom struct {
		p1, p2 Point
		ctrl   Point
		curved bool
		length float64
	}
	secs := make([]sectionGeom, n-1)

	for i := 0; i < n-1; i++ {
		p1 := pts[i]
		p2 := pts[i+1]
		dx := p2.X - p1.X
		dy := p2.Y - p1.Y
		chord := math.Hypot(dx, dy)
		if chord < 1e-6 {
			chord = 1
		}
		bend := 0.0
		if i < len(bends) {
			bend = bends[i]
		}
		if math.Abs(bend) < 1e-3 {
			secs[i] = sectionGeom{p1: p1, p2: p2, curved: false, length: chord}
		} else {
			nx := -dy / chord
			ny := dx / chord
			offset := (bend / 100.0) * (chord * 0.4)
			cx := (p1.X+p2.X)/2.0 + nx*offset
			cy := (p1.Y+p2.Y)/2.0 + ny*offset
			// Sample quadratic Bezier arc length
			steps := 40
			arcLen := 0.0
			prev := p1
			for s := 1; s <= steps; s++ {
				u := float64(s) / float64(steps)
				om := 1.0 - u
				qx := om*om*p1.X + 2*om*u*cx + u*u*p2.X
				qy := om*om*p1.Y + 2*om*u*cy + u*u*p2.Y
				arcLen += math.Hypot(qx-prev.X, qy-prev.Y)
				prev = Point{qx, qy}
			}
			secs[i] = sectionGeom{p1: p1, p2: p2, ctrl: Point{cx, cy}, curved: true, length: arcLen}
		}
		totalLen += secs[i].length
	}

	// Start tangent at P0
	if secs[0].curved {
		dx := secs[0].ctrl.X - secs[0].p1.X
		dy := secs[0].ctrl.Y - secs[0].p1.Y
		l := math.Hypot(dx, dy)
		t0 = Point{dx / l, dy / l}
	} else {
		dx := secs[0].p2.X - secs[0].p1.X
		dy := secs[0].p2.Y - secs[0].p1.Y
		l := math.Hypot(dx, dy)
		t0 = Point{dx / l, dy / l}
	}

	// End tangent at P_{n-1}
	last := secs[n-2]
	if last.curved {
		dx := last.p2.X - last.ctrl.X
		dy := last.p2.Y - last.ctrl.Y
		l := math.Hypot(dx, dy)
		tL = Point{dx / l, dy / l}
	} else {
		dx := last.p2.X - last.p1.X
		dy := last.p2.Y - last.p1.Y
		l := math.Hypot(dx, dy)
		tL = Point{dx / l, dy / l}
	}

	var sb strings.Builder
	startPt := pts[0]
	if leadIn > 0 {
		startPt = Point{pts[0].X - leadIn*t0.X, pts[0].Y - leadIn*t0.Y}
		sb.WriteString(fmt.Sprintf("M %.2f,%.2f L %.2f,%.2f", startPt.X, startPt.Y, pts[0].X, pts[0].Y))
	} else {
		sb.WriteString(fmt.Sprintf("M %.2f,%.2f", pts[0].X, pts[0].Y))
	}

	for _, sc := range secs {
		if sc.curved {
			sb.WriteString(fmt.Sprintf(" Q %.2f,%.2f %.2f,%.2f", sc.ctrl.X, sc.ctrl.Y, sc.p2.X, sc.p2.Y))
		} else {
			sb.WriteString(fmt.Sprintf(" L %.2f,%.2f", sc.p2.X, sc.p2.Y))
		}
	}

	if leadOut > 0 {
		endExt := Point{pts[n-1].X + leadOut*tL.X, pts[n-1].Y + leadOut*tL.Y}
		sb.WriteString(fmt.Sprintf(" L %.2f,%.2f", endExt.X, endExt.Y))
	}

	return sb.String(), totalLen, pts[0], pts[n-1], t0, tL
}

// buildEndpointClipPath creates an evenodd clip-path that leaves the canvas intact except for
// the cap-overhang rectangles immediately behind P0 and beyond P_{n-1}.
func buildEndpointClipPath(p0, pL, t0, tL Point, r float64) string {
	margin := r + 6.0
	depth := 2*r + 8.0
	n0 := Point{-t0.Y, t0.X}
	nL := Point{-tL.Y, tL.X}

	// Box behind P0
	a1 := Point{p0.X + margin*n0.X, p0.Y + margin*n0.Y}
	a2 := Point{p0.X - margin*n0.X, p0.Y - margin*n0.Y}
	a3 := Point{a2.X - depth*t0.X, a2.Y - depth*t0.Y}
	a4 := Point{a1.X - depth*t0.X, a1.Y - depth*t0.Y}

	// Box beyond PL
	b1 := Point{pL.X + margin*nL.X, pL.Y + margin*nL.Y}
	b2 := Point{pL.X - margin*nL.X, pL.Y - margin*nL.Y}
	b3 := Point{b2.X + depth*tL.X, b2.Y + depth*tL.Y}
	b4 := Point{b1.X + depth*tL.X, b1.Y + depth*tL.Y}

	return fmt.Sprintf(
		"M -200,-200 L 2000,-200 L 2000,1200 L -200,1200 Z "+
			"M %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f Z "+
			"M %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f Z",
		a1.X, a1.Y, a2.X, a2.Y, a3.X, a3.Y, a4.X, a4.Y,
		b1.X, b1.Y, b2.X, b2.Y, b3.X, b3.Y, b4.X, b4.Y,
	)
}

type FlowCase struct {
	ID        string
	Variant   string // "single", "dashed", "dotted"
	StartMode string // "tunnel", "abrupt"
	EndMode   string // "tunnel", "abrupt"
	Direction string // "forward", "reverse"
}

func renderFlowCaseSVG(fc FlowCase, pts []Point, bends []float64) string {
	dashLen := 28.0
	gapLen := 22.0
	sw := 4.0
	linecap := "butt"
	leadR := 0.0

	if fc.Variant == "single" {
		dashLen = 48.0
		gapLen = 1000.0
	} else if fc.Variant == "dotted" {
		dotSize := 10.0
		sw = dotSize
		dashLen = 0.2 // spine length for round dot
		gapLen = 18.0 // clear gap between dots
		linecap = "round"
		leadR = dotSize / 2.0
	}

	// If reverse, reverse points and negate bends in reverse order
	usePts := make([]Point, len(pts))
	useBends := make([]float64, len(bends))
	if fc.Direction == "reverse" {
		for i := range pts {
			usePts[i] = pts[len(pts)-1-i]
		}
		for i := range bends {
			// Reversing chord direction (p2->p1) flips perpendicular normal (-dy, dx), so negate bend to preserve exact curve geometry!
			useBends[i] = -bends[len(bends)-1-i]
		}
	} else {
		copy(usePts, pts)
		copy(useBends, bends)
	}

	pathD, L, p0, pL, t0, tL := buildMultiPointPath(usePts, useBends, leadR, leadR)
	extL := L + 2*leadR

	// Effective visual element size and period
	visLen := dashLen + 2*leadR
	period := visLen + gapLen

	// Determine off_start and off_end on the extended path [0, extL]
	// Note: Physical start P0 is at s_ext = leadR. Physical end PL is at s_ext = leadR + L.
	// A dash of spine length dashLen at x_back = -off has:
	//   visual back edge at x_vis_back = x_back - leadR
	//   visual front edge at x_vis_front = x_back + dashLen + leadR = x_vis_back + visLen
	// At Tunnel Entry: x_vis_front starts at P0 (s_ext = leadR) -> x_vis_back = leadR - visLen -> x_back = 2*leadR - visLen = -dashLen -> off_start = +dashLen
	// At Abrupt Entry: x_vis_back starts at P0 (s_ext = leadR) -> x_back = 2*leadR -> off_start = -2*leadR
	// At Abrupt Exit:  x_vis_front ends at PL (s_ext = leadR + L) -> x_back + dashLen + leadR = leadR + L -> x_back = L - dashLen -> off_end = -(L - dashLen)
	// At Tunnel Exit:  x_vis_back ends at PL (s_ext = leadR + L) -> x_back - leadR = leadR + L -> x_back = L + 2*leadR -> off_end = -(L + 2*leadR)

	var offStart, offEnd float64
	if fc.StartMode == "tunnel" {
		offStart = dashLen
	} else {
		offStart = -2 * leadR
	}
	if fc.EndMode == "tunnel" {
		offEnd = -(L + 2*leadR)
	} else {
		offEnd = -(L - dashLen)
	}

	activeDist := offStart - offEnd // total travel distance of each segment between entry and exit
	M := 1
	cycleDist := activeDist
	if fc.Variant != "single" {
		M = int(math.Ceil(activeDist/period)) + 1
		if M < 2 {
			M = 2
		}
		cycleDist = float64(M) * period
	}

	speed := 150.0 // px per second
	cycleDur := cycleDist / speed
	activePct := (activeDist / cycleDist) * 100.0
	if activePct > 99.9 {
		activePct = 99.9
	}

	clearGap := extL + cycleDist + visLen + 100.0
	clipID := "clip_" + fc.ID
	trackID := "track_" + fc.ID
	kfName := "kf_" + fc.ID

	clipDef := ""
	clipAttr := ""
	if leadR > 0 {
		clipPathD := buildEndpointClipPath(p0, pL, t0, tL, leadR)
		clipDef = fmt.Sprintf(`<clipPath id="%s" clipPathUnits="userSpaceOnUse"><path clip-rule="evenodd" d="%s" /></clipPath>`, clipID, clipPathD)
		clipAttr = fmt.Sprintf(` clip-path="url(#%s)"`, clipID)
	}

	var style strings.Builder
	if fc.Variant == "single" {
		style.WriteString(fmt.Sprintf(`@keyframes %s {
  0%% { stroke-dashoffset: %.2fpx; opacity: 1; }
  100%% { stroke-dashoffset: %.2fpx; opacity: 1; }
}`, kfName, offStart, offEnd))
	} else {
		style.WriteString(fmt.Sprintf(`@keyframes %s {
  0%% { stroke-dashoffset: %.2fpx; opacity: 1; animation-timing-function: linear; }
  %.3f%% { stroke-dashoffset: %.2fpx; opacity: 1; }
  %.3f%%, 100%% { stroke-dashoffset: %.2fpx; opacity: 0; }
}`, kfName, offStart, activePct, offEnd, activePct+0.01, offEnd))
	}

	var segs strings.Builder
	for k := 0; k < M; k++ {
		delay := -(float64(k) / float64(M)) * cycleDur
		segs.WriteString(fmt.Sprintf(
			`<use href="#%s" class="flow-seg" data-seg-idx="%d" fill="none" stroke="#38bdf8" stroke-width="%.2f" stroke-linecap="%s" stroke-dasharray="%.2f %.2f" style="animation: %s %.4fs linear %.4fs infinite;" />`,
			trackID, k, sw, linecap, dashLen, clearGap, kfName, cycleDur, delay,
		))
	}

	return fmt.Sprintf(`<g id="%s" class="flow-case" data-length="%.2f" data-vis-len="%.2f" data-period="%.2f" data-off-start="%.2f" data-off-end="%.2f" data-active-dist="%.2f" data-cycle-dur="%.4f" data-m="%d">
  <defs>
    %s
    <path id="%s" d="%s" />
    <style>%s</style>
  </defs>
  <g%s>
    %s
  </g>
</g>`, fc.ID, L, visLen, period, offStart, offEnd, activeDist, cycleDur, M, clipDef, trackID, pathD, style.String(), clipAttr, segs.String())
}

func main() {
	os.Exit(run())
}

func run() int {
	pts := []Point{{60, 80}, {180, 80}, {320, 50}, {460, 100}}
	bends := []float64{0, 55, -45} // Section 0 straight, Section 1 curved up, Section 2 curved down

	variants := []string{"single", "dashed", "dotted"}
	modes := []string{"tunnel", "abrupt"}
	dirs := []string{"forward", "reverse"}

	var cases []FlowCase
	var groups []string
	for _, v := range variants {
		for _, sm := range modes {
			for _, em := range modes {
				for _, d := range dirs {
					fc := FlowCase{
						ID:        fmt.Sprintf("fc_%s_%s_%s_%s", v, sm, em, d),
						Variant:   v,
						StartMode: sm,
						EndMode:   em,
						Direction: d,
					}
					cases = append(cases, fc)
					groups = append(groups, renderFlowCaseSVG(fc, pts, bends))
				}
			}
		}
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Flow Lite Feasibility Gate</title></head>
<body style="background:#0d1117;color:#e6edf3;margin:0;padding:16px;">
<svg id="testSvg" width="600" height="200" viewBox="0 0 600 200" xmlns="http://www.w3.org/2000/svg">
  %s
</svg>
</body>
</html>`, strings.Join(groups, "\n"))

	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen error: %v\n", err)
		return 1
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(html))
		}))
	}()

	// Determine local IP reachable by CDP browser (or 127.0.0.1 if local)
	hostIP := "127.0.0.1"
	if conn, err := net.Dial("udp", "192.168.100.40:80"); err == nil {
		hostIP = conn.LocalAddr().(*net.UDPAddr).IP.String()
		conn.Close()
	}
	testURL := fmt.Sprintf("http://%s:%d/feasibility.html", hostIP, port)

	sess, err := animlab.AcquireBrowserSession(context.Background(), "http://127.0.0.1:9222", "nigel@192.168.100.40")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to acquire browser session: %v\n", err)
		return 1
	}
	defer sess.Close()
	ctx, cancelTimeout := context.WithTimeout(sess.AllocCtx, 30*time.Second)
	defer cancelTimeout()
	taskCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	type CaseVerifyResult struct {
		ID             string  `json:"id"`
		Passed         bool    `json:"passed"`
		EntryVisLen    float64 `json:"entryVisLen"`
		MidVisLen      float64 `json:"midVisLen"`
		ExitVisLen     float64 `json:"exitVisLen"`
		PostExitOp     float64 `json:"postExitOp"`
		ActiveSpacing  float64 `json:"activeSpacing"`
		FailureReason  string  `json:"failureReason"`
	}

	jsVerify := `
(function() {
  const svg = document.getElementById('testSvg');
  const cases = Array.from(svg.querySelectorAll('.flow-case'));
  const out = [];

  // Helper: given a case group and an animation progress u in [0, 1] for segment 0,
  // pause Web Animations at exact time t = u * cycleDur and measure computed strokeDashoffset, opacity, and visible physical length in [0, L]!
  for (const g of cases) {
    const id = g.id;
    const L = parseFloat(g.getAttribute('data-length'));
    const visLen = parseFloat(g.getAttribute('data-vis-len'));
    const period = parseFloat(g.getAttribute('data-period'));
    const offStart = parseFloat(g.getAttribute('data-off-start'));
    const offEnd = parseFloat(g.getAttribute('data-off-end'));
    const activeDist = parseFloat(g.getAttribute('data-active-dist'));
    const cycleDur = parseFloat(g.getAttribute('data-cycle-dur'));
    const M = parseInt(g.getAttribute('data-m'), 10);
    const segs = Array.from(g.querySelectorAll('.flow-seg'));
    const seg0 = segs[0];
    const anims = segs.map(s => s.getAnimations()[0]);

    const parts = id.split('_'); // fc, variant, startMode, endMode, dir
    const variant = parts[1];
    const startMode = parts[2];
    const endMode = parts[3];
    const leadR = variant === 'dotted' ? (visLen / 2.0) : 0.0;
    const dashLen = parseFloat(seg0.getAttribute('stroke-dasharray').split(' ')[0]);

    // Function to compute actual visible length of a segment inside physical path [0, L]
    // from its live computed strokeDashoffset and opacity
    function measureVisibleLength(segEl) {
      const cs = window.getComputedStyle(segEl);
      const op = parseFloat(cs.opacity);
      if (op < 0.05) return { vis: 0, op: op, off: 0 };
      const off = parseFloat(cs.strokeDashoffset);
      // On extended path, physical path [0, L] is [leadR, leadR + L].
      // Segment visual span on extended path is [-off - leadR, -off + dashLen + leadR].
      const vBack = -off - leadR;
      const vFront = -off + dashLen + leadR;
      const clippedStart = Math.max(leadR, vBack);
      const clippedEnd = Math.min(leadR + L, vFront);
      const vis = Math.max(0, clippedEnd - clippedStart);
      return { vis, op, off, vBack: vBack - leadR };
    }

    function setTimeSec(tSec) {
      segs.forEach((s, idx) => {
        const a = anims[idx];
        if (a) {
          a.pause();
          // Web Animations API a.currentTime already applies the CSS animation-delay of the animation effect!
          a.currentTime = tSec * 1000.0;
        }
      });
    }

    // 1. Sample near entry: when segment 0 has travelled 35% of visLen from offStart
    const tEntry = ((0.35 * visLen) / (M === 1 ? activeDist : (M * period))) * cycleDur;
    setTimeSec(tEntry);
    const mEntry = measureVisibleLength(seg0);

    // 2. Sample at mid-path: when segment 0 is near L/2
    const tMid = ((0.5 * activeDist) / (M === 1 ? activeDist : (M * period))) * cycleDur;
    setTimeSec(tMid);
    const mMid = measureVisibleLength(seg0);

    // Verify spacing between active segments at mid-path for multi-segment variants
    let spacing = period;
    if (M > 1 && segs.length > 1) {
      const m0 = measureVisibleLength(segs[0]);
      const m1 = measureVisibleLength(segs[1]);
      spacing = Math.abs(m0.off - m1.off);
    }

    // 3. Sample near exit: when segment 0 is 35% of visLen before offEnd
    const tExit = (((activeDist - 0.35 * visLen)) / (M === 1 ? activeDist : (M * period))) * cycleDur;
    setTimeSec(tExit);
    const mExit = measureVisibleLength(seg0);

    // 4. Sample just after exit (for multi-segment stream)
    let postExitOp = 0;
    if (M > 1) {
      const tPost = ((activeDist + 0.25 * period) / (M * period)) * cycleDur;
      setTimeSec(tPost);
      postExitOp = measureVisibleLength(seg0).op;
    }

    let ok = true;
    let reason = '';

    // Check entry semantics:
    // In tunnel mode, at 35% of visLen after offStart, visible length must be partial (~0.35 * visLen, strictly between 0.15*visLen and 0.65*visLen)
    // In abrupt mode, at 35% of visLen after offStart, visible length must ALREADY be 100% full visLen!
    if (startMode === 'tunnel') {
      if (!(mEntry.vis > 0.15 * visLen && mEntry.vis < 0.65 * visLen)) {
        ok = false;
        reason += 'tunnel entry expected partial length, got ' + mEntry.vis.toFixed(2) + '/' + visLen.toFixed(2) + '; ';
      }
    } else {
      if (Math.abs(mEntry.vis - visLen) > 0.5) {
        ok = false;
        reason += 'abrupt entry expected full length ' + visLen.toFixed(2) + ', got ' + mEntry.vis.toFixed(2) + '; ';
      }
    }

    // Check mid-path full length
    if (Math.abs(mMid.vis - visLen) > 0.5) {
      ok = false;
      reason += 'mid-path expected full length ' + visLen.toFixed(2) + ', got ' + mMid.vis.toFixed(2) + '; ';
    }

    // Check exit semantics:
    // In tunnel mode, at 35% of visLen before offEnd, visible length must be partial (~0.35 * visLen)
    // In abrupt mode, at 35% of visLen before offEnd, visible length must STILL be 100% full visLen, and postExitOp must be 0!
    if (endMode === 'tunnel') {
      if (!(mExit.vis > 0.15 * visLen && mExit.vis < 0.65 * visLen)) {
        ok = false;
        reason += 'tunnel exit expected partial shortening, got ' + mExit.vis.toFixed(2) + '/' + visLen.toFixed(2) + '; ';
      }
    } else {
      if (Math.abs(mExit.vis - visLen) > 0.5) {
        ok = false;
        reason += 'abrupt exit expected full length ' + visLen.toFixed(2) + ' until exit, got ' + mExit.vis.toFixed(2) + '; ';
      }
      if (M > 1 && postExitOp > 0.05) {
        ok = false;
        reason += 'abrupt exit did not disappear after exit (op=' + postExitOp + '); ';
      }
    }

    if (M > 1 && Math.abs(spacing - period) > 1.0) {
      ok = false;
      reason += 'segment spacing mismatch: got ' + spacing.toFixed(2) + ', expected ' + period.toFixed(2);
    }

    out.push({
      id,
      passed: ok,
      entryVisLen: Math.round(mEntry.vis * 100) / 100,
      midVisLen: Math.round(mMid.vis * 100) / 100,
      exitVisLen: Math.round(mExit.vis * 100) / 100,
      postExitOp: Math.round(postExitOp * 100) / 100,
      activeSpacing: Math.round(spacing * 100) / 100,
      failureReason: reason
    });
  }
  return out;
})()
`

	if _, err := chromedp.Run(taskCtx, chromedp.Navigate(testURL)); err != nil {
		fmt.Fprintf(os.Stderr, "CDP Navigate error: %v\n", err)
		return 1
	}
	if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#testSvg")); err != nil {
		fmt.Fprintf(os.Stderr, "CDP WaitVisible error: %v\n", err)
		return 1
	}
	results, err := chromedp.Run(taskCtx, chromedp.Evaluate[[]CaseVerifyResult](jsVerify))
	if err != nil {
		fmt.Fprintf(os.Stderr, "CDP Evaluate error: %v\n", err)
		return 1
	}

	passedCount := 0
	for _, r := range results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL (" + r.FailureReason + ")"
		} else {
			passedCount++
		}
		fmt.Printf("[%s] %-36s | entry=%.1fpx mid=%.1fpx exit=%.1fpx spacing=%.1fpx\n",
			status, r.ID, r.EntryVisLen, r.MidVisLen, r.ExitVisLen, r.ActiveSpacing)
	}
	fmt.Printf("\nFEASIBILITY GATE SUMMARY: %d / %d combinations PASSED\n", passedCount, len(results))
	if passedCount != len(results) {
		return 1
	}
	return 0
}
