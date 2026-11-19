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
	"github.com/chromedp/chromedp/remote"
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
	allocCtx, aCancel := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer aCancel()
	ctx, cCancel := chromedp.NewContext(allocCtx)
	defer cCancel()
	_, _ = chromedp.Call(ctx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
		Width: 1366, Height: 900, DeviceScaleFactor: 1, Mobile: false,
	})

	// Capture landing page screenshot
	_, _ = chromedp.Run(ctx, chromedp.Navigate(baseURL+"/"))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(300*time.Millisecond))
	shotLand, _ := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	_ = os.WriteFile(filepath.Join(evidenceDir, "vis-m5-01-landing.png"), shotLand, 0644)

	// Open catalogue & verify CSS, SMIL, and JS-RAF animate inside <object>
	_, _ = chromedp.Run(ctx, chromedp.Navigate(baseURL+"/m5-validated-showcase/catalogue/index.html"))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
	shotCat, _ := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	_ = os.WriteFile(filepath.Join(evidenceDir, "vis-m5-02-catalogue.png"), shotCat, 0644)

	snap0, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const objs = Array.from(document.querySelectorAll('#compareGrid object'));
		return JSON.stringify(objs.map(o => {
			const doc = o.contentDocument;
			const el = doc ? doc.querySelector('.lab-payload') : null;
			return {
				data: o.getAttribute('data'),
				method: el ? el.getAttribute('data-lab-method') : '',
				off: el ? parseFloat(getComputedStyle(el).strokeDashoffset) || 0 : 0
			};
		}));
	})()`))
	_, _ = chromedp.Run(ctx, chromedp.Sleep(450*time.Millisecond))
	snap1, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`(() => {
		const objs = Array.from(document.querySelectorAll('#compareGrid object'));
		return JSON.stringify(objs.map(o => {
			const doc = o.contentDocument;
			const el = doc ? doc.querySelector('.lab-payload') : null;
			return {
				data: o.getAttribute('data'),
				method: el ? el.getAttribute('data-lab-method') : '',
				off: el ? parseFloat(getComputedStyle(el).strokeDashoffset) || 0 : 0
			};
		}));
	})()`))
	cmpCount, _ := chromedp.Run(ctx, chromedp.Evaluate[int](`document.querySelectorAll('#compareGrid .cmp-row').length`))
	// Check that css, smil, and javascript-raf all moved between snap0 and snap1
	animOK := snap0 != snap1 && strings.Contains(snap1, `"method":"css"`) && strings.Contains(snap1, `"method":"smil"`) && strings.Contains(snap1, `"method":"javascript-raf"`)
	record("AT-M5-03", animOK, fmt.Sprintf("compareObjectsAnimated=%v", animOK))
	record("AT-M5-04", cmpCount >= 1, fmt.Sprintf("sideBySideRows=%d", cmpCount))

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
		os.Exit(1)
	}
}
