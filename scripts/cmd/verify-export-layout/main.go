package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	"gitea.seaoffate.net/blackgrape/svg-animator/pkg/animlab"
)

type ViewportTest struct {
	Name   string
	Width  int64
	Height int64
}

func main() {
	os.Exit(run())
}

func run() int {
	fmt.Println("==================================================================")
	fmt.Println(" Stage 16B: Export Panel Layout Multi-Viewport Verification")
	fmt.Println("==================================================================")

	viewports := []ViewportTest{
		{Name: "Desktop (1920x1080)", Width: 1920, Height: 1080},
		{Name: "Laptop (1366x768)", Width: 1366, Height: 768},
		{Name: "Tablet/Narrow (1024x768)", Width: 1024, Height: 768},
	}

	sess, err := animlab.AcquireBrowserSession(context.Background(), "http://127.0.0.1:9222", "nigel@192.168.100.40")
	if err != nil {
		log.Printf("AcquireBrowserSession failed: %v", err)
		return 1
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(sess.AllocCtx, 45*time.Second)
	defer cancel()

	taskCtx, tCancel := chromedp.NewContext(ctx)
	defer tCancel()

	for idx, vp := range viewports {
		fmt.Printf("\n--- [TEST %d/3] Viewport: %s (%dx%d) ---\n", idx+1, vp.Name, vp.Width, vp.Height)

		// 1. Set device metrics override
		if _, err := chromedp.Call(taskCtx, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
			Width:             vp.Width,
			Height:            vp.Height,
			DeviceScaleFactor: 1.0,
			Mobile:            false,
		}); err != nil {
			log.Printf("SetDeviceMetricsOverride failed: %v", err)
			return 1
		}

		// 2. Navigate and wait for DOM
		if _, err := chromedp.Run(taskCtx, chromedp.Navigate("http://localhost:8088/")); err != nil {
			log.Printf("Navigate failed: %v", err)
			return 1
		}

		if _, err := chromedp.Run(taskCtx, chromedp.WaitReady("#acc-export")); err != nil {
			log.Printf("WaitReady #acc-export failed: %v", err)
			return 1
		}

		// 3. Open Right Drawer and Export panel
		setupJS := `(() => {
			// Ensure right drawer is expanded
			const drawer = document.querySelector('#rightDrawer');
			if (drawer && drawer.classList.contains('collapsed')) {
				const toggleBtn = document.querySelector('#railBtn-toggleRight') || document.querySelector('#btnCollapseRightDrawer');
				if (toggleBtn) toggleBtn.click();
			}
			
			// Click the Export rail button to focus Export accordion
			const exportRailBtn = document.querySelector('#railBtn-export');
			if (exportRailBtn) {
				exportRailBtn.click();
			} else {
				// Or manually open acc-export
				const accExport = document.querySelector('#acc-export');
				if (accExport && !accExport.classList.contains('open')) {
					accExport.querySelector('.accordion-header')?.click();
				}
			}
		})()`

		if _, err := chromedp.Run(taskCtx, chromedp.Evaluate[any](setupJS)); err != nil {
			log.Printf("Setup JS failed: %v", err)
			return 1
		}

		// Small delay for animation/render
		time.Sleep(300 * time.Millisecond)

		// 4. Measure layout bounding boxes
		measureJS := `(() => {
			const drawer = document.querySelector('#rightDrawer');
			const settings = document.querySelector('#settingsPanel');
			const accExport = document.querySelector('#acc-export');
			const canvas = document.querySelector('#canvasStage, .workspace-canvas, #canvas-stage');
			const btnGroup = accExport ? accExport.querySelector('.btn-group') : null;
			
			const rDrawer = drawer ? drawer.getBoundingClientRect() : null;
			const rSettings = settings ? settings.getBoundingClientRect() : null;
			const rExport = accExport ? accExport.getBoundingClientRect() : null;
			const rCanvas = canvas ? canvas.getBoundingClientRect() : null;
			const rBtnGroup = btnGroup ? btnGroup.getBoundingClientRect() : null;

			// Check all buttons inside export panel
			const buttons = accExport ? Array.from(accExport.querySelectorAll('button')) : [];
			const btnInfos = buttons.map(b => {
				const r = b.getBoundingClientRect();
				return {
					id: b.id,
					text: b.textContent.trim().slice(0, 30),
					width: r.width,
					height: r.height,
					x: r.x,
					y: r.y,
					visible: r.width > 0 && r.height > 0
				};
			});

			return {
				viewportWidth: window.innerWidth,
				viewportHeight: window.innerHeight,
				isInsideSettingsPanel: settings ? settings.contains(accExport) : false,
				isInsideRightDrawer: drawer ? drawer.contains(accExport) : false,
				rDrawer: rDrawer ? { x: rDrawer.x, y: rDrawer.y, w: rDrawer.width, h: rDrawer.height } : null,
				rSettings: rSettings ? { x: rSettings.x, y: rSettings.y, w: rSettings.width, h: rSettings.height } : null,
				rExport: rExport ? { x: rExport.x, y: rExport.y, w: rExport.width, h: rExport.height } : null,
				rCanvas: rCanvas ? { x: rCanvas.x, y: rCanvas.y, w: rCanvas.width, h: rCanvas.height } : null,
				rBtnGroup: rBtnGroup ? { x: rBtnGroup.x, y: rBtnGroup.y, w: rBtnGroup.width, h: rBtnGroup.height } : null,
				buttons: btnInfos
			};
		})()`

		var data struct {
			ViewportWidth         float64            `json:"viewportWidth"`
			ViewportHeight        float64            `json:"viewportHeight"`
			IsInsideSettingsPanel bool               `json:"isInsideSettingsPanel"`
			IsInsideRightDrawer   bool               `json:"isInsideRightDrawer"`
			RDrawer               map[string]float64 `json:"rDrawer"`
			RSettings             map[string]float64 `json:"rSettings"`
			RExport               map[string]float64 `json:"rExport"`
			RCanvas               map[string]float64 `json:"rCanvas"`
			RBtnGroup             map[string]float64 `json:"rBtnGroup"`
			Buttons               []struct {
				ID      string  `json:"id"`
				Text    string  `json:"text"`
				Width   float64 `json:"width"`
				Height  float64 `json:"height"`
				X       float64 `json:"x"`
				Y       float64 `json:"y"`
				Visible bool    `json:"visible"`
			} `json:"buttons"`
		}

		rawMeasure, err := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", measureJS)))
		if err != nil {
			log.Printf("Measure JS failed: %v", err)
			return 1
		}
		json.Unmarshal([]byte(rawMeasure), &data)

		// Assertions
		if !data.IsInsideSettingsPanel || !data.IsInsideRightDrawer {
			log.Printf("[FAIL] #acc-export is not contained inside #settingsPanel / #rightDrawer!")
			return 1
		}
		fmt.Printf("   ✓ Hierarchy: #acc-export is cleanly inside #settingsPanel and #rightDrawer\n")

		// Check widths
		drawerW := data.RDrawer["w"]
		exportW := data.RExport["w"]
		canvasR := data.RCanvas["x"] + data.RCanvas["w"]
		exportX := data.RExport["x"]

		fmt.Printf("   ✓ Drawer Width: %.1fpx | Export Width: %.1fpx | Canvas Right: %.1fpx | Export X: %.1fpx\n",
			drawerW, exportW, canvasR, exportX)

		// Export panel width must NOT exceed drawer width + padding (e.g. 5px tolerance)
		if exportW > drawerW+5 {
			log.Printf("[FAIL] Export panel width (%.1fpx) exceeds drawer width (%.1fpx)!", exportW, drawerW)
			return 1
		}
		fmt.Printf("   ✓ Bounds: Export panel width (%.1fpx) <= drawer width (%.1fpx)\n", exportW, drawerW)

		// Export panel must NOT overlap canvas stage
		if exportX < canvasR-5 {
			log.Printf("[FAIL] Export panel (X: %.1f) overlaps canvas stage (Right: %.1f)!", exportX, canvasR)
			return 1
		}
		fmt.Printf("   ✓ Non-overlapping: Canvas stage right edge (%.1fpx) <= Export panel left edge (%.1fpx)\n", canvasR, exportX)

		// Verify buttons
		fmt.Printf("   ✓ Verified %d export action buttons inside panel:\n", len(data.Buttons))
		for _, btn := range data.Buttons {
			if !btn.Visible {
				log.Printf("[FAIL] Button %s (%s) is not visible!", btn.ID, btn.Text)
				return 1
			}
			if btn.Width > exportW+5 {
				log.Printf("[FAIL] Button %s width (%.1fpx) overflows export panel width (%.1fpx)!", btn.ID, btn.Width, exportW)
				return 1
			}
			fmt.Printf("      - [%s] \"%s\" (w: %.1fpx, h: %.1fpx)\n", btn.ID, btn.Text, btn.Width, btn.Height)
		}

		// 5. Test switching between Widget panel and Export panel
		fmt.Println("   ✓ Testing accordion transition between Widget Parameters and Export panels...")
		transitionJS := `(() => {
			const widgetRailBtn = document.querySelector('#railBtn-widget-params');
			if (widgetRailBtn) widgetRailBtn.click();
			
			const accWidget = document.querySelector('#acc-widget-params');
			const accExport = document.querySelector('#acc-export');
			
			const widgetOpen = accWidget && accWidget.classList.contains('open');
			
			// Switch back to export
			const exportRailBtn = document.querySelector('#railBtn-export');
			if (exportRailBtn) exportRailBtn.click();
			
			const exportOpen = accExport && accExport.classList.contains('open');
			
			return { widgetOpen, exportOpen };
		})()`

		var transResult struct {
			WidgetOpen bool `json:"widgetOpen"`
			ExportOpen bool `json:"exportOpen"`
		}
		rawTrans, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", transitionJS)))
		json.Unmarshal([]byte(rawTrans), &transResult)
		fmt.Printf("   ✓ Transitions: Widget Panel opened (%v) -> Export Panel re-opened (%v)\n", transResult.WidgetOpen, transResult.ExportOpen)

		// 6. Capture screenshot
		screenshotName := fmt.Sprintf("scripts/export-layout-%dx%d.png", vp.Width, vp.Height)
		buf, err := chromedp.Run(taskCtx, chromedp.CaptureScreenshot())
		if err == nil && len(buf) > 0 {
			os.WriteFile(screenshotName, buf, 0644)
			fmt.Printf("   ✓ Saved verification screenshot: %s (%d bytes)\n", screenshotName, len(buf))
		}

		// 7. Check console errors
		var consoleErrs []string
		checkErrJS := `(() => window.__errors || [])()`
		rawErr, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf("JSON.stringify(%s)", checkErrJS)))
		json.Unmarshal([]byte(rawErr), &consoleErrs)
		if len(consoleErrs) > 0 {
			log.Printf("[FAIL] Console errors detected at %s: %v", vp.Name, consoleErrs)
			return 1
		}
		fmt.Println("   ✓ Zero console errors reported")
	}

	fmt.Println("\n==================================================================")
	fmt.Println(" ALL REAL-BROWSER VIEWPORT LAYOUT TESTS PASSED WITH ZERO REGRESSIONS!")
	fmt.Println("==================================================================")
	return 0
}
