package scene

import "gitea.seaoffate.net/blackgrape/svg-animator/pkg/engine/geometry"

// deepCopyBoolPtr returns a new pointer with the copied boolean value.
func deepCopyBoolPtr(b *bool) *bool {
	if b == nil {
		return nil
	}
	v := *b
	return &v
}

// deepCopyMetadata returns a shallow copy of the metadata map (extension data).
func deepCopyMetadata(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

// DeepCopyNode creates an isolated deep copy of a Node.
func DeepCopyNode(n Node) Node {
	cp := n
	cp.Visible = deepCopyBoolPtr(n.Visible)
	cp.Metadata = deepCopyMetadata(n.Metadata)
	return cp
}

// DeepCopyFlow creates an isolated deep copy of a Flow.
func DeepCopyFlow(f Flow) Flow {
	cp := f
	if f.Nodes != nil {
		cp.Nodes = make([]geometry.Point, len(f.Nodes))
		copy(cp.Nodes, f.Nodes)
	}
	cp.Visible = deepCopyBoolPtr(f.Visible)
	cp.Metadata = deepCopyMetadata(f.Metadata)
	return cp
}

// DeepCopyGlow creates an isolated deep copy of a Glow.
func DeepCopyGlow(g Glow) Glow {
	cp := g
	cp.Visible = deepCopyBoolPtr(g.Visible)
	cp.Metadata = deepCopyMetadata(g.Metadata)
	return cp
}

// DeepCopyRipple creates an isolated deep copy of a Ripple.
func DeepCopyRipple(r Ripple) Ripple {
	cp := r
	cp.Visible = deepCopyBoolPtr(r.Visible)
	cp.Metadata = deepCopyMetadata(r.Metadata)
	return cp
}

// DeepCopyGroup creates an isolated deep copy of a Group.
func DeepCopyGroup(g Group) Group {
	cp := g
	if g.Members != nil {
		cp.Members = make([]GroupMember, len(g.Members))
		copy(cp.Members, g.Members)
	}
	cp.Visible = deepCopyBoolPtr(g.Visible)
	cp.Metadata = deepCopyMetadata(g.Metadata)
	return cp
}

// DeepCopyImage creates an isolated deep copy of an Image.
func DeepCopyImage(img Image) Image {
	cp := img
	if img.Crop != nil {
		c := *img.Crop
		cp.Crop = &c
	}
	if img.Clip != nil {
		c := *img.Clip
		cp.Clip = &c
	}
	if img.CropVersions != nil {
		cp.CropVersions = make([]CropVersion, len(img.CropVersions))
		copy(cp.CropVersions, img.CropVersions)
	}
	cp.Visible = deepCopyBoolPtr(img.Visible)
	cp.Metadata = deepCopyMetadata(img.Metadata)
	return cp
}

// DeepCopyBackground creates an isolated deep copy of BackgroundConfig.
func DeepCopyBackground(bg *BackgroundConfig) *BackgroundConfig {
	if bg == nil {
		return nil
	}
	cp := *bg
	if bg.Crop != nil {
		c := *bg.Crop
		cp.Crop = &c
	}
	if bg.CropVersions != nil {
		cp.CropVersions = make([]BackgroundCropVersion, len(bg.CropVersions))
		copy(cp.CropVersions, bg.CropVersions)
	}
	return &cp
}

// DeepCopyLayerEntry creates an isolated deep copy of a LayerEntry.
func DeepCopyLayerEntry(l LayerEntry) LayerEntry {
	cp := l
	cp.Visible = deepCopyBoolPtr(l.Visible)
	cp.Locked = deepCopyBoolPtr(l.Locked)
	return cp
}

// DeepCopySelection creates an isolated deep copy of SelectionState.
func DeepCopySelection(sel SelectionState) SelectionState {
	cp := sel
	if sel.Items != nil {
		cp.Items = make([]SelectionItem, len(sel.Items))
		copy(cp.Items, sel.Items)
	}
	return cp
}

func deepCopyFloatPtr(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// DeepCopyLiteEffect creates an isolated deep copy of a LiteEffect.
func DeepCopyLiteEffect(l LiteEffect) LiteEffect {
	cp := l
	cp.Metadata = deepCopyMetadata(l.Metadata)
	cp.MaintainAspectRatio = deepCopyBoolPtr(l.MaintainAspectRatio)
	cp.ShowGuide = deepCopyBoolPtr(l.ShowGuide)
	cp.Visible = deepCopyBoolPtr(l.Visible)
	cp.Value = deepCopyFloatPtr(l.Value)
	cp.MinValue = deepCopyFloatPtr(l.MinValue)
	cp.MaxValue = deepCopyFloatPtr(l.MaxValue)
	cp.WarnThreshold = deepCopyFloatPtr(l.WarnThreshold)
	cp.CritThreshold = deepCopyFloatPtr(l.CritThreshold)
	cp.BgOpacity = deepCopyFloatPtr(l.BgOpacity)
	if l.Thresholds != nil {
		cp.Thresholds = make([]GaugeThresholdBand, len(l.Thresholds))
		copy(cp.Thresholds, l.Thresholds)
	}
	if l.Points != nil {
		cp.Points = make([]geometry.Point, len(l.Points))
		copy(cp.Points, l.Points)
	}
	if l.Bends != nil {
		cp.Bends = make([]float64, len(l.Bends))
		copy(cp.Bends, l.Bends)
	}
	return cp
}

// DeepCopySnapshot creates a complete, deeply isolated copy of a Snapshot.
func DeepCopySnapshot(snap Snapshot) Snapshot {
	cp := snap
	cp.Background = DeepCopyBackground(snap.Background)

	if snap.Nodes != nil {
		cp.Nodes = make([]Node, len(snap.Nodes))
		for i, n := range snap.Nodes {
			cp.Nodes[i] = DeepCopyNode(n)
		}
	} else {
		cp.Nodes = make([]Node, 0)
	}

	if snap.Flows != nil {
		cp.Flows = make([]Flow, len(snap.Flows))
		for i, f := range snap.Flows {
			cp.Flows[i] = DeepCopyFlow(f)
		}
	} else {
		cp.Flows = make([]Flow, 0)
	}

	if snap.Glows != nil {
		cp.Glows = make([]Glow, len(snap.Glows))
		for i, g := range snap.Glows {
			cp.Glows[i] = DeepCopyGlow(g)
		}
	} else {
		cp.Glows = make([]Glow, 0)
	}

	if snap.Ripples != nil {
		cp.Ripples = make([]Ripple, len(snap.Ripples))
		for i, r := range snap.Ripples {
			cp.Ripples[i] = DeepCopyRipple(r)
		}
	} else {
		cp.Ripples = make([]Ripple, 0)
	}

	if snap.Groups != nil {
		cp.Groups = make([]Group, len(snap.Groups))
		for i, g := range snap.Groups {
			cp.Groups[i] = DeepCopyGroup(g)
		}
	} else {
		cp.Groups = make([]Group, 0)
	}

	if snap.Images != nil {
		cp.Images = make([]Image, len(snap.Images))
		for i, img := range snap.Images {
			cp.Images[i] = DeepCopyImage(img)
		}
	} else {
		cp.Images = make([]Image, 0)
	}

	if snap.LiteEffects != nil {
		cp.LiteEffects = make([]LiteEffect, len(snap.LiteEffects))
		for i, le := range snap.LiteEffects {
			cp.LiteEffects[i] = DeepCopyLiteEffect(le)
		}
	} else {
		cp.LiteEffects = make([]LiteEffect, 0)
	}

	if snap.Layers != nil {
		cp.Layers = make([]LayerEntry, len(snap.Layers))
		for i, l := range snap.Layers {
			cp.Layers[i] = DeepCopyLayerEntry(l)
		}
	} else {
		cp.Layers = make([]LayerEntry, 0)
	}

	if snap.Parameters != nil {
		cp.Parameters = make([]WidgetParameter, len(snap.Parameters))
		for i, p := range snap.Parameters {
			pCopy := p
			if p.Min != nil {
				v := *p.Min
				pCopy.Min = &v
			}
			if p.Max != nil {
				v := *p.Max
				pCopy.Max = &v
			}
			if p.Options != nil {
				pCopy.Options = append([]string(nil), p.Options...)
			}
			cp.Parameters[i] = pCopy
		}
	} else {
		cp.Parameters = make([]WidgetParameter, 0)
	}

	return cp
}
