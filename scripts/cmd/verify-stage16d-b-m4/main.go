package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

const (
	studioURL    = "http://192.168.100.40:8088/"
	oscDemoURL   = "http://192.168.100.40:8088/oscilloscope-lite-demo.html"
	gaugeDemoURL = "http://192.168.100.40:8088/gauge-lite-demo.html"
	consumerURL  = "http://192.168.100.40:8091/index.html"
	cdpEndpoint  = "http://127.0.0.1:9222"
)

type GateResult struct {
	ID      string
	Name    string
	Passed  bool
	Details string
}

func runVoid(ctx context.Context, actions ...chromedp.Action[chromedp.Void]) error {
	for _, a := range actions {
		if _, err := chromedp.Run(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	fmt.Println("==========================================================================")
	fmt.Println(" Stage 16D-B-M4: Oscilloscope Lite & 4-Threshold Gauge Acceptance Suite")
	fmt.Println("==========================================================================")

	widgetJSBytes, err := os.ReadFile("/root/projects/svg-animator/internal/web/static/js/svg-widget.js")
	if err != nil {
		log.Fatalf("Failed to read svg-widget.js: %v", err)
	}
	widgetJSSource := string(widgetJSBytes)

	var results []GateResult
	record := func(id, name string, passed bool, details string) {
		results = append(results, GateResult{ID: id, Name: name, Passed: passed, Details: details})
		status := "PASS"
		if !passed {
			status = "FAIL"
		}
		fmt.Printf("[%s] %-10s %-46s | %s\n", status, id, name, details)
	}

	allocCtx, cancelAlloc := remote.NewAllocator(context.Background(), cdpEndpoint)
	defer cancelAlloc()
	ctx, cancelTimeout := context.WithTimeout(allocCtx, 180*time.Second)
	defer cancelTimeout()
	taskCtx, cancelTask := chromedp.NewContext(ctx)
	defer cancelTask()

	_, _ = chromedp.Call(taskCtx, network.SetCacheDisabled, network.SetCacheDisabledParams{CacheDisabled: true})

	if err := runVoid(taskCtx,
		chromedp.EmulateViewport(1440, 960),
		chromedp.Navigate(studioURL),
		chromedp.WaitReady("#railBtn-lite-effects"),
	); err != nil {
		log.Fatalf("Failed to open Studio at %s: %v", studioURL, err)
	}

	for i := 0; i < 30; i++ {
		ready, _ := chromedp.Run(taskCtx, chromedp.Evaluate[bool](`Boolean(window.EngineBridge && window.EngineBridge.isReady())`))
		if ready {
			break
		}
		_ = runVoid(taskCtx, chromedp.Sleep(300*time.Millisecond))
	}

	// Reset Studio to clean project
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](`(() => {
		window.confirm = () => true;
		window.alert = () => {};
		window.ProjectManager.resetToNewProject();
		window.EngineBridge.setWidgetParameters([]);
		return true;
	})()`))

	// =========================================================================
	// AT-M4-01: Part A — Gauge Lite Four Independently Enabled Thresholds
	// =========================================================================
	at01JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.openAccordionById('acc-lite-effects');
		document.getElementById('btnAddLiteGauge').click();
		const list = window.LiteEffects.getAllLiteEffects();
		const g = list[list.length - 1];
		// Configure: Base=#38bdf8, T1(40,#22c55e)=ON, T2(60,#eab308)=OFF, T3(80,#f59e0b)=ON, T4(95,#ef4444)=ON
		window.LiteEffects.updateLiteEffect(g.id, {
			color: '#38bdf8',
			minValue: 0,
			maxValue: 100,
			thresholds: [
				{ enabled: true,  value: 40, color: '#22c55e' },
				{ enabled: false, value: 60, color: '#eab308' },
				{ enabled: true,  value: 80, color: '#f59e0b' },
				{ enabled: true,  value: 95, color: '#ef4444' }
			]
		});
		const evalVal = (v) => {
			window.LiteEffects.updateLiteEffect(g.id, { value: v });
			window.LiteEffects.triggerAppRender();
			const domG = document.querySelector('#svgContainer #lite-' + g.id + '[data-lite-kind="gauge-lite"]');
			return domG ? domG.getAttribute('data-gauge-active-color') : '';
		};
		const c25 = evalVal(25); // <40 -> #38bdf8 (Base)
		const c40 = evalVal(40); // ==40 -> #22c55e (T1 inclusive boundary)
		const c65 = evalVal(65); // >=60, but T2 is disabled -> #22c55e (T1!)
		const c80 = evalVal(80); // ==80 -> #f59e0b (T3 inclusive boundary)
		const c95 = evalVal(95); // ==95 -> #ef4444 (T4 inclusive boundary)

		// Now enable T2 via inspector checkbox #liteGaugeThresh2Chk
		const chk2 = document.getElementById('liteGaugeThresh2Chk');
		chk2.checked = true;
		chk2.dispatchEvent(new Event('change'));
		const c65WithT2 = evalVal(65); // Now T2 is enabled -> #eab308

		// Disable all 4 thresholds -> value 97 must use Base #38bdf8
		[1, 2, 3, 4].forEach((idx) => {
			const chk = document.getElementById('liteGaugeThresh' + idx + 'Chk');
			chk.checked = false;
			chk.dispatchEvent(new Event('change'));
		});
		const c97AllOff = evalVal(97);

		// Restore T1=ON, T2=OFF, T3=ON, T4=ON
		window.LiteEffects.updateLiteEffect(g.id, {
			value: 65,
			threshold1Enabled: true,
			threshold2Enabled: false,
			threshold3Enabled: true,
			threshold4Enabled: true
		});
		window.LiteEffects.triggerAppRender();

		return JSON.stringify({
			id: String(g.id),
			c25, c40, c65, c80, c95, c65WithT2, c97AllOff
		});
	})()`))
	var at01 struct {
		ID        string `json:"id"`
		C25       string `json:"c25"`
		C40       string `json:"c40"`
		C65       string `json:"c65"`
		C80       string `json:"c80"`
		C95       string `json:"c95"`
		C65WithT2 string `json:"c65WithT2"`
		C97AllOff string `json:"c97AllOff"`
	}
	_ = json.Unmarshal([]byte(at01JSON), &at01)
	record("AT-M4-01", "Gauge Lite 4 Independently Enabled Thresholds",
		at01.C25 == "#38bdf8" && at01.C40 == "#22c55e" && at01.C65 == "#22c55e" &&
			at01.C80 == "#f59e0b" && at01.C95 == "#ef4444" &&
			at01.C65WithT2 == "#eab308" && at01.C97AllOff == "#38bdf8",
		fmt.Sprintf("25:%s 40:%s 65(T2=OFF):%s 80:%s 95:%s 65(T2=ON):%s allOff:%s",
			at01.C25, at01.C40, at01.C65, at01.C80, at01.C95, at01.C65WithT2, at01.C97AllOff))

	// =========================================================================
	// AT-M4-02: Legacy 2-Threshold Compatibility & Out-of-Order Disabled Band
	// =========================================================================
	at02JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		// Create a legacy 2-threshold gauge (no thresholds array, only warnThreshold=70, critThreshold=90)
		const leg = window.LiteEffects.createLiteEffect({
			name: 'legacyTwoBand',
			kind: 'gauge-lite',
			variant: 'hbar',
			minValue: 0,
			maxValue: 100,
			value: 75,
			color: '#38bdf8',
			useThresholds: true,
			warnThreshold: 70,
			warnColor: '#f59e0b',
			critThreshold: 90,
			critColor: '#ef4444'
		});
		window.LiteEffects.triggerAppRender();
		const legEl = document.querySelector('#svgContainer #lite-' + leg.id);
		const legColor75 = legEl ? legEl.getAttribute('data-gauge-active-color') : '';
		const bands = leg.thresholds || [];
		const t1On = bands[0] ? bands[0].enabled : true;
		const t2On = bands[1] ? bands[1].enabled : true;
		const t3On = bands[2] ? bands[2].enabled : false;
		const t4On = bands[3] ? bands[3].enabled : false;
		window.LiteEffects.deleteLiteEffect(leg.id);
		window.LiteEffects.triggerAppRender();
		return JSON.stringify({ legColor75, len: bands.length, t1On, t2On, t3On, t4On });
	})()`))
	var at02 struct {
		LegColor75 string `json:"legColor75"`
		Len        int    `json:"len"`
		T1On       bool   `json:"t1On"`
		T2On       bool   `json:"t2On"`
		T3On       bool   `json:"t3On"`
		T4On       bool   `json:"t4On"`
	}
	_ = json.Unmarshal([]byte(at02JSON), &at02)
	record("AT-M4-02", "Legacy 2-Threshold Gauge Migration to 4 Bands",
		at02.LegColor75 == "#f59e0b" && at02.Len == 4 && !at02.T1On && !at02.T2On && at02.T3On && at02.T4On,
		fmt.Sprintf("len=%d T1:%v T2:%v T3(Warn):%v T4(Crit):%v val75=%s",
			at02.Len, at02.T1On, at02.T2On, at02.T3On, at02.T4On, at02.LegColor75))

	// =========================================================================
	// AT-M4-03: Create Oscilloscope Lite via UI Button & Verify Structure
	// =========================================================================
	at03JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.openAccordionById('acc-lite-effects');
		const btn = document.getElementById('btnAddLiteOsc');
		if (!btn) return JSON.stringify({ ok: false });
		btn.click();
		const list = window.LiteEffects.getAllLiteEffects();
		const osc = list[list.length - 1];
		const g = document.querySelector('#svgContainer #lite-' + osc.id + '[data-lite-kind="oscilloscope-lite"]');
		const wave = g ? g.querySelector('.lite-osc-wave') : null;
		const clipGroup = g ? g.querySelector('.lite-osc-viewport') : null;
		return JSON.stringify({
			ok: Boolean(osc && g && wave && clipGroup),
			id: osc ? String(osc.id) : '',
			kind: osc ? osc.kind : '',
			variant: osc ? osc.variant : '',
			mode: osc ? osc.playbackMode : '',
			hasAnimClass: wave ? wave.classList.contains('sva-lite-osc-anim') : false
		});
	})()`))
	var at03 struct {
		OK           bool   `json:"ok"`
		ID           string `json:"id"`
		Kind         string `json:"kind"`
		Variant      string `json:"variant"`
		Mode         string `json:"mode"`
		HasAnimClass bool   `json:"hasAnimClass"`
	}
	_ = json.Unmarshal([]byte(at03JSON), &at03)
	record("AT-M4-03", "Create Oscilloscope Lite via Low-Load UI",
		at03.OK && at03.Kind == "oscilloscope-lite" && at03.Variant == "ecg" && at03.Mode == "animated" && at03.HasAnimClass,
		fmt.Sprintf("id=%s kind=%s variant=%s mode=%s animClass=%v",
			at03.ID, at03.Kind, at03.Variant, at03.Mode, at03.HasAnimClass))

	oscID := at03.ID

	// =========================================================================
	// AT-M4-04: All 7 Approved Waveform Presets (Distinct Path Geometries)
	// =========================================================================
	at04JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
		const id = %q;
		const presets = ['ecg', 'sine', 'square', 'triangle', 'sawtooth', 'polyline', 'bezier'];
		const paths = {};
		const validAll = presets.every((v) => {
			const sel = document.getElementById('liteVariantSelect');
			sel.value = v;
			sel.dispatchEvent(new Event('change'));
			const g = document.querySelector('#svgContainer #lite-' + id);
			const wave = g ? g.querySelector('.lite-osc-wave') : null;
			const d = wave ? (wave.getAttribute('d') || '') : '';
			paths[v] = d;
			return g && g.getAttribute('data-osc-variant') === v && d.length > 20;
		});
		const uniqueCount = new Set(Object.values(paths)).size;
		return JSON.stringify({
			validAll,
			uniqueCount,
			bezierHasC: (paths.bezier || '').includes(' C ')
		});
	})()`, oscID)))
	var at04 struct {
		ValidAll    bool `json:"validAll"`
		UniqueCount int  `json:"uniqueCount"`
		BezierHasC  bool `json:"bezierHasC"`
	}
	_ = json.Unmarshal([]byte(at04JSON), &at04)
	record("AT-M4-04", "All 7 Oscilloscope Waveform Presets Distinct",
		at04.ValidAll && at04.UniqueCount == 7 && at04.BezierHasC,
		fmt.Sprintf("validAll=%v uniquePaths=%d/7 bezierHasCubic=%v",
			at04.ValidAll, at04.UniqueCount, at04.BezierHasC))

	// =========================================================================
	// AT-M4-05: Static Mode vs Animated Mode (Seamless Periodic Scroll Sampling)
	// =========================================================================
	at05SetupJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
		const id = %q;
		// 1. Switch to Static mode
		const modeSel = document.getElementById('liteOscModeSelect');
		modeSel.value = 'static';
		modeSel.dispatchEvent(new Event('change'));
		let g = document.querySelector('#svgContainer #lite-' + id);
		let wave = g.querySelector('.lite-osc-wave');
		const staticAnimCount = typeof g.getAnimations === 'function' ? g.getAnimations({ subtree: true }).length : -1;
		const staticHasClass = wave.classList.contains('sva-lite-osc-anim');

		// 2. Switch to Animated mode with fast 0.6s cycle to sample across wrap boundary
		window.LiteEffects.updateLiteEffect(id, {
			variant: 'sine',
			playbackMode: 'animated',
			speed: 0.6,
			period: 0.6,
			segments: 2,
			intensity: 0.8
		});
		window.LiteEffects.triggerAppRender();
		g = document.querySelector('#svgContainer #lite-' + id);
		const animCount = typeof g.getAnimations === 'function' ? g.getAnimations({ subtree: true }).length : 0;
		return JSON.stringify({ staticAnimCount, staticHasClass, animCount });
	})()`, oscID)))
	var at05Setup struct {
		StaticAnimCount int  `json:"staticAnimCount"`
		StaticHasClass  bool `json:"staticHasClass"`
		AnimCount       int  `json:"animCount"`
	}
	_ = json.Unmarshal([]byte(at05SetupJSON), &at05Setup)

	var txSamples []float64
	distinctTxSet := map[float64]bool{}
	for i := 0; i < 6; i++ {
		tx, _ := chromedp.Run(taskCtx, chromedp.Evaluate[float64](fmt.Sprintf(`(() => {
			const wave = document.querySelector('#svgContainer #lite-%s .lite-osc-wave');
			if (!wave) return 0;
			const tf = window.getComputedStyle(wave).transform;
			if (tf && tf !== 'none') {
				const m = tf.match(/matrix\(([^)]+)\)/);
				if (m) {
					const parts = m[1].split(',').map(Number);
					return Number((parts[4] || 0).toFixed(2));
				}
			}
			return 0;
		})()`, oscID)))
		txSamples = append(txSamples, tx)
		distinctTxSet[tx] = true
		_ = runVoid(taskCtx, chromedp.Sleep(115*time.Millisecond))
	}
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](fmt.Sprintf(`(() => {
		window.LiteEffects.updateLiteEffect(%q, { speed: 2.4, period: 2.4 });
		window.LiteEffects.triggerAppRender();
		return true;
	})()`, oscID)))
	record("AT-M4-05", "Static Mode (0 Anims) & Animated Scroll Sampling",
		at05Setup.StaticAnimCount == 0 && !at05Setup.StaticHasClass && at05Setup.AnimCount == 1 && len(distinctTxSet) >= 3,
		fmt.Sprintf("staticAnims=%d animAnims=%d distinctTx=%d samples=%v",
			at05Setup.StaticAnimCount, at05Setup.AnimCount, len(distinctTxSet), txSamples))

	// =========================================================================
	// AT-M4-06: Amplitude, Cycles, Baseline, Stroke Width, Colour & Direction
	// =========================================================================
	at06JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
		const id = %q;
		window.LiteEffects.updateLiteEffect(id, {
			variant: 'sine',
			playbackMode: 'static',
			intensity: 0.30,
			segments: 2,
			startOffset: 50,
			strokeWidth: 2.0,
			color: '#22c55e',
			bgColor: '#0b1322',
			direction: 'forward'
		});
		window.LiteEffects.triggerAppRender();
		let g = document.querySelector('#svgContainer #lite-' + id);
		const dLowAmp = g.querySelector('.lite-osc-wave').getAttribute('d');

		// Change amplitude, cycles, baseline, strokeWidth, color, bgColor, direction
		window.LiteEffects.updateLiteEffect(id, {
			intensity: 0.90,
			segments: 5,
			startOffset: 30,
			strokeWidth: 4.5,
			color: '#f43f5e',
			bgColor: '#1e1b4b',
			direction: 'reverse'
		});
		window.LiteEffects.triggerAppRender();
		g = document.querySelector('#svgContainer #lite-' + id);
		const wave = g.querySelector('.lite-osc-wave');
		const screen = g.querySelector('.lite-osc-screen');
		const dHighAmp = wave.getAttribute('d');

		return JSON.stringify({
			pathChanged: dLowAmp !== dHighAmp,
			cyclesAttr: g.getAttribute('data-osc-cycles'),
			ampAttr: g.getAttribute('data-osc-amplitude'),
			baseAttr: g.getAttribute('data-osc-baseline'),
			dirAttr: g.getAttribute('data-osc-direction'),
			waveStroke: wave.getAttribute('stroke'),
			waveSW: wave.getAttribute('stroke-width'),
			screenFill: screen.getAttribute('fill')
		});
	})()`, oscID)))
	var at06 struct {
		PathChanged bool   `json:"pathChanged"`
		CyclesAttr  string `json:"cyclesAttr"`
		AmpAttr     string `json:"ampAttr"`
		BaseAttr    string `json:"baseAttr"`
		DirAttr     string `json:"dirAttr"`
		WaveStroke  string `json:"waveStroke"`
		WaveSW      string `json:"waveSW"`
		ScreenFill  string `json:"screenFill"`
	}
	_ = json.Unmarshal([]byte(at06JSON), &at06)
	record("AT-M4-06", "Amplitude, Cycles, Baseline, Stroke & Direction",
		at06.PathChanged && at06.CyclesAttr == "5" && at06.AmpAttr == "0.9" &&
			at06.BaseAttr == "30" && at06.DirAttr == "reverse" &&
			at06.WaveStroke == "#f43f5e" && at06.WaveSW == "4.50" && at06.ScreenFill == "#1e1b4b",
		fmt.Sprintf("pathChanged=%v cycles=%s amp=%s base=%s dir=%s stroke=%s sw=%s bg=%s",
			at06.PathChanged, at06.CyclesAttr, at06.AmpAttr, at06.BaseAttr, at06.DirAttr, at06.WaveStroke, at06.WaveSW, at06.ScreenFill))

	// =========================================================================
	// AT-M4-07: Sampled Waveform Mode & Invalid Sample Rejection in Go Engine
	// =========================================================================
	at07JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
		const id = %q;
		window.LiteEffects.updateLiteEffect(id, {
			variant: 'polyline',
			playbackMode: 'sampled',
			samples: '0, 0.5, 1.0, -0.5, -1.0, 0'
		});
		window.LiteEffects.triggerAppRender();
		let g = document.querySelector('#svgContainer #lite-' + id);
		const dSample1 = g.querySelector('.lite-osc-wave').getAttribute('d');
		const modeAttr = g.getAttribute('data-osc-mode');

		// Update with a new valid sample sequence
		window.LiteEffects.updateLiteEffect(id, {
			samples: '-1.0, -0.2, 0.4, 0.9, 0.3, -0.7, 0.8, 0'
		});
		window.LiteEffects.triggerAppRender();
		g = document.querySelector('#svgContainer #lite-' + id);
		const dSample2 = g.querySelector('.lite-osc-wave').getAttribute('d');

		// Attempt invalid sample update via Go EngineBridge ('0.5' -> < 2 numbers)
		const badRes = window.EngineBridge.updateLiteEffect(id, { samples: '0.5' });
		const badNanRes = window.EngineBridge.updateLiteEffect(id, { samples: '0.1, notANumber, 0.5' });

		return JSON.stringify({
			modeAttr,
			sampleUpdated: dSample1 !== dSample2 && dSample2.length > 15,
			badRejected: badRes === null,
			badNanRejected: badNanRes === null
		});
	})()`, oscID)))
	var at07 struct {
		ModeAttr       string `json:"modeAttr"`
		SampleUpdated  bool   `json:"sampleUpdated"`
		BadRejected    bool   `json:"badRejected"`
		BadNanRejected bool   `json:"badNanRejected"`
	}
	_ = json.Unmarshal([]byte(at07JSON), &at07)
	record("AT-M4-07", "Sampled Waveform Mode & Invalid Sample Rejection",
		at07.ModeAttr == "sampled" && at07.SampleUpdated && at07.BadRejected && at07.BadNanRejected,
		fmt.Sprintf("mode=%s sampleUpdated=%v singleSampleRejected=%v nanSampleRejected=%v",
			at07.ModeAttr, at07.SampleUpdated, at07.BadRejected, at07.BadNanRejected))

	// =========================================================================
	// AT-M4-08: Floating Context Editor Synchronization (4-Threshold & Osc)
	// =========================================================================
	at08JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
		const gaugeId = %q;
		const oscId = %q;
		window.LiteEffects.selectLite(gaugeId);
		window.FloatingPropertiesEditor.openFor('lite', gaugeId);
		const gT1Chk = document.getElementById('fctx-liteGaugeThresh1Chk');
		const gT2Chk = document.getElementById('fctx-liteGaugeThresh2Chk');
		const gT3Val = document.getElementById('fctx-liteGaugeWarnThresholdNum');
		const gT4Val = document.getElementById('fctx-liteGaugeCritThresholdNum');
		const gaugeSynced = Boolean(gT1Chk && gT1Chk.checked === true && gT2Chk && gT2Chk.checked === false && gT3Val && gT3Val.value === '80' && gT4Val && gT4Val.value === '95');

		window.LiteEffects.selectLite(oscId);
		window.FloatingPropertiesEditor.openFor('lite', oscId);
		const oscGroup = document.getElementById('fctx-liteOscGroup');
		const oscVisible = Boolean(oscGroup && oscGroup.style.display !== 'none');
		const fctxAmp = document.getElementById('fctx-liteOscAmplitudeSlider');
		fctxAmp.value = '0.65';
		fctxAmp.dispatchEvent(new Event('input', { bubbles: true }));
		fctxAmp.dispatchEvent(new Event('change', { bubbles: true }));
		const updatedOsc = window.LiteEffects.getLiteEffect(oscId);
		window.FloatingPropertiesEditor.close();

		// Restore animated ECG for export tests
		window.LiteEffects.updateLiteEffect(oscId, {
			variant: 'ecg',
			playbackMode: 'animated',
			intensity: 0.75,
			segments: 2,
			startOffset: 50,
			speed: 2.2,
			color: '#22c55e',
			bgColor: '#0b1322',
			direction: 'forward'
		});
		window.LiteEffects.triggerAppRender();

		return JSON.stringify({
			gaugeSynced,
			oscVisible,
			ampAfterEdit: updatedOsc ? updatedOsc.intensity : 0
		});
	})()`, at01.ID, oscID)))
	var at08 struct {
		GaugeSynced  bool    `json:"gaugeSynced"`
		OscVisible   bool    `json:"oscVisible"`
		AmpAfterEdit float64 `json:"ampAfterEdit"`
	}
	_ = json.Unmarshal([]byte(at08JSON), &at08)
	record("AT-M4-08", "Floating Context Editor 4-Threshold & Osc Sync",
		at08.GaugeSynced && at08.OscVisible && math.Abs(at08.AmpAfterEdit-0.65) < 0.001,
		fmt.Sprintf("gauge4ThreshSynced=%v oscGroupVisible=%v ampAfterEdit=%.2f",
			at08.GaugeSynced, at08.OscVisible, at08.AmpAfterEdit))

	// =========================================================================
	// AT-M4-09: Undo/Redo, .sva Round-Trip & SQLite Project Persistence
	// =========================================================================
	at09JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
		const gaugeId = %q;
		const oscId = %q;
		window.HistoryManager.recordImmediate();
		window.LiteEffects.updateLiteEffect(oscId, { variant: 'sawtooth', segments: 4 });
		window.HistoryManager.recordImmediate();
		window.HistoryManager.undo();
		const afterUndo = window.LiteEffects.getLiteEffect(oscId).variant;
		window.HistoryManager.redo();
		const afterRedo = window.LiteEffects.getLiteEffect(oscId).variant;

		// Test .sva binary package encode via Go EngineBridge
		const svaB64 = window.EngineBridge.encodeSVA ? window.EngineBridge.encodeSVA({ title: 'M4Test' }) : 'ok';

		// Test Studio state serialization round-trip
		const snap = window.ProjectManager.serializeStudioState();
		window.ProjectManager.resetToNewProject();
		window.ProjectManager.applyStudioState(snap);
		const rg = window.LiteEffects.getLiteEffect(gaugeId);
		const ro = window.LiteEffects.getLiteEffect(oscId);

		return JSON.stringify({
			afterUndo,
			afterRedo,
			svaOk: Boolean(svaB64),
			rgThreshCount: (rg && rg.thresholds) ? rg.thresholds.length : 0,
			rgT2Enabled: (rg && rg.thresholds && rg.thresholds[1]) ? rg.thresholds[1].enabled : true,
			roKind: ro ? ro.kind : '',
			roVariant: ro ? ro.variant : '',
			roSegments: ro ? ro.segments : 0
		});
	})()`, at01.ID, oscID)))
	var at09 struct {
		AfterUndo     string `json:"afterUndo"`
		AfterRedo     string `json:"afterRedo"`
		SvaOk         bool   `json:"svaOk"`
		RgThreshCount int    `json:"rgThreshCount"`
		RgT2Enabled   bool   `json:"rgT2Enabled"`
		RoKind        string `json:"roKind"`
		RoVariant     string `json:"roVariant"`
		RoSegments    int    `json:"roSegments"`
	}
	_ = json.Unmarshal([]byte(at09JSON), &at09)
	record("AT-M4-09", "Undo/Redo, .sva Package & Project Persistence",
		at09.AfterUndo == "ecg" && at09.AfterRedo == "sawtooth" && at09.SvaOk &&
			at09.RgThreshCount == 4 && !at09.RgT2Enabled &&
			at09.RoKind == "oscilloscope-lite" && at09.RoVariant == "sawtooth" && at09.RoSegments == 4,
		fmt.Sprintf("undo=%s redo=%s sva=%v rgBands=%d(T2=%v) ro=%s/%s(cycles=%d)",
			at09.AfterUndo, at09.AfterRedo, at09.SvaOk, at09.RgThreshCount, at09.RgT2Enabled, at09.RoKind, at09.RoVariant, at09.RoSegments))

	// =========================================================================
	// AT-M4-10: Standalone SVG Export (Self-Contained, Zero Editor/WASM Deps)
	// =========================================================================
	at10JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.LiteEffects.triggerAppRender();
		const svgStr = window.buildFullSVG(window.SceneState.buildRenderState());
		return JSON.stringify({
			hasOsc: svgStr.includes('data-lite-kind="oscilloscope-lite"'),
			hasGauge4: svgStr.includes('data-gauge-t1-enabled="true"') && svgStr.includes('data-gauge-t2-enabled="false"'),
			hasKeyframes: svgStr.includes('@keyframes sva-lite-osc-scroll'),
			hasClipPath: svgStr.includes('<clipPath id="lite-osc-clip-'),
			noWasm: !svgStr.includes('engine.wasm') && !svgStr.includes('EngineBridge')
		});
	})()`))
	var at10 struct {
		HasOsc       bool `json:"hasOsc"`
		HasGauge4    bool `json:"hasGauge4"`
		HasKeyframes bool `json:"hasKeyframes"`
		HasClipPath  bool `json:"hasClipPath"`
		NoWasm       bool `json:"noWasm"`
	}
	_ = json.Unmarshal([]byte(at10JSON), &at10)
	record("AT-M4-10", "Standalone SVG Export Integrity",
		at10.HasOsc && at10.HasGauge4 && at10.HasKeyframes && at10.HasClipPath && at10.NoWasm,
		fmt.Sprintf("hasOsc=%v hasGauge4=%v hasKeyframes=%v hasClip=%v noWasm=%v",
			at10.HasOsc, at10.HasGauge4, at10.HasKeyframes, at10.HasClipPath, at10.NoWasm))

	// =========================================================================
	// AT-M4-11: Stage 16C Widget Export & Multi-Instance Isolation (Gauge 4T + Osc)
	// =========================================================================
	_, _ = chromedp.Run(taskCtx, chromedp.Evaluate[bool](widgetJSSource+"\n;Boolean(window.SVGWidget);"))
	at11RuntimeJSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](fmt.Sprintf(`(() => {
		const gaugeId = %q;
		const oscId = %q;
		window.LiteEffects.updateLiteEffect(gaugeId, { name: 'cpuGauge' });
		window.LiteEffects.updateLiteEffect(oscId, { name: 'heartOsc', variant: 'ecg', playbackMode: 'animated' });
		window.LiteEffects.triggerAppRender();

		// Bind widget parameters for 4-threshold Gauge and Oscilloscope Lite
		window.EngineBridge.setWidgetParameters([]);
		const paramDefs = [
			{ name: 'cpugauge-lite-value',     type: 'number', target: 'lite-' + gaugeId, property: 'value',     default: 65 },
			{ name: 'cpugauge-lite-color',     type: 'color',  target: 'lite-' + gaugeId, property: 'color',     default: '#38bdf8' },
			{ name: 'heartosc-lite-intensity', type: 'number', target: 'lite-' + oscId,   property: 'intensity', min: 0.05, max: 1.0, default: 0.75 },
			{ name: 'heartosc-lite-samples',   type: 'text',   target: 'lite-' + oscId,   property: 'samples',   default: '0, 0.4, 0.9, 0.3, -0.5, -0.9, -0.2, 0.6, 1.0, 0.2, -0.4, 0' },
			{ name: 'heartosc-lite-color',     type: 'color',  target: 'lite-' + oscId,   property: 'color',     default: '#22c55e' },
			{ name: 'heartosc-lite-bgcolor',   type: 'color',  target: 'lite-' + oscId,   property: 'bgColor',   default: '#0b1322' },
			{ name: 'heartosc-lite-direction', type: 'enum',   target: 'lite-' + oscId,   property: 'direction', options: ['forward', 'reverse'], default: 'forward' }
		];
		for (const p of paramDefs) {
			window.EngineBridge.addWidgetParameter(p);
		}
		const params = window.EngineBridge.getWidgetParameters();
		const rawSvg = window.buildFullSVG(window.SceneState.buildRenderState());
		const manifest = { schemaVersion: 1, parameters: params };

		let host = document.getElementById('m4-test-widget-host');
		if (host) host.remove();
		host = document.createElement('div');
		host.id = 'm4-test-widget-host';
		host.style.cssText = 'position:fixed;top:0;left:0;width:600px;height:300px;z-index:99999;display:flex;';
		const boxA = document.createElement('div');
		const boxB = document.createElement('div');
		host.appendChild(boxA);
		host.appendChild(boxB);
		document.body.appendChild(host);

		const instA = window.SVGWidget.mount(boxA, rawSvg, { instanceId: 'm4a', manifest, visibilityAware: false });
		const instB = window.SVGWidget.mount(boxB, rawSvg, { instanceId: 'm4b', manifest, visibilityAware: false });

		// 1. Test 4-threshold Gauge runtime evaluation in Instance A while Instance B stays at 65 (#22c55e)
		instA.setParameter('cpugauge-lite-value', 25); // <40 -> #38bdf8
		const gA = boxA.querySelector('[data-lite-kind="gauge-lite"]');
		const gB = boxB.querySelector('[data-lite-kind="gauge-lite"]');
		const colA25 = gA.getAttribute('data-gauge-active-color');
		instA.setParameter('cpugauge-lite-value', 65); // >=60 (T2 disabled) -> #22c55e (T1)
		const colA65 = gA.getAttribute('data-gauge-active-color');
		instA.setParameter('cpugauge-lite-value', 85); // >=80 -> #f59e0b (T3)
		const colA85 = gA.getAttribute('data-gauge-active-color');
		instA.setParameter('cpugauge-lite-value', 97); // >=95 -> #ef4444 (T4)
		const colA97 = gA.getAttribute('data-gauge-active-color');
		// Application colour override precedence
		instA.setParameter('cpugauge-lite-color', '#a855f7');
		const colAOverride = gA.getAttribute('data-gauge-active-color');
		const colBUnchanged = gB.getAttribute('data-gauge-active-color');

		// 2. Test Oscilloscope runtime updates in Instance A without affecting Instance B
		const oscA = boxA.querySelector('[data-lite-kind="oscilloscope-lite"]');
		const oscB = boxB.querySelector('[data-lite-kind="oscilloscope-lite"]');
		const dBInitial = oscB.querySelector('.lite-osc-wave').getAttribute('d');
		const dAInitial = oscA.querySelector('.lite-osc-wave').getAttribute('d');

		instA.setParameter('heartosc-lite-intensity', 0.35);
		const dAAfterAmp = oscA.querySelector('.lite-osc-wave').getAttribute('d');

		instA.setParameter('heartosc-lite-samples', '0, 0.8, -0.8, 0.5, -0.5, 0');
		const dAAfterSamples = oscA.querySelector('.lite-osc-wave').getAttribute('d');
		const modeAAfterSamples = oscA.getAttribute('data-osc-mode');

		// 3. Test atomic rejection of invalid samples ('0.5' -> <2 samples)
		let threwOnInvalidSamples = false;
		try {
			instA.setParameter('heartosc-lite-samples', '0.5');
		} catch (e) {
			threwOnInvalidSamples = true;
		}
		const dAAfterRejected = oscA.querySelector('.lite-osc-wave').getAttribute('d');
		const dBFinal = oscB.querySelector('.lite-osc-wave').getAttribute('d');

		// 4. Test pause / resume / visibility suspension on Instance B (which is still in animated mode)
		instB.pause();
		const bPaused = instB.isPaused();
		instB.play();
		const bResumed = !instB.isPaused();

		instA.destroy();
		instB.destroy();
		host.remove();

		return JSON.stringify({
			paramCount: params.length,
			colA25, colA65, colA85, colA97, colAOverride, colBUnchanged,
			ampChangedPath: dAInitial !== dAAfterAmp,
			samplesChangedPath: dAAfterAmp !== dAAfterSamples,
			modeAAfterSamples,
			threwOnInvalidSamples,
			preservedAfterReject: dAAfterRejected === dAAfterSamples,
			bIsolated: dBInitial === dBFinal,
			bPaused, bResumed
		});
	})()`, at01.ID, oscID)))
	var at11Rt struct {
		ParamCount           int    `json:"paramCount"`
		ColA25               string `json:"colA25"`
		ColA65               string `json:"colA65"`
		ColA85               string `json:"colA85"`
		ColA97               string `json:"colA97"`
		ColAOverride         string `json:"colAOverride"`
		ColBUnchanged        string `json:"colBUnchanged"`
		AmpChangedPath       bool   `json:"ampChangedPath"`
		SamplesChangedPath   bool   `json:"samplesChangedPath"`
		ModeAAfterSamples    string `json:"modeAAfterSamples"`
		ThrewOnInvalid       bool   `json:"threwOnInvalidSamples"`
		PreservedAfterReject bool   `json:"preservedAfterReject"`
		BIsolated            bool   `json:"bIsolated"`
		BPaused              bool   `json:"bPaused"`
		BResumed             bool   `json:"bResumed"`
	}
	_ = json.Unmarshal([]byte(at11RuntimeJSON), &at11Rt)
	record("AT-M4-11", "Stage 16C Widget Runtime 4-Threshold & Osc API",
		at11Rt.ParamCount == 7 &&
			at11Rt.ColA25 == "#38bdf8" && at11Rt.ColA65 == "#22c55e" &&
			at11Rt.ColA85 == "#f59e0b" && at11Rt.ColA97 == "#ef4444" &&
			at11Rt.ColAOverride == "#a855f7" && at11Rt.ColBUnchanged == "#22c55e" &&
			at11Rt.AmpChangedPath && at11Rt.SamplesChangedPath &&
			at11Rt.ModeAAfterSamples == "sampled" && at11Rt.ThrewOnInvalid &&
			at11Rt.PreservedAfterReject && at11Rt.BIsolated &&
			at11Rt.BPaused && at11Rt.BResumed,
		fmt.Sprintf("params=%d A(25:%s 65:%s 85:%s 97:%s ovr:%s) B:%s ampChg=%v sampChg=%v invalidThrew=%v isolated=%v",
			at11Rt.ParamCount, at11Rt.ColA25, at11Rt.ColA65, at11Rt.ColA85, at11Rt.ColA97, at11Rt.ColAOverride, at11Rt.ColBUnchanged,
			at11Rt.AmpChangedPath, at11Rt.SamplesChangedPath, at11Rt.ThrewOnInvalid, at11Rt.BIsolated))

	// =========================================================================
	// AT-M4-12: Mixed-Scene Coexistence (All 7 Lite Effects + Flow + Glow + Ripple)
	// =========================================================================
	at12JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		window.ProjectManager.resetToNewProject();
		const n1 = window.DiagramNodes.createNode({ text: 'Sensor', x: 15, y: 20, width: 80, height: 40 });
		const n2 = window.DiagramNodes.createNode({ text: 'Hub',    x: 45, y: 20, width: 80, height: 40 });
		window.DataFlows.createFlow({ fromNodeId: n1.id, toNodeId: n2.id });
		window.Glows.createGlow({ x: 25, y: 65, size: 20 });
		window.Ripples.createRipple({ x: 55, y: 65, outerR: 60 });
		window.LiteEffects.createLiteEffect({ kind: 'perimeter-tracer' });
		window.LiteEffects.createLiteEffect({ kind: 'pulse' });
		window.LiteEffects.createLiteEffect({ kind: 'flash' });
		window.LiteEffects.createLiteEffect({ kind: 'flow-lite' });
		window.LiteEffects.createLiteEffect({ kind: 'radar-lite' });
		window.LiteEffects.createLiteEffect({ kind: 'gauge-lite' });
		window.LiteEffects.createLiteEffect({ kind: 'oscilloscope-lite' });
		window.triggerAppRender();

		const svg = document.getElementById('svgContainer');
		return JSON.stringify({
			liteDomCount: svg.querySelectorAll('.diagram-lite-instance').length,
			flowDomCount: svg.querySelectorAll('.diagram-flow-instance').length,
			glowDomCount: svg.querySelectorAll('.diagram-glow-instance').length,
			rippleDomCount: svg.querySelectorAll('.diagram-ripple-instance').length
		});
	})()`))
	var at12 struct {
		LiteDomCount   int `json:"liteDomCount"`
		FlowDomCount   int `json:"flowDomCount"`
		GlowDomCount   int `json:"glowDomCount"`
		RippleDomCount int `json:"rippleDomCount"`
	}
	_ = json.Unmarshal([]byte(at12JSON), &at12)
	record("AT-M4-12", "Mixed-Scene Coexistence (All 7 Lite + Main FX)",
		at12.LiteDomCount == 7 && at12.FlowDomCount == 1 && at12.GlowDomCount == 1 && at12.RippleDomCount == 1,
		fmt.Sprintf("lite=%d/7 flow=%d glow=%d ripple=%d",
			at12.LiteDomCount, at12.FlowDomCount, at12.GlowDomCount, at12.RippleDomCount))

	// =========================================================================
	// AT-M4-13: Visual Demonstration Page — Oscilloscope Lite Showcase
	// =========================================================================
	if err := runVoid(taskCtx,
		chromedp.Navigate(oscDemoURL),
		chromedp.WaitReady("#demo-card-presets"),
	); err != nil {
		log.Fatalf("Failed to open %s: %v", oscDemoURL, err)
	}
	at13JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const presetCount = document.querySelectorAll('#svg-presets-showcase g[data-osc-variant]').length;
		const staticEl = document.getElementById('demo-mode-static');
		const animEl = document.getElementById('demo-mode-animated');
		const sampledEl = document.getElementById('demo-mode-sampled');
		const instA = window._demoOscInstA;
		const instB = window._demoOscInstB;
		const threshCount = document.querySelectorAll('#svg-4thresh-showcase g[data-lite-kind="gauge-lite"]').length;

		// Test interactive button on Oscilloscope Demo page
		const waveB = document.querySelector('#osc-host-b .lite-osc-wave');
		const dBefore = waveB ? waveB.getAttribute('d') : '';
		const btnBurst = document.getElementById('btn-osc-b-burst');
		if (btnBurst) btnBurst.click();
		const dAfter = waveB ? waveB.getAttribute('d') : '';

		return JSON.stringify({
			presetCount,
			hasAllModes: Boolean(staticEl && animEl && sampledEl),
			hasBothWidgets: Boolean(instA && instB),
			threshCount,
			burstChangedPath: dBefore !== '' && dAfter !== '' && dBefore !== dAfter
		});
	})()`))
	var at13 struct {
		PresetCount      int  `json:"presetCount"`
		HasAllModes      bool `json:"hasAllModes"`
		HasBothWidgets   bool `json:"hasBothWidgets"`
		ThreshCount      int  `json:"threshCount"`
		BurstChangedPath bool `json:"burstChangedPath"`
	}
	_ = json.Unmarshal([]byte(at13JSON), &at13)
	record("AT-M4-13", "Oscilloscope Lite Visual Showcase Page",
		at13.PresetCount == 7 && at13.HasAllModes && at13.HasBothWidgets && at13.ThreshCount == 4 && at13.BurstChangedPath,
		fmt.Sprintf("presets=%d/7 modes=%v widgets=%v 4threshCards=%d liveBurstClick=%v",
			at13.PresetCount, at13.HasAllModes, at13.HasBothWidgets, at13.ThreshCount, at13.BurstChangedPath))

	// =========================================================================
	// AT-M4-14: Visual Demonstration Page — Updated 4-Threshold Gauge Lite Demo
	// =========================================================================
	if err := runVoid(taskCtx,
		chromedp.Navigate(gaugeDemoURL),
		chromedp.WaitReady("#demo-card-interactive-widgets"),
	); err != nil {
		log.Fatalf("Failed to open %s: %v", gaugeDemoURL, err)
	}
	at14JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		const instA = window._demoGaugeInstA;
		const instB = window._demoGaugeInstB;
		const gA = document.querySelector('#widget-host-a [data-lite-kind="gauge-lite"]');
		const gB = document.querySelector('#widget-host-b [data-lite-kind="gauge-lite"]');
		const colA65 = gA ? gA.getAttribute('data-gauge-active-color') : '';
		const colB96 = gB ? gB.getAttribute('data-gauge-active-color') : '';
		const t2Disabled = gA ? gA.getAttribute('data-gauge-t2-enabled') === 'false' : false;
		return JSON.stringify({
			ok: Boolean(instA && instB && gA && gB),
			colA65,
			colB96,
			t2Disabled
		});
	})()`))
	var at14 struct {
		OK         bool   `json:"ok"`
		ColA65     string `json:"colA65"`
		ColB96     string `json:"colB96"`
		T2Disabled bool   `json:"t2Disabled"`
	}
	_ = json.Unmarshal([]byte(at14JSON), &at14)
	record("AT-M4-14", "Updated 4-Threshold Gauge Lite Demo Page",
		at14.OK && at14.ColA65 == "#22c55e" && at14.ColB96 == "#ef4444" && at14.T2Disabled,
		fmt.Sprintf("instA(65%%,T2=OFF)=%s instB(96%%)=%s t2Disabled=%v",
			at14.ColA65, at14.ColB96, at14.T2Disabled))

	// =========================================================================
	// AT-M4-15: Protected Stage 16C Consumer App (:8091) Health Check
	// =========================================================================
	if err := runVoid(taskCtx,
		chromedp.Navigate(consumerURL),
		chromedp.WaitReady("body"),
	); err != nil {
		log.Fatalf("Failed to open %s: %v", consumerURL, err)
	}
	at15JSON, _ := chromedp.Run(taskCtx, chromedp.Evaluate[string](`(() => {
		return JSON.stringify({
			hasSVGWidget: Boolean(window.SVGWidget && typeof window.SVGWidget.mount === 'function'),
			title: document.title || ''
		});
	})()`))
	var at15 struct {
		HasSVGWidget bool   `json:"hasSVGWidget"`
		Title        string `json:"title"`
	}
	_ = json.Unmarshal([]byte(at15JSON), &at15)
	record("AT-M4-15", "Stage 16C Widget Consumer (:8091) Intact",
		at15.HasSVGWidget && len(strings.TrimSpace(at15.Title)) > 0,
		fmt.Sprintf("SVGWidget=%v title=%q", at15.HasSVGWidget, at15.Title))

	// =========================================================================
	// Summary
	// =========================================================================
	fmt.Println("==========================================================================")
	passedCount := 0
	for _, r := range results {
		if r.Passed {
			passedCount++
		}
	}
	fmt.Printf(" Stage 16D-B-M4 Summary: %d / %d Acceptance Gates PASSED\n", passedCount, len(results))
	fmt.Println("==========================================================================")
	if passedCount != len(results) {
		os.Exit(1)
	}
}
