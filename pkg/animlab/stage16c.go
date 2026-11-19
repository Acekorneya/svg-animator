package animlab

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Stage16CParam mirrors an exported parameter definition in widget-manifest.json.
type Stage16CParam struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Default  any      `json:"default"`
	Target   string   `json:"target"`
	Property string   `json:"property"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
}

// Stage16CManifest mirrors widget-manifest.json (schemaVersion 1).
type Stage16CManifest struct {
	SchemaVersion  int             `json:"schemaVersion"`
	PackageVersion string          `json:"packageVersion"`
	PackageID      string          `json:"packageId"`
	WidgetName     string          `json:"widgetName"`
	Parameters     []Stage16CParam `json:"parameters"`
}

// Stage16CPackage holds a validated, self-consistent Stage 16C exported widget package.
type Stage16CPackage struct {
	Dir       string
	Manifest  Stage16CManifest
	SVGFile   string
	SVGText   string
	RuntimeJS string
}

var embeddedManifestRe = regexp.MustCompile(`(?is)<script\b[^>]*\bid=["'](?:[^"']*__)?sva-widget-manifest["'][^>]*>([\s\S]*?)</script>`)

// LoadStage16CPackage loads and validates a real Studio-exported Stage 16C widget package directory.
// It enforces the same package-consistency contract as demos/stage16c-widget-consumer/consumer.js.
func LoadStage16CPackage(dir string) (*Stage16CPackage, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("widget_runtime.package_dir is required for stage16c-widgets")
	}
	manifestPath := filepath.Join(dir, "widget-manifest.json")
	mRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading widget-manifest.json in %s: %w", dir, err)
	}
	var mf Stage16CManifest
	if err := json.Unmarshal(mRaw, &mf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", manifestPath, err)
	}
	if mf.SchemaVersion != 1 || len(mf.Parameters) == 0 {
		return nil, fmt.Errorf("invalid widget-manifest.json in %s: schemaVersion=1 and non-empty parameters required", dir)
	}

	// Locate SVG artwork (<widgetName>.svg or first *.svg in package dir)
	svgFile := filepath.Join(dir, mf.WidgetName+".svg")
	if _, err := os.Stat(svgFile); err != nil {
		matches, _ := filepath.Glob(filepath.Join(dir, "*.svg"))
		if len(matches) == 0 {
			return nil, fmt.Errorf("no .svg artwork found in Stage 16C package %s", dir)
		}
		sort.Strings(matches)
		svgFile = matches[0]
	}
	svgBytes, err := os.ReadFile(svgFile)
	if err != nil {
		return nil, fmt.Errorf("reading SVG artwork %s: %w", svgFile, err)
	}
	svgText := string(svgBytes)
	if !strings.Contains(svgText, "<svg") {
		return nil, fmt.Errorf("malformed SVG artwork %s", svgFile)
	}

	// Validate embedded sva-widget-manifest consistency against widget-manifest.json
	m := embeddedManifestRe.FindStringSubmatch(svgText)
	if len(m) < 2 {
		return nil, fmt.Errorf("package consistency error: %s missing embedded sva-widget-manifest", svgFile)
	}
	var embedded Stage16CManifest
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &embedded); err != nil {
		return nil, fmt.Errorf("package consistency error: invalid embedded manifest in %s: %w", svgFile, err)
	}
	if mf.PackageID != "" && embedded.PackageID != "" && mf.PackageID != embedded.PackageID {
		return nil, fmt.Errorf("package consistency error: manifest packageId %q != SVG embedded packageId %q", mf.PackageID, embedded.PackageID)
	}
	if mf.WidgetName != "" && embedded.WidgetName != "" && mf.WidgetName != embedded.WidgetName {
		return nil, fmt.Errorf("package consistency error: manifest widgetName %q != SVG embedded widgetName %q", mf.WidgetName, embedded.WidgetName)
	}
	if paramNameSig(mf.Parameters) != paramNameSig(embedded.Parameters) {
		return nil, fmt.Errorf("package consistency error: manifest parameters differ from embedded SVG manifest in %s", dir)
	}

	// Load genuine Stage 16C public runtime (js/svg-widget.js)
	rtPath := filepath.Join(dir, "js", "svg-widget.js")
	rtBytes, err := os.ReadFile(rtPath)
	if err != nil {
		return nil, fmt.Errorf("reading Stage 16C runtime %s: %w", rtPath, err)
	}
	if !strings.Contains(string(rtBytes), "SVGWidget") {
		return nil, fmt.Errorf("invalid Stage 16C runtime %s: SVGWidget export not found", rtPath)
	}

	return &Stage16CPackage{
		Dir:       dir,
		Manifest:  mf,
		SVGFile:   svgFile,
		SVGText:   svgText,
		RuntimeJS: string(rtBytes),
	}, nil
}

func paramNameSig(params []Stage16CParam) string {
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.Name
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// ValidateWidgetProperties verifies requested property updates against the widget manifest.
// Required properties missing from the manifest return an error; optional missing properties are skipped and reported.
func ValidateWidgetProperties(pkg *Stage16CPackage, props []WidgetPropertyUpdate) ([]WidgetPropertyUpdate, []string, error) {
	if pkg == nil {
		return nil, nil, fmt.Errorf("nil Stage 16C package")
	}
	known := make(map[string]Stage16CParam, len(pkg.Manifest.Parameters))
	for _, p := range pkg.Manifest.Parameters {
		known[p.Name] = p
	}
	var valid []WidgetPropertyUpdate
	var skipped []string
	for _, up := range props {
		if _, ok := known[up.Name]; !ok {
			if up.Optional {
				skipped = append(skipped, fmt.Sprintf("optional property %q not exposed by widget %s; skipped", up.Name, pkg.Manifest.WidgetName))
				continue
			}
			return nil, skipped, fmt.Errorf("widget %q does not expose required property %q (exposed: %s)",
				pkg.Manifest.WidgetName, up.Name, paramNameSig(pkg.Manifest.Parameters))
		}
		if len(up.Values) == 0 {
			return nil, skipped, fmt.Errorf("widget property update %q must specify at least one value", up.Name)
		}
		valid = append(valid, up)
	}
	return valid, skipped, nil
}

func resolveAssetPath(configDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	if _, err := os.Stat(p); err == nil {
		return p
	}
	dir := configDir
	for i := 0; i < 4 && dir != "" && dir != "/" && dir != "."; i++ {
		cand := filepath.Join(dir, p)
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
		dir = filepath.Dir(dir)
	}
	return p
}
