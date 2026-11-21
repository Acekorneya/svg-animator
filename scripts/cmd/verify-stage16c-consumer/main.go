package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

const (
	demoURL     = "http://192.168.100.40:8091/index.html"
	cdpEndpoint = "http://127.0.0.1:9222"
	cpuHost     = "nigel@192.168.100.40"
)

type CheckResult struct {
	Number  int
	Name    string
	Passed  bool
	Message string
}

func runVoid(ctx context.Context, actions ...chromedp.Action[chromedp.Void]) error {
	for _, act := range actions {
		if _, err := chromedp.Run(ctx, act); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	os.Exit(run())
}

func run() int {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16C: External Data-Driven Widget Integration Verification")
	fmt.Println(" Target: http://192.168.100.40:8091/index.html via Chrome CDP")
	fmt.Println("==================================================================")

	var (
		results []CheckResult
		mu      sync.Mutex
	)
	record := func(num int, name string, passed bool, msg string) {
		mu.Lock()
		defer mu.Unlock()
		results = append(results, CheckResult{Number: num, Name: name, Passed: passed, Message: msg})
		status := "PASS"
		if !passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] Criteria %2d: %-36s - %s\n", status, num, name, msg)
	}

	sess, err := animlab.AcquireBrowserSession(context.Background(), cdpEndpoint, cpuHost)
	if err != nil {
		log.Printf("Failed to acquire browser session: %v", err)
		return 1
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(sess.AllocCtx, 60*time.Second)
	defer cancel()

	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	// Navigate and wait for SVG mount
	if err := runVoid(taskCtx,
		chromedp.Navigate(demoURL),
		chromedp.WaitReady("#stage-host-a svg"),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		log.Printf("Failed to navigate/mount at %s: %v", demoURL, err)
		return 1
	}

	// 1. Check zero Studio dependencies and 3 mounted instances
	initReport, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const hasStudio = Boolean(window.EngineBridge || window.AppCoordinator || window.DiagramNodes || window.SceneState);
		const svgA = document.querySelector('#stage-host-a svg');
		const svgB = document.querySelector('#stage-host-b svg');
		const svgC = document.querySelector('#stage-host-c svg');
		const textA = svgA ? svgA.querySelector('[data-semantic-target="node-1"] text')?.textContent : '';
		const textB = svgB ? svgB.querySelector('[data-semantic-target="node-1"] text')?.textContent : '';
		const textC = svgC ? svgC.querySelector('[data-semantic-target="node-1"] text')?.textContent : '';
		const flowBVis = svgB ? svgB.querySelector('[data-semantic-target="flow-1"]')?.getAttribute('visibility') : '';
		const errText = document.getElementById('errorBanner')?.textContent || '';

		const idsA = svgA ? Array.from(svgA.querySelectorAll('*[id]')).map(e => e.id) : [];
		const idsB = svgB ? Array.from(svgB.querySelectorAll('*[id]')).map(e => e.id) : [];
		const idsC = svgC ? Array.from(svgC.querySelectorAll('*[id]')).map(e => e.id) : [];
		const overlapAB = idsA.filter(id => idsB.includes(id)).length;
		const overlapAC = idsA.filter(id => idsC.includes(id)).length;

		return JSON.stringify({
			hasStudio,
			mountedAll: Boolean(svgA && svgB && svgC),
			textA, textB, textC, flowBVis, errText,
			overlap: overlapAB + overlapAC,
			idCount: idsA.length
		});
	})()`))
	if err != nil {
		log.Printf("Initial evaluation failed: %v", err)
		return 1
	}

	var initData struct {
		HasStudio  bool   `json:"hasStudio"`
		MountedAll bool   `json:"mountedAll"`
		TextA      string `json:"textA"`
		TextB      string `json:"textB"`
		TextC      string `json:"textC"`
		FlowBVis   string `json:"flowBVis"`
		ErrText    string `json:"errText"`
		Overlap    int    `json:"overlap"`
		IDCount    int    `json:"idCount"`
	}
	_ = json.Unmarshal([]byte(initReport), &initData)

	record(1, "Independent Consumer (Zero Studio)", !initData.HasStudio && initData.MountedAll && initData.ErrText == "",
		fmt.Sprintf("Mounted 3 instances without Studio internals (hasStudio=%v, err=%q)", initData.HasStudio, initData.ErrText))

	record(2, "Zero Cross-Instance ID Collisions", initData.Overlap == 0 && initData.IDCount > 0,
		fmt.Sprintf("Overlap=%d across %d IDs per instance", initData.Overlap, initData.IDCount))

	record(3, "Initial Data-Driven DOM State", initData.TextA == "pve01" && initData.TextB == "pve02 [OFF]" && initData.TextC == "pve03" && initData.FlowBVis == "hidden",
		fmt.Sprintf("host-a=%q, host-b=%q (flow=%s), host-c=%q", initData.TextA, initData.TextB, initData.FlowBVis, initData.TextC))

	// 2. Test CPU = 0 vs CPU = Missing on host-a via real UI clicks
	if err := runVoid(taskCtx,
		chromedp.Click(`button[data-action="cpu-zero"][data-host="host-a"]`),
		chromedp.Sleep(200*time.Millisecond),
	); err != nil {
		log.Printf("Click cpu-zero failed: %v", err)
		return 1
	}

	zeroReport, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const svgA = document.querySelector('#stage-host-a svg');
		const stopColor = svgA.querySelector('radialGradient stop')?.getAttribute('stop-color') || '';
		const textA = svgA.querySelector('[data-semantic-target="node-1"] text')?.textContent || '';
		const mountCount = window.Stage16CDemo.manager.getInstanceInfo('host-a').mountCount;
		return JSON.stringify({ stopColor, textA, mountCount });
	})()`))

	if err := runVoid(taskCtx,
		chromedp.Click(`button[data-action="cpu-missing"][data-host="host-a"]`),
		chromedp.Sleep(200*time.Millisecond),
	); err != nil {
		log.Printf("Click cpu-missing failed: %v", err)
		return 1
	}

	missingReport, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const svgA = document.querySelector('#stage-host-a svg');
		const stopColor = svgA.querySelector('radialGradient stop')?.getAttribute('stop-color') || '';
		const textA = svgA.querySelector('[data-semantic-target="node-1"] text')?.textContent || '';
		const textC = document.querySelector('#stage-host-c svg [data-semantic-target="node-1"] text')?.textContent || '';
		const mountCount = window.Stage16CDemo.manager.getInstanceInfo('host-a').mountCount;
		return JSON.stringify({ stopColor, textA, textC, mountCount });
	})()`))

	var zData, mData struct {
		StopColor  string `json:"stopColor"`
		TextA      string `json:"textA"`
		TextC      string `json:"textC"`
		MountCount int    `json:"mountCount"`
	}
	_ = json.Unmarshal([]byte(zeroReport), &zData)
	_ = json.Unmarshal([]byte(missingReport), &mData)

	zeroVsMissingOk := zData.StopColor == "#38bdf8" && zData.TextA == "pve01" &&
		mData.StopColor == "#f59e0b" && mData.TextA == "pve01 [NO CPU]" &&
		mData.TextC == "pve03" && mData.MountCount == 1
	record(4, "Zero vs Missing & No Remount", zeroVsMissingOk,
		fmt.Sprintf("CPU=0 (%s/%s) vs Missing (%s/%s), mountCount=%d, host-c=%q", zData.StopColor, zData.TextA, mData.StopColor, mData.TextA, mData.MountCount, mData.TextC))

	// 3. Online/Offline transition on host-b
	if err := runVoid(taskCtx,
		chromedp.Click(`button[data-action="toggle-online"][data-host="host-b"]`),
		chromedp.Sleep(200*time.Millisecond),
	); err != nil {
		log.Printf("Toggle online failed: %v", err)
		return 1
	}
	hostBOnlineOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const svgB = document.querySelector('#stage-host-b svg');
		return svgB.querySelector('[data-semantic-target="node-1"] text')?.textContent === 'pve02' &&
			svgB.querySelector('[data-semantic-target="flow-1"]')?.getAttribute('visibility') === 'visible';
	})()`))
	record(5, "Offline -> Online Transition", hostBOnlineOk, "host-b transitioned to pve02 with flow visibility=visible")

	// 4. Invalid value rejection without state corruption
	if err := runVoid(taskCtx,
		chromedp.Click(`button[data-action="test-invalid"][data-host="host-a"]`),
		chromedp.Sleep(150*time.Millisecond),
	); err != nil {
		log.Printf("Click test-invalid failed: %v", err)
		return 1
	}
	invalidReport, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`document.getElementById('errorBanner')?.textContent || ''`))
	record(6, "Invalid Value Explicit Error", strings.Contains(invalidReport, "above maximum allowed"),
		fmt.Sprintf("Error banner displayed: %q", invalidReport))

	// 5. Unmount & Re-mount on host-b
	if err := runVoid(taskCtx,
		chromedp.Click(`button[data-action="toggle-mount"][data-host="host-b"]`),
		chromedp.Sleep(150*time.Millisecond),
	); err != nil {
		log.Printf("Unmount failed: %v", err)
		return 1
	}
	unmountedOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`!document.querySelector('#stage-host-b svg') && Boolean(document.querySelector('#stage-host-a svg'))`))
	if err := runVoid(taskCtx,
		chromedp.Click(`button[data-action="toggle-mount"][data-host="host-b"]`),
		chromedp.Sleep(150*time.Millisecond),
	); err != nil {
		log.Printf("Remount failed: %v", err)
		return 1
	}
	remountedOk, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(document.querySelector('#stage-host-b svg'))`))
	record(7, "Safe Unmount & Remount Lifecycle", unmountedOk && remountedOk, "host-b unmounted cleanly without affecting host-a and remounted cleanly")

	// 6. Reject inconsistent/mixed package
	if err := runVoid(taskCtx,
		chromedp.SetValue(`#packageSelect`, "broken-inconsistent"),
		chromedp.Click(`#btnLoadPackage`),
		chromedp.Sleep(300*time.Millisecond),
	); err != nil {
		log.Printf("Broken package test failed: %v", err)
		return 1
	}
	brokenErr, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`document.getElementById('errorBanner')?.textContent || ''`))
	record(8, "Inconsistent Package Rejection", strings.Contains(brokenErr, "Package consistency error"),
		fmt.Sprintf("Broken package rejected: %q", brokenErr))

	// 7. Reject stale v1 mapping on v2 package, then load v2 with v2 mapping
	if err := runVoid(taskCtx,
		chromedp.SetValue(`#packageSelect`, "host-monitor-v2"),
		chromedp.SetValue(`#mappingSelect`, "host-monitor-v1"),
		chromedp.Click(`#btnLoadPackage`),
		chromedp.Sleep(300*time.Millisecond),
	); err != nil {
		log.Printf("Stale mapping test failed: %v", err)
		return 1
	}
	staleErr, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`document.getElementById('errorBanner')?.textContent || ''`))

	if err := runVoid(taskCtx,
		chromedp.SetValue(`#mappingSelect`, "host-monitor-v2"),
		chromedp.Click(`#btnLoadPackage`),
		chromedp.Sleep(300*time.Millisecond),
	); err != nil {
		log.Printf("v2 package load failed: %v", err)
		return 1
	}
	v2Ok, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		const err = document.getElementById('errorBanner')?.textContent || '';
		const widgetName = document.getElementById('stWidgetName')?.textContent || '';
		const textC = document.querySelector('#stage-host-c svg [data-semantic-target="node-1"] text')?.textContent || '';
		return err === '' && widgetName === 'host-monitor-v2' && textC === 'pve03';
	})()`))
	record(9, "Version Switch & Stale Mapping Guard", strings.Contains(staleErr, "Invalid parameter mapping") && v2Ok,
		fmt.Sprintf("Stale v1 mapping rejected (%q); v2 loaded cleanly", staleErr))

	// Summary
	fmt.Println("\n==================================================================")
	fmt.Println(" STAGE 16C REAL-BROWSER CDP ACCEPTANCE SUMMARY")
	fmt.Println("==================================================================")
	allPassed := true
	for _, r := range results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
			allPassed = false
		}
		fmt.Printf("[%s] Criteria %2d: %-36s : %s\n", status, r.Number, r.Name, r.Message)
	}
	if !allPassed {
		return 1
	}
	fmt.Println("\n>>> ALL 9 STAGE 16C REAL-BROWSER CDP CRITERIA PASS 100% ON KIWIBERRY <<<")
	return 0
}
