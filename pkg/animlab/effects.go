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
	RAFScheduling  string // "shared" | "per-instance"
	Color          string
}

// RenderedEffect holds the generated CSS, SVG markup, optional JS driver, and equivalence/support flags.
type RenderedEffect struct {
	CSS           string
	SVGInner      string
	JSDriver      string
	Limitation    string
	Unsupported   bool
	NonEquivalent bool
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
		"flow-line":        flowLineGen{},
		"flow-particle":    flowParticleGen{},
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
	// F-B04: Circle uses min(w, h) so it remains circular without overflowing viewBox height
	if p.Shape == "circle" {
		d := math.Min(w, h)
		w = d
		h = d
	}
	// Ensure shape + stroke fits within viewBox
	maxW := float64(viewW) - 12.0
	maxH := float64(viewH) - 12.0
	if p.Shape == "circle" {
		d := math.Min(math.Min(w, maxW), maxH)
		w, h = d, d
	} else {
		if w > maxW {
			w = maxW
		}
		if h > maxH {
			h = maxH
		}
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
	common := fmt.Sprintf(`class="lab-payload" data-lab-family="perimeter-tracer" data-lab-method="%s" data-lab-dir="%s" data-lab-dur="%.3f" data-lab-segs="%d" pathLength="1000" fill="none" stroke="%s" stroke-width="2.5" stroke-dasharray="%s" stroke-linecap="round" shape-rendering="geometricPrecision"`,
		p.Method, p.Direction, dur, p.Segments, p.Color, dashAttr)

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
		js := fmt.Sprintf(`window.__labRafDrivers.push({ id: %q, kind: 'tracer', dur: %.3f, dir: %q, perInstance: %v });`,
			p.ID, dur, p.Direction, p.RAFScheduling == "per-instance")
		return RenderedEffect{SVGInner: guide + buildShape(common), JSDriver: js}

	case "clip-reveal":
		clipID := p.ID + "_clip"
		kf := p.ID + "_clipkf"
		css := fmt.Sprintf(`@keyframes %s { 0%% { transform: rotate(0deg); } 100%% { transform: rotate(%sdeg); } }
#%s_sweep { transform-origin: %.2fpx %.2fpx; animation: %s %.3fs linear infinite; }`,
			kf, map[bool]string{true: "-360", false: "360"}[p.Direction == "ccw"], p.ID, cx, cy, kf, dur)
		clip := fmt.Sprintf(`<defs><clipPath id="%s"><path id="%s_sweep" d="M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 0 1 %.2f %.2f Z"/></clipPath></defs>`,
			clipID, p.ID, cx, cy, cx, cy-math.Max(rx, ry)*1.6, math.Max(rx, ry)*1.6, math.Max(rx, ry)*1.6, cx+math.Max(rx, ry)*1.6, cy)
		pay := buildShape(fmt.Sprintf(`class="lab-payload" data-lab-family="perimeter-tracer" data-lab-method="clip-reveal" data-lab-dir="%s" data-lab-dur="%.3f" fill="none" stroke="%s" stroke-width="2.5" clip-path="url(#%s)" shape-rendering="geometricPrecision"`, p.Direction, dur, p.Color, clipID))
		return RenderedEffect{
			CSS:           css,
			SVGInner:      clip + guide + pay,
			NonEquivalent: true,
			Limitation:    "clip-reveal sweeps a single angular wedge and cannot independently space N segments at arbitrary line:gap ratios",
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
		d := math.Min(w, h)
		w, h = d, d
	}
	rx, ry := w/2.0, h/2.0
	dur := math.Max(0.2, p.Speed)
	amp := p.PulseAmplitude
	if amp <= 0 {
		amp = 0.2
	}
	mode := p.PulseMode
	if mode == "" {
		mode = "scale-opacity"
	}

	minS := math.Max(0.2, 1.0-amp)
	maxS := 1.0 + amp
	minOp := 0.35
	if mode == "scale-only" {
		minOp = 1.0
	}
	if mode == "opacity-only" {
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

	tag := "ellipse"
	if p.Shape == "rectangle" || p.Shape == "rounded-rectangle" {
		tag = "rect"
	} else if p.Shape == "diamond" {
		tag = "polygon"
	}

	common := fmt.Sprintf(`class="lab-payload" data-lab-family="pulse" data-lab-method="%s" data-lab-mode="%s" data-lab-dur="%.3f" data-lab-amp="%.3f" fill="%s" shape-rendering="geometricPrecision"`,
		p.Method, mode, dur, amp, p.Color)

	switch p.Method {
	case "css":
		kf := p.ID + "_pkf"
		var kfBody string
		switch mode {
		case "opacity-only":
			kfBody = fmt.Sprintf(`@keyframes %s { 0%%, 100%% { opacity: %.3f; } 50%% { opacity: 1; } }`, kf, minOp)
		case "scale-only":
			kfBody = fmt.Sprintf(`@keyframes %s { 0%%, 100%% { transform: scale(%.3f); } 50%% { transform: scale(%.3f); } }`, kf, minS, maxS)
		default:
			kfBody = fmt.Sprintf(`@keyframes %s { 0%%, 100%% { transform: scale(%.3f); opacity: %.3f; } 50%% { transform: scale(%.3f); opacity: 1; } }`, kf, minS, minOp, maxS)
		}
		css := fmt.Sprintf(`%s
#%s .lab-payload { transform-box: fill-box; transform-origin: center; animation: %s %.3fs ease-in-out infinite; }`,
			kfBody, p.ID, kf, dur)
		return RenderedEffect{CSS: css, SVGInner: buildShape(cx, cy, common)}

	case "smil":
		var anims string
		if mode != "opacity-only" {
			anims += fmt.Sprintf(`<animateTransform attributeName="transform" type="scale" values="%.3f;%.3f;%.3f" keyTimes="0;0.5;1" calcMode="spline" keySplines="0.42 0 0.58 1;0.42 0 0.58 1" dur="%.3fs" repeatCount="indefinite" />`,
				minS, maxS, minS, dur)
		}
		if mode != "scale-only" {
			anims += fmt.Sprintf(`<animate attributeName="opacity" values="%.3f;1;%.3f" keyTimes="0;0.5;1" calcMode="spline" keySplines="0.42 0 0.58 1;0.42 0 0.58 1" dur="%.3fs" repeatCount="indefinite" />`,
				minOp, minOp, dur)
		}
		inner := buildShape(0, 0, common)
		inner = strings.TrimSuffix(inner, " />") + ">" + anims + "</" + tag + ">"
		return RenderedEffect{SVGInner: fmt.Sprintf(`<g transform="translate(%.2f,%.2f)">%s</g>`, cx, cy, inner)}

	case "javascript-raf":
		js := fmt.Sprintf(`window.__labRafDrivers.push({ id: %q, kind: 'pulse', mode: %q, dur: %.3f, minS: %.3f, maxS: %.3f, minOp: %.3f, perInstance: %v });`,
			p.ID, mode, dur, minS, maxS, minOp, p.RAFScheduling == "per-instance")
		return RenderedEffect{SVGInner: fmt.Sprintf(`<g transform="translate(%.2f,%.2f)">%s</g>`, cx, cy, buildShape(0, 0, common)), JSDriver: js}

	default:
		return RenderedEffect{SVGInner: buildShape(cx, cy, common)}
	}
}

// ---------------------------------------------------------------------------
// 3. Flash (aligned scale + opacity across CSS, SMIL, RAF; genuine star shape)
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

	buildShape := func(lx, ly float64, extra string) (string, string) {
		switch p.Shape {
		case "cross":
			ha := math.Max(0.5, math.Min(math.Min(rx*0.9, ry*0.9), 1.5))
			d := fmt.Sprintf("M %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f L %.2f,%.2f Z",
				lx-ha, ly-ry, lx+ha, ly-ry, lx+ha, ly-ha, lx+rx, ly-ha, lx+rx, ly+ha, lx+ha, ly+ha, lx+ha, ly+ry, lx-ha, ly+ry, lx-ha, ly+ha, lx-rx, ly+ha, lx-rx, ly-ha, lx-ha, ly-ha)
			return fmt.Sprintf(`<path d="%s" %s />`, d, extra), "path"
		case "star":
			// F-B06: Genuine 8-point starburst geometry
			pts := make([]string, 0, 8)
			waistX := math.Max(1.0, rx*0.24)
			waistY := math.Max(1.0, ry*0.24)
			for i := 0; i < 8; i++ {
				ang := (math.Pi/4.0)*float64(i) - math.Pi/2.0
				radX, radY := rx, ry
				if i%2 == 1 {
					radX, radY = waistX, waistY
				}
				pts = append(pts, fmt.Sprintf("%.2f,%.2f", lx+math.Cos(ang)*radX, ly+math.Sin(ang)*radY))
			}
			return fmt.Sprintf(`<polygon points="%s" %s />`, strings.Join(pts, " "), extra), "polygon"
		case "circle":
			d := math.Min(rx, ry)
			return fmt.Sprintf(`<ellipse cx="%.2f" cy="%.2f" rx="%.2f" ry="%.2f" %s />`, lx, ly, d, d, extra), "ellipse"
		default:
			pts := fmt.Sprintf("%.2f,%.2f %.2f,%.2f %.2f,%.2f %.2f,%.2f", lx, ly-ry, lx+rx, ly, lx, ly+ry, lx-rx, ly)
			return fmt.Sprintf(`<polygon points="%s" %s />`, pts, extra), "polygon"
		}
	}

	common := fmt.Sprintf(`class="lab-payload" data-lab-family="flash" data-lab-method="%s" data-lab-mode="%s" data-lab-dur="%.3f" data-lab-interval="%.3f" fill="%s" shape-rendering="geometricPrecision"`,
		p.Method, p.FlashMode, durS, ivS, p.Color)

	switch p.Method {
	case "css":
		kf := p.ID + "_fkf"
		pct := math.Min(90, (durS/ivS)*100.0)
		iter := "infinite"
		totalDur := ivS
		if p.FlashMode == "single-shot" {
			iter = "1"
			totalDur = durS
			pct = 100
		}
		delay := p.StaggerDelayS
		css := fmt.Sprintf(`@keyframes %s { 0%% { opacity: 0; transform: scale(0.85); } %.1f%% { opacity: 0.95; transform: scale(1.08); } %.1f%%, 100%% { opacity: 0; transform: scale(1.0); } }
#%s .lab-payload { transform-box: fill-box; transform-origin: center; animation: %s %.3fs ease-out %.3fs %s both; }`,
			kf, pct*0.25, pct, p.ID, kf, totalDur, delay, iter)
		shape, _ := buildShape(cx, cy, common)
		return RenderedEffect{CSS: css, SVGInner: shape}

	case "smil":
		totalDur := ivS
		rep := "indefinite"
		kt := math.Min(0.9, durS/totalDur)
		if p.FlashMode == "single-shot" {
			totalDur = durS
			rep = "1"
			kt = 1.0
		}
		animO := fmt.Sprintf(`<animate attributeName="opacity" values="0;0.95;0;0" keyTimes="0;%.3f;%.3f;1" dur="%.3fs" begin="%.3fs" repeatCount="%s" fill="freeze" />`,
			kt*0.25, kt, totalDur, p.StaggerDelayS, rep)
		animS := fmt.Sprintf(`<animateTransform attributeName="transform" type="scale" values="0.85;1.08;1.0;1.0" keyTimes="0;%.3f;%.3f;1" dur="%.3fs" begin="%.3fs" repeatCount="%s" fill="freeze" />`,
			kt*0.25, kt, totalDur, p.StaggerDelayS, rep)
		if p.FlashMode == "single-shot" {
			animO = fmt.Sprintf(`<animate attributeName="opacity" values="0;0.95;0" keyTimes="0;0.25;1" dur="%.3fs" begin="%.3fs" repeatCount="1" fill="freeze" />`,
				totalDur, p.StaggerDelayS)
			animS = fmt.Sprintf(`<animateTransform attributeName="transform" type="scale" values="0.85;1.08;1.0" keyTimes="0;0.25;1" dur="%.3fs" begin="%.3fs" repeatCount="1" fill="freeze" />`,
				totalDur, p.StaggerDelayS)
		}
		shape, tag := buildShape(0, 0, common+" opacity=\"0\"")
		shape = strings.TrimSuffix(shape, " />") + ">" + animO + animS + "</" + tag + ">"
		return RenderedEffect{SVGInner: fmt.Sprintf(`<g transform="translate(%.2f,%.2f)">%s</g>`, cx, cy, shape)}

	case "javascript-raf":
		js := fmt.Sprintf(`window.__labRafDrivers.push({ id: %q, kind: 'flash', dur: %.3f, interval: %.3f, stagger: %.3f, single: %v, perInstance: %v });`,
			p.ID, durS, ivS, p.StaggerDelayS, p.FlashMode == "single-shot", p.RAFScheduling == "per-instance")
		shape, _ := buildShape(0, 0, common+" opacity=\"0\"")
		return RenderedEffect{SVGInner: fmt.Sprintf(`<g transform="translate(%.2f,%.2f)">%s</g>`, cx, cy, shape), JSDriver: js}

	default:
		shape, _ := buildShape(cx, cy, common)
		return RenderedEffect{SVGInner: shape}
	}
}

// ---------------------------------------------------------------------------
// 4a. Flow-Line (animated dashed line, no travelling particle)
// ---------------------------------------------------------------------------
type flowLineGen struct{}

func (flowLineGen) Family() string      { return "flow-line" }
func (flowLineGen) Description() string { return "Line-based connector flow using animated dashes" }
func (flowLineGen) SupportedMethods() []string {
	return []string{"css", "smil", "javascript-raf", "static"}
}
func (f flowLineGen) SupportsMethod(m string) bool { return containsStr(f.SupportedMethods(), m) }

func (flowLineGen) Render(p InstanceParams, viewW, viewH int) RenderedEffect {
	x1, y1 := 14.0, float64(viewH)*0.65
	x2, y2 := float64(viewW)-14.0, float64(viewH)*0.35
	pathD := fmt.Sprintf("M %.1f %.1f L %.1f %.1f", x1, y1, x2, y2)
	if p.Shape == "curved" {
		pathD = fmt.Sprintf("M %.1f %.1f Q %.1f %.1f %.1f %.1f", x1, y1, float64(viewW)/2, 8.0, x2, y2)
	}
	dur := math.Max(0.3, p.Speed)
	dir := "cw"
	toOff := "-1000"
	if p.Direction == "ccw" {
		dir = "ccw"
		toOff = "1000"
	}

	common := fmt.Sprintf(`class="lab-payload" data-lab-family="flow-line" data-lab-method="%s" data-lab-dir="%s" data-lab-dur="%.3f" d="%s" pathLength="1000" fill="none" stroke="%s" stroke-width="2.5" stroke-dasharray="140 160" stroke-linecap="round"`,
		p.Method, dir, dur, pathD, p.Color)

	switch p.Method {
	case "css":
		kf := p.ID + "_flkf"
		css := fmt.Sprintf(`@keyframes %s { from { stroke-dashoffset: 0px; } to { stroke-dashoffset: %spx; } }
#%s .lab-payload { animation: %s %.3fs linear infinite; }`, kf, toOff, p.ID, kf, dur)
		return RenderedEffect{CSS: css, SVGInner: fmt.Sprintf(`<path %s />`, common)}

	case "smil":
		anim := fmt.Sprintf(`<animate attributeName="stroke-dashoffset" from="0" to="%s" dur="%.3fs" repeatCount="indefinite" />`, toOff, dur)
		return RenderedEffect{SVGInner: fmt.Sprintf(`<path %s>%s</path>`, common, anim)}

	case "javascript-raf":
		js := fmt.Sprintf(`window.__labRafDrivers.push({ id: %q, kind: 'tracer', dur: %.3f, dir: %q, perInstance: %v });`,
			p.ID, dur, dir, p.RAFScheduling == "per-instance")
		return RenderedEffect{SVGInner: fmt.Sprintf(`<path %s />`, common), JSDriver: js}

	default:
		return RenderedEffect{SVGInner: fmt.Sprintf(`<path %s />`, common)}
	}
}

// ---------------------------------------------------------------------------
// 4b. Flow-Particle (distinct object travelling along a path)
// ---------------------------------------------------------------------------
type flowParticleGen struct{}

func (flowParticleGen) Family() string      { return "flow-particle" }
func (flowParticleGen) Description() string { return "Distinct particle object travelling along a connector path" }
func (flowParticleGen) SupportedMethods() []string {
	return []string{"smil", "css", "javascript-raf", "static"}
}
func (f flowParticleGen) SupportsMethod(m string) bool { return containsStr(f.SupportedMethods(), m) }

func (flowParticleGen) Render(p InstanceParams, viewW, viewH int) RenderedEffect {
	x1, y1 := 14.0, float64(viewH)*0.65
	x2, y2 := float64(viewW)-14.0, float64(viewH)*0.35
	pathD := fmt.Sprintf("M %.1f %.1f L %.1f %.1f", x1, y1, x2, y2)
	if p.Shape == "curved" {
		pathD = fmt.Sprintf("M %.1f %.1f Q %.1f %.1f %.1f %.1f", x1, y1, float64(viewW)/2, 8.0, x2, y2)
	}
	dur := math.Max(0.3, p.Speed)
	pathID := p.ID + "_path"
	guide := fmt.Sprintf(`<path id="%s" class="lab-guide" d="%s" fill="none" stroke="%s" stroke-width="1.5" stroke-opacity="0.35" stroke-dasharray="6 4" />`, pathID, pathD, p.Color)

	switch p.Method {
	case "smil":
		particle := fmt.Sprintf(`<circle class="lab-payload" data-lab-family="flow-particle" data-lab-method="smil" data-lab-dur="%.3f" r="4.5" fill="%s"><animateMotion calcMode="linear" dur="%.3fs" repeatCount="indefinite" rotate="auto"><mpath href="#%s" /></animateMotion></circle>`,
			dur, p.Color, dur, pathID)
		return RenderedEffect{SVGInner: guide + particle}

	case "css":
		kf := p.ID + "_fpkf"
		css := fmt.Sprintf(`@keyframes %s { from { offset-distance: 0%%; } to { offset-distance: 100%%; } }
#%s .lab-payload { offset-path: path('%s'); animation: %s %.3fs linear infinite; }`, kf, p.ID, pathD, kf, dur)
		particle := fmt.Sprintf(`<circle class="lab-payload" data-lab-family="flow-particle" data-lab-method="css" data-lab-dur="%.3f" r="4.5" fill="%s" />`, dur, p.Color)
		return RenderedEffect{CSS: css, SVGInner: guide + particle}

	case "javascript-raf":
		js := fmt.Sprintf(`window.__labRafDrivers.push({ id: %q, kind: 'particle', pathId: %q, dur: %.3f, perInstance: %v });`,
			p.ID, pathID, dur, p.RAFScheduling == "per-instance")
		particle := fmt.Sprintf(`<circle class="lab-payload" data-lab-family="flow-particle" data-lab-method="javascript-raf" data-lab-dur="%.3f" cx="%.1f" cy="%.1f" r="4.5" fill="%s" />`,
			dur, x1, y1, p.Color)
		return RenderedEffect{SVGInner: guide + particle, JSDriver: js}

	default:
		particle := fmt.Sprintf(`<circle class="lab-payload" data-lab-family="flow-particle" data-lab-method="static" cx="%.1f" cy="%.1f" r="4.5" fill="%s" />`,
			(x1+x2)/2, (y1+y2)/2, p.Color)
		return RenderedEffect{SVGInner: guide + particle}
	}
}

// ---------------------------------------------------------------------------
// 5. Static Gauge (static or genuine data-driven updates)
// ---------------------------------------------------------------------------
type gaugeGen struct{}

func (gaugeGen) Family() string      { return "static-gauge" }
func (gaugeGen) Description() string { return "Static or data-driven arc gauge for mixed dashboard workloads" }
func (gaugeGen) SupportedMethods() []string {
	return []string{"static", "data-driven"}
}
func (g gaugeGen) SupportsMethod(m string) bool { return containsStr(g.SupportedMethods(), m) }

func (gaugeGen) Render(p InstanceParams, viewW, viewH int) RenderedEffect {
	cx, cy := float64(viewW)/2.0, float64(viewH)/2.0
	r := math.Max(10, math.Min(float64(p.Width), float64(p.Height))/2.0-4)
	bg := fmt.Sprintf(`<circle cx="%.2f" cy="%.2f" r="%.2f" fill="none" stroke="#30363d" stroke-width="5" />`, cx, cy, r)
	arc := fmt.Sprintf(`<circle class="lab-payload" data-lab-family="static-gauge" data-lab-method="%s" cx="%.2f" cy="%.2f" r="%.2f" pathLength="100" fill="none" stroke="%s" stroke-width="5" stroke-dasharray="68 100" stroke-linecap="round" transform="rotate(-90 %.2f %.2f)" />`,
		p.Method, cx, cy, r, p.Color, cx, cy)
	if p.Method == "data-driven" {
		js := fmt.Sprintf(`window.__labDataDrivers.push({ id: %q, baseVal: 68 });`, p.ID)
		return RenderedEffect{SVGInner: bg + arc, JSDriver: js}
	}
	return RenderedEffect{SVGInner: bg + arc}
}
