/**
 * Low-Load Effects Module (`window.LiteEffects`) — Stage 16D-A
 *
 * Implements economical, reusable SVG effects:
 *   - Perimeter Tracer Lite (`perimeter-tracer`: rounded, rect, circle, ellipse)
 *   - Pulse Lite (`pulse`: circle, square, diamond, cross, ring)
 *   - Flash Lite (`flash`: diamond, circle, square, cross, star)
 *
 * Go/WASM (`window.EngineBridge`) is authoritative for object creation,
 * validation, persistence, layer ordering, and perimeter dash math.
 */
window.LiteEffects = {
  list: [],
  activeId: null,
  counter: 0,
  _initialized: false,

  escapeXML(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&apos;');
  },

  sanitizeColor(col, fallback = '#38bdf8') {
    if (typeof col !== 'string') return fallback;
    col = col.trim();
    if (/^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.test(col)) return col;
    if (/^rgba?\(/.test(col) || /^hsla?\(/.test(col)) return col;
    return fallback;
  },

  _migrateLiteEffect(raw) {
    if (!raw || typeof raw !== 'object') return raw;
    const le = Object.assign({}, raw);
    if (!le.originMode) {
      if (le.kind === 'perimeter-tracer') {
        const w = Number(le.width) > 0 ? Number(le.width) : 140;
        const h = Number(le.height) > 0 ? Number(le.height) : 80;
        le.x = (Number(le.x) || 100) + w / 2;
        le.y = (Number(le.y) || 100) + h / 2;
        le.width = w;
        le.height = h;
      } else if (le.kind === 'pulse' || le.kind === 'flash') {
        const r = Number(le.radius) > 0 ? Number(le.radius) : 28;
        if (!le.width || (Number(le.width) === 140 && Number(le.height) === 80)) {
          le.width = r * 2;
          le.height = r * 2;
        }
      }
      le.originMode = 'center';
    }
    if (le.kind === 'perimeter-tracer') {
      if (le.showGuide === undefined || le.showGuide === null) le.showGuide = true;
      if (le.maintainAspectRatio === undefined || le.maintainAspectRatio === null) {
        le.maintainAspectRatio = (le.variant === 'circle');
      }
    } else if (le.kind === 'pulse') {
      if (!le.minScale || le.minScale <= 0) le.minScale = 0.85;
      if (!le.maxScale || le.maxScale <= 0) le.maxScale = 1.15;
      if (le.maintainAspectRatio === undefined || le.maintainAspectRatio === null) le.maintainAspectRatio = true;
    } else if (le.kind === 'flash') {
      if (!le.dur || le.dur <= 0) le.dur = 0.10;
      if (le.maintainAspectRatio === undefined || le.maintainAspectRatio === null) le.maintainAspectRatio = true;
    }
    return le;
  },

  getLiteEffect(id) {
    const sid = String(id);
    return this.list.find((l) => String(l.id) === sid) || null;
  },

  getAllLiteEffects() {
    return this.list.slice();
  },

  createLiteEffect(initialProps = {}) {
    const kind = initialProps.kind || 'perimeter-tracer';
    const isTracer = kind === 'perimeter-tracer';
    const defaultW = isTracer ? 140 : 56;
    const defaultH = isTracer ? 80 : 56;
    const payload = Object.assign({
      kind,
      originMode: 'center',
      variant: isTracer ? 'rounded' : (kind === 'flash' ? 'diamond' : 'circle'),
      x: 240,
      y: 180,
      width: defaultW,
      height: defaultH,
      radius: 28,
      cornerRadius: 10,
      color: kind === 'flash' ? '#f59e0b' : (kind === 'pulse' ? '#22c55e' : '#38bdf8'),
      opacity: 0.9,
      strokeWidth: 2.5,
      segments: 2,
      lineGapRatio: '1:3',
      segmentStyle: 'rounded',
      startOffset: 0,
      direction: 'cw',
      speed: 3.0,
      period: kind === 'flash' ? 3.0 : 1.8,
      dur: 0.10,
      intensity: kind === 'flash' ? 0.75 : 0.85,
      minScale: 0.85,
      maxScale: 1.15,
      maintainAspectRatio: !isTracer,
      showGuide: true,
      repeat: false,
      triggerNonce: 0,
      visible: true
    }, initialProps);

    let created = null;
    if (window.EngineBridge && typeof window.EngineBridge.createLiteEffect === 'function' && window.EngineBridge.isReady()) {
      created = window.EngineBridge.createLiteEffect(payload);
    }
    if (!created) {
      this.counter += 1;
      created = Object.assign({ id: String(this.counter) }, payload);
    } else {
      created = Object.assign({}, payload, created);
      const idNum = parseInt(String(created.id), 10);
      if (!isNaN(idNum) && idNum > this.counter) this.counter = idNum;
    }
    created = this._migrateLiteEffect(created);

    const idx = this.list.findIndex((l) => String(l.id) === String(created.id));
    if (idx >= 0) this.list[idx] = created;
    else this.list.push(created);

    if (window.LayerManager) {
      if (typeof window.LayerManager.syncFromEngine === 'function' && window.EngineBridge && window.EngineBridge.isReady()) {
        window.LayerManager.syncFromEngine();
      } else if (typeof window.LayerManager.addEntry === 'function') {
        window.LayerManager.addEntry('lite', created.id);
      }
    }
    if (window.AppCoordinator && typeof window.AppCoordinator.emit === 'function') {
      window.AppCoordinator.emit('layers:changed');
    }

    return created;
  },

  updateLiteEffect(id, patch = {}) {
    const existing = this.getLiteEffect(id);
    if (!existing) return null;

    const normPatch = Object.assign({}, patch);
    if (normPatch.durationMs !== undefined) {
      const ms = Math.max(10, Math.min(1500, Number(normPatch.durationMs) || 100));
      normPatch.dur = ms / 1000;
      normPatch.duration = ms / 1000;
    }

    let updated = null;
    if (window.EngineBridge && typeof window.EngineBridge.updateLiteEffect === 'function' && window.EngineBridge.isReady()) {
      updated = window.EngineBridge.updateLiteEffect(id, normPatch);
    }
    if (!updated) {
      updated = Object.assign({}, existing, normPatch);
    } else {
      updated = Object.assign({}, existing, normPatch, updated);
    }
    updated = this._migrateLiteEffect(updated);
    const idx = this.list.findIndex((l) => String(l.id) === String(id));
    if (idx >= 0) this.list[idx] = updated;
    return updated;
  },

  deleteLiteEffect(id) {
    const sid = String(id);
    if (window.EngineBridge && typeof window.EngineBridge.deleteLiteEffect === 'function' && window.EngineBridge.isReady()) {
      window.EngineBridge.deleteLiteEffect(sid);
    }
    this.list = this.list.filter((l) => String(l.id) !== sid);
    if (String(this.activeId) === sid) this.activeId = null;
    if (window.LayerManager && typeof window.LayerManager.removeEntry === 'function') {
      window.LayerManager.removeEntry('lite', sid);
    }
    if (window.AppCoordinator && typeof window.AppCoordinator.emit === 'function') {
      window.AppCoordinator.emit('layers:changed');
    }
    return true;
  },

  getState() {
    return {
      liteEffects: this.list.map((l) => Object.assign({}, l)),
      liteEffectsCounter: this.counter
    };
  },

  setState(data) {
    if (!data) return;
    const arr = Array.isArray(data.liteEffects) ? data.liteEffects : (Array.isArray(data) ? data : []);
    this.list = arr.map((l) => this._migrateLiteEffect(l));
    this.counter = data.liteEffectsCounter || this.list.reduce((m, l) => Math.max(m, parseInt(String(l.id), 10) || 0), 0);
    this.activeId = null;
    this.renderList();
    this.syncEditorUI(null);
  },

  getLiteDefs() {
    if (this.list.length === 0) return '';
    return `<style>
@keyframes sva-lite-trace-cw {
  from { stroke-dashoffset: var(--sva-start-offset, 0); }
  to { stroke-dashoffset: calc(var(--sva-start-offset, 0) - 1000); }
}
@keyframes sva-lite-trace-ccw {
  from { stroke-dashoffset: var(--sva-start-offset, 0); }
  to { stroke-dashoffset: calc(var(--sva-start-offset, 0) + 1000); }
}
@keyframes sva-lite-pulse {
  0%, 100% { transform: scale(var(--sva-pulse-min, 0.85)); opacity: calc(var(--sva-base-op, 0.9) * (1 - var(--sva-pulse-amp, 0.55))); }
  50% { transform: scale(var(--sva-pulse-max, 1.15)); opacity: var(--sva-base-op, 0.9); }
}
@keyframes sva-lite-flash {
  0% { opacity: 0; transform: scale(0.85); }
  25% { opacity: var(--sva-flash-int, 0.75); transform: scale(1.08); }
  100% { opacity: 0; transform: scale(1.0); }
}
.sva-lite-tracer-anim {
  animation-timing-function: linear;
  animation-iteration-count: infinite;
}
.sva-lite-pulse-anim {
  transform-box: fill-box;
  transform-origin: center;
  animation-name: sva-lite-pulse;
  animation-timing-function: ease-in-out;
  animation-iteration-count: infinite;
}
.sva-lite-flash-anim {
  transform-box: fill-box;
  transform-origin: center;
  animation-name: sva-lite-flash;
  animation-timing-function: ease-out;
  animation-fill-mode: both;
}
[data-sva-widget-instance] .lite-editor-marker {
  display: none !important;
}
@media (prefers-reduced-motion: reduce) {
  .sva-lite-tracer-anim, .sva-lite-pulse-anim, .sva-lite-flash-anim {
    animation: none !important;
  }
}
</style>`;
  },

  _resolveGeometry(le) {
    let cx = Number(le.x) || 240;
    let cy = Number(le.y) || 180;
    const isTracer = le.kind === 'perimeter-tracer';
    const defDim = isTracer ? 140 : ((Number(le.radius) || 28) * 2);
    let w = Math.max(12, Number(le.width) || defDim);
    let h = Math.max(12, Number(le.height) || (isTracer ? 80 : defDim));
    if (isTracer && le.variant === 'circle') {
      h = w;
    }
    let cr = Math.max(0, Number(le.cornerRadius != null ? le.cornerRadius : 10));

    if (le.targetId && window.DiagramNodes && typeof window.DiagramNodes.getNode === 'function') {
      const cleanNodeId = String(le.targetId).replace(/^node-/, '');
      const node = window.DiagramNodes.getNode(cleanNodeId);
      if (node) {
        w = Number(node.width) || w;
        h = Number(node.height) || h;
        const cw = (window.Stage && window.Stage.canvasWidth) || 1280;
        const ch = (window.Stage && window.Stage.canvasHeight) || 720;
        cx = ((Number(node.x) || 50) / 100) * cw;
        cy = ((Number(node.y) || 50) / 100) * ch;
      }
    }
    const rx = w / 2;
    const ry = h / 2;
    const r = Math.min(rx, ry);
    return { cx, cy, x: cx - rx, y: cy - ry, w, h, rx, ry, r, cr };
  },

  getBoundingBox(le) {
    if (!le) return { cx: 0, cy: 0, x: 0, y: 0, width: 0, height: 0 };
    const g = this._resolveGeometry(le);
    return {
      cx: g.cx,
      cy: g.cy,
      x: g.x,
      y: g.y,
      width: g.w,
      height: g.h
    };
  },

  _buildShapeGeometry(variant, cx, cy, w, h, extraAttrs) {
    const rx = w / 2;
    const ry = h / 2;
    if (variant === 'square' || variant === 'rect') {
      return `<rect x="${(cx - rx).toFixed(2)}" y="${(cy - ry).toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="3" ${extraAttrs} />`;
    }
    if (variant === 'diamond') {
      const pts = `${cx.toFixed(2)},${(cy - ry).toFixed(2)} ${(cx + rx).toFixed(2)},${cy.toFixed(2)} ${cx.toFixed(2)},${(cy + ry).toFixed(2)} ${(cx - rx).toFixed(2)},${cy.toFixed(2)}`;
      return `<polygon points="${pts}" ${extraAttrs} />`;
    }
    if (variant === 'cross') {
      const armX = rx * 0.35;
      const armY = ry * 0.35;
      const d = `M ${(cx - armX).toFixed(2)},${(cy - ry).toFixed(2)} L ${(cx + armX).toFixed(2)},${(cy - ry).toFixed(2)} L ${(cx + armX).toFixed(2)},${(cy - armY).toFixed(2)} L ${(cx + rx).toFixed(2)},${(cy - armY).toFixed(2)} L ${(cx + rx).toFixed(2)},${(cy + armY).toFixed(2)} L ${(cx + armX).toFixed(2)},${(cy + armY).toFixed(2)} L ${(cx + armX).toFixed(2)},${(cy + ry).toFixed(2)} L ${(cx - armX).toFixed(2)},${(cy + ry).toFixed(2)} L ${(cx - armX).toFixed(2)},${(cy + armY).toFixed(2)} L ${(cx - rx).toFixed(2)},${(cy + armY).toFixed(2)} L ${(cx - rx).toFixed(2)},${(cy - armY).toFixed(2)} L ${(cx - armX).toFixed(2)},${(cy - armY).toFixed(2)} Z`;
      return `<path d="${d}" ${extraAttrs} />`;
    }
    if (variant === 'star') {
      const pts = [];
      for (let i = 0; i < 10; i++) {
        const ang = (Math.PI / 5) * i - Math.PI / 2;
        const radX = i % 2 === 0 ? rx : rx * 0.44;
        const radY = i % 2 === 0 ? ry : ry * 0.44;
        pts.push(`${(cx + Math.cos(ang) * radX).toFixed(2)},${(cy + Math.sin(ang) * radY).toFixed(2)}`);
      }
      return `<polygon points="${pts.join(' ')}" ${extraAttrs} />`;
    }
    if (Math.abs(w - h) > 0.5) {
      return `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${rx.toFixed(2)}" ry="${ry.toFixed(2)}" ${extraAttrs} />`;
    }
    return `<circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="${rx.toFixed(2)}" ${extraAttrs} />`;
  },

  renderLiteElement(le) {
    if (!le || le.visible === false) return '';
    const color = this.sanitizeColor(le.color, '#38bdf8');
    const opacity = le.opacity != null ? Math.max(0, Math.min(1, Number(le.opacity))) : 0.9;
    const sw = Number(le.strokeWidth) || 2.5;
    const { cx, cy, x, y, w, h, rx, ry, cr } = this._resolveGeometry(le);
    const nameAttr = le.name ? ` data-semantic-name="${this.escapeXML(le.name)}"` : '';
    const baseAttrs = `id="lite-${le.id}" class="diagram-lite-instance" data-lite-id="${le.id}" data-lite-kind="${this.escapeXML(le.kind)}" data-semantic-target="lite-${le.id}"${nameAttr}`;

    if (le.kind === 'perimeter-tracer') {
      const segs = Math.max(1, Math.min(16, Number(le.segments) || 2));
      const dashInfo = (window.EngineBridge && typeof window.EngineBridge.computePerimeterDash === 'function')
        ? window.EngineBridge.computePerimeterDash(segs, le.lineGapRatio || '1:3')
        : { dashLen: 125, gapLen: 375 };

      let dashArray = `${dashInfo.dashLen} ${dashInfo.gapLen}`;
      if (le.segmentStyle === 'dashed' && dashInfo.dashLen > 12) {
        const subDash = Math.max(4, Math.round(dashInfo.dashLen / 4));
        const subGap = Math.max(3, Math.round(subDash * 0.5));
        const count = Math.max(1, Math.floor(dashInfo.dashLen / (subDash + subGap)));
        const used = count * (subDash + subGap) - subGap;
        const remGap = Math.max(1, dashInfo.gapLen + (dashInfo.dashLen - used));
        const parts = [];
        for (let i = 0; i < count; i++) {
          parts.push(String(subDash));
          parts.push(i === count - 1 ? remGap.toFixed(2) : String(subGap));
        }
        dashArray = parts.join(' ');
      }

      const linecap = le.segmentStyle === 'rounded' ? 'round' : 'butt';
      const dur = Math.max(0.2, Number(le.speed) || 3.0);
      const animName = le.direction === 'ccw' ? 'sva-lite-trace-ccw' : 'sva-lite-trace-cw';
      const startOffsetUnits = -((Number(le.startOffset) || 0) / 100) * 1000;
      const styleStr = `animation-name:${animName};animation-duration:${dur}s;--sva-base-dur:${dur}s;--sva-start-offset:${startOffsetUnits};`;

      let shapeMarkup = '';
      let guideMarkup = '';
      let hitMarkup = '';
      const variant = le.variant || 'rounded';
      const payloadAttrs = `class="lite-payload sva-lite-tracer-anim" data-semantic-role="payload" pathLength="1000" fill="none" stroke="${color}" stroke-width="${sw}" stroke-opacity="${opacity}" stroke-dasharray="${dashArray}" stroke-linecap="${linecap}" style="${styleStr}" pointer-events="none"`;
      const hitAttrs = `class="lite-hit-target" data-semantic-role="hit-target" fill="transparent" stroke="transparent" stroke-width="${Math.max(16, sw + 12)}" pointer-events="stroke" style="cursor:pointer;"`;
      const guideAttrs = `class="lite-guide-track" fill="none" stroke="${color}" stroke-width="${Math.max(1, sw * 0.55).toFixed(2)}" stroke-opacity="${(opacity * 0.22).toFixed(2)}" pointer-events="none"`;
      const editorGuideAttrs = `fill="none" stroke="${color}" stroke-width="1" stroke-opacity="0.25" stroke-dasharray="4 4" pointer-events="none"`;

      if (variant === 'circle') {
        const crR = Math.max(6, rx);
        shapeMarkup = `<circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="${crR.toFixed(2)}" ${payloadAttrs} />`;
        hitMarkup = `<circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="${crR.toFixed(2)}" ${hitAttrs} />`;
        guideMarkup = le.showGuide !== false
          ? `<circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="${crR.toFixed(2)}" ${guideAttrs} />`
          : `<g class="lite-editor-marker" data-editor-only="true"><circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="${crR.toFixed(2)}" ${editorGuideAttrs} /></g>`;
      } else if (variant === 'ellipse') {
        shapeMarkup = `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${rx.toFixed(2)}" ry="${ry.toFixed(2)}" ${payloadAttrs} />`;
        hitMarkup = `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${rx.toFixed(2)}" ry="${ry.toFixed(2)}" ${hitAttrs} />`;
        guideMarkup = le.showGuide !== false
          ? `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${rx.toFixed(2)}" ry="${ry.toFixed(2)}" ${guideAttrs} />`
          : `<g class="lite-editor-marker" data-editor-only="true"><ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${rx.toFixed(2)}" ry="${ry.toFixed(2)}" ${editorGuideAttrs} /></g>`;
      } else {
        const cornerR = variant === 'rounded' ? Math.min(cr, rx, ry) : 0;
        shapeMarkup = `<rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${cornerR}" ${payloadAttrs} />`;
        hitMarkup = `<rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${cornerR}" ${hitAttrs} />`;
        guideMarkup = le.showGuide !== false
          ? `<rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${cornerR}" ${guideAttrs} />`
          : `<g class="lite-editor-marker" data-editor-only="true"><rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${cornerR}" ${editorGuideAttrs} /></g>`;
      }

      return `<g ${baseAttrs}>${hitMarkup}${guideMarkup}${shapeMarkup}</g>`;
    }

    if (le.kind === 'pulse') {
      const period = Math.max(0.2, Number(le.period) || 1.8);
      const amp = Math.max(0.1, Math.min(1, Number(le.intensity) || 0.85));
      const minScale = Math.max(0.2, Math.min(2.0, Number(le.minScale) || 0.85));
      const maxScale = Math.max(minScale, Math.min(3.0, Number(le.maxScale) || 1.15));
      const variant = le.variant || 'circle';
      const isRing = variant === 'ring';
      const fillVal = isRing ? 'none' : color;
      const strokeVal = isRing ? color : 'none';
      const styleStr = `animation-duration:${period}s;--sva-base-dur:${period}s;--sva-base-op:${opacity};--sva-pulse-amp:${amp};--sva-pulse-min:${minScale};--sva-pulse-max:${maxScale};`;
      const payloadAttrs = `class="lite-payload sva-lite-pulse-anim" data-semantic-role="payload" data-pulse-min="${minScale}" data-pulse-max="${maxScale}" fill="${fillVal}" stroke="${strokeVal}" stroke-width="${isRing ? sw : 0}" style="${styleStr}" pointer-events="none"`;
      const hitMarkup = `<rect class="lite-hit-target" data-semantic-role="hit-target" x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" fill="transparent" pointer-events="all" style="cursor:pointer;" />`;
      const shapeMarkup = this._buildShapeGeometry(variant, cx, cy, w, h, payloadAttrs);
      return `<g ${baseAttrs}>${hitMarkup}${shapeMarkup}</g>`;
    }

    if (le.kind === 'flash') {
      const dur = Math.max(0.02, Math.min(1.5, Number(le.dur) || 0.10));
      const durMs = Math.round(dur * 1000);
      const intensity = Math.max(0.1, Math.min(1, Number(le.intensity) || 0.75));
      const variant = le.variant || 'diamond';
      const iterCount = le.repeat ? 'infinite' : '1';
      const styleStr = `animation-duration:${dur}s;animation-iteration-count:${iterCount};--sva-base-dur:${dur}s;--sva-flash-int:${intensity};`;
      const payloadAttrs = `class="lite-payload lite-flash-anim sva-lite-flash-anim" data-semantic-role="payload" data-trigger-nonce="${le.triggerNonce || 0}" data-flash-duration-ms="${durMs}" fill="${color}" style="${styleStr}" pointer-events="none"`;
      const hitMarkup = `<rect class="lite-hit-target" data-semantic-role="hit-target" x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" fill="transparent" pointer-events="all" style="cursor:pointer;" />`;
      const markerAttrs = `fill="none" stroke="${color}" stroke-opacity="0.58" stroke-width="1.4" stroke-dasharray="4 3" pointer-events="none"`;
      const markerShape = this._buildShapeGeometry(variant, cx, cy, w, h, markerAttrs);
      const markerCross = `<circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="3" fill="${color}" fill-opacity="0.75" pointer-events="none" />`;
      const editorMarker = `<g class="lite-editor-marker" data-editor-only="true" pointer-events="none">${markerShape}${markerCross}</g>`;
      const shapeMarkup = this._buildShapeGeometry(variant, cx, cy, w, h, payloadAttrs);
      return `<g ${baseAttrs}>${hitMarkup}${editorMarker}${shapeMarkup}</g>`;
    }

    return '';
  },

  injectIntoSVG(svgString) {
    if (!svgString || this.list.length === 0) return svgString;
    const defs = this.getLiteDefs();
    const elements = this.list.map((le) => this.renderLiteElement(le)).join('\n');
    const block = `\n${defs}\n<g id="lite-effects-layer" class="lite-effects-layer">\n${elements}\n</g>\n`;
    if (svgString.includes('<!-- INJECT_CAMERA_RIG_ELEMENTS -->')) {
      return svgString.replace('<!-- INJECT_CAMERA_RIG_ELEMENTS -->', block + '<!-- INJECT_CAMERA_RIG_ELEMENTS -->');
    }
    return svgString.replace('</svg>', block + '</svg>');
  },

  triggerFlash(id) {
    const le = this.getLiteEffect(id);
    if (!le) return null;
    const nextNonce = (Number(le.triggerNonce) || 0) + 1;
    const updated = this.updateLiteEffect(id, { triggerNonce: nextNonce });
    this.triggerAppRender();
    const el = typeof document !== 'undefined'
      ? document.querySelector(`#svgContainer [data-lite-id="${id}"] .lite-flash-anim`)
      : null;
    if (el && el.style) {
      el.style.animationName = 'none';
      try { void el.getBoundingClientRect(); } catch (_) {}
      el.style.animationName = 'sva-lite-flash';
    }
    return updated;
  },

  triggerAppRender() {
    if (window.AppCoordinator && typeof window.AppCoordinator.requestRender === 'function') {
      window.AppCoordinator.requestRender();
      if (typeof window.AppCoordinator.updateActiveOverlays === 'function') {
        window.AppCoordinator.updateActiveOverlays();
      }
    } else if (typeof window.triggerRender === 'function') {
      window.triggerRender();
    }
  },

  // =========================================================================
  // UI Presentation & Event Bindings
  // =========================================================================

  init() {
    if (this._initialized || typeof document === 'undefined') return;
    this._initialized = true;

    const bindBtn = (id, fn) => {
      const el = document.getElementById(id);
      if (el) el.addEventListener('click', fn);
    };

    bindBtn('btnAddLiteTracer', () => this.addByKind('perimeter-tracer'));
    bindBtn('btnAddLitePulse', () => this.addByKind('pulse'));
    bindBtn('btnAddLiteFlash', () => this.addByKind('flash'));
    bindBtn('btnTriggerLiteFlash', () => {
      if (this.activeId) this.triggerFlash(this.activeId);
    });

    const bindField = (elId, prop, isNum = false, isBool = false) => {
      const el = document.getElementById(elId);
      if (!el) return;
      const evt = (el.type === 'color' || el.type === 'text' || el.type === 'number' || el.type === 'range') ? 'input' : 'change';
      el.addEventListener(evt, (e) => {
        let val = isBool ? e.target.checked : e.target.value;
        if (isNum) val = parseFloat(val);
        this.updateActive(prop, val);
      });
      if (evt === 'input') {
        el.addEventListener('change', () => {
          if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
        });
      }
    };

    const bindPair = (baseId, prop, isInt = false) => {
      const slider = document.getElementById(baseId + 'Slider');
      const num = document.getElementById(baseId + 'Num');
      const handle = (src, dst) => {
        if (!src) return;
        src.addEventListener('input', (e) => {
          const val = isInt ? parseInt(e.target.value, 10) : parseFloat(e.target.value);
          if (!Number.isFinite(val)) return;
          if (dst) dst.value = val;
          this.updateActive(prop, val);
        });
        src.addEventListener('change', () => {
          if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
        });
      };
      handle(slider, num);
      handle(num, slider);
    };

    const deselectBtn = document.getElementById('btnDeselectLite');
    if (deselectBtn) deselectBtn.addEventListener('click', () => this.deselectLite());

    bindField('liteNameInput', 'name');
    bindField('liteXNum', 'x', true);
    bindField('liteYNum', 'y', true);
    bindField('liteVariantSelect', 'variant');
    bindField('liteColorPicker', 'color');
    bindPair('liteOpacity', 'opacity');
    bindPair('liteWidth', 'width', true);
    bindPair('liteHeight', 'height', true);
    bindField('liteLockAspectChk', 'maintainAspectRatio', false, true);
    bindField('liteShowGuideChk', 'showGuide', false, true);
    bindPair('liteCornerRadius', 'cornerRadius', true);
    bindPair('liteStrokeWidth', 'strokeWidth');
    bindPair('liteSegments', 'segments', true);
    bindField('liteRatioInput', 'lineGapRatio');
    bindField('liteDirectionSelect', 'direction');
    bindField('liteStyleSelect', 'segmentStyle');
    bindPair('liteSpeed', 'speed');
    bindPair('liteStartOffset', 'startOffset', true);
    bindPair('liteMinScale', 'minScale');
    bindPair('liteMaxScale', 'maxScale');
    bindPair('litePeriod', 'period');
    bindPair('liteFlashDur', 'durationMs', true);
    bindField('liteRepeatChk', 'repeat', false, true);
    bindField('liteVisibleChk', 'visible', false, true);
  },

  addByKind(kind) {
    const offset = (this.list.length % 5) * 28;
    const created = this.createLiteEffect({
      kind,
      x: 240 + offset,
      y: 180 + offset
    });
    this.selectLite(created.id);
    this.renderList();
    this.triggerAppRender();
    if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
    return created;
  },

  selectLite(id, opts = {}) {
    this.activeId = String(id);
    const le = this.getLiteEffect(id);
    this.syncEditorUI(le);
    this.renderList();
    if (typeof window.syncInspectorLayerSections === 'function') {
      window.syncInspectorLayerSections();
    }
    if (!opts.skipCoordinator && window.AppCoordinator && typeof window.AppCoordinator.selectObject === 'function') {
      window.AppCoordinator.selectObject('lite', String(id), { openAccordionId: 'acc-lite-effects' });
    }
  },

  handleStageClick(xPct, yPct) {
    if (!this.activeId) return false;
    const le = this.getLiteEffect(this.activeId);
    if (!le) return false;
    const cw = (window.Stage && window.Stage.canvasWidth) || 1280;
    const ch = (window.Stage && window.Stage.canvasHeight) || 720;
    const nextX = Math.round((Number(xPct) / 100) * cw);
    const nextY = Math.round((Number(yPct) / 100) * ch);
    const updated = this.updateLiteEffect(this.activeId, { x: nextX, y: nextY });
    if (updated) {
      this.syncEditorUI(updated);
      this.triggerAppRender();
      return true;
    }
    return false;
  },

  deselectLite() {
    this.activeId = null;
    this.syncEditorUI(null);
    this.renderList();
    if (typeof window.syncInspectorLayerSections === 'function') {
      window.syncInspectorLayerSections();
    }
    if (window.AppCoordinator &&
        window.AppCoordinator.activeSelection &&
        window.AppCoordinator.activeSelection.type === 'lite' &&
        !window.AppCoordinator._isCoordinatingSelection) {
      window.AppCoordinator.clearSelection();
    }
  },

  updateActive(key, value) {
    if (!this.activeId) return;
    const existing = this.getLiteEffect(this.activeId);
    if (!existing) return;

    const patch = { [key]: value };
    const isCircleTracer = existing.kind === 'perimeter-tracer' && (key === 'variant' ? value === 'circle' : existing.variant === 'circle');
    const lockAspect = isCircleTracer || (key === 'maintainAspectRatio' ? Boolean(value) : Boolean(existing.maintainAspectRatio));

    if (key === 'width' && lockAspect) {
      if (isCircleTracer || existing.width === existing.height) {
        patch.height = value;
      } else if (existing.width > 0 && existing.height > 0) {
        patch.height = Math.max(12, Math.round(value * (existing.height / existing.width)));
      }
    } else if (key === 'height' && lockAspect) {
      if (isCircleTracer || existing.width === existing.height) {
        patch.width = value;
      } else if (existing.width > 0 && existing.height > 0) {
        patch.width = Math.max(12, Math.round(value * (existing.width / existing.height)));
      }
    } else if (key === 'variant' && existing.kind === 'perimeter-tracer' && value === 'circle') {
      patch.height = existing.width || 140;
      patch.maintainAspectRatio = true;
    }

    if (key === 'minScale' && Number(value) > (Number(existing.maxScale) || 1.15)) {
      patch.maxScale = Number(value);
    } else if (key === 'maxScale' && Number(value) < (Number(existing.minScale) || 0.85)) {
      patch.minScale = Number(value);
    }

    const updated = this.updateLiteEffect(this.activeId, patch);
    if (updated) {
      this.syncEditorUI(updated);
      this.renderList();
      this.triggerAppRender();
      if (window.AppCoordinator && typeof window.AppCoordinator.emit === 'function') {
        window.AppCoordinator.emit('layers:changed');
      }
      if (typeof window.renderWidgetParametersOverview === 'function') {
        window.renderWidgetParametersOverview();
      }
      if (window.FloatingPropertiesEditor && window.FloatingPropertiesEditor.isOpen && window.FloatingPropertiesEditor.activeType === 'lite') {
        window.FloatingPropertiesEditor.syncFromActiveObject();
      }
    }
  },

  removeLite(id) {
    this.deleteLiteEffect(id);
    this.syncEditorUI(null);
    this.renderList();
    this.triggerAppRender();
    if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
  },

  syncEditorUI(le) {
    const editor = document.getElementById('liteEditorControls');
    const empty = document.getElementById('liteEmptyHint');
    if (!editor || !empty) return;
    if (!le) {
      editor.style.display = 'none';
      empty.style.display = 'block';
      return;
    }
    editor.style.display = 'block';
    empty.style.display = 'none';

    const setVal = (id, v) => {
      const el = document.getElementById(id);
      if (el && document.activeElement !== el) el.value = v;
    };
    const setPair = (baseId, v) => { setVal(baseId + 'Slider', v); setVal(baseId + 'Num', v); };
    const setChk = (id, v) => { const el = document.getElementById(id); if (el) el.checked = Boolean(v); };

    const lbl = document.getElementById('editingLiteLabel');
    if (lbl) lbl.textContent = `Editing ${le.name || (le.kind + ' ' + le.id)}`;

    const variantSel = document.getElementById('liteVariantSelect');
    if (variantSel) {
      const optsByKind = {
        'perimeter-tracer': ['rounded', 'rect', 'circle', 'ellipse'],
        'pulse': ['circle', 'square', 'diamond', 'cross', 'ring'],
        'flash': ['diamond', 'circle', 'square', 'cross', 'star']
      };
      const opts = optsByKind[le.kind] || ['circle'];
      variantSel.innerHTML = opts.map((o) => `<option value="${o}">${o}</option>`).join('');
    }

    const durMs = Math.round((Number(le.dur) || 0.10) * 1000);
    setVal('liteNameInput', le.name || '');
    setVal('liteXNum', Math.round(Number(le.x) || 240));
    setVal('liteYNum', Math.round(Number(le.y) || 180));
    setVal('liteVariantSelect', le.variant || '');
    setVal('liteColorPicker', le.color || '#38bdf8');
    setPair('liteOpacity', le.opacity != null ? le.opacity : 0.9);
    setPair('liteWidth', Math.round(Number(le.width) || (le.kind === 'perimeter-tracer' ? 140 : 56)));
    setPair('liteHeight', Math.round(Number(le.height) || (le.kind === 'perimeter-tracer' ? 80 : 56)));
    setChk('liteLockAspectChk', le.kind === 'perimeter-tracer' && le.variant === 'circle' ? true : Boolean(le.maintainAspectRatio));
    setChk('liteShowGuideChk', le.showGuide !== false);
    setPair('liteCornerRadius', Math.round(Number(le.cornerRadius != null ? le.cornerRadius : 10)));
    setPair('liteStrokeWidth', le.strokeWidth || 2.5);
    setPair('liteSegments', le.segments || 2);
    setVal('liteRatioInput', le.lineGapRatio || '1:3');
    setVal('liteDirectionSelect', le.direction || 'cw');
    setVal('liteStyleSelect', le.segmentStyle || 'rounded');
    setPair('liteSpeed', le.speed || 3.0);
    setPair('liteStartOffset', Math.round(Number(le.startOffset) || 0));
    setPair('liteMinScale', le.minScale != null ? le.minScale : 0.85);
    setPair('liteMaxScale', le.maxScale != null ? le.maxScale : 1.15);
    setPair('litePeriod', le.period || 1.8);
    setPair('liteFlashDur', durMs);
    setChk('liteRepeatChk', le.repeat);
    setChk('liteVisibleChk', le.visible !== false);

    const tracerGroup = document.getElementById('liteTracerGroup');
    const cornerRow = document.getElementById('liteCornerRadiusRow');
    const pulseGroup = document.getElementById('litePulseGroup');
    const flashGroup = document.getElementById('liteFlashGroup');
    if (tracerGroup) tracerGroup.style.display = le.kind === 'perimeter-tracer' ? 'block' : 'none';
    if (cornerRow) cornerRow.style.display = (le.kind === 'perimeter-tracer' && le.variant === 'rounded') ? '' : 'none';
    if (pulseGroup) pulseGroup.style.display = le.kind === 'pulse' ? 'block' : 'none';
    if (flashGroup) flashGroup.style.display = le.kind === 'flash' ? 'block' : 'none';

    if (typeof window.syncInspectorLayerSections === 'function') {
      window.syncInspectorLayerSections();
    }
    if (typeof window.renderWidgetParametersOverview === 'function') {
      window.renderWidgetParametersOverview();
    }
  },

  renderList() {
    const listEl = document.getElementById('liteEffectsList');
    if (!listEl) return;
    listEl.innerHTML = '';
    if (this.list.length === 0) {
      listEl.innerHTML = '<div class="stage-hint" id="emptyLiteListHint">No low-load effects active.</div>';
      return;
    }
    const iconByKind = { 'perimeter-tracer': '▭', 'pulse': '◎', 'flash': '✦' };
    const kindShort = { 'perimeter-tracer': 'Tracer', 'pulse': 'Pulse', 'flash': 'Flash' };
    this.list.forEach((le) => {
      const row = document.createElement('div');
      const isActive = String(this.activeId) === String(le.id);
      row.className = 'flow-list-item compact-instance-row' + (isActive ? ' active' : '');
      row.dataset.liteListId = String(le.id);
      const label = le.name || `${kindShort[le.kind] || le.kind}-${le.id}`;
      const meta = kindShort[le.kind] || le.kind;
      row.innerHTML = `<span class="instance-icon">${iconByKind[le.kind] || '⚡'}</span>
        <span class="instance-label">${this.escapeXML(label)}</span>
        <span class="compact-meta">${this.escapeXML(meta)}</span>
        <button type="button" class="btn-row-action btn-row-delete" title="Delete">&times;</button>`;
      row.addEventListener('click', (e) => {
        if (e.target.closest('.btn-row-delete')) {
          this.removeLite(le.id);
        } else {
          this.selectLite(le.id);
        }
      });
      listEl.appendChild(row);
    });
  },
};
