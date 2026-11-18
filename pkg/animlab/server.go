package animlab

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// NewResultsServer returns an http.Handler serving a dynamic landing page at "/"
// and static experiment outputs (catalogues, reports, samples, screenshots) beneath it.
func NewResultsServer(rootDir string) http.Handler {
	fs := http.FileServer(http.Dir(rootDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(renderLandingPage(rootDir)))
			return
		}
		fs.ServeHTTP(w, r)
	})
}

func renderLandingPage(rootDir string) string {
	entries, err := os.ReadDir(rootDir)
	var rows strings.Builder
	count := 0
	if err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
		for _, ent := range entries {
			if !ent.IsDir() {
				continue
			}
			name := ent.Name()
			sub := filepath.Join(rootDir, name)
			count++
			links := []string{}
			addIfExists := func(rel, label string) {
				if _, err := os.Stat(filepath.Join(sub, rel)); err == nil {
					links = append(links, fmt.Sprintf(`<a class="pill" href="%s/%s">%s</a>`, html.EscapeString(name), html.EscapeString(rel), html.EscapeString(label)))
				}
			}
			addIfExists("catalogue/index.html", "Visual Catalogue")
			addIfExists("report.html", "HTML Report")
			addIfExists("report.md", "Markdown Report")
			addIfExists("resolved-config.yaml", "Resolved YAML")
			addIfExists("raw-results.json", "Raw JSON")

			decs, _ := filepath.Glob(filepath.Join(sub, "decision-*.md"))
			sort.Strings(decs)
			for _, d := range decs {
				base := filepath.Base(d)
				links = append(links, fmt.Sprintf(`<a class="pill pill-dec" href="%s/%s">%s</a>`, html.EscapeString(name), html.EscapeString(base), html.EscapeString(base)))
			}

			modTime := ""
			if info, err := ent.Info(); err == nil {
				modTime = info.ModTime().Format("2006-01-02 15:04:05")
			}

			rows.WriteString(fmt.Sprintf(`<div class="exp-card">
  <div class="exp-head">
    <strong>%s</strong>
    <span class="exp-time">%s</span>
  </div>
  <div class="exp-links">%s</div>
</div>`, html.EscapeString(name), html.EscapeString(modTime), strings.Join(links, " ")))
		}
	}

	emptyMsg := ""
	if count == 0 {
		emptyMsg = `<div class="empty">No experiments found in results directory yet.</div>`
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<title>SVG Animation Test Lab — Results &amp; Sample Catalogue</title>
<style>
  body { margin: 0; padding: 24px; background: #0d1117; color: #c9d1d9; font-family: system-ui, -apple-system, sans-serif; max-width: 1100px; }
  h1 { margin: 0 0 6px; font-size: 22px; color: #f0f6fc; }
  .notice { background: #161b22; border: 1px solid #30363d; border-left: 3px solid #58a6ff; border-radius: 6px; padding: 10px 14px; font-size: 13px; color: #8b949e; margin: 12px 0 20px; }
  .exp-card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 14px 16px; margin-bottom: 12px; }
  .exp-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
  .exp-head strong { font-size: 15px; color: #f0f6fc; }
  .exp-time { font-size: 12px; color: #8b949e; }
  .exp-links { display: flex; flex-wrap: wrap; gap: 8px; }
  .pill { display: inline-block; padding: 5px 10px; background: #21262d; border: 1px solid #30363d; border-radius: 6px; color: #58a6ff; text-decoration: none; font-size: 12px; font-weight: 500; }
  .pill:hover { background: #30363d; }
  .pill-dec { color: #3fb950; }
  .empty { color: #8b949e; font-size: 13px; padding: 20px; background: #161b22; border-radius: 8px; }
</style>
</head>
<body>
<h1>SVG Animation Test Lab — Results &amp; Visual Catalogue</h1>
<div class="notice">
  <strong>Visual Inspection Interface:</strong> This web portal provides read-only browsing of generated SVG sample catalogues, comparison reports, decision reports, screenshots, and resolved YAML configurations. Experiment configuration, validation, and execution are performed via YAML files and the <code>anim-test-lab</code> CLI.
</div>
%s
%s
</body>
</html>`, emptyMsg, rows.String())
}
