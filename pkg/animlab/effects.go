package animlab

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// InstanceParams defines the concrete authored parameters for a single widget/object instance.
type InstanceParams struct {
	ID             string
	Family         string
	Method         string
	Shape          string
	Width          int
	Height         int
	Speed          float64 // circuit or cycle duration in seconds
	Segments       int
	LineGapRatio   string
	Direction      string // "cw" | "ccw"
	PulseMode      string // "opacity-only" | "scale-only" | "scale-opacity"
	PulseAmplitude float64
	FlashMode      string // "single-shot" | "repeating" | "concurrent" | "staggered"
	FlashDurMs     int
	FlashIntervalS float64
	StaggerDelayS  float64
	Color          string
}

// RenderedEffect holds the generated CSS, SVG markup, optional JS driver, and any method limitation.
type RenderedEffect struct {
	CSS        string
	SVGInner   string
	JSDriver   string
	Limitation string
}

// EffectGenerator is the pluggable interface for adding new effect families and rendering methods.
type EffectGenerator interface {
	Family() string
	Description() string
	SupportedMethods() []string
	SupportsMethod(method string) bool
	Render(p InstanceParams, viewW, viewH int) RenderedEffect
}

// DefaultRegistry returns all registered effect families.
func DefaultRegistry() map[string]EffectGenerator {
	return map[string]EffectGenerator{
		"perimeter-tracer": tracerGen{},
		"pulse":            pulseGen{},
		"flash":            flashGen{},
		"flow":             flowGen{},
		"static-gauge":     gaugeGen{},
	}
}

func containsStr(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func computeDash(segments int, ratio string) (float64, float64) {
	if segments < 1 {
		segments = 1
	}
	l, g := 1.0, 3.0
	parts := strings.Split(strings.TrimSpace(ratio), ":")
	if len(parts) == 2 {
		if lv, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil && lv > 0 {
			if gv, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err2 == nil && gv > 0 {
				l, g = lv, gv
			}
		}
	}
	unit := 1000.0 / float64(segments)
	dash := math.Round((unit*(l/(l+g)))*1000) / 1000
	gap := math.Round((unit-dash)*1000) / 1000
	return dash, gap
}

// ---------------------------------------------------------------------------
// 1. Perimeter Tracer
// ---------------------------------------------------------------------------
type tracerGen struct{}

func (tracerGen) Family() string      { return "perimeter-tracer" }
func (tracerGen) Description() string { return "Illuminated segments travelling around closed shapes" }
func (tracerGen) SupportedMethods() []string {
	return []string{"css", "smil", "javascript-raf", "clip-reveal", "static"}
}
func (t tracerGen) SupportsMethod(m string) bool { return containsStr(t.SupportedMethods(), m) }

func (tracerGen) Render(p InstanceParams, viewW, viewH int) RenderedEffect {
	cx := float64(viewW) / 2.0
	cy := float64(viewH) / 2.0
	w := float64(p.Width)
	h := float64(p.Height)
	if w <= 0 {
		w = 120
	}
	if h <= 0 {
		h = 80
	}
	if p.Shape == "circle" {
		h = w
	}
	rx := w / 2.0
	ry := h / 2.0
	x := cx - rx
	y := cy - ry
	dur := math.Max(0.2, p.Speed)
	dash, gap := computeDash(p.Segments, p.LineGapRatio)
	dashAttr := fmt.Sprintf("%.3f %.3f", dash, gap)
	toOffset := "-1000"
	if p.Direction == "ccw" {
		toOffset = "1000"
	}

	buildShape := func(extra string) string {
		switch p.Shape {
		case "circle", "ellipse":
			return fmt.Sprintf(`<ellipse cx="%.2f" cy="%.2f" rx="%.2f" ry="%.2f" %s />`, cx, cy, rx, ry, extra)
		case "rounded-rectangle":
			cr := math.Min(10, math.Min(rx, ry))
			return fmt.Sprintf(`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.2f" %s />`, x, y, w, h, cr, extra)
		default:
			return fmt.Sprintf(`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" %s />`, x, y, w, h, extra)
		}
	}

	guide := buildShape(fmt.Sprintf(`class="lab-guide" fill="none" stroke="%s" stroke-width="1.2" stroke-opacity="0.22" shape-rendering="geometricPrecision"`, p.Color))
	common := fmt.Sprintf(`class="lab-payload" data-lab-family="perimeter-tracer" data-lab-method="%s" data-lab-dir="%s" data-lab-dur="%.3f" pathLength="1000" fill="none" stroke="%s" stroke-width="2.5" stroke-dasharray="%s" stroke-linecap="round" shape-rendering="geometricPrecision"`,
		p.Method, p.Direction, dur, p.Color, dashAttr)

	switch p.Method {
	case "css":
		kf := p.ID + "_kf"
		css := fmt.Sprintf(`@keyframes %s { from { stroke-dashoffset: 0px; } to { stroke-dashoffset: %spx; } }
#%s .lab-payload { animation: %s %.3fs linear infinite; }`, kf, toOffset, p.ID, kf, dur)
		return RenderedEffect{CSS: css, SVGInner: guide + buildShape(common)}

	case "smil":
		anim := fmt.Sprintf(`<animate attributeName="stroke-dashoffset" from="0" to="%s" dur="%.3fs" repeatCount="indefinite" />`, toOffset, dur)
		shape := buildShape(common)
		shape = strings.TrimSuffix(shape, " />") + ">" + anim + "</" + shapeTagName(p.Shape) + ">"
		return RenderedEffect{SVGInner: guide + shape}

	case "javascript-raf":
		js := fmt.Sprintf(`window.__labRafDrivers.push({ id: %q, kind: 'tracer', dur: %.3f, dir: %q });`, p.ID, dur, p.Direction)
		return RenderedEffect{SVGInner: guide + buildShape(common), JSDriver: js}

	case "clip-reveal":
		clipID := p.ID + "_clip"
		kf := p.ID + "_clipkf"
		css := fmt.Sprintf(`@keyframes %s { 0%% { transform: rotate(0deg); } 100%% { transform: rotate(%sdeg); } }
#%s_sweep { transform-origin: %.2fpx %.2fpx; animation: %s %.3fs linear infinite; }`,
			kf, map[bool]string{true: "-360", false: "360"}[p.Direction == "ccw"], p.ID, cx, cy, kf, dur)
		clip := fmt.Sprintf(`<defs><clipPath id="%s"><path id="%s_sweep" d="M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 1 %.2f %.2f Z"/></clipPath></defs>`,
			clipID, p.ID, cx, cy, cx, cy-math.Max(rx, ry)*1.6, math.Max(rx, ry)*1.6, math.Max(rx, ry)*1.6, cx+math.Max(rx, ry)*1.6, cy)
		pay := buildShape(fmt.Sprintf(`class="lab-payload" data-lab-family="perimeter-tracer" data-lab-method="clip-reveal" fill="none" stroke="%s" stroke-width="2.5" clip-path="url(#%s)" shape-rendering="geometricPrecision"`, p.Color, clipID))
		return RenderedEffect{
			CSS:        css,
			SVGInner:   clip + guide + pay,
			Limitation: "clip-reveal sweeps a single angular wedge and cannot independently space N segments at arbitrary line:gap ratios",
		}

	default: // static
		return RenderedEffect{SVGInner: guide + buildShape(common)}
	}
}

func shapeTagName(shape string) string {
	if shape == "circle" || shape == "ellipse" {
		return "ellipse"
	}
	return "rect"
}

// ---------------------------------------------------------------------------
// 2. Pulse
// ---------------------------------------------------------------------------
type pulseGen struct{}

func (pulseGen) Family() string      { return "pulse" }
func (pulseGen) Description() string { return "Breathing scale and/or opacity indicator" }
func (pulseGen) SupportedMethods() []string {
	return []string{"css", "smil", "javascript-raf", "static"}
}
func (p pulseGen) SupportsMethod(m string) bool { return containsStr(p.SupportedMethods(), m) }

func (pulseGen) Render(p InstanceParams, viewW, viewH int) RenderedEffect {
	cx := float64(viewW) / 2.0
	cy := float64(viewH) / 2.0
	w := math.Max(5, float64(p.Width))
	h := math.Max(5, float64(p.Height))
	if p.Shape == "circle" {
		h = w
	}
	rx, ry := w/2.0, h/2.0
	dur := math.Max(0.2, p.Speed)
	amp := p.PulseAmplitude
	if amp <= 0 {
		amp = 0.2
	}
	minS := math.Max(0.2, 1.0-amp)
	maxS := 1.0 + amp
	minOp := 0.35
	if p.PulseMode == "scale-only" {
		minOp = 0.95
	}
	if p.PulseMode == "opacity-only" {
		minS, maxS = 1.0, 1.0
	}

	buildShape := func(lx, ly float64, extra string) string {
		switch p.Shape {
		case "rectangle", "rounded-rectangle":
			return fmt.Sprintf(`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="3" %s />`, lx-rx, ly-ry, w, h, extra)
		case "diamond":
			pts := fmt.Sprintf("%.2f,%.2f %.2f,%.2f %.2f,%.2f %.2f,%.2f", lx, ly-ry, lx+rx, ly, lx, ly+ry, lx-rx, ly)
			return fmt.Sprintf(`<polygon points="%s" %s />`, pts, extra)
		default:
			return fmt.Sprintf(`<ellipse cx="%.2f" cy="%.2f" rx="%.2f" ry="%.2f" %s />`, lx, ly, rx, ry, extra)
		}
	}

	common := fmt.Sprintf(`class="lab-payload" data-lab-family="pulse" data-lab-method="%s" data-lab-mode="%s" data-lab-dur="%.3f" fill="%s" shape-rendering="geometricPrecision"`,
		p.Method, p.PulseMode, dur, p.Color)

	switch p.Method {
	case "css":
		kf := p.ID + "_pkf"
		css := fmt.Sprintf(`@keyframes %s { 0%%, 100%% { transform: scale(%.3f); opacity: %.3f; } 50%% { transform: scale(%.3f); opacity: 1; } }
#%s .lab-payload { transform-box: fill-box; transform-origin: center; will-change: transform, opacity; animation: %s %.3fs ease-in-out infinite; }`,
			kf, minS, minOp, maxS, p.ID, kf, dur)
		return RenderedEffect{CSS: css, SVGInner: buildShape(cx, cy, common)}

	case "smil":
		animT := fmt.Sprintf(`<animateTransform attributeName="transform" type="scale" values="%.3f;%.3f;%.3f" keyTimes="0;0.5;1" calcMode="spline" keySplines="0.42 0 0.58 1;0.42 0 0.58 1" dur="%.3fs" repeatCount="indefinite" />`,
			minS, maxS, minS, dur)
		animO := fmt.Sprintf(`<animate attributeName="opacity" values="%.3f;1;%.3f" keyTimes="0;0.5;1" calcMode="spline" keySplines="0.42 0 0.58 1;0.42 0 0.58 1" dur="%.3fs" repeatCount="indefinite" />`,
			minOp, minOp, dur)
		inner := buildShape(0, 0, common)
		tag := "ellipse"
		if p.Shape == "rectangle" || p.Shape == "rounded-rectangle" {
			tag = "rect"
		} else if p.Shape == "diamond" {
			tag = "polygon"
		}
		inner = strings.TrimSuffix(inner, " />") + ">" + animT + animO + "</" + tag + ">"
		return RenderedEffect{SVGInner: fmt.Sprintf(`<g transform="translate(%.2f,%.2f)">%s</g>`, cx, cy, inner)}

	case "javascript-raf":
		js := fmt.Sprintf(`window.__labRafDrivers.push({ id: %q, kind: 'pulse', dur: %.3f, minS: %.3f, maxS: %.3f, minOp: %.3f });`,
			p.ID, dur, minS, maxS, minOp)
		return RenderedEffect{SVGInner: fmt.Sprintf(`<g transform="translate(%.2f,%.2f)">%s</g>`, cx, cy, buildShape(0, 0, common)), JSDriver: js}

	default:
		return RenderedEffect{SVGInner: buildShape(cx, cy, common)}
	}
}

// ---------------------------------------------------------------------------
// 3. Flash
// ---------------------------------------------------------------------------
type flashGen struct{}

func (flashGen) Family() string      { return "flash" }
func (flashGen) Description() string { return "Brief event flash with single-shot, repeating, or staggered timing" }
func (flashGen) SupportedMethods() []string {
	return []string{"css", "smil", "javascript-raf", "static"}
}
func (f flashGen) SupportsMethod(m string) bool { return containsStr(f.SupportedMethods(), m) }

func (flashGen) Render(p InstanceParams, viewW, viewH int) RenderedEffect {
	cx, cy := float64(viewW)/2.0, float64(viewH)/2.0
	w := math.Max(5, float64(p.Width))
	h := math.Max(5, float64(p.Height))
	rx, ry := w/2.0, h/2.0
	durS := math.Max(0.03, float64(p.FlashDurMs)/1000.0)
	if durS <= 0 {
		durS = 0.12
	}
	ivS := math.Max(durS+0.05, p.FlashIntervalS)
	if ivS <= 0 {
		ivS = 1.2
	}

	buildShape := func(extra string) string {
		if p.Shape == "cross" {
			ha := math.Max(0.5, math.Min(math.Min(rx*0.9, ry*0.9), 1.5))
			d := fmt.Sprintf("M %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f Z",
				cx-ha, cy-ry, cx+ha, cy-ry, cx+ha, cy-ha, cx+rx, cy-ha, cx+rx, cy+ha, cx+ha, cy+ha, cx+ha, cy+ry, cx-ha, cy+ry, cx-ha, cy+ha, cx-rx, cy+ha, cx-rx, cy-ha, cx-ha, cy-ha)
			return fmt.Sprintf(`<path d="%s" %s />`, d, extra)
		}
		pts := fmt.Sprintf("%.2f,%.2f %.2f,%.2f %.2f,%.2f %.2f,%.2f", cx, cy-ry, cx+rx, cy, cx, cy+ry, cx-rx, cy)
		return fmt.Sprintf(`<polygon points="%s" %s />`, pts, extra)
	}

	common := fmt.Sprintf(`class="lab-payload" data-lab-family="flash" data-lab-method="%s" data-lab-mode="%s" fill="%s" shape-rendering="geometricPrecision"`,
		p.Method, p.FlashMode, p.Color)

	switch p.Method {
	case "css":
		kf := p.ID + "_fkf"
		pct := math.Min(90, (durS/ivS)*100.0)
		iter := "infinite"
		totalDur := ivS
		if p.FlashMode == "single-shot" {
			iter = "1"
			totalDur = durS
			pct = 30
		}
		delay := p.StaggerDelayS
		css := fmt.Sprintf(`@keyframes %s { 0%% { opacity: 0; transform: scale(0.85); } %.1f%% { opacity: 0.95; transform: scale(1.08); } %.1f%%, 100%% { opacity: 0; transform: scale(1.0); } }
#%s .lab-payload { transform-box: fill-box; transform-origin: center; will-change: transform, opacity; animation: %s %.3fs ease-out %.3fs %s both; }`,
			kf, pct*0.25, pct, p.ID, kf, totalDur, delay, iter)
		return RenderedEffect{CSS: css, SVGInner: buildShape(common)}

	case "smil":
		totalDur := ivS
		rep := "indefinite"
		if p.FlashMode == "single-shot" {
			totalDur = durS
			rep = "1"
		}
		kt := math.Min(0.9, durS/totalDur)
		anim := fmt.Sprintf(`<animate attributeName="opacity" values="0;0.95;0;0" keyTimes="0;%.3f;%.3f;1" dur="%.3fs" begin="%.3fs" repeatCount="%s" fill="freeze" />`,
			kt*0.25, kt, totalDur, p.StaggerDelayS, rep)
		shape := buildShape(common)
		tag := "polygon"
		if p.Shape == "cross" {
			tag = "path"
		}
		shape = strings.TrimSuffix(shape, " />") + ">" + anim + "</" + tag + ">"
		return RenderedEffect{SVGInner: shape}

	case "javascript-raf":
		js := fmt.Sprintf(`window.__labRafDrivers.push({ id: %q, kind: 'flash', dur: %.3f, interval: %.3f, stagger: %.3f, single: %v });`,
			p.ID, durS, ivS, p.StaggerDelayS, p.FlashMode == "single-shot")
		return RenderedEffect{SVGInner: buildShape(common), JSDriver: js}

	default:
		return RenderedEffect{SVGInner: buildShape(common)}
	}
}

// ---------------------------------------------------------------------------
// 4. Flow (Existing Studio reference & simpler alternatives)
// ---------------------------------------------------------------------------
type flowGen struct{}

func (flowGen) Family() string      { return "flow" }
func (flowGen) Description() string { return "Connector flow animation (Studio SMIL reference vs CSS/RAF line alternatives)" }
func (flowGen) SupportedMethods() []string {
	return []string{"smil", "css", "javascript-raf", "static"}
}
func (f flowGen) SupportsMethod(m string) bool { return containsStr(f.SupportedMethods(), m) }

func (flowGen) Render(p InstanceParams, viewW, viewH int) RenderedEffect {
	x1, y1 := 14.0, float64(viewH)*0.65
	x2, y2 := float64(viewW)-14.0, float64(viewH)*0.35
	pathD := fmt.Sprintf("M %.1f %.1f L %.1f %.1f", x1, y1, x2, y2)
	if p.Shape == "curved" {
		pathD = fmt.Sprintf("M %.1f %.1f Q %.1f %.1f %.1f %.1f", x1, y1, float64(viewW)/2, 8.0, x2, y2)
	}
	dur := math.Max(0.3, p.Speed)
	pathID := p.ID + "_path"

	switch p.Method {
	case "smil":
		// Existing Studio Flow pattern: dashed track animate + animateMotion payload
		track := fmt.Sprintf(`<path id="%s" d="%s" fill="none" stroke="%s" stroke-width="2.2" stroke-dasharray="10,6" stroke-linecap="round" opacity="0.85"><animate attributeName="stroke-dashoffset" from="32" to="0" dur="1s" repeatCount="indefinite" /></path>`,
			pathID, pathD, p.Color)
		particle := fmt.Sprintf(`<circle class="lab-payload" data-lab-family="flow" data-lab-method="smil" r="4.5" fill="%s"><animateMotion calcMode="linear" dur="%.3fs" repeatCount="indefinite" rotate="auto"><mpath href="#%s" /></animateMotion></circle>`,
			p.Color, dur, pathID)
		return RenderedEffect{SVGInner: track + particle}

	case "css":
		kf := p.ID + "_flowkf"
		css := fmt.Sprintf(`@keyframes %s { from { stroke-dashoffset: 0px; } to { stroke-dashoffset: -1000px; } }
#%s .lab-payload { animation: %s %.3fs linear infinite; }`, kf, p.ID, kf, dur)
		line := fmt.Sprintf(`<path class="lab-payload" data-lab-family="flow" data-lab-method="css" d="%s" pathLength="1000" fill="none" stroke="%s" stroke-width="2.5" stroke-dasharray="140 160" stroke-linecap="round" />`,
			pathD, p.Color)
		return RenderedEffect{CSS: css, SVGInner: line}

	case "javascript-raf":
		js := fmt.Sprintf(`window.__labRafDrivers.push({ id: %q, kind: 'tracer', dur: %.3f, dir: 'cw' });`, p.ID, dur)
		line := fmt.Sprintf(`<path class="lab-payload" data-lab-family="flow" data-lab-method="javascript-raf" d="%s" pathLength="1000" fill="none" stroke="%s" stroke-width="2.5" stroke-dasharray="140 160" stroke-linecap="round" />`,
			pathD, p.Color)
		return RenderedEffect{SVGInner: line, JSDriver: js}

	default:
		line := fmt.Sprintf(`<path class="lab-payload" data-lab-family="flow" data-lab-method="static" d="%s" fill="none" stroke="%s" stroke-width="2.5" stroke-dasharray="10,6" stroke-linecap="round" />`,
			pathD, p.Color)
		return RenderedEffect{SVGInner: line}
	}
}

// ---------------------------------------------------------------------------
// 5. Static Gauge (for mixed workloads & data-driven updates)
// ---------------------------------------------------------------------------
type gaugeGen struct{}

func (gaugeGen) Family() string             { return "static-gauge" }
func (gaugeGen) Description() string        { return "Static or data-driven arc gauge for mixed dashboard workloads" }
func (gaugeGen) SupportedMethods() []string { return []string{"static", "data-driven", "css", "smil", "javascript-raf"} }
func (g gaugeGen) SupportsMethod(m string) bool { return containsStr(g.SupportedMethods(), m) }

func (gaugeGen) Render(p InstanceParams, viewW, viewH int) RenderedEffect {
	cx, cy := float64(viewW)/2.0, float64(viewH)/2.0
	r := math.Max(10, math.Min(float64(p.Width), float64(p.Height))/2.0-4)
	bg := fmt.Sprintf(`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="none" stroke="#30363d" stroke-width="5" />`, cx, cy, r)
	arc := fmt.Sprintf(`<circle class="lab-payload" data-lab-family="static-gauge" data-lab-method="%s" cx="%.2f" cy="%.2f" r="%.2f" pathLength="100" fill="none" stroke="%s" stroke-width="5" stroke-dasharray="68 100" stroke-linecap="round" transform="rotate(-90 %.2f %.2f)" />`,
		p.Method, cx, cy, r, p.Color, cx, cy)
	return RenderedEffect{SVGInner: bg + arc}
}
