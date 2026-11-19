package animlab

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

func loadBackgroundDataURI(configDir, file string) (string, int, error) {
	if file == "" {
		return "", 0, nil
	}
	p := resolveAssetPath(configDir, file)
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", 0, err
	}
	ext := strings.ToLower(filepath.Ext(p))
	mime := "image/png"
	switch ext {
	case ".webp":
		mime = "image/webp"
	case ".jpg", ".jpeg":
		mime = "image/jpeg"
	case ".svg":
		mime = "image/svg+xml"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), len(raw), nil
}

// buildDTSceneHTML builds a Shared Digital Twin scene: one static raster <img> background
// plus one shared <svg> containing N independently positioned <g class="lab-instance"> overlays.
func buildDTSceneHTML(spec TestRunSpec) string {
	reg := DefaultRegistry()
	sceneW, sceneH := 1280, 720
	if spec.Scene != nil {
		if spec.Scene.Width > 0 {
			sceneW = spec.Scene.Width
		}
		if spec.Scene.Height > 0 {
			sceneH = spec.Scene.Height
		}
	}

	bgTag := ""
	bgInfo := "none"
	if spec.Scene != nil && spec.Scene.Background != nil && spec.Scene.Background.File != "" {
		fit := spec.Scene.Background.Fit
		if fit == "" {
			fit = "contain"
		}
		if uri, sizeBytes, err := loadBackgroundDataURI(spec.ConfigDir, spec.Scene.Background.File); err == nil && uri != "" {
			bgTag = fmt.Sprintf(`<img id="labDtBg" class="lab-dt-bg" src="%s" alt="Digital Twin static background" style="position:absolute;inset:0;width:100%%;height:100%%;object-fit:%s;pointer-events:none;user-select:none;" />`, uri, fit)
			bgInfo = fmt.Sprintf("%s (%d bytes, fit=%s)", filepath.Base(spec.Scene.Background.File), sizeBytes, fit)
		}
	}

	// Compute deterministic window-bay overlay coordinates across the building elevation
	n := len(spec.Instances)
	cols := 10
	if n > 100 {
		cols = int(math.Ceil(math.Sqrt(float64(n) * 1.5)))
	}
	if cols < 1 {
		cols = 1
	}
	rows := (n + cols - 1) / cols
	if rows < 1 {
		rows = 1
	}

	// Building elevation interior bounds (matches 1280x720 dt-building-bg.png window bays)
	padX, padY := 80, 60
	availW := sceneW - padX*2
	availH := sceneH - padY - 40
	if availW < 200 {
		padX = 10
		availW = sceneW - 20
	}
	if availH < 150 {
		padY = 10
		availH = sceneH - 20
	}
	cellW := availW / cols
	cellH := availH / rows
	if cellW < 24 {
		cellW = 24
	}
	if cellH < 20 {
		cellH = 20
	}

	var cssBuf, jsBuf, inner strings.Builder
	for i, inst := range spec.Instances {
		gen := reg[inst.Family]
		if gen == nil {
			continue
		}
		// Scale overlay geometry cleanly inside its window bay without clipping
		adj := inst
		if adj.Width > cellW-8 {
			adj.Width = cellW - 8
		}
		if adj.Height > cellH-8 {
			adj.Height = cellH - 8
		}
		if adj.Width < 14 {
			adj.Width = 14
		}
		if adj.Height < 12 {
			adj.Height = 12
		}
		re := gen.Render(adj, cellW, cellH)
		if re.CSS != "" {
			cssBuf.WriteString(re.CSS + "\n")
		}
		if re.JSDriver != "" {
			jsBuf.WriteString(re.JSDriver + "\n")
		}
		x := padX + (i%cols)*cellW
		y := padY + (i/cols)*cellH
		if spec.Scene != nil && i < len(spec.Scene.Positions) {
			x = spec.Scene.Positions[i][0]
			y = spec.Scene.Positions[i][1]
		}
		inner.WriteString(fmt.Sprintf(`<g id="%s" class="lab-instance" data-lab-idx="%d" transform="translate(%d,%d)">%s</g>`,
			inst.ID, i, x, y, re.SVGInner))
	}

	visHeightCSS := ""
	if spec.WorkloadVisibility == "offscreen" {
		visHeightCSS = "#labGrid { max-height: 480px; overflow-y: auto; }"
	} else if spec.VisibleCount > 0 {
		rowsVis := (spec.VisibleCount + cols - 1) / cols
		if rowsVis < 1 {
			rowsVis = 1
		}
		visHeightCSS = fmt.Sprintf("#labGrid { max-height: %dpx; overflow-y: auto; }", padY+rowsVis*cellH)
	}
	offscreenSpacer := ""
	if spec.WorkloadVisibility == "offscreen" {
		offscreenSpacer = `<div class="lab-offscreen-spacer" style="height:2200px;"></div>`
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>SVG Animation Test Lab (Shared DT Scene) — %s</title>
<style>
  body { margin: 0; padding: 12px; background: #0d1117; color: #c9d1d9; font-family: system-ui, sans-serif; }
  .lab-dt-stage { position: relative; width: %dpx; height: %dpx; background: #0d1117; border: 1px solid #30363d; border-radius: 6px; overflow: hidden; }
  .lab-instance.lab-suspended, .lab-instance.lab-suspended * { animation-play-state: paused !important; }
  %s
  %s
</style>
</head>
<body>
<div id="labHeader" style="font-size:12px;margin-bottom:8px;color:#8b949e;">Run: %s | Arrangement: shared-dt-scene | Overlays: %d | Background: %s | Workload: %s</div>
<div id="labGrid">%s<div class="lab-dt-stage">%s<svg class="lab-shared-svg lab-dt-overlay" xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" style="position:absolute;inset:0;width:100%%;height:100%%;">%s</svg></div></div>
<script>
window.__labRafDrivers = [];
window.__labDataDrivers = [];
window.__labLongTasks = [];
window.__labSuspendedSet = new Set();
window.__labSuspendedCount = 0;
window.__labVisibleCount = %d;
window.__labArrangement = 'shared-dt-scene';
if (typeof PerformanceObserver !== 'undefined') {
  try {
    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) {
        window.__labLongTasks.push({ start: e.startTime, dur: e.duration });
      }
    }).observe({ entryTypes: ['longtask'] });
  } catch (_) {}
}
%s
(function() {
  const suspendOffscreen = %v;
  const totalCount = %d;
  const suspendedSet = window.__labSuspendedSet;
  const rootEl = document.getElementById('labGrid');
  if (!suspendOffscreen) return;
  const updateSharedVisibility = () => {
    const rr = rootEl ? rootEl.getBoundingClientRect() : { top: 0, left: 0, bottom: window.innerHeight, right: window.innerWidth };
    const groups = document.querySelectorAll('.lab-shared-svg .lab-instance');
    groups.forEach(g => {
      const r = g.getBoundingClientRect();
      const visible = (r.bottom > rr.top + 1) && (r.top < rr.bottom - 1) && (r.right > rr.left + 1) && (r.left < rr.right - 1);
      const id = g.id || '';
      if (visible) {
        g.classList.remove('lab-suspended');
        suspendedSet.delete(id);
      } else {
        g.classList.add('lab-suspended');
        suspendedSet.add(id);
      }
    });
    window.__labSuspendedCount = suspendedSet.size;
    window.__labVisibleCount = totalCount - suspendedSet.size;
  };
  updateSharedVisibility();
  if (rootEl) rootEl.addEventListener('scroll', updateSharedVisibility, { passive: true });
  window.addEventListener('scroll', updateSharedVisibility, { passive: true });
  if (typeof ResizeObserver !== 'undefined' && rootEl) {
    new ResizeObserver(updateSharedVisibility).observe(rootEl);
  }
  window.__labRefreshVisibility = updateSharedVisibility;
})();
%s
</script>
</body>
</html>`, spec.RunID, sceneW, sceneH, visHeightCSS, cssBuf.String(), spec.RunID, spec.Count, bgInfo, spec.WorkloadName, offscreenSpacer, bgTag, sceneW, sceneH, sceneW, sceneH, inner.String(), spec.Count, jsBuf.String(), spec.SuspendOffscreen, spec.Count, labSharedRuntimeJS)
}

// buildStage16CWidgetsHTML builds a Sentinel-PVE-style dashboard mounting N independent instances
// of a genuine Stage 16C exported widget package using its unmodified public js/svg-widget.js runtime.
func buildStage16CWidgetsHTML(spec TestRunSpec) string {
	if spec.WidgetRuntime == nil {
		return "<!DOCTYPE html><html><body>Missing widget_runtime configuration</body></html>"
	}
	pkgDir := resolveAssetPath(spec.ConfigDir, spec.WidgetRuntime.PackageDir)
	pkg, err := LoadStage16CPackage(pkgDir)
	if err != nil {
		return fmt.Sprintf("<!DOCTYPE html><html><body>Stage 16C package error: %s</body></html>", err.Error())
	}

	var validProps []WidgetPropertyUpdate
	intervalMs := 1000
	updatesEnabled := false
	if spec.WidgetRuntime.Updates != nil && spec.WidgetRuntime.Updates.Enabled {
		updatesEnabled = true
		if spec.WidgetRuntime.Updates.IntervalMs > 0 {
			intervalMs = spec.WidgetRuntime.Updates.IntervalMs
		}
		validProps, _, _ = ValidateWidgetProperties(pkg, spec.WidgetRuntime.Updates.Properties)
	}

	rawSvgJSON, _ := json.Marshal(pkg.SVGText)
	propsJSON, _ := json.Marshal(validProps)
	mfJSON, _ := json.Marshal(pkg.Manifest)

	var cards strings.Builder
	if spec.WorkloadVisibility == "offscreen" {
		cards.WriteString(`<div class="lab-offscreen-spacer" style="height:2200px;grid-column:1/-1;"></div>`)
	}
	for i := 0; i < spec.Count; i++ {
		cards.WriteString(fmt.Sprintf(
			`<div class="lab-widget-card" id="wcard_%d" data-lab-idx="%d"><div class="lab-meta">#%d %s (v%s)</div><div id="wmount_%d" class="lab-widget-mount"></div></div>`,
			i, i, i+1, pkg.Manifest.WidgetName, pkg.Manifest.PackageVersion, i))
	}

	visHeightCSS := ""
	if spec.WorkloadVisibility == "offscreen" {
		visHeightCSS = "#labGrid { max-height: 480px; overflow-y: auto; }"
	} else if spec.VisibleCount > 0 {
		rowsVis := (spec.VisibleCount + 3) / 4
		if rowsVis < 1 {
			rowsVis = 1
		}
		visHeightCSS = fmt.Sprintf("#labGrid { max-height: %dpx; overflow-y: auto; }", rowsVis*195)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>SVG Animation Test Lab (Stage 16C Widgets) — %s</title>
<style>
  body { margin: 0; padding: 12px; background: #0d1117; color: #c9d1d9; font-family: system-ui, sans-serif; }
  #labGrid { display: grid; grid-template-columns: repeat(4, 240px); gap: 10px; }
  .lab-widget-card { background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 6px; }
  .lab-meta { font-size: 11px; color: #8b949e; margin-bottom: 4px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .lab-widget-mount { width: 226px; height: 150px; overflow: hidden; }
  .lab-widget-mount svg { width: 100%%; height: 100%%; display: block; }
  %s
</style>
</head>
<body>
<div id="labHeader" style="font-size:12px;margin-bottom:8px;color:#8b949e;">Run: %s | Arrangement: stage16c-widgets | Package: %s (%s) | Instances: %d | Workload: %s</div>
<div id="labGrid">%s</div>
<script>
%s
</script>
<script>
window.__labArrangement = 'stage16c-widgets';
window.__labLongTasks = [];
window.__labSuspendedSet = new Set();
window.__labSuspendedCount = 0;
window.__labVisibleCount = %d;
window.__labWidgetManifest = %s;
window.__labWidgetInstances = [];
window.__labWidgetUpdateTimer = null;

if (typeof PerformanceObserver !== 'undefined') {
  try {
    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) {
        window.__labLongTasks.push({ start: e.startTime, dur: e.duration });
      }
    }).observe({ entryTypes: ['longtask'] });
  } catch (_) {}
}

(function() {
  const rawSvg = %s;
  const count = %d;
  const suspendOffscreen = %v;
  const updatesEnabled = %v;
  const intervalMs = %d;
  const updateProps = %s;
  const suspendedSet = window.__labSuspendedSet;

  for (let i = 0; i < count; i++) {
    const mountEl = document.getElementById('wmount_' + i);
    const inst = window.SVGWidget.mount(mountEl, rawSvg, {
      idPrefix: 's16c_' + i,
      useShadowDOM: false
    });
    window.__labWidgetInstances.push(inst);
  }

  window.__labSetWidgetParam = (idx, name, val) => {
    const inst = window.__labWidgetInstances[idx];
    if (!inst) return false;
    inst.setParameter(name, val);
    return true;
  };
  window.__labGetWidgetParam = (idx, name) => {
    const inst = window.__labWidgetInstances[idx];
    return inst ? inst.getParameter(name) : undefined;
  };
  window.__labDisposeWidgets = () => {
    if (window.__labWidgetUpdateTimer) {
      clearInterval(window.__labWidgetUpdateTimer);
      window.__labWidgetUpdateTimer = null;
    }
    while (window.__labWidgetInstances.length > 0) {
      const inst = window.__labWidgetInstances.pop();
      try { if (inst && typeof inst.destroy === 'function') inst.destroy(); } catch (_) {}
    }
    return true;
  };

  if (suspendOffscreen && typeof IntersectionObserver !== 'undefined') {
    const rootEl = document.getElementById('labGrid');
    const io = new IntersectionObserver((entries) => {
      for (const ent of entries) {
        const idx = parseInt(ent.target.getAttribute('data-lab-idx'), 10);
        const inst = window.__labWidgetInstances[idx];
        const key = 'wcard_' + idx;
        if (ent.isIntersecting) {
          ent.target.classList.remove('lab-suspended');
          suspendedSet.delete(key);
          if (inst && typeof inst.resume === 'function') inst.resume();
        } else {
          ent.target.classList.add('lab-suspended');
          suspendedSet.add(key);
          if (inst && typeof inst.pause === 'function') inst.pause();
        }
      }
      window.__labSuspendedCount = suspendedSet.size;
      window.__labVisibleCount = count - suspendedSet.size;
    }, { root: rootEl, threshold: 0.01 });
    document.querySelectorAll('.lab-widget-card').forEach(c => io.observe(c));
  }

  if (updatesEnabled && updateProps && updateProps.length > 0) {
    let step = 0;
    const applyTick = () => {
      step++;
      for (let i = 0; i < window.__labWidgetInstances.length; i++) {
        const inst = window.__labWidgetInstances[i];
        if (!inst) continue;
        for (let p = 0; p < updateProps.length; p++) {
          const prop = updateProps[p];
          if (!prop.values || prop.values.length === 0) continue;
          const val = prop.values[(step + i) %% prop.values.length];
          try { inst.setParameter(prop.name, val); } catch (_) {}
        }
      }
    };
    applyTick();
    window.__labWidgetUpdateTimer = setInterval(applyTick, intervalMs);
  }
})();
</script>
</body>
</html>`, spec.RunID, visHeightCSS, spec.RunID, pkg.Manifest.WidgetName, pkg.Manifest.PackageID, spec.Count, spec.WorkloadName, cards.String(), pkg.RuntimeJS, spec.Count, string(mfJSON), string(rawSvgJSON), spec.Count, spec.SuspendOffscreen, updatesEnabled, intervalMs, string(propsJSON))
}
