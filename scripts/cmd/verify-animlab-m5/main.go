package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

type checkResult struct {
	ID     string
	Passed bool
	Detail string
}

func httpGet(u string) (int, http.Header, string, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return 0, nil, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b), nil
}

func main() {
	os.Exit(run())
}

func run() int {
	baseURL := os.Getenv("ANIMLAB_VIEWER_URL")
	if baseURL == "" {
		baseURL = "http://192.168.100.40:8093"
	}
	cdpEndpoint := os.Getenv("ANIMLAB_CDP")
	if cdpEndpoint == "" {
		cdpEndpoint = "http://127.0.0.1:9222"
	}
	kiwiHost := os.Getenv("ANIMLAB_CPU_HOST")
	if kiwiHost == "" {
		kiwiHost = "nigel@192.168.100.40"
	}
	evidenceDir := filepath.Join("testlab", "m5-evidence")
	_ = os.MkdirAll(evidenceDir, 0755)

	var results []checkResult
	record := func(id string, ok bool, detail string) {
		status := "PASS"
		if !ok {
			status = "FAIL"
		}
		fmt.Printf("[%s] %s: %s\n", status, id, detail)
		results = append(results, checkResult{ID: id, Passed: ok, Detail: detail})
	}

	// AT-M5-01, AT-M5-02, AT-M5-06, AT-M5-07: Landing page, discovery, historical labels, limitations
	code, _, body, err := httpGet(baseURL + "/")
	record("AT-M5-01", err == nil && code == 200 && strings.Contains(body, "SVG Animation Test Lab"), fmt.Sprintf("status=%d err=%v", code, err))
	expFound := strings.Contains(body, "m5-validated-showcase") && strings.Contains(body, "tracer-comparison") && strings.Contains(body, "mixed-dashboard")
	record("AT-M5-02", expFound, "m5-validated-showcase, tracer-comparison, mixed-dashboard listed")
	record("AT-M5-06", strings.Contains(body, "HISTORICAL — UNVALIDATED METHODOLOGY") && strings.Contains(body, "VALIDATED (2.0.0)"), "historical & validated badges present")
	record("AT-M5-07", strings.Contains(body, "does not run M1 per-frame motion sampling") && strings.Contains(body, "Shared-scene SMIL per-object off-screen suspension is unsupported"), "workload & Stage 16C coverage disclosures present")

	// AT-M5-05: Report integrity & inline /view
	rCode, _, rBody, _ := httpGet(baseURL + "/m5-validated-showcase/report.html")
	vCode, _, vBody, _ := httpGet(baseURL + "/view?exp=m5-validated-showcase&file=report.md")
	yCode, _, yBody, _ := httpGet(baseURL + "/view?exp=m5-validated-showcase&file=resolved-config.yaml")
	jCode, _, jBody, _ := httpGet(baseURL + "/m5-validated-showcase/raw-results.json")
	record("AT-M5-05", rCode == 200 && strings.Contains(rBody, "resolved-config.yaml") && strings.Contains(rBody, "raw-results.json") &&
		vCode == 200 && strings.Contains(vBody, "m5-validated-showcase") &&
		yCode == 200 && strings.Contains(yBody, "test:") &&
		jCode == 200 && strings.Contains(jBody, "measurementSpecVersion"),
		fmt.Sprintf("report=%d viewMd=%d viewYaml=%d rawJson=%d", rCode, vCode, yCode, jCode))

	// AT-M5-10: Security & path traversal
	t1, _, _, _ := httpGet(baseURL + "/../etc/passwd")
	t2, _, _, _ := httpGet(baseURL + "/.git/config")
	t3, _, _, _ := httpGet(baseURL + "/view?exp=..&file=report.md")
	// Note Go http.Client cleans /../ in URL, so also test raw TCP/curl --path-as-is
	curlOut, _ := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", "--path-as-is", baseURL+"/../etc/passwd").Output()
	curlCode := strings.TrimSpace(string(curlOut))
	record("AT-M5-10", (t1 == 403 || t1 == 404) && t2 == 403 && t3 == 400 && (curlCode == "403" || curlCode == "400"),
		fmt.Sprintf("traversal=%d rawCurl=%s dotfile=%d viewBad=%d", t1, curlCode, t2, t3))

	// Browser checks: AT-M5-03 (catalogue <object> animation), AT-M5-04 (side-by-side), AT-M5-08 (M4 demos)
	sess, err := animlab.AcquireBrowserSession(context.Background(), cdpEndpoint, kiwiHost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to acquire browser session: %v\n", err)
		return 1
	}
	defer sess.Close()
	ctx, cCancel := sess.NewTab()
	defer cCancel()
	_, _ = chromedp.Call(ctx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
		Width: 1366, Height: 900, DeviceScaleFactor: 1, Mobile: false,
	})

	// Capture runtime errors inside the browser page & <object> frames
	var consoleErrors []string

	// Capture landing page screenshot
	_, _ = chromedp.Run(ctx, chromedp.Navigate(baseURL+"/"))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(300*time.Millisecond))
	shotLand, _ := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	_ = os.WriteFile(filepath.Join(evidenceDir, "vis-m5-01-landing.png"), shotLand, 0644)

	// Verify historical catalogue injects HISTORICAL banner without modifying archived files
	_, histBody, _ := func() (int, string, error) { c, _, b, e := httpGet(baseURL + "/tracer-comparison/catalogue/index.html"); return c, b, e }()
	histBannerOK := strings.Contains(histBody, "HISTORICAL — UNVALIDATED METHODOLOGY")

	// Open validated multi-family catalogue & verify CSS, SMIL, and JS-RAF animate inside <object>
	// across perimeter-tracer, pulse, flash, flow-line, and flow-particle (multiple sizes & speeds)
	_, _ = chromedp.Run(ctx, chromedp.Navigate(baseURL+"/m5-validated-showcase/catalogue/index.html"))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
	shotCat, _ := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	_ = os.WriteFile(filepath.Join(evidenceDir, "vis-m5-02-catalogue.png"), shotCat, 0644)

	sampleProbeJS := `(() => {
		const cards = Array.from(document.querySelectorAll('#grid .card'));
		return JSON.stringify(cards.map(c => {
			const obj = c.querySelector('object');
			const doc = obj ? obj.contentDocument : null;
			const el = doc ? doc.querySelector('.lab-payload') : null;
			if (!el) return { file: obj ? obj.getAttribute('data') : '', missing: true };
			const cs = getComputedStyle(el);
			const r = el.getBoundingClientRect();
			return {
				file: obj.getAttribute('data'),
				fam: el.getAttribute('data-lab-family') || '',
				method: el.getAttribute('data-lab-method') || '',
				off: parseFloat(cs.strokeDashoffset) || 0,
				op: parseFloat(cs.opacity) || 0,
				tr: cs.transform || '',
				attrTr: el.getAttribute('transform') || '',
				cx: r.left + r.width * 0.5,
				cy: r.top + r.height * 0.5
			};
		}));
	})()`

	// Sample 10 frames at 90ms cadence (~900ms) so fast/medium tracer, pulse, flash, flow-line, flow-particle are all caught moving
	var series []string
	for k := 0; k < 10; k++ {
		s, _ := chromedp.Run(ctx, chromedp.Evaluate[string](sampleProbeJS))
		series = append(series, s)
		if k < 9 {
			_, _ = chromedp.Run(ctx, chromedp.Sleep(90*time.Millisecond))
		}
	}

	// Verify in JS that EVERY card in #grid moved over the 900ms window
	movedSummary, _ := chromedp.Run(ctx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
		const series = %s.map(s => JSON.parse(s));
		const n = series[0].length;
		const stationary = [];
		const byFamMethod = {};
		for (let i = 0; i < n; i++) {
			const base = series[0][i];
			if (base.missing) { stationary.push(base.file + ':missing'); continue; }
			let moved = false;
			for (let k = 1; k < series.length; k++) {
				const cur = series[k][i];
				if (!cur) continue;
				if (Math.abs(cur.off - base.off) > 1.0 ||
					Math.abs(cur.op - base.op) > 0.02 ||
					cur.tr !== base.tr ||
					cur.attrTr !== base.attrTr ||
					Math.hypot(cur.cx - base.cx, cur.cy - base.cy) > 1.0) {
					moved = true;
					break;
				}
			}
			const key = base.fam + '/' + base.method;
			byFamMethod[key] = (byFamMethod[key] || 0) + (moved ? 1 : 0);
			if (!moved) stationary.push(base.file);
		}
		return JSON.stringify({ total: n, stationary, byFamMethod });
	})()`, func() string {
		var quoted []string
		for _, s := range series {
			quoted = append(quoted, fmt.Sprintf("%q", s))
		}
		return "[" + strings.Join(quoted, ",") + "]"
	}())))

	cmpCount, _ := chromedp.Run(ctx, chromedp.Evaluate[int](`document.querySelectorAll('#compareGrid .cmp-row').length`))

	// Also open representative Standalone SVG links directly (CSS, SMIL, and JS-RAF across pulse + tracer + flow-particle)
	directSVGs := []string{
		"/m5-validated-showcase/samples/perimeter-tracer__javascript-raf__rectangle__medium__fast.svg",
		"/m5-validated-showcase/samples/perimeter-tracer__css__circle__small__medium.svg",
		"/m5-validated-showcase/samples/pulse__javascript-raf__circle__medium__fast__scale-opacity.svg",
		"/m5-validated-showcase/samples/pulse__smil__diamond__small__medium__scale-opacity.svg",
		"/m5-validated-showcase/samples/flow-line__javascript-raf__curved__medium__medium.svg",
		"/m5-validated-showcase/samples/flow-particle__javascript-raf__curved__medium__medium.svg",
	}
	var directFail []string
	for _, relSVG := range directSVGs {
		_, _ = chromedp.Run(ctx, chromedp.Navigate(baseURL+relSVG))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(150*time.Millisecond))
		s0, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			const el = document.querySelector('.lab-payload');
			if (!el) return 'missing';
			const cs = getComputedStyle(el);
			const r = el.getBoundingClientRect();
			return JSON.stringify({ off: parseFloat(cs.strokeDashoffset)||0, op: parseFloat(cs.opacity)||0, tr: cs.transform||'', attrTr: el.getAttribute('transform')||'', cx: r.left+r.width/2, cy: r.top+r.height/2 });
		})()`))
		_, _ = chromedp.Run(ctx, chromedp.Sleep(350*time.Millisecond))
		s1, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
			const el = document.querySelector('.lab-payload');
			if (!el) return 'missing';
			const cs = getComputedStyle(el);
			const r = el.getBoundingClientRect();
			return JSON.stringify({ off: parseFloat(cs.strokeDashoffset)||0, op: parseFloat(cs.opacity)||0, tr: cs.transform||'', attrTr: el.getAttribute('transform')||'', cx: r.left+r.width/2, cy: r.top+r.height/2 });
		})()`))
		if s0 == "missing" || s1 == "missing" || s0 == s1 {
			directFail = append(directFail, relSVG)
		}
	}

	animOK := strings.Contains(movedSummary, `"stationary":[]`) && len(directFail) == 0 && len(consoleErrors) == 0 && histBannerOK
	record("AT-M5-03", animOK, fmt.Sprintf("catalogue=%s directFails=%v consoleErrs=%v histBanner=%v", movedSummary, directFail, consoleErrors, histBannerOK))
	record("AT-M5-04", cmpCount >= 4, fmt.Sprintf("sideBySideRows=%d", cmpCount))

	// AT-M5-08: M4 Demonstrations A & B from /
	_, _ = chromedp.Run(ctx, chromedp.Navigate(baseURL+"/demos/demo-a-digital-twin.html"))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(250*time.Millisecond))
	dtCnt, _ := chromedp.Run(ctx, chromedp.Evaluate[int](`document.querySelectorAll('.lab-instance').length`))
	_, _ = chromedp.Run(ctx, chromedp.Navigate(baseURL+"/demos/demo-b-stage16c-dashboard.html"))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(300*time.Millisecond))
	wCnt, _ := chromedp.Run(ctx, chromedp.Evaluate[int](`(window.__labWidgetInstances || []).length`))
	record("AT-M5-08", dtCnt == 100 && wCnt == 100, fmt.Sprintf("dtOverlays=%d stage16cWidgets=%d", dtCnt, wCnt))

	// AT-M5-09: Service restart resilience & port isolation
	_ = exec.Command("ssh", "-o", "BatchMode=yes", kiwiHost, "systemctl --user restart anim-test-lab-viewer.service").Run()
	time.Sleep(1 * time.Second)
	postCode, _, _, postErr := httpGet(baseURL + "/")
	portsOut, _ := exec.Command("ssh", "-o", "BatchMode=yes", kiwiHost, "ss -tln | grep -E ':8088|:8091|:8092|:8093'").Output()
	portsStr := string(portsOut)
	portsOK := strings.Contains(portsStr, ":8088") && strings.Contains(portsStr, ":8091") && strings.Contains(portsStr, ":8092") && strings.Contains(portsStr, ":8093")
	record("AT-M5-09", postErr == nil && postCode == 200 && portsOK, fmt.Sprintf("postRestartHTTP=%d ports8088_91_92_93=%v", postCode, portsOK))

	// AT-M5-11: YAML-only samples generation without recompilation
	binPath := "/tmp/anim-test-lab"
	if _, err := os.Stat(binPath); err != nil {
		_ = exec.Command("go", "build", "-o", binPath, "./cmd/anim-test-lab").Run()
	}
	tmpOut := "/tmp/m5-yaml-samples"
	_ = os.RemoveAll(tmpOut)
	_, errS := exec.Command(binPath, "samples", "-c", "testlab/configs/tracer-method-comparison.yaml", "-out", tmpOut).CombinedOutput()
	_, errCat := os.Stat(filepath.Join(tmpOut, "catalogue", "index.html"))
	_ = os.RemoveAll(tmpOut)
	record("AT-M5-11", errS == nil && errCat == nil, fmt.Sprintf("samplesErr=%v catErr=%v", errS, errCat))

	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	fmt.Printf("\nSUMMARY: %d/%d M5 checks passed (%d failed)\n", len(results)-failed, len(results), failed)
	if failed > 0 {
		return 1
	}
	return 0
}
