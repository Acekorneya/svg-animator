package animlab

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ServerOptions configures optional sibling directories for the Test Lab results viewer.
type ServerOptions struct {
	DemosDir   string
	ConfigsDir string
}

// NewResultsServer returns an http.Handler serving the Test Lab landing page,
// visual sample catalogues, HTML/Markdown/decision reports, resolved YAMLs, and M4 demonstrations.
func NewResultsServer(rootDir string) http.Handler {
	parent := filepath.Dir(filepath.Clean(rootDir))
	opts := ServerOptions{}
	for _, cand := range []string{filepath.Join(parent, "m4-evidence"), filepath.Join(parent, "demos")} {
		if info, err := os.Stat(cand); err == nil && info.IsDir() {
			opts.DemosDir = cand
			break
		}
	}
	if cfgCand := filepath.Join(parent, "configs"); true {
		if info, err := os.Stat(cfgCand); err == nil && info.IsDir() {
			opts.ConfigsDir = cfgCand
		}
	}
	return NewResultsServerWithOptions(rootDir, opts)
}

// NewResultsServerWithOptions constructs the Test Lab HTTP viewer with explicit roots.
func NewResultsServerWithOptions(rootDir string, opts ServerOptions) http.Handler {
	absRoot, _ := filepath.Abs(rootDir)
	absDemos, _ := filepath.Abs(opts.DemosDir)
	absConfigs, _ := filepath.Abs(opts.ConfigsDir)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")

		reqPath := r.URL.Path
		if hasUnsafePath(reqPath) {
			http.Error(w, "forbidden path", http.StatusForbidden)
			return
		}

		if reqPath == "/" || reqPath == "/index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(renderLandingPage(absRoot, absDemos, absConfigs)))
			return
		}

		if reqPath == "/view" {
			serveInlineTextViewer(w, r, absRoot)
			return
		}

		if strings.HasPrefix(reqPath, "/demos/") {
			if opts.DemosDir == "" {
				http.NotFound(w, r)
				return
			}
			serveScopedFile(w, r, absDemos, strings.TrimPrefix(reqPath, "/demos/"))
			return
		}

		if strings.HasPrefix(reqPath, "/configs/") {
			if opts.ConfigsDir == "" {
				http.NotFound(w, r)
				return
			}
			serveScopedFile(w, r, absConfigs, strings.TrimPrefix(reqPath, "/configs/"))
			return
		}

		serveScopedFile(w, r, absRoot, strings.TrimPrefix(reqPath, "/"))
	})
}

func hasUnsafePath(p string) bool {
	if strings.Contains(p, "..") || strings.Contains(p, "\x00") || strings.Contains(p, "\\") {
		return true
	}
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, ".") && seg != "" {
			return true
		}
	}
	return false
}

func serveScopedFile(w http.ResponseWriter, r *http.Request, baseDir, rel string) {
	if baseDir == "" || hasUnsafePath(rel) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	cleanRel := filepath.Clean("/" + rel)
	target := filepath.Join(baseDir, strings.TrimPrefix(cleanRel, "/"))
	rPath, err := filepath.Rel(baseDir, target)
	if err != nil || rPath == ".." || strings.HasPrefix(rPath, ".."+string(filepath.Separator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if info.IsDir() {
		idx := filepath.Join(target, "index.html")
		if _, err := os.Stat(idx); err == nil {
			http.ServeFile(w, r, idx)
			return
		}
		http.Error(w, "directory listing disabled", http.StatusForbidden)
		return
	}
	switch strings.ToLower(filepath.Ext(target)) {
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	case ".md", ".yaml", ".yml":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	http.ServeFile(w, r, target)
}

func serveInlineTextViewer(w http.ResponseWriter, r *http.Request, absRoot string) {
	exp := r.URL.Query().Get("exp")
	file := r.URL.Query().Get("file")
	if exp == "" || file == "" || strings.ContainsAny(exp, "/\\") || strings.ContainsAny(file, "/\\") || hasUnsafePath(exp) || hasUnsafePath(file) {
		http.Error(w, "invalid exp or file parameter", http.StatusBadRequest)
		return
	}
	ext := strings.ToLower(filepath.Ext(file))
	if ext != ".md" && ext != ".yaml" && ext != ".yml" {
		http.Error(w, "only .md and .yaml files are supported in /view", http.StatusBadRequest)
		return
	}
	target := filepath.Join(absRoot, exp, file)
	raw, err := os.ReadFile(target)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	meta := inspectExperiment(filepath.Join(absRoot, exp), exp)
	histBanner := ""
	if meta.Historical {
		histBanner = `<div class="hist-banner"><strong>HISTORICAL — UNVALIDATED METHODOLOGY:</strong> This experiment predates M1–M4 measurement and correctness fixes and must not be used for current engineering decisions.</div>`
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8" /><title>%s / %s — Test Lab Viewer</title>
<style>
  body { margin: 0; padding: 24px; background: #0d1117; color: #c9d1d9; font-family: system-ui, sans-serif; max-width: 1100px; }
  .top { display: flex; gap: 12px; align-items: center; margin-bottom: 14px; font-size: 13px; }
  a { color: #58a6ff; text-decoration: none; }
  .hist-banner { background: #3b1d1d; border: 1px solid #f85149; color: #ffa198; padding: 10px 14px; border-radius: 6px; margin-bottom: 14px; font-size: 13px; }
  pre { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 16px; overflow-x: auto; font-size: 12.5px; line-height: 1.5; white-space: pre-wrap; }
</style></head><body>
<div class="top">
  <a href="/">&larr; Lab Home</a>
  <strong>%s / %s</strong>
  <a href="/%s/%s">Raw File</a>
</div>
%s
<pre>%s</pre>
</body></html>`, html.EscapeString(exp), html.EscapeString(file),
		html.EscapeString(exp), html.EscapeString(file),
		html.EscapeString(exp), html.EscapeString(file),
		histBanner, html.EscapeString(string(raw)))
}

type expSummary struct {
	Name        string
	ModTime     string
	SpecVersion string
	Historical  bool
	Arrangement string
	Methods     []string
	Counts      []int
	PassCount   int
	TotalCount  int
	HasRaw      bool
	HasReport   bool
	HasCatalogue bool
}

func inspectExperiment(dir, name string) expSummary {
	s := expSummary{Name: name, Historical: true}
	if _, err := os.Stat(filepath.Join(dir, "report.html")); err == nil {
		s.HasReport = true
	}
	if _, err := os.Stat(filepath.Join(dir, "catalogue", "index.html")); err == nil {
		s.HasCatalogue = true
	}
	rawPath := filepath.Join(dir, "raw-results.json")
	b, err := os.ReadFile(rawPath)
	if err != nil {
		return s
	}
	s.HasRaw = true
	var res LabResults
	if err := json.Unmarshal(b, &res); err != nil {
		return s
	}
	if v, ok := res.Environment["measurementSpecVersion"].(string); ok && v != "" {
		s.SpecVersion = v
		s.Historical = false
	}
	mSet, cSet, iSet := map[string]bool{}, map[int]bool{}, map[string]bool{}
	for _, sc := range res.Scenarios {
		s.TotalCount++
		if sc.CorrectnessPassed {
			s.PassCount++
		}
		if sc.Method != "" {
			mSet[sc.Method] = true
		}
		if sc.Count > 0 {
			cSet[sc.Count] = true
		}
		if sc.Isolation != "" {
			iSet[sc.Isolation] = true
		}
	}
	for m := range mSet {
		s.Methods = append(s.Methods, m)
	}
	sort.Strings(s.Methods)
	for c := range cSet {
		s.Counts = append(s.Counts, c)
	}
	sort.Ints(s.Counts)
	var arrs []string
	for a := range iSet {
		arrs = append(arrs, a)
	}
	sort.Strings(arrs)
	s.Arrangement = strings.Join(arrs, ", ")
	return s
}

func renderLandingPage(rootDir, demosDir, configsDir string) string {
	entries, err := os.ReadDir(rootDir)
	var validRows, histRows strings.Builder
	validCount, histCount := 0, 0

	if err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
		for _, ent := range entries {
			if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
				continue
			}
			name := ent.Name()
			sub := filepath.Join(rootDir, name)
			infoSummary := inspectExperiment(sub, name)
			if fi, err := ent.Info(); err == nil {
				infoSummary.ModTime = fi.ModTime().Format("2006-01-02 15:04:05")
			}

			var links []string
			addIfExists := func(rel, label, cls string) {
				if _, err := os.Stat(filepath.Join(sub, rel)); err == nil {
					links = append(links, fmt.Sprintf(`<a class="pill %s" href="/%s/%s">%s</a>`, cls, html.EscapeString(name), html.EscapeString(rel), html.EscapeString(label)))
				}
			}
			addInlineView := func(file, label, cls string) {
				if _, err := os.Stat(filepath.Join(sub, file)); err == nil {
					links = append(links, fmt.Sprintf(`<a class="pill %s" href="/view?exp=%s&amp;file=%s">%s</a>`, cls, html.EscapeString(name), html.EscapeString(file), html.EscapeString(label)))
				}
			}

			addIfExists("catalogue/index.html", "Visual Catalogue", "")
			addIfExists("report.html", "HTML Report", "")
			addInlineView("report.md", "Markdown Report", "")
			addInlineView("resolved-config.yaml", "Resolved YAML", "")
			addIfExists("raw-results.json", "Raw JSON", "")

			decs, _ := filepath.Glob(filepath.Join(sub, "decision-*.md"))
			sort.Strings(decs)
			for _, d := range decs {
				base := filepath.Base(d)
				addInlineView(base, base, "pill-dec")
			}

			if !infoSummary.HasReport {
				links = append(links, `<span class="missing-tag">No report</span>`)
			}
			if !infoSummary.HasCatalogue {
				links = append(links, `<span class="missing-tag">No samples</span>`)
			}

			badge := `<span class="badge-hist">HISTORICAL — UNVALIDATED METHODOLOGY</span>`
			if !infoSummary.Historical {
				badge = fmt.Sprintf(`<span class="badge-ok">VALIDATED (%s)</span>`, html.EscapeString(infoSummary.SpecVersion))
			}

			metaLine := "Metadata unavailable"
			if infoSummary.HasRaw {
				metaLine = fmt.Sprintf("Arrangement: <code>%s</code> | Methods: <code>%s</code> | Counts: <code>%v</code> | Correctness: <strong>%d/%d</strong>",
					html.EscapeString(infoSummary.Arrangement),
					html.EscapeString(strings.Join(infoSummary.Methods, ", ")),
					infoSummary.Counts, infoSummary.PassCount, infoSummary.TotalCount)
			}

			card := fmt.Sprintf(`<div class="exp-card">
  <div class="exp-head">
    <div><strong>%s</strong> %s</div>
    <span class="exp-time">%s</span>
  </div>
  <div class="exp-meta">%s</div>
  <div class="exp-links">%s</div>
</div>`, html.EscapeString(name), badge, html.EscapeString(infoSummary.ModTime), metaLine, strings.Join(links, " "))

			if infoSummary.Historical {
				histCount++
				histRows.WriteString(card)
			} else {
				validCount++
				validRows.WriteString(card)
			}
		}
	}

	demoSection := ""
	if demosDir != "" {
		var dLinks []string
		if _, err := os.Stat(filepath.Join(demosDir, "demo-a-digital-twin.html")); err == nil {
			dLinks = append(dLinks, `<div class="demo-card"><strong>Demonstration A — Digital Twin (100 Window Tracers)</strong><div class="exp-meta">One flattened static building raster background + one shared overlay SVG with 100 independent window tracers.</div><div class="exp-links"><a class="pill" href="/demos/demo-a-digital-twin.html">Open Digital Twin Demo</a> <a class="pill" href="/demos/vis-m4-01-dt-scene.png">Screenshot</a> <a class="pill" href="/configs/dt-building-window-tracers.yaml">YAML Config</a></div></div>`)
		}
		if _, err := os.Stat(filepath.Join(demosDir, "demo-b-stage16c-dashboard.html")); err == nil {
			dLinks = append(dLinks, `<div class="demo-card"><strong>Demonstration B — Stage 16C Widget Dashboard (100 Complete Widgets)</strong><div class="exp-meta">100 independently mounted <code>host-monitor-v1</code> packages using unmodified <code>js/svg-widget.js</code>. <em>Coverage note: verifies mounting, unique identity, independent <code>setParameter()</code> updates, suspension, and lifecycle stability; does not run M1 per-frame motion sampling inside exported widgets.</em></div><div class="exp-links"><a class="pill" href="/demos/demo-b-stage16c-dashboard.html">Open Stage 16C Dashboard Demo</a> <a class="pill" href="/demos/vis-m4-02-stage16c-widgets.png">Screenshot</a> <a class="pill" href="/configs/stage16c-widget-dashboard.yaml">YAML Config</a></div></div>`)
		}
		if len(dLinks) > 0 {
			demoSection = `<h2>M4 Representative Workload Demonstrations</h2>` + strings.Join(dLinks, "")
		}
	}

	validBlock := `<div class="empty">No post-correction validated runs in this directory yet.</div>`
	if validCount > 0 {
		validBlock = validRows.String()
	}
	histBlock := `<div class="empty">No historical unvalidated runs found.</div>`
	if histCount > 0 {
		histBlock = histRows.String()
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>SVG Animation Test Lab — Results &amp; Sample Catalogue</title>
<style>
  body { margin: 0; padding: 24px; background: #0d1117; color: #c9d1d9; font-family: system-ui, -apple-system, sans-serif; max-width: 1150px; }
  h1 { margin: 0 0 6px; font-size: 22px; color: #f0f6fc; }
  h2 { margin: 22px 0 10px; font-size: 16px; color: #f0f6fc; border-bottom: 1px solid #21262d; padding-bottom: 6px; }
  .notice { background: #161b22; border: 1px solid #30363d; border-left: 3px solid #58a6ff; border-radius: 6px; padding: 10px 14px; font-size: 13px; color: #8b949e; margin: 12px 0 18px; }
  .exp-card, .demo-card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 14px 16px; margin-bottom: 12px; }
  .exp-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
  .exp-head strong { font-size: 15px; color: #f0f6fc; }
  .exp-time { font-size: 12px; color: #8b949e; }
  .exp-meta { font-size: 12px; color: #8b949e; margin-bottom: 10px; }
  .exp-links { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .pill { display: inline-block; padding: 5px 10px; background: #21262d; border: 1px solid #30363d; border-radius: 6px; color: #58a6ff; text-decoration: none; font-size: 12px; font-weight: 500; }
  .pill:hover { background: #30363d; }
  .pill-dec { color: #3fb950; }
  .badge-ok { font-size: 11px; padding: 2px 7px; background: #122d1b; border: 1px solid #238636; color: #3fb950; border-radius: 4px; margin-left: 8px; font-weight: 600; }
  .badge-hist { font-size: 11px; padding: 2px 7px; background: #3b1d1d; border: 1px solid #f85149; color: #ffa198; border-radius: 4px; margin-left: 8px; font-weight: 600; }
  .missing-tag { font-size: 11px; color: #8b949e; padding: 4px 8px; border: 1px dashed #30363d; border-radius: 6px; }
  .empty { color: #8b949e; font-size: 13px; padding: 14px; background: #161b22; border-radius: 8px; }
</style>
</head>
<body>
<h1>SVG Animation Test Lab — Results &amp; Visual Catalogue</h1>
<div class="notice">
  <strong>Read-Only Visual Inspection Portal:</strong> Browse generated SVG sample catalogues, side-by-side method comparisons, HTML/Markdown/decision reports, M4 Digital Twin &amp; Stage 16C demonstrations, and resolved YAML configs. Experiments remain YAML-driven via the <code>anim-test-lab</code> CLI (no remote execution from browser). <em>Note:</em> Shared-scene SMIL per-object off-screen suspension is unsupported; <code>rAF</code> callback Hz is distinct from unavailable compositor presented FPS (<code>N/A</code>).
</div>
%s
<h2>Validated Methodology Experiments</h2>
%s
<h2>Historical Experiments (Pre-Correction — Unvalidated)</h2>
%s
</body>
</html>`, demoSection, validBlock, histBlock)
}
