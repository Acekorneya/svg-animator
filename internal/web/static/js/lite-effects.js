/**
 * Low-Load Effects Module (`window.LiteEffects`) — Stage 16D-A
 *
 * Implements economical, reusable SVG effects:
 *   - Perimeter Tracer Lite (`perimeter-tracer`: rect, rounded, circle, ellipse)
 *   - Pulse Lite (`pulse`: circle, square, diamond, cross, ring)
 *   - Flash Lite (`flash`: circle, square, diamond, cross, star)
 *
 * Go/WASM (`window.EngineBridge`) is authoritative for object creation,
 * validation, persistence, and perimeter dash math.
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

  getLiteEffect(id) {
    const sid = String(id);
    return this.list.find((l) => String(l.id) === sid) || null;
  },

  getAllLiteEffects() {
    return this.list.slice();
  },

  createLiteEffect(initialProps = {}) {
    let created = null;
    if (window.EngineBridge && typeof window.EngineBridge.createLiteEffect === 'function' && window.EngineBridge.isReady()) {
      created = window.EngineBridge.createLiteEffect(initialProps);
    }
    if (!created) {
      this.counter += 1;
      const kind = initialProps.kind || 'perimeter-tracer';
      created = Object.assign({
        id: String(this.counter),
        kind,
        variant: kind === 'perimeter-tracer' ? 'rounded' : (kind === 'flash' ? 'diamond' : 'circle'),
        x: 200,
        y: 160,
        width: 140,
        height: 80,
        radius: 26,
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
        dur: 0.22,
        intensity: kind === 'flash' ? 0.75 : 0.85,
        repeat: false,
        triggerNonce: 0,
        visible: true
      }, initialProps);
    } else {
      const idNum = parseInt(String(created.id), 10);
      if (!isNaN(idNum) && idNum > this.counter) this.counter = idNum;
    }

    const idx = this.list.findIndex((l) => String(l.id) === String(created.id));
    if (idx >= 0) this.list[idx] = created;
    else this.list.push(created);

    return created;
  },

  updateLiteEffect(id, patch = {}) {
    const existing = this.getLiteEffect(id);
    if (!existing) return null;

    let updated = null;
    if (window.EngineBridge && typeof window.EngineBridge.updateLiteEffect === 'function' && window.EngineBridge.isReady()) {
      updated = window.EngineBridge.updateLiteEffect(id, patch);
    }
    if (!updated) {
      updated = Object.assign({}, existing, patch);
    }
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
    this.list = arr.map((l) => Object.assign({}, l));
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
  0%, 100% { transform: scale(0.86); opacity: calc(var(--sva-base-op, 0.9) * (1 - var(--sva-pulse-amp, 0.65))); }
  50% { transform: scale(1.12); opacity: var(--sva-base-op, 0.9); }
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
@media (prefers-reduced-motion: reduce) {
  .sva-lite-tracer-anim, .sva-lite-pulse-anim, .sva-lite-flash-anim {
    animation: none !important;
  }
}
</style>`;
  },

  _resolveGeometry(le) {
    let x = Number(le.x) || 100;
    let y = Number(le.y) || 100;
    let w = Number(le.width) || 140;
    let h = Number(le.height) || 80;
    let r = Number(le.radius) || 26;
    let cr = Number(le.cornerRadius != null ? le.cornerRadius : 10);

    if (le.targetId && window.DiagramNodes && typeof window.DiagramNodes.getNode === 'function') {
      const cleanNodeId = String(le.targetId).replace(/^node-/, '');
      const node = window.DiagramNodes.getNode(cleanNodeId);
      if (node) {
        w = Number(node.width) || w;
        h = Number(node.height) || h;
        // DiagramNodes stores center X/Y in percentage or px; handle pixel coordinates cleanly
        const nx = Number(node.x) || 0;
        const ny = Number(node.y) || 0;
        x = nx - w / 2;
        y = ny - h / 2;
        r = Math.max(12, Math.min(w, h) / 2);
      }
    }
    return { x, y, w, h, r, cr };
  },

  _buildShapeGeometry(variant, cx, cy, r, extraAttrs) {
    if (variant === 'square') {
      return `<rect x="${(cx - r).toFixed(2)}" y="${(cy - r).toFixed(2)}" width="${(r * 2).toFixed(2)}" height="${(r * 2).toFixed(2)}" rx="2" ${extraAttrs} />`;
    }
    if (variant === 'diamond') {
      const pts = `${cx},${cy - r} ${cx + r},${cy} ${cx},${cy + r} ${cx - r},${cy}`;
      return `<polygon points="${pts}" ${extraAttrs} />`;
    }
    if (variant === 'cross') {
      const arm = r * 0.35;
      const d = `M ${cx - arm},${cy - r} L ${cx + arm},${cy - r} L ${cx + arm},${cy - arm} L ${cx + r},${cy - arm} L ${cx + r},${cy + arm} L ${cx + arm},${cy + arm} L ${cx + arm},${cy + r} L ${cx - arm},${cy + r} L ${cx - arm},${cy + arm} L ${cx - r},${cy + arm} L ${cx - r},${cy - arm} L ${cx - arm},${cy - arm} Z`;
      return `<path d="${d}" ${extraAttrs} />`;
    }
    if (variant === 'star') {
      const pts = [];
      for (let i = 0; i < 10; i++) {
        const ang = (Math.PI / 5) * i - Math.PI / 2;
        const rad = i % 2 === 0 ? r : r * 0.44;
        pts.push(`${(cx + Math.cos(ang) * rad).toFixed(2)},${(cy + Math.sin(ang) * rad).toFixed(2)}`);
      }
      return `<polygon points="${pts.join(' ')}" ${extraAttrs} />`;
    }
    // circle or ring
    return `<circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="${r.toFixed(2)}" ${extraAttrs} />`;
  },

  renderLiteElement(le) {
    if (!le || le.visible === false) return '';
    const color = this.sanitizeColor(le.color, '#38bdf8');
    const opacity = le.opacity != null ? Math.max(0, Math.min(1, Number(le.opacity))) : 0.9;
    const sw = Number(le.strokeWidth) || 2.5;
    const { x, y, w, h, r, cr } = this._resolveGeometry(le);
    const nameAttr = le.name ? ` data-semantic-name="${this.escapeXML(le.name)}"` : '';
    const baseAttrs = `id="lite-${le.id}" class="diagram-lite-instance" data-lite-id="${le.id}" data-lite-kind="${this.escapeXML(le.kind)}" data-semantic-target="lite-${le.id}"${nameAttr}`;

    if (le.kind === 'perimeter-tracer') {
      const segs = Math.max(1, Math.min(16, Number(le.segments) || 2));
      const dashInfo = (window.EngineBridge && typeof window.EngineBridge.computePerimeterDash === 'function')
        ? window.EngineBridge.computePerimeterDash(segs, le.lineGapRatio || '1:3')
        : { dashLen: 125, gapLen: 375 };

      let dashArray = `${dashInfo.dashLen} ${dashInfo.gapLen}`;
      if (le.segmentStyle === 'dashed' && dashInfo.dashLen > 12) {
        // Sub-divide illuminated segment into crisp dashes followed by the gap
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
      let hitMarkup = '';
      const variant = le.variant || 'rounded';
      const payloadAttrs = `class="lite-payload sva-lite-tracer-anim" data-semantic-role="payload" pathLength="1000" fill="none" stroke="${color}" stroke-width="${sw}" stroke-opacity="${opacity}" stroke-dasharray="${dashArray}" stroke-linecap="${linecap}" style="${styleStr}" pointer-events="none"`;
      const hitAttrs = `class="lite-hit-target" data-semantic-role="hit-target" fill="transparent" stroke="transparent" stroke-width="${Math.max(14, sw + 10)}" pointer-events="stroke" style="cursor:pointer;"`;

      if (variant === 'circle') {
        const cx = x + w / 2;
        const cy = y + h / 2;
        const crR = Math.max(10, Math.min(w, h) / 2);
        shapeMarkup = `<circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="${crR.toFixed(2)}" ${payloadAttrs} />`;
        hitMarkup = `<circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="${crR.toFixed(2)}" ${hitAttrs} />`;
      } else if (variant === 'ellipse') {
        const cx = x + w / 2;
        const cy = y + h / 2;
        shapeMarkup = `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${(w / 2).toFixed(2)}" ry="${(h / 2).toFixed(2)}" ${payloadAttrs} />`;
        hitMarkup = `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${(w / 2).toFixed(2)}" ry="${(h / 2).toFixed(2)}" ${hitAttrs} />`;
      } else {
        const rx = variant === 'rounded' ? cr : 0;
        shapeMarkup = `<rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${rx}" ${payloadAttrs} />`;
        hitMarkup = `<rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${rx}" ${hitAttrs} />`;
      }

      return `<g ${baseAttrs}>${hitMarkup}${shapeMarkup}</g>`;
    }

    if (le.kind === 'pulse') {
      const variant = le.variant || 'circle';
      const period = Math.max(0.2, Number(le.period) || 1.8);
      const amp = le.intensity != null ? Math.max(0, Math.min(1, Number(le.intensity))) : 0.85;
      const isRing = variant === 'ring';
      const fillVal = isRing ? 'none' : color;
      const strokeVal = isRing ? color : 'none';
      const styleStr = `animation-duration:${period}s;--sva-base-dur:${period}s;--sva-base-op:${opacity};--sva-pulse-amp:${amp};`;
      const payloadAttrs = `class="lite-payload sva-lite-pulse-anim" data-semantic-role="payload" fill="${fillVal}" stroke="${strokeVal}" stroke-width="${isRing ? sw : 0}" style="${styleStr}" pointer-events="none"`;
      const hitMarkup = `<circle class="lite-hit-target" data-semantic-role="hit-target" cx="${x.toFixed(2)}" cy="${y.toFixed(2)}" r="${Math.max(20, r).toFixed(2)}" fill="transparent" pointer-events="all" style="cursor:pointer;" />`;
      const shapeMarkup = this._buildShapeGeometry(variant, x, y, r, payloadAttrs);
      return `<g ${baseAttrs}>${hitMarkup}${shapeMarkup}</g>`;
    }

    if (le.kind === 'flash') {
      const variant = le.variant || 'diamond';
      const flashDur = Math.max(0.08, Math.min(1.5, Number(le.dur) || 0.22));
      const period = Math.max(0.5, Number(le.period) || 3.0);
      const intensity = le.intensity != null ? Math.max(0, Math.min(1, Number(le.intensity))) : 0.75;
      const repeat = Boolean(le.repeat);
      const animDur = repeat ? period : flashDur;
      const iterCount = repeat ? 'infinite' : '1';
      const styleStr = `animation-duration:${animDur}s;animation-iteration-count:${iterCount};--sva-base-dur:${animDur}s;--sva-flash-int:${intensity};`;
      const payloadAttrs = `class="lite-payload lite-flash-anim sva-lite-flash-anim" data-semantic-role="payload" data-trigger-nonce="${le.triggerNonce || 0}" fill="${color}" style="${styleStr}" pointer-events="none"`;
      const hitMarkup = `<circle class="lite-hit-target" data-semantic-role="hit-target" cx="${x.toFixed(2)}" cy="${y.toFixed(2)}" r="${Math.max(20, r).toFixed(2)}" fill="transparent" pointer-events="all" style="cursor:pointer;" />`;
      const shapeMarkup = this._buildShapeGeometry(variant, x, y, r, payloadAttrs);
      return `<g ${baseAttrs}>${hitMarkup}${shapeMarkup}</g>`;
    }

    return '';
  },

  injectIntoSVG(svgString) {
    if (this.list.length === 0) return svgString;
    const defs = this.getLiteDefs();
    let elements = '';
    this.list.forEach((le) => {
      elements += this.renderLiteElement(le);
    });
    return svgString.replace('<!-- INJECT_CAMERA_RIG_ELEMENTS -->', `\n<defs>\n${defs}\n</defs>\n<g id="lite-effects-layer">\n${elements}\n</g>\n<!-- INJECT_CAMERA_RIG_ELEMENTS -->`);
  },

  // =========================================================================
  // Layer 1: Studio UI Presentation & Bindings
  // =========================================================================

  init() {
    if (this._initialized) return;
    this._initialized = true;

    const btnTracer = document.getElementById('btnAddLiteTracer');
    if (btnTracer) btnTracer.addEventListener('click', () => this.addLiteEffect('perimeter-tracer'));

    const btnPulse = document.getElementById('btnAddLitePulse');
    if (btnPulse) btnPulse.addEventListener('click', () => this.addLiteEffect('pulse'));

    const btnFlash = document.getElementById('btnAddLiteFlash');
    if (btnFlash) btnFlash.addEventListener('click', () => this.addLiteEffect('flash'));

    const btnTrigger = document.getElementById('btnTriggerLiteFlash');
    if (btnTrigger) {
      btnTrigger.addEventListener('click', () => {
        if (this.activeId) this.triggerFlash(this.activeId);
      });
    }

    const btnDeselect = document.getElementById('btnDeselectLite');
    if (btnDeselect) {
      btnDeselect.addEventListener('click', () => {
        if (window.AppCoordinator && typeof window.AppCoordinator.clearSelection === 'function') {
          window.AppCoordinator.clearSelection({ reason: 'sidebar_deselect_lite' });
        } else {
          this.deselectLite();
        }
      });
    }

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

    bindField('liteNameInput', 'name');
    bindField('liteVariantSelect', 'variant');
    bindField('liteColorPicker', 'color');
    bindField('liteOpacityNum', 'opacity', true);
    bindField('liteStrokeWidthNum', 'strokeWidth', true);
    bindField('liteSegmentsNum', 'segments', true);
    bindField('liteRatioInput', 'lineGapRatio');
    bindField('liteDirectionSelect', 'direction');
    bindField('liteStyleSelect', 'segmentStyle');
    bindField('liteSpeedNum', 'speed', true);
    bindField('litePeriodNum', 'period', true);
    bindField('liteFlashDurNum', 'dur', true);
    bindField('liteRepeatChk', 'repeat', false, true);
    bindField('liteVisibleChk', 'visible', false, true);
  },

  addLiteEffect(kind) {
    const created = this.createLiteEffect({ kind, x: 240, y: 180 });
    this.selectLite(created.id);
    this.renderList();
    this.triggerAppRender();
    if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
    return created;
  },

  triggerFlash(id) {
    const le = this.getLiteEffect(id);
    if (!le || le.kind !== 'flash') return;
    this.updateLiteEffect(id, { triggerNonce: (Number(le.triggerNonce) || 0) + 1 });
    this.triggerAppRender();
  },

  selectLite(id) {
    this.activeId = String(id);
    const le = this.getLiteEffect(id);
    this.syncEditorUI(le);
    this.renderList();
    if (window.AppCoordinator && typeof window.AppCoordinator.selectObject === 'function') {
      window.AppCoordinator.selectObject('lite', String(id));
    }
  },

  deselectLite() {
    this.activeId = null;
    this.syncEditorUI(null);
    this.renderList();
  },

  updateActive(key, value) {
    if (!this.activeId) return;
    const patch = {};
    patch[key] = value;
    const updated = this.updateLiteEffect(this.activeId, patch);
    if (updated) {
      this.renderList();
      this.triggerAppRender();
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

    const setVal = (id, v) => { const el = document.getElementById(id); if (el) el.value = v; };
    const setChk = (id, v) => { const el = document.getElementById(id); if (el) el.checked = Boolean(v); };

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

    setVal('liteNameInput', le.name || '');
    setVal('liteVariantSelect', le.variant || '');
    setVal('liteColorPicker', le.color || '#38bdf8');
    setVal('liteOpacityNum', le.opacity != null ? le.opacity : 0.9);
    setVal('liteStrokeWidthNum', le.strokeWidth || 2.5);
    setVal('liteSegmentsNum', le.segments || 2);
    setVal('liteRatioInput', le.lineGapRatio || '1:3');
    setVal('liteDirectionSelect', le.direction || 'cw');
    setVal('liteStyleSelect', le.segmentStyle || 'rounded');
    setVal('liteSpeedNum', le.speed || 3.0);
    setVal('litePeriodNum', le.period || 1.8);
    setVal('liteFlashDurNum', le.dur || 0.22);
    setChk('liteRepeatChk', le.repeat);
    setChk('liteVisibleChk', le.visible !== false);

    const tracerGroup = document.getElementById('liteTracerGroup');
    const pulseGroup = document.getElementById('litePulseGroup');
    const flashGroup = document.getElementById('liteFlashGroup');
    if (tracerGroup) tracerGroup.style.display = le.kind === 'perimeter-tracer' ? 'block' : 'none';
    if (pulseGroup) pulseGroup.style.display = le.kind === 'pulse' ? 'block' : 'none';
    if (flashGroup) flashGroup.style.display = le.kind === 'flash' ? 'block' : 'none';
  },

  renderList() {
    const listEl = document.getElementById('liteEffectsList');
    if (!listEl) return;
    listEl.innerHTML = '';
    this.list.forEach((le) => {
      const row = document.createElement('div');
      row.className = 'item-row' + (String(this.activeId) === String(le.id) ? ' active' : '');
      row.innerHTML = `<span>${this.escapeXML(le.name || `${le.kind}-${le.id}`)} <small>(${le.kind})</small></span>
        <button type="button" class="btn-mini-del" title="Delete">&times;</button>`;
      row.addEventListener('click', (e) => {
        if (e.target.closest('.btn-mini-del')) {
          this.removeLite(le.id);
        } else {
          this.selectLite(le.id);
        }
      });
      listEl.appendChild(row);
    });
  },

  triggerAppRender() {
    if (window.AppCoordinator && typeof window.AppCoordinator.triggerRender === 'function') {
      window.AppCoordinator.triggerRender();
    } else if (typeof window.triggerRender === 'function') {
      window.triggerRender();
    }
  }
};
