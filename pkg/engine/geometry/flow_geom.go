package geometry

import (
	"fmt"
	"math"
	"strings"
)

// FlowBounds represents axis-aligned bounding box coordinates in pixels.
type FlowBounds struct {
	MinX   float64 `json:"minX"`
	MinY   float64 `json:"minY"`
	MaxX   float64 `json:"maxX"`
	MaxY   float64 `json:"maxY"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// FlowGeometryDescription contains the complete calculated geometry for a flow connector.
type FlowGeometryDescription struct {
	PathD    string     `json:"pathD"`
	Midpoint Point      `json:"midpoint"`
	Bounds   FlowBounds `json:"bounds"`
	Length   float64    `json:"length"`
	StartPt  Point      `json:"startPt"`
	EndPt    Point      `json:"endPt"`
}

// ComputeFlowBounds samples the flow path to calculate its true axis-aligned bounding box.
func ComputeFlowBounds(nodes []Point, curvature float64, width, height float64, routeMode string) (FlowBounds, Point) {
	if len(nodes) == 0 {
		return FlowBounds{}, Point{}
	}
	mid := GetFlowMidpoint(nodes, curvature, width, height)
	if len(nodes) == 1 {
		p := Point{X: px(nodes[0].X, width), Y: py(nodes[0].Y, height)}
		return FlowBounds{
			MinX:   p.X,
			MinY:   p.Y,
			MaxX:   p.X,
			MaxY:   p.Y,
			Width:  0,
			Height: 0,
		}, mid
	}

	samples := 40
	minX := math.MaxFloat64
	minY := math.MaxFloat64
	maxX := -math.MaxFloat64
	maxY := -math.MaxFloat64

	for i := 0; i <= samples; i++ {
		t := float64(i) / float64(samples)
		pt, _ := SampleFlowPathAtProgress(nodes, curvature, width, height, routeMode, t)
		if pt.X < minX {
			minX = pt.X
		}
		if pt.X > maxX {
			maxX = pt.X
		}
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}

	return FlowBounds{
		MinX:   math.Round(minX*100) / 100,
		MinY:   math.Round(minY*100) / 100,
		MaxX:   math.Round(maxX*100) / 100,
		MaxY:   math.Round(maxY*100) / 100,
		Width:  math.Round((maxX-minX)*100) / 100,
		Height: math.Round((maxY-minY)*100) / 100,
	}, mid
}

// ComputeFlowLength approximates the total pixel length of the flow path.
func ComputeFlowLength(nodes []Point, curvature float64, width, height float64, routeMode string) float64 {
	if len(nodes) < 2 {
		return 0
	}
	samples := 50
	totalLen := 0.0
	prev, _ := SampleFlowPathAtProgress(nodes, curvature, width, height, routeMode, 0.0)

	for i := 1; i <= samples; i++ {
		t := float64(i) / float64(samples)
		curr, _ := SampleFlowPathAtProgress(nodes, curvature, width, height, routeMode, t)
		dx := curr.X - prev.X
		dy := curr.Y - prev.Y
		totalLen += math.Sqrt(dx*dx + dy*dy)
		prev = curr
	}

	return math.Round(totalLen*100) / 100
}

// DistanceToFlowPath calculates the minimum Euclidean distance in pixels from (px, py) to the flow path.
func DistanceToFlowPath(nodes []Point, curvature float64, width, height float64, routeMode string, testPx, testPy float64) float64 {
	if len(nodes) < 2 {
		if len(nodes) == 1 {
			p := Point{X: px(nodes[0].X, width), Y: py(nodes[0].Y, height)}
			dx := testPx - p.X
			dy := testPy - p.Y
			return math.Sqrt(dx*dx + dy*dy)
		}
		return math.MaxFloat64
	}

	samples := 40
	minDist := math.MaxFloat64
	prev, _ := SampleFlowPathAtProgress(nodes, curvature, width, height, routeMode, 0.0)

	for i := 1; i <= samples; i++ {
		t := float64(i) / float64(samples)
		curr, _ := SampleFlowPathAtProgress(nodes, curvature, width, height, routeMode, t)

		// Point-to-segment distance from testPt to segment (prev, curr)
		segDx := curr.X - prev.X
		segDy := curr.Y - prev.Y
		lenSq := segDx*segDx + segDy*segDy

		var dist float64
		if lenSq <= 0.0001 {
			dx := testPx - prev.X
			dy := testPy - prev.Y
			dist = math.Sqrt(dx*dx + dy*dy)
		} else {
			u := ((testPx-prev.X)*segDx + (testPy-prev.Y)*segDy) / lenSq
			if u < 0 {
				u = 0
			} else if u > 1 {
				u = 1
			}
			closestX := prev.X + u*segDx
			closestY := prev.Y + u*segDy
			dx := testPx - closestX
			dy := testPy - closestY
			dist = math.Sqrt(dx*dx + dy*dy)
		}

		if dist < minDist {
			minDist = dist
		}
		prev = curr
	}

	return math.Round(minDist*100) / 100
}

// DescribeFlowGeometry returns the full geometric description for a sequence of flow nodes.
func DescribeFlowGeometry(nodes []Point, curvature float64, width, height float64, routeMode string) FlowGeometryDescription {
	path := GenerateFlowPath(nodes, curvature, width, height, routeMode)
	bounds, mid := ComputeFlowBounds(nodes, curvature, width, height, routeMode)
	length := ComputeFlowLength(nodes, curvature, width, height, routeMode)

	var startPt, endPt Point
	if len(nodes) > 0 {
		startPt = Point{X: px(nodes[0].X, width), Y: py(nodes[0].Y, height)}
		endPt = Point{X: px(nodes[len(nodes)-1].X, width), Y: py(nodes[len(nodes)-1].Y, height)}
	}

	return FlowGeometryDescription{
		PathD:    path,
		Midpoint: mid,
		Bounds:   bounds,
		Length:   length,
		StartPt:  startPt,
		EndPt:    endPt,
	}
}

// CalculateNodeAnchor determines the anchor point (in percent coordinates) on a node facing the target point.
func CalculateNodeAnchor(nodeX, nodeY, nodeW, nodeH, canvasW, canvasH float64, targetX, targetY float64, preferEdge bool) Point {
	if canvasW <= 0 {
		canvasW = 1280
	}
	if canvasH <= 0 {
		canvasH = 720
	}

	center := Point{
		X: math.Round(nodeX*100) / 100,
		Y: math.Round(nodeY*100) / 100,
	}

	if !preferEdge {
		return center
	}

	// Calculate 4 cardinal edge anchors in percent coordinates
	hwPct := ((nodeW / 2.0) / canvasW) * 100.0
	hhPct := ((nodeH / 2.0) / canvasH) * 100.0

	top := Point{X: nodeX, Y: nodeY - hhPct}
	bottom := Point{X: nodeX, Y: nodeY + hhPct}
	left := Point{X: nodeX - hwPct, Y: nodeY}
	right := Point{X: nodeX + hwPct, Y: nodeY}

	candidates := []Point{top, bottom, left, right}
	bestPt := center
	bestDistSq := math.MaxFloat64

	for _, cand := range candidates {
		dx := (cand.X - targetX) * canvasW / 100.0
		dy := (cand.Y - targetY) * canvasH / 100.0
		distSq := dx*dx + dy*dy
		if distSq < bestDistSq {
			bestDistSq = distSq
			bestPt = cand
		}
	}

	return Point{
		X: math.Round(bestPt.X*100) / 100,
		Y: math.Round(bestPt.Y*100) / 100,
	}
}

// effectiveFlowLiteBend returns the effective bend (-100..+100) for section i by combining
// the per-section bend with the optional global curvature modifier.
func effectiveFlowLiteBend(bends []float64, sectionIdx int, globalCurvature float64) float64 {
	b := 0.0
	if sectionIdx >= 0 && sectionIdx < len(bends) {
		b = bends[sectionIdx]
	}
	eff := b + globalCurvature
	if eff < -100 {
		eff = -100
	} else if eff > 100 {
		eff = 100
	}
	return eff
}

// flowLiteSectionControl computes the quadratic Bezier control point in pixel coordinates for section i.
func flowLiteSectionControl(p1, p2 Point, bend float64) (Point, bool) {
	if math.Abs(bend) < 0.01 {
		return Point{X: (p1.X + p2.X) / 2.0, Y: (p1.Y + p2.Y) / 2.0}, false
	}
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	dist := math.Hypot(dx, dy)
	if dist < 1e-6 {
		return p1, false
	}
	nx := -dy / dist
	ny := dx / dist
	offset := dist * (bend / 100.0) * 0.5
	return Point{
		X: (p1.X+p2.X)/2.0 + nx*offset,
		Y: (p1.Y+p2.Y)/2.0 + ny*offset,
	}, true
}

// GenerateFlowLitePath generates an SVG path "d" string for a multi-point Flow Lite path
// with independent per-section bends and an optional whole-path curvature modifier.
func GenerateFlowLitePath(points []Point, bends []float64, globalCurvature, width, height float64) string {
	if len(points) == 0 {
		return ""
	}
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 720
	}
	p0 := Point{X: px(points[0].X, width), Y: py(points[0].Y, height)}
	if len(points) == 1 {
		return fmt.Sprintf("M %s %s", formatNum(p0.X), formatNum(p0.Y))
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("M %s %s", formatNum(p0.X), formatNum(p0.Y)))
	prev := p0
	for i := 0; i < len(points)-1; i++ {
		next := Point{X: px(points[i+1].X, width), Y: py(points[i+1].Y, height)}
		bend := effectiveFlowLiteBend(bends, i, globalCurvature)
		ctrl, curved := flowLiteSectionControl(prev, next, bend)
		if !curved {
			sb.WriteString(fmt.Sprintf(" L %s %s", formatNum(next.X), formatNum(next.Y)))
		} else {
			sb.WriteString(fmt.Sprintf(" Q %s %s %s %s", formatNum(ctrl.X), formatNum(ctrl.Y), formatNum(next.X), formatNum(next.Y)))
		}
		prev = next
	}
	return sb.String()
}

// SampleFlowLitePath samples a multi-point Flow Lite path in pixel coordinates for bounds, length, and hit-testing.
func SampleFlowLitePath(points []Point, bends []float64, globalCurvature, width, height float64, samplesPerSection int) []Point {
	if len(points) == 0 {
		return nil
	}
	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 720
	}
	if samplesPerSection < 4 {
		samplesPerSection = 16
	}
	p0 := Point{X: px(points[0].X, width), Y: py(points[0].Y, height)}
	if len(points) == 1 {
		return []Point{p0}
	}
	out := make([]Point, 0, (len(points)-1)*samplesPerSection+1)
	out = append(out, p0)
	prev := p0
	for i := 0; i < len(points)-1; i++ {
		next := Point{X: px(points[i+1].X, width), Y: py(points[i+1].Y, height)}
		bend := effectiveFlowLiteBend(bends, i, globalCurvature)
		ctrl, curved := flowLiteSectionControl(prev, next, bend)
		for step := 1; step <= samplesPerSection; step++ {
			t := float64(step) / float64(samplesPerSection)
			if !curved {
				out = append(out, Point{
					X: prev.X + (next.X-prev.X)*t,
					Y: prev.Y + (next.Y-prev.Y)*t,
				})
			} else {
				u := 1.0 - t
				out = append(out, Point{
					X: u*u*prev.X + 2*u*t*ctrl.X + t*t*next.X,
					Y: u*u*prev.Y + 2*u*t*ctrl.Y + t*t*next.Y,
				})
			}
		}
		prev = next
	}
	return out
}

// ComputeFlowLiteLength calculates the total arc length in pixels of a multi-point Flow Lite path.
func ComputeFlowLiteLength(points []Point, bends []float64, globalCurvature, width, height float64) float64 {
	samples := SampleFlowLitePath(points, bends, globalCurvature, width, height, 24)
	if len(samples) < 2 {
		return 0
	}
	total := 0.0
	for i := 1; i < len(samples); i++ {
		total += math.Hypot(samples[i].X-samples[i-1].X, samples[i].Y-samples[i-1].Y)
	}
	return math.Round(total*100) / 100
}

// ComputeFlowLiteBounds returns the axis-aligned bounding box and midpoint of a Flow Lite path in pixels.
func ComputeFlowLiteBounds(points []Point, bends []float64, globalCurvature, width, height float64) (FlowBounds, Point) {
	samples := SampleFlowLitePath(points, bends, globalCurvature, width, height, 20)
	if len(samples) == 0 {
		return FlowBounds{}, Point{}
	}
	minX, maxX := samples[0].X, samples[0].X
	minY, maxY := samples[0].Y, samples[0].Y
	for _, pt := range samples[1:] {
		if pt.X < minX {
			minX = pt.X
		}
		if pt.X > maxX {
			maxX = pt.X
		}
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}
	mid := samples[len(samples)/2]
	return FlowBounds{
		MinX:   math.Round(minX*100) / 100,
		MinY:   math.Round(minY*100) / 100,
		MaxX:   math.Round(maxX*100) / 100,
		MaxY:   math.Round(maxY*100) / 100,
		Width:  math.Round((maxX-minX)*100) / 100,
		Height: math.Round((maxY-minY)*100) / 100,
	}, Point{
		X: math.Round(mid.X*100) / 100,
		Y: math.Round(mid.Y*100) / 100,
	}
}

// DistanceToFlowLitePath calculates the minimum pixel distance from (testPx, testPy) to the Flow Lite path.
func DistanceToFlowLitePath(points []Point, bends []float64, globalCurvature, width, height, testPx, testPy float64) float64 {
	samples := SampleFlowLitePath(points, bends, globalCurvature, width, height, 20)
	if len(samples) == 0 {
		return math.MaxFloat64
	}
	if len(samples) == 1 {
		return math.Hypot(testPx-samples[0].X, testPy-samples[0].Y)
	}
	minDist := math.MaxFloat64
	prev := samples[0]
	for i := 1; i < len(samples); i++ {
		curr := samples[i]
		vx := curr.X - prev.X
		vy := curr.Y - prev.Y
		wx := testPx - prev.X
		wy := testPy - prev.Y
		c1 := wx*vx + wy*vy
		var dist float64
		if c1 <= 0 {
			dist = math.Hypot(testPx-prev.X, testPy-prev.Y)
		} else {
			c2 := vx*vx + vy*vy
			if c2 <= c1 {
				dist = math.Hypot(testPx-curr.X, testPy-curr.Y)
			} else {
				b := c1 / c2
				pbx := prev.X + b*vx
				pby := prev.Y + b*vy
				dist = math.Hypot(testPx-pbx, testPy-pby)
			}
		}
		if dist < minDist {
			minDist = dist
		}
		prev = curr
	}
	return math.Round(minDist*100) / 100
}
