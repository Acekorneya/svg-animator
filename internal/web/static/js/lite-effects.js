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
  activePointIndex: 0,
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
      if (le.burstCycles === undefined || le.burstCycles === null) le.burstCycles = 0;
      if (le.pauseSeconds === undefined || le.pauseSeconds === null) le.pauseSeconds = 0;
      if (!le.pauseVisibility) le.pauseVisibility = 'hide';
    } else if (le.kind === 'pulse') {
      if (!le.minScale || le.minScale <= 0) le.minScale = 0.85;
      if (!le.maxScale || le.maxScale <= 0) le.maxScale = 1.15;
      if (!le.armThickness || le.armThickness <= 0) le.armThickness = 3;
      if (le.maintainAspectRatio === undefined || le.maintainAspectRatio === null) le.maintainAspectRatio = true;
    } else if (le.kind === 'flash') {
      if (!le.dur || le.dur <= 0) le.dur = 0.10;
      if (!le.repeatInterval || le.repeatInterval <= 0) le.repeatInterval = Number(le.period) > 0 ? Number(le.period) : 2.0;
      if (!le.armThickness || le.armThickness <= 0) le.armThickness = 3;
      if (le.maintainAspectRatio === undefined || le.maintainAspectRatio === null) le.maintainAspectRatio = true;
    } else if (le.kind === 'flow-lite') {
      if (!le.variant) le.variant = 'dashed';
      if (!Array.isArray(le.points) || le.points.length < 2) {
        le.points = [
          { x: 25, y: 50 },
          { x: 75, y: 50 }
        ];
      } else {
        le.points = le.points.map((p) => ({ x: Number(p.x) || 0, y: Number(p.y) || 0 }));
      }
      if (!Array.isArray(le.bends) || le.bends.length !== le.points.length - 1) {
        const nextBends = new Array(Math.max(1, le.points.length - 1)).fill(0);
        if (Array.isArray(le.bends)) {
          for (let i = 0; i < nextBends.length && i < le.bends.length; i++) {
            nextBends[i] = Math.max(-100, Math.min(100, Number(le.bends[i]) || 0));
          }
        }
        le.bends = nextBends;
      }
      le.curvature = Math.max(-100, Math.min(100, Number(le.curvature) || 0));
      if (!le.dashLength || le.dashLength <= 0) le.dashLength = le.variant === 'single' ? 36 : 18;
      if (!le.gapLength || le.gapLength <= 0) le.gapLength = 14;
      if (!le.dotSize || le.dotSize <= 0) le.dotSize = 5;
      if (!le.strokeWidth || le.strokeWidth <= 0) le.strokeWidth = 3;
      if (!le.segmentStyle) le.segmentStyle = 'rounded';
      if (!le.startMode) le.startMode = 'tunnel';
      if (!le.endMode) le.endMode = 'tunnel';
      if (!le.direction || le.direction === 'cw') le.direction = 'forward';
      else if (le.direction === 'ccw') le.direction = 'reverse';
      if (!le.playbackMode) le.playbackMode = 'loop';
      if (!le.speed || le.speed <= 0) le.speed = 2.0;
      le.delay = Math.max(0, Math.min(60, Number(le.delay) || 0));
      le.endDelay = Math.max(0, Math.min(60, Number(le.endDelay) || 0));
      if (le.showGuide === undefined || le.showGuide === null) le.showGuide = true;
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
    const isFlowLite = kind === 'flow-lite';
    const defaultW = isTracer ? 140 : 56;
    const defaultH = isTracer ? 80 : 56;
    const defaultVariant = isFlowLite
      ? 'dashed'
      : (isTracer ? 'rounded' : (kind === 'flash' ? 'diamond' : 'circle'));
    const payload = Object.assign({
      kind,
      originMode: 'center',
      variant: defaultVariant,
      x: 240,
      y: 180,
      width: defaultW,
      height: defaultH,
      radius: 28,
      cornerRadius: 10,
      color: kind === 'flash' ? '#f59e0b' : (kind === 'pulse' ? '#22c55e' : '#38bdf8'),
      opacity: 0.9,
      strokeWidth: isFlowLite ? 3.0 : 2.5,
      segments: 2,
      lineGapRatio: '1:3',
      segmentStyle: 'rounded',
      startOffset: 0,
      direction: isFlowLite ? 'forward' : 'cw',
      playbackMode: 'loop',
      speed: isFlowLite ? 2.0 : 3.0,
      period: kind === 'flash' ? 3.0 : 1.8,
      dur: 0.10,
      intensity: kind === 'flash' ? 0.75 : 0.85,
      minScale: 0.85,
      maxScale: 1.15,
      maintainAspectRatio: !isTracer && !isFlowLite,
      showGuide: true,
      repeat: false,
      triggerNonce: 0,
      points: isFlowLite ? [{ x: 25, y: 50 }, { x: 75, y: 50 }] : undefined,
      bends: isFlowLite ? [0] : undefined,
      curvature: 0,
      dashLength: 18,
      gapLength: 14,
      dotSize: 5,
      startMode: 'tunnel',
      endMode: 'tunnel',
      delay: 0,
      endDelay: 0,
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
  from { stroke-dashoffset: var(--sva-start-offset, 0px); }
  to { stroke-dashoffset: calc(var(--sva-start-offset, 0px) - 1000px); }
}
@keyframes sva-lite-trace-ccw {
  from { stroke-dashoffset: var(--sva-start-offset, 0px); }
  to { stroke-dashoffset: calc(var(--sva-start-offset, 0px) + 1000px); }
}
@keyframes sva-lite-pulse {
  0%, 100% { transform: scale(var(--sva-pulse-min, 0.85)); opacity: calc(1 - var(--sva-pulse-amp, 0.55)); }
  50% { transform: scale(var(--sva-pulse-max, 1.15)); opacity: 1; }
}
@keyframes sva-lite-flash {
  0% { opacity: 0; transform: scale(0.85); }
  25% { opacity: var(--sva-flash-int, 0.75); transform: scale(1.08); }
  100% { opacity: 0; transform: scale(1.0); }
}
.sva-lite-tracer-anim {
  animation-timing-function: linear;
}
.sva-lite-trace-cw { animation-name: sva-lite-trace-cw; }
.sva-lite-trace-ccw { animation-name: sva-lite-trace-ccw; }
.sva-lite-mode-loop { animation-iteration-count: infinite; animation-direction: normal; }
.sva-lite-mode-pingpong { animation-iteration-count: infinite; animation-direction: alternate; }
.sva-lite-mode-once { animation-iteration-count: 1; animation-direction: normal; animation-fill-mode: forwards; }
.sva-lite-pulse-anim {
  transform-box: fill-box;
  transform-origin: center;
  will-change: transform, opacity;
  animation-name: sva-lite-pulse;
  animation-timing-function: ease-in-out;
  animation-iteration-count: infinite;
}
.sva-lite-flash-anim {
  transform-box: fill-box;
  transform-origin: center;
  will-change: transform, opacity;
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
    let w = Math.max(5, Number(le.width) || defDim);
    let h = Math.max(5, Number(le.height) || (isTracer ? 80 : defDim));
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
    if (le.kind === 'flow-lite') {
      const geom = this._computeFlowLiteGeometry(le);
      return {
        cx: geom.minX + geom.width / 2,
        cy: geom.minY + geom.height / 2,
        x: geom.minX,
        y: geom.minY,
        width: Math.max(12, geom.width),
        height: Math.max(12, geom.height)
      };
    }
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

  _computeFlowLiteGeometry(le, leadIn = 0, leadOut = 0) {
    const cw = (window.Stage && window.Stage.canvasWidth) || 1280;
    const ch = (window.Stage && window.Stage.canvasHeight) || 720;
    const rawPts = (Array.isArray(le.points) && le.points.length >= 2)
      ? le.points
      : [{ x: 25, y: 50 }, { x: 75, y: 50 }];
    const pts = rawPts.map((p) => ({
      x: ((Number(p.x) || 0) / 100) * cw,
      y: ((Number(p.y) || 0) / 100) * ch
    }));
    const bends = Array.isArray(le.bends) ? le.bends : [];
    const globalCurv = Number(le.curvature) || 0;

    const sections = [];
    let totalLen = 0;
    let minX = pts[0].x;
    let maxX = pts[0].x;
    let minY = pts[0].y;
    let maxY = pts[0].y;

    for (let i = 0; i < pts.length - 1; i++) {
      const p1 = pts[i];
      const p2 = pts[i + 1];
      const rawB = Number(bends[i]) || 0;
      const effBend = Math.max(-100, Math.min(100, rawB + globalCurv));
      const dx = p2.x - p1.x;
      const dy = p2.y - p1.y;
      const dist = Math.hypot(dx, dy);
      let ctrl = { x: (p1.x + p2.x) / 2, y: (p1.y + p2.y) / 2 };
      let curved = false;
      let secLen = dist;

      if (Math.abs(effBend) >= 0.01 && dist > 1e-4) {
        curved = true;
        const nx = -dy / dist;
        const ny = dx / dist;
        const offset = dist * (effBend / 100) * 0.5;
        ctrl = {
          x: (p1.x + p2.x) / 2 + nx * offset,
          y: (p1.y + p2.y) / 2 + ny * offset
        };
        secLen = 0;
        let prev = p1;
        const steps = 24;
        for (let s = 1; s <= steps; s++) {
          const t = s / steps;
          const u = 1 - t;
          const cur = {
            x: u * u * p1.x + 2 * u * t * ctrl.x + t * t * p2.x,
            y: u * u * p1.y + 2 * u * t * ctrl.y + t * t * p2.y
          };
          secLen += Math.hypot(cur.x - prev.x, cur.y - prev.y);
          if (cur.x < minX) minX = cur.x;
          if (cur.x > maxX) maxX = cur.x;
          if (cur.y < minY) minY = cur.y;
          if (cur.y > maxY) maxY = cur.y;
          prev = cur;
        }
      } else {
        if (p2.x < minX) minX = p2.x;
        if (p2.x > maxX) maxX = p2.x;
        if (p2.y < minY) minY = p2.y;
        if (p2.y > maxY) maxY = p2.y;
      }
      sections.push({ p1, p2, ctrl, curved, length: secLen });
      totalLen += secLen;
    }

    // Start tangent t0 and end tangent tL
    let t0 = { x: 1, y: 0 };
    if (sections.length > 0) {
      const s0 = sections[0];
      const vx = s0.curved ? (s0.ctrl.x - s0.p1.x) : (s0.p2.x - s0.p1.x);
      const vy = s0.curved ? (s0.ctrl.y - s0.p1.y) : (s0.p2.y - s0.p1.y);
      const m = Math.hypot(vx, vy) || 1;
      t0 = { x: vx / m, y: vy / m };
    }
    let tL = { x: 1, y: 0 };
    if (sections.length > 0) {
      const sLast = sections[sections.length - 1];
      const vx = sLast.curved ? (sLast.p2.x - sLast.ctrl.x) : (sLast.p2.x - sLast.p1.x);
      const vy = sLast.curved ? (sLast.p2.y - sLast.ctrl.y) : (sLast.p2.y - sLast.p1.y);
      const m = Math.hypot(vx, vy) || 1;
      tL = { x: vx / m, y: vy / m };
    }

    const p0 = pts[0];
    const pLast = pts[pts.length - 1];
    const parts = [];
    if (leadIn > 0) {
      const pre = { x: p0.x - t0.x * leadIn, y: p0.y - t0.y * leadIn };
      parts.push(`M ${pre.x.toFixed(2)} ${pre.y.toFixed(2)} L ${p0.x.toFixed(2)} ${p0.y.toFixed(2)}`);
    } else {
      parts.push(`M ${p0.x.toFixed(2)} ${p0.y.toFixed(2)}`);
    }
    for (const sec of sections) {
      if (sec.curved) {
        parts.push(`Q ${sec.ctrl.x.toFixed(2)} ${sec.ctrl.y.toFixed(2)} ${sec.p2.x.toFixed(2)} ${sec.p2.y.toFixed(2)}`);
      } else {
        parts.push(`L ${sec.p2.x.toFixed(2)} ${sec.p2.y.toFixed(2)}`);
      }
    }
    if (leadOut > 0) {
      const post = { x: pLast.x + tL.x * leadOut, y: pLast.y + tL.y * leadOut };
      parts.push(`L ${post.x.toFixed(2)} ${post.y.toFixed(2)}`);
    }

    return {
      pathD: parts.join(' '),
      totalLen: Math.max(1, totalLen),
      p0,
      pLast,
      t0,
      tL,
      pts,
      sections,
      minX,
      minY,
      maxX,
      maxY,
      width: maxX - minX,
      height: maxY - minY
    };
  },

  _buildFlowLiteOverhangMaskD(pt, tang, r, isStart) {
    const dir = isStart ? -1 : 1;
    const ux = tang.x * dir;
    const uy = tang.y * dir;
    const nx = -tang.y;
    const ny = tang.x;
    const h = r * 2.5;
    const d = r * 2.5;
    const c1 = { x: pt.x + nx * h, y: pt.y + ny * h };
    const c2 = { x: pt.x + nx * h + ux * d, y: pt.y + ny * h + uy * d };
    const c3 = { x: pt.x - nx * h + ux * d, y: pt.y - ny * h + uy * d };
    const c4 = { x: pt.x - nx * h, y: pt.y - ny * h };
    return `M ${c1.x.toFixed(2)} ${c1.y.toFixed(2)} L ${c2.x.toFixed(2)} ${c2.y.toFixed(2)} L ${c3.x.toFixed(2)} ${c3.y.toFixed(2)} L ${c4.x.toFixed(2)} ${c4.y.toFixed(2)} Z`;
  },

  buildFlowLiteContent(le, idPrefix = '') {
    const color = this.sanitizeColor(le.color, '#38bdf8');
    const variant = le.variant || 'dashed';
    const startMode = le.startMode === 'abrupt' ? 'abrupt' : 'tunnel';
    const endMode = le.endMode === 'abrupt' ? 'abrupt' : 'tunnel';
    const dir = le.direction === 'reverse' ? 'reverse' : 'forward';
    const mode = le.playbackMode === 'pingpong' ? 'pingpong' : (le.playbackMode === 'once' ? 'once' : 'loop');
    const transitDur = Math.max(0.2, Number(le.speed) || 2.0);
    const startDelay = Math.max(0, Math.min(60, Number(le.delay) || 0));
    const endDelay = Math.max(0, Math.min(60, Number(le.endDelay) || 0));

    let dashLen = Math.max(2, Number(le.dashLength) || (variant === 'single' ? 36 : 18));
    let gapLen = Math.max(2, Number(le.gapLength) || 14);
    let sw = Math.max(0.5, Number(le.strokeWidth) || 3.0);
    let linecap = le.segmentStyle === 'solid' ? 'butt' : 'round';
    let leadR = 0;

    if (variant === 'dotted') {
      const dotDia = Math.max(1, Number(le.dotSize) || 5.0);
      sw = dotDia;
      dashLen = 0.2;
      linecap = 'round';
      leadR = dotDia / 2.0;
    } else if (linecap === 'round') {
      leadR = sw / 2.0;
    }

    const baseGeom = this._computeFlowLiteGeometry(le, 0, 0);
    const extGeom = leadR > 0 ? this._computeFlowLiteGeometry(le, leadR, leadR) : baseGeom;
    const L = baseGeom.totalLen;
    const visLen = variant === 'dotted' ? sw : dashLen;
    const period = variant === 'single' ? (L + visLen * 2) : (visLen + gapLen);

    let offStart, offEnd;
    if (dir === 'forward') {
      offStart = startMode === 'tunnel' ? dashLen : -(2 * leadR);
      offEnd = endMode === 'tunnel' ? -(L + 2 * leadR) : -(L - dashLen);
    } else {
      offStart = endMode === 'tunnel' ? -(L + 2 * leadR) : -(L - dashLen);
      offEnd = startMode === 'tunnel' ? dashLen : -(2 * leadR);
    }

    const activeDist = Math.max(1, Math.abs(offEnd - offStart));
    const M = variant === 'single' ? 1 : (Math.ceil(activeDist / period) + 1);
    const totalCycleDist = M === 1 ? activeDist : (M * period);
    const moveDur = M === 1 ? transitDur : (transitDur * (totalCycleDist / activeDist));
    const activeRatio = Math.min(0.999, activeDist / totalCycleDist);

    const trackId = `${idPrefix}lite-flow-track-${le.id}`;
    const clipId = `${idPrefix}lite-flow-clip-${le.id}`;
    const kfName = `${idPrefix}sva-fl-kf-${le.id}`;

    let maskHoles = '';
    if (leadR > 0) {
      if (startMode === 'tunnel') {
        maskHoles += ' ' + this._buildFlowLiteOverhangMaskD(baseGeom.p0, baseGeom.t0, leadR, true);
      }
      if (endMode === 'tunnel') {
        maskHoles += ' ' + this._buildFlowLiteOverhangMaskD(baseGeom.pLast, baseGeom.tL, leadR, false);
      }
    }

    const clearGap = L + totalCycleDist + 500;
    let totalDur = moveDur;
    let kfCss = '';

    if (mode === 'pingpong' && M === 1) {
      // Explicit forward + endDelay + reverse + startDelay cycle
      totalDur = startDelay + moveDur + endDelay + moveDur;
      const p1 = ((startDelay / totalDur) * 100).toFixed(3);
      const p2 = (((startDelay + moveDur) / totalDur) * 100).toFixed(3);
      const p3 = (((startDelay + moveDur + endDelay) / totalDur) * 100).toFixed(3);
      kfCss = `@keyframes ${kfName} {
  0%, ${p1}% { stroke-dashoffset: ${offStart.toFixed(2)}px; opacity: 1; }
  ${p2}%, ${p3}% { stroke-dashoffset: ${offEnd.toFixed(2)}px; opacity: 1; }
  100% { stroke-dashoffset: ${offStart.toFixed(2)}px; opacity: 1; }
}`;
    } else if (M === 1 && (startDelay > 0 || endDelay > 0)) {
      totalDur = startDelay + moveDur + endDelay;
      const pStart = ((startDelay / totalDur) * 100).toFixed(3);
      const pEnd = (((startDelay + moveDur) / totalDur) * 100).toFixed(3);
      const startHoldOp = startDelay > 0 && ((dir === 'forward' && startMode === 'abrupt') || (dir === 'reverse' && endMode === 'abrupt')) ? 0 : 1;
      const endHoldOp = endDelay > 0 && ((dir === 'forward' && endMode === 'abrupt') || (dir === 'reverse' && startMode === 'abrupt')) ? 0 : 1;
      kfCss = `@keyframes ${kfName} {
  0% { stroke-dashoffset: ${offStart.toFixed(2)}px; opacity: ${startHoldOp}; }
  ${pStart}% { stroke-dashoffset: ${offStart.toFixed(2)}px; opacity: 1; }
  ${pEnd}% { stroke-dashoffset: ${offEnd.toFixed(2)}px; opacity: 1; }
  ${(Math.min(99.999, parseFloat(pEnd) + 0.01)).toFixed(3)}%, 100% { stroke-dashoffset: ${offEnd.toFixed(2)}px; opacity: ${endHoldOp}; }
}`;
    } else {
      const pctActive = (activeRatio * 100).toFixed(3);
      const pctHide = (Math.min(99.999, activeRatio * 100 + 0.01)).toFixed(3);
      kfCss = `@keyframes ${kfName} {
  0% { stroke-dashoffset: ${offStart.toFixed(2)}px; opacity: 1; }
  ${pctActive}% { stroke-dashoffset: ${offEnd.toFixed(2)}px; opacity: 1; }
  ${pctHide}%, 100% { stroke-dashoffset: ${offEnd.toFixed(2)}px; opacity: 0; }
}`;
    }

    const iterCss = mode === 'once' ? '1' : 'infinite';
    const dirCss = (mode === 'pingpong' && M > 1) ? 'alternate' : 'normal';
    const fillCss = mode === 'once' ? 'forwards' : 'none';
    const hasCyclePause = M > 1 && mode !== 'once' && (startDelay + endDelay) > 0;
    const pauseSec = hasCyclePause ? (startDelay + endDelay) : 0;

    const uses = [];
    for (let k = 0; k < M; k++) {
      const phaseDelay = -(k / M) * totalDur;
      const burstAttrs = (k === 0 && hasCyclePause)
        ? ` data-burst-cycles="1" data-pause-seconds="${pauseSec.toFixed(3)}" data-pause-visibility="hide"`
        : '';
      const styleStr = `animation-name:${kfName};animation-duration:${totalDur.toFixed(4)}s;animation-timing-function:linear;animation-delay:${phaseDelay.toFixed(4)}s;animation-iteration-count:${iterCss};animation-direction:${dirCss};animation-fill-mode:${fillCss};--sva-base-dur:${totalDur.toFixed(4)}s;--sva-phase-ratio:${(k / M).toFixed(6)};`;
      uses.push(`<use href="#${trackId}" class="lite-payload lite-flow-seg" data-semantic-role="payload" data-seg-index="${k}" data-seg-count="${M}"${burstAttrs} fill="none" stroke="${color}" stroke-width="${sw.toFixed(2)}" stroke-linecap="${linecap}" stroke-dasharray="${dashLen.toFixed(2)} ${clearGap.toFixed(2)}" shape-rendering="geometricPrecision" style="${styleStr}" pointer-events="none" />`);
    }

    const guideAttrs = `class="lite-guide-track" fill="none" stroke="${color}" stroke-width="${Math.max(1, sw * 0.45).toFixed(2)}" stroke-opacity="0.20" shape-rendering="geometricPrecision" pointer-events="none"`;
    const editorGuideAttrs = `fill="none" stroke="${color}" stroke-width="1" stroke-opacity="0.25" stroke-dasharray="4 4" pointer-events="none"`;
    const guideMarkup = le.showGuide !== false
      ? `<path d="${baseGeom.pathD}" ${guideAttrs} />`
      : `<g class="lite-editor-marker" data-editor-only="true"><path d="${baseGeom.pathD}" ${editorGuideAttrs} /></g>`;
    const hitMarkup = `<path d="${baseGeom.pathD}" class="lite-hit-target" data-semantic-role="hit-target" fill="none" stroke="transparent" stroke-width="${Math.max(18, sw + 14).toFixed(2)}" pointer-events="stroke" style="cursor:pointer;" />`;
    const clipAttr = maskHoles ? ` clip-path="url(#${clipId})"` : '';
    const clipDef = maskHoles ? `<clipPath id="${clipId}"><path clip-rule="evenodd" d="M -2000 -2000 H 5000 V 5000 H -2000 Z${maskHoles}" /></clipPath>` : '';

    return {
      innerMarkup: `<defs><path id="${trackId}" d="${extGeom.pathD}" />${clipDef}</defs><style>${kfCss}</style>${hitMarkup}${guideMarkup}<g class="lite-flow-stream"${clipAttr}>${uses.join('')}</g>`,
      dataAttrs: `data-flow-variant="${this.escapeXML(variant)}" data-flow-start-mode="${startMode}" data-flow-end-mode="${endMode}" data-flow-direction="${dir}" data-flow-playback-mode="${mode}" data-flow-dash-length="${dashLen}" data-flow-gap-length="${gapLen}" data-flow-dot-size="${le.dotSize || 5}" data-flow-stroke-width="${sw}" data-flow-cap-style="${le.segmentStyle || 'rounded'}" data-flow-speed="${transitDur}" data-flow-delay="${startDelay}" data-flow-end-delay="${endDelay}" data-flow-length="${L.toFixed(2)}" data-flow-lead-r="${leadR.toFixed(2)}" data-flow-seg-count="${M}" data-flow-off-start="${offStart.toFixed(2)}" data-flow-off-end="${offEnd.toFixed(2)}" data-flow-active-dist="${activeDist.toFixed(2)}" data-flow-period="${period.toFixed(2)}"`
    };
  },

  _buildShapeGeometry(variant, cx, cy, w, h, extraAttrs, armThickness = 3) {
    const rx = w / 2;
    const ry = h / 2;
    if (variant === 'square' || variant === 'rect') {
      return `<rect x="${(cx - rx).toFixed(2)}" y="${(cy - ry).toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="3" shape-rendering="geometricPrecision" ${extraAttrs} />`;
    }
    if (variant === 'diamond') {
      const pts = `${cx.toFixed(2)},${(cy - ry).toFixed(2)} ${(cx + rx).toFixed(2)},${cy.toFixed(2)} ${cx.toFixed(2)},${(cy + ry).toFixed(2)} ${(cx - rx).toFixed(2)},${cy.toFixed(2)}`;
      return `<polygon points="${pts}" shape-rendering="geometricPrecision" ${extraAttrs} />`;
    }
    if (variant === 'cross') {
      const halfArm = Math.max(0.5, Math.min(rx * 0.9, ry * 0.9, (Number(armThickness) || 3) / 2));
      const d = `M ${(cx - halfArm).toFixed(2)},${(cy - ry).toFixed(2)} L ${(cx + halfArm).toFixed(2)},${(cy - ry).toFixed(2)} L ${(cx + halfArm).toFixed(2)},${(cy - halfArm).toFixed(2)} L ${(cx + rx).toFixed(2)},${(cy - halfArm).toFixed(2)} L ${(cx + rx).toFixed(2)},${(cy + halfArm).toFixed(2)} L ${(cx + halfArm).toFixed(2)},${(cy + halfArm).toFixed(2)} L ${(cx + halfArm).toFixed(2)},${(cy + ry).toFixed(2)} L ${(cx - halfArm).toFixed(2)},${(cy + ry).toFixed(2)} L ${(cx - halfArm).toFixed(2)},${(cy + halfArm).toFixed(2)} L ${(cx - rx).toFixed(2)},${(cy + halfArm).toFixed(2)} L ${(cx - rx).toFixed(2)},${(cy - halfArm).toFixed(2)} L ${(cx - halfArm).toFixed(2)},${(cy - halfArm).toFixed(2)} Z`;
      return `<path d="${d}" shape-rendering="geometricPrecision" ${extraAttrs} />`;
    }
    if (variant === 'star') {
      const pts = [];
      const waist = Math.max(0.5, Math.min(rx * 0.45, ry * 0.45, (Number(armThickness) || 3) * 0.65));
      for (let i = 0; i < 8; i++) {
        const ang = (Math.PI / 4) * i - Math.PI / 2;
        const radX = i % 2 === 0 ? rx : waist;
        const radY = i % 2 === 0 ? ry : waist;
        pts.push(`${(cx + Math.cos(ang) * radX).toFixed(2)},${(cy + Math.sin(ang) * radY).toFixed(2)}`);
      }
      return `<polygon points="${pts.join(' ')}" shape-rendering="geometricPrecision" ${extraAttrs} />`;
    }
    return `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${rx.toFixed(2)}" ry="${ry.toFixed(2)}" shape-rendering="geometricPrecision" ${extraAttrs} />`;
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
      const dir = le.direction === 'ccw' ? 'ccw' : 'cw';
      const traceClass = `sva-lite-trace-${dir}`;
      const mode = le.playbackMode === 'pingpong' ? 'pingpong' : (le.playbackMode === 'once' ? 'once' : 'loop');
      const modeClass = `sva-lite-mode-${mode}`;
      const startOffsetUnits = -((Number(le.startOffset) || 0) / 100) * 1000;
      const burstCycles = Math.max(0, Math.round(Number(le.burstCycles) || 0));
      const pauseSeconds = Math.max(0, Number(le.pauseSeconds) || 0);
      const pauseVisibility = le.pauseVisibility === 'hold' ? 'hold' : 'hide';
      const hasBurst = mode !== 'once' && burstCycles > 0 && pauseSeconds > 0;
      const iterCss = mode === 'once' ? '1' : (hasBurst ? String(burstCycles) : 'infinite');
      const dirCss = mode === 'pingpong' ? 'alternate' : 'normal';
      const fillCss = (mode === 'once' || hasBurst) ? 'forwards' : 'none';
      const styleStr = `animation-name:${traceClass};animation-duration:${dur}s;animation-iteration-count:${iterCss};animation-direction:${dirCss};animation-fill-mode:${fillCss};--sva-base-dur:${dur}s;--sva-start-offset:${startOffsetUnits}px;`;

      let shapeMarkup = '';
      let guideMarkup = '';
      let hitMarkup = '';
      const variant = le.variant || 'rounded';
      const burstAttrs = hasBurst ? ` data-burst-cycles="${burstCycles}" data-pause-seconds="${pauseSeconds}" data-pause-visibility="${pauseVisibility}"` : '';
      const payloadAttrs = `class="lite-payload sva-lite-tracer-anim ${traceClass} ${modeClass}" data-semantic-role="payload" data-direction="${dir}" data-playback-mode="${mode}"${burstAttrs} pathLength="1000" fill="none" stroke="${color}" stroke-width="${sw}" stroke-dasharray="${dashArray}" stroke-linecap="${linecap}" shape-rendering="geometricPrecision" style="${styleStr}" pointer-events="none"`;
      const hitAttrs = `class="lite-hit-target" data-semantic-role="hit-target" fill="transparent" stroke="transparent" stroke-width="${Math.max(16, sw + 12)}" pointer-events="stroke" style="cursor:pointer;"`;
      const guideAttrs = `class="lite-guide-track" fill="none" stroke="${color}" stroke-width="${Math.max(1, sw * 0.55).toFixed(2)}" stroke-opacity="0.22" shape-rendering="geometricPrecision" pointer-events="none"`;
      const editorGuideAttrs = `fill="none" stroke="${color}" stroke-width="1" stroke-opacity="0.25" stroke-dasharray="4 4" pointer-events="none"`;

      if (variant === 'circle' || variant === 'ellipse') {
        const erx = variant === 'circle' ? Math.max(2.5, rx) : rx;
        const ery = variant === 'circle' ? Math.max(2.5, rx) : ry;
        shapeMarkup = `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${erx.toFixed(2)}" ry="${ery.toFixed(2)}" ${payloadAttrs} />`;
        hitMarkup = `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${erx.toFixed(2)}" ry="${ery.toFixed(2)}" ${hitAttrs} />`;
        guideMarkup = le.showGuide !== false
          ? `<ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${erx.toFixed(2)}" ry="${ery.toFixed(2)}" ${guideAttrs} />`
          : `<g class="lite-editor-marker" data-editor-only="true"><ellipse cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" rx="${erx.toFixed(2)}" ry="${ery.toFixed(2)}" ${editorGuideAttrs} /></g>`;
      } else {
        const cornerR = variant === 'rounded' ? Math.min(cr, rx, ry) : 0;
        shapeMarkup = `<rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${cornerR}" ${payloadAttrs} />`;
        hitMarkup = `<rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${cornerR}" ${hitAttrs} />`;
        guideMarkup = le.showGuide !== false
          ? `<rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${cornerR}" ${guideAttrs} />`
          : `<g class="lite-editor-marker" data-editor-only="true"><rect x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" rx="${cornerR}" ${editorGuideAttrs} /></g>`;
      }

      return `<g ${baseAttrs} style="opacity:${opacity};" data-lite-segments="${segs}" data-lite-ratio="${this.escapeXML(le.lineGapRatio || '1:3')}">${hitMarkup}${guideMarkup}${shapeMarkup}</g>`;
    }

    if (le.kind === 'pulse') {
      const period = Math.max(0.2, Number(le.period) || 1.8);
      const amp = Math.max(0.1, Math.min(1, Number(le.intensity) || 0.85));
      const minScale = Math.max(0.2, Math.min(2.0, Number(le.minScale) || 0.85));
      const maxScale = Math.max(minScale, Math.min(3.0, Number(le.maxScale) || 1.15));
      const armThickness = Math.max(1, Math.min(20, Number(le.armThickness) || 3));
      const variant = le.variant || 'circle';
      const isRing = variant === 'ring';
      const fillVal = isRing ? 'none' : color;
      const strokeVal = isRing ? color : 'none';
      const styleStr = `animation-duration:${period}s;--sva-base-dur:${period}s;--sva-pulse-amp:${amp};--sva-pulse-min:${minScale};--sva-pulse-max:${maxScale};`;
      const payloadAttrs = `class="lite-payload sva-lite-pulse-anim" data-semantic-role="payload" data-pulse-min="${minScale}" data-pulse-max="${maxScale}" fill="${fillVal}" stroke="${strokeVal}" stroke-width="${isRing ? sw : 0}" style="${styleStr}" pointer-events="none"`;
      const hitMarkup = `<rect class="lite-hit-target" data-semantic-role="hit-target" x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" fill="transparent" pointer-events="all" style="cursor:pointer;" />`;
      const shapeMarkup = this._buildShapeGeometry(variant, cx, cy, w, h, payloadAttrs, armThickness);
      return `<g ${baseAttrs} style="opacity:${opacity};">${hitMarkup}${shapeMarkup}</g>`;
    }

    if (le.kind === 'flash') {
      const dur = Math.max(0.02, Math.min(1.5, Number(le.dur) || 0.10));
      const durMs = Math.round(dur * 1000);
      const repeatInterval = Math.max(0.2, Math.min(60, Number(le.repeatInterval) || Number(le.period) || 2.0));
      const intensity = Math.max(0.1, Math.min(1, Number(le.intensity) || 0.75));
      const armThickness = Math.max(1, Math.min(20, Number(le.armThickness) || 3));
      const variant = le.variant || 'diamond';
      const hasRepeatSched = Boolean(le.repeat);
      const pauseSec = hasRepeatSched ? Math.max(0.05, repeatInterval - dur) : 0;
      const schedAttrs = hasRepeatSched
        ? ` data-burst-cycles="1" data-pause-seconds="${pauseSec.toFixed(3)}" data-pause-visibility="hide" data-repeat-interval="${repeatInterval}"`
        : '';
      const styleStr = `animation-duration:${dur}s;animation-iteration-count:1;--sva-base-dur:${dur}s;--sva-flash-int:${intensity};`;
      const payloadAttrs = `class="lite-payload lite-flash-anim sva-lite-flash-anim" data-semantic-role="payload" data-trigger-nonce="${le.triggerNonce || 0}" data-flash-duration-ms="${durMs}" data-repeat="${hasRepeatSched ? 'true' : 'false'}"${schedAttrs} fill="${color}" style="${styleStr}" pointer-events="none"`;
      const hitMarkup = `<rect class="lite-hit-target" data-semantic-role="hit-target" x="${x.toFixed(2)}" y="${y.toFixed(2)}" width="${w.toFixed(2)}" height="${h.toFixed(2)}" fill="transparent" pointer-events="all" style="cursor:pointer;" />`;
      const markerAttrs = `fill="none" stroke="${color}" stroke-opacity="0.58" stroke-width="1.4" stroke-dasharray="4 3" pointer-events="none"`;
      const markerShape = this._buildShapeGeometry(variant, cx, cy, w, h, markerAttrs, armThickness);
      const markerCross = `<circle cx="${cx.toFixed(2)}" cy="${cy.toFixed(2)}" r="3" fill="${color}" fill-opacity="0.75" pointer-events="none" />`;
      const editorMarker = `<g class="lite-editor-marker" data-editor-only="true" pointer-events="none">${markerShape}${markerCross}</g>`;
      const shapeMarkup = this._buildShapeGeometry(variant, cx, cy, w, h, payloadAttrs, armThickness);
      return `<g ${baseAttrs} style="opacity:${opacity};">${hitMarkup}${editorMarker}${shapeMarkup}</g>`;
    }

    if (le.kind === 'flow-lite') {
      const flowBuilt = this.buildFlowLiteContent(le);
      return `<g ${baseAttrs} ${flowBuilt.dataAttrs} style="opacity:${opacity};">${flowBuilt.innerMarkup}</g>`;
    }

    return '';
  },

  getLiteBurstScript() {
    const needsBurstScript = this.list.some((le) =>
      (le.kind === 'perimeter-tracer' && Number(le.burstCycles) > 0 && Number(le.pauseSeconds) > 0 && le.playbackMode !== 'once') ||
      (le.kind === 'flash' && Boolean(le.repeat)) ||
      (le.kind === 'flow-lite' && ((Number(le.delay) || 0) + (Number(le.endDelay) || 0)) > 0 && le.playbackMode !== 'once')
    );
    if (!needsBurstScript) return '';
    return `\n<script type="application/ecmascript"><![CDATA[
(function(){
  var cs=document.currentScript;
  var s=(cs&&(cs.ownerSVGElement||(cs.closest&&cs.closest('svg'))))||document.documentElement;
  if(!s||s.hasAttribute('data-sva-runtime-managed')||(s.closest&&s.closest('.sva-widget-wrapper')))return;
  var nodes=s.querySelectorAll('[data-burst-cycles]');
  for(var i=0;i<nodes.length;i++){(function(n){
    n.addEventListener('animationend',function(){
      var cyc=parseInt(n.getAttribute('data-burst-cycles')||'0',10);
      var pSec=parseFloat(n.getAttribute('data-pause-seconds')||'0');
      if(!(cyc>0&&pSec>0))return;
      var vis=n.getAttribute('data-pause-visibility')||'hide';
      if(vis==='hide')n.style.visibility='hidden';
      setTimeout(function(){
        var anim=n.style.animationName||'';
        n.style.animationName='none';
        try{void n.getBoundingClientRect();}catch(_){}
        n.style.visibility='';
        n.style.animationName=anim;
      },Math.max(10,pSec*1000));
    });
  })(nodes[i]);}
})();
]]></script>`;
  },

  injectIntoSVG(svgString) {
    if (!svgString || this.list.length === 0) return svgString;
    const defs = this.getLiteDefs();
    const elements = this.list.map((le) => this.renderLiteElement(le)).join('\n');
    const burstScript = this.getLiteBurstScript();
    const block = `\n${defs}\n<g id="lite-effects-layer" class="lite-effects-layer">\n${elements}\n</g>${burstScript}\n`;
    if (svgString.includes('<!-- INJECT_CAMERA_RIG_ELEMENTS -->')) {
      return svgString.replace('<!-- INJECT_CAMERA_RIG_ELEMENTS -->', block + '<!-- INJECT_CAMERA_RIG_ELEMENTS -->');
    }
    return svgString.replace('</svg>', block + '</svg>');
  },

  bindBurstSchedulers(container) {
    if (!container || typeof container.querySelectorAll !== 'function') return;
    if (this._studioBurstTimers) {
      this._studioBurstTimers.forEach((t) => clearTimeout(t));
    }
    this._studioBurstTimers = [];
    container.querySelectorAll('[data-burst-cycles]').forEach((node) => {
      node.addEventListener('animationend', () => {
        const cyc = parseInt(node.getAttribute('data-burst-cycles') || '0', 10);
        const pSec = parseFloat(node.getAttribute('data-pause-seconds') || '0');
        if (!(cyc > 0 && pSec > 0)) return;
        const vis = node.getAttribute('data-pause-visibility') || 'hide';
        if (vis === 'hide') node.style.visibility = 'hidden';
        const tid = setTimeout(() => {
          if (!node.isConnected) return;
          const prev = node.style.animationName || '';
          node.style.animationName = 'none';
          try { void node.getBoundingClientRect(); } catch (_) {}
          node.style.visibility = '';
          node.style.animationName = prev;
        }, Math.max(10, pSec * 1000));
        this._studioBurstTimers.push(tid);
      });
    });
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
    bindBtn('btnAddLiteFlow', () => this.addByKind('flow-lite'));
    bindBtn('btnTriggerLiteFlash', () => {
      if (this.activeId) this.triggerFlash(this.activeId);
    });
    bindBtn('btnAddLiteFlowNode', () => this.addWaypointToActive());
    bindBtn('btnDropLiteFlowNode', () => this.dropWaypointFromActive());

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
    bindPair('liteArmThickness', 'armThickness');
    bindField('liteLockAspectChk', 'maintainAspectRatio', false, true);
    bindField('liteShowGuideChk', 'showGuide', false, true);
    bindPair('liteCornerRadius', 'cornerRadius', true);
    bindPair('liteStrokeWidth', 'strokeWidth');
    bindPair('liteSegments', 'segments', true);
    const ratioPresetSel = document.getElementById('liteRatioPresetSelect');
    if (ratioPresetSel) {
      ratioPresetSel.addEventListener('change', (e) => {
        const val = e.target.value;
        const customRow = document.getElementById('liteRatioCustomRow');
        if (val === 'custom') {
          if (customRow) customRow.style.display = '';
        } else {
          if (customRow) customRow.style.display = 'none';
          this.updateActive('lineGapRatio', val);
          if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
        }
      });
    }
    bindField('liteRatioInput', 'lineGapRatio');
    bindField('liteDirectionSelect', 'direction');
    bindField('litePlaybackModeSelect', 'playbackMode');
    bindField('liteStyleSelect', 'segmentStyle');
    bindPair('liteSpeed', 'speed');
    bindField('liteBurstCyclesNum', 'burstCycles', true);
    bindField('litePauseSecondsNum', 'pauseSeconds', true);
    bindField('litePauseVisibilitySelect', 'pauseVisibility');
    bindPair('liteMinScale', 'minScale');
    bindPair('liteMaxScale', 'maxScale');
    bindPair('litePeriod', 'period');
    bindPair('liteIntensity', 'intensity');
    bindPair('liteFlashDur', 'durationMs', true);
    bindField('liteRepeatChk', 'repeat', false, true);
    bindPair('liteFlashRepeatInterval', 'repeatInterval');
    bindField('liteVisibleChk', 'visible', false, true);

    // Flow Lite inspector bindings
    const bindFlowActivePt = (srcId, dstId) => {
      const src = document.getElementById(srcId);
      const dst = document.getElementById(dstId);
      if (!src) return;
      src.addEventListener('input', (e) => {
        const idx = Math.max(0, (parseInt(e.target.value, 10) || 1) - 1);
        if (dst) dst.value = idx + 1;
        this.selectWaypoint(idx);
      });
    };
    bindFlowActivePt('liteFlowActivePtSlider', 'liteFlowActivePtNum');
    bindFlowActivePt('liteFlowActivePtNum', 'liteFlowActivePtSlider');

    const bindFlowPtCoord = (sliderId, numId, axis) => {
      const s = document.getElementById(sliderId);
      const n = document.getElementById(numId);
      const handle = (src, dst) => {
        if (!src) return;
        src.addEventListener('input', (e) => {
          const val = parseFloat(e.target.value);
          if (!Number.isFinite(val)) return;
          if (dst) dst.value = val;
          this.updateActiveWaypointCoord(axis, val);
        });
        src.addEventListener('change', () => {
          if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
        });
      };
      handle(s, n);
      handle(n, s);
    };
    bindFlowPtCoord('liteFlowPtXSlider', 'liteFlowPtXNum', 'x');
    bindFlowPtCoord('liteFlowPtYSlider', 'liteFlowPtYNum', 'y');

    const bindFlowBend = (sliderId, numId) => {
      const s = document.getElementById(sliderId);
      const n = document.getElementById(numId);
      const handle = (src, dst) => {
        if (!src) return;
        src.addEventListener('input', (e) => {
          const val = parseFloat(e.target.value);
          if (!Number.isFinite(val)) return;
          if (dst) dst.value = val;
          this.updateActiveSectionBend(val);
        });
        src.addEventListener('change', () => {
          if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
        });
      };
      handle(s, n);
      handle(n, s);
    };
    bindFlowBend('liteFlowSectionBendSlider', 'liteFlowSectionBendNum');

    bindPair('liteFlowCurvature', 'curvature');
    bindField('liteFlowShowGuideChk', 'showGuide', false, true);
    bindPair('liteFlowDashLength', 'dashLength');
    bindPair('liteFlowGapLength', 'gapLength');
    bindPair('liteFlowDotSize', 'dotSize');
    bindPair('liteFlowStrokeWidth', 'strokeWidth');
    bindField('liteFlowCapStyleSelect', 'segmentStyle');
    bindField('liteFlowStartModeSelect', 'startMode');
    bindField('liteFlowEndModeSelect', 'endMode');
    bindField('liteFlowDirectionSelect', 'direction');
    bindField('liteFlowPlaybackModeSelect', 'playbackMode');
    bindPair('liteFlowSpeed', 'speed');
    bindPair('liteFlowDelay', 'delay');
    bindPair('liteFlowEndDelay', 'endDelay');
  },

  addWaypointToActive() {
    if (!this.activeId) return null;
    const le = this.getLiteEffect(this.activeId);
    if (!le || le.kind !== 'flow-lite') return null;
    const pts = Array.isArray(le.points) ? le.points.map((p) => ({ x: p.x, y: p.y })) : [{ x: 25, y: 50 }, { x: 75, y: 50 }];
    const bends = Array.isArray(le.bends) ? le.bends.slice() : new Array(Math.max(1, pts.length - 1)).fill(0);
    const insertAfter = Math.max(0, Math.min(pts.length - 2, this.activePointIndex));
    const p1 = pts[insertAfter];
    const p2 = pts[insertAfter + 1] || p1;
    const midPt = {
      x: Math.round(((p1.x + p2.x) / 2) * 100) / 100,
      y: Math.round(((p1.y + p2.y) / 2) * 100) / 100
    };
    pts.splice(insertAfter + 1, 0, midPt);
    bends.splice(insertAfter + 1, 0, 0);
    this.activePointIndex = insertAfter + 1;
    const updated = this.updateLiteEffect(this.activeId, { points: pts, bends });
    if (updated) {
      this.syncEditorUI(updated);
      this.triggerAppRender();
      if (window.FloatingPropertiesEditor && window.FloatingPropertiesEditor.isOpen && window.FloatingPropertiesEditor.activeType === 'lite') {
        window.FloatingPropertiesEditor.syncFromActiveObject();
      }
      if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
    }
    return updated;
  },

  dropWaypointFromActive() {
    if (!this.activeId) return null;
    const le = this.getLiteEffect(this.activeId);
    if (!le || le.kind !== 'flow-lite') return null;
    const pts = Array.isArray(le.points) ? le.points.map((p) => ({ x: p.x, y: p.y })) : [];
    if (pts.length <= 2) return le;
    const bends = Array.isArray(le.bends) ? le.bends.slice() : new Array(pts.length - 1).fill(0);
    const dropIdx = Math.max(0, Math.min(pts.length - 1, this.activePointIndex));
    pts.splice(dropIdx, 1);
    const bendDropIdx = Math.min(bends.length - 1, dropIdx);
    bends.splice(bendDropIdx, 1);
    this.activePointIndex = Math.max(0, Math.min(pts.length - 1, this.activePointIndex));
    const updated = this.updateLiteEffect(this.activeId, { points: pts, bends });
    if (updated) {
      this.syncEditorUI(updated);
      this.triggerAppRender();
      if (window.FloatingPropertiesEditor && window.FloatingPropertiesEditor.isOpen && window.FloatingPropertiesEditor.activeType === 'lite') {
        window.FloatingPropertiesEditor.syncFromActiveObject();
      }
      if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
    }
    return updated;
  },

  selectWaypoint(index) {
    if (!this.activeId) return;
    const le = this.getLiteEffect(this.activeId);
    if (!le || le.kind !== 'flow-lite' || !Array.isArray(le.points)) return;
    this.activePointIndex = Math.max(0, Math.min(le.points.length - 1, Number(index) || 0));
    this.syncEditorUI(le);
    if (window.FloatingPropertiesEditor && window.FloatingPropertiesEditor.isOpen && window.FloatingPropertiesEditor.activeType === 'lite') {
      window.FloatingPropertiesEditor.syncFromActiveObject();
    }
    if (window.AppCoordinator && typeof window.AppCoordinator.updateActiveOverlays === 'function') {
      window.AppCoordinator.updateActiveOverlays();
    }
  },

  updateActiveWaypointCoord(axis, value) {
    if (!this.activeId) return;
    const le = this.getLiteEffect(this.activeId);
    if (!le || le.kind !== 'flow-lite' || !Array.isArray(le.points)) return;
    const idx = Math.max(0, Math.min(le.points.length - 1, this.activePointIndex));
    const pts = le.points.map((p) => ({ x: p.x, y: p.y }));
    pts[idx][axis] = Math.round(Number(value) * 100) / 100;
    this.updateActive('points', pts);
  },

  updateActiveSectionBend(value) {
    if (!this.activeId) return;
    const le = this.getLiteEffect(this.activeId);
    if (!le || le.kind !== 'flow-lite' || !Array.isArray(le.points)) return;
    const secCount = Math.max(1, le.points.length - 1);
    const secIdx = Math.max(0, Math.min(secCount - 1, this.activePointIndex));
    const bends = Array.isArray(le.bends) ? le.bends.slice() : new Array(secCount).fill(0);
    while (bends.length < secCount) bends.push(0);
    bends[secIdx] = Math.max(-100, Math.min(100, Number(value) || 0));
    this.updateActive('bends', bends);
  },

  translateFlowLitePoints(id, deltaXPct, deltaYPct) {
    const le = this.getLiteEffect(id);
    if (!le || le.kind !== 'flow-lite' || !Array.isArray(le.points)) return null;
    const pts = le.points.map((p) => ({
      x: Math.round((p.x + deltaXPct) * 1000) / 1000,
      y: Math.round((p.y + deltaYPct) * 1000) / 1000
    }));
    return this.updateLiteEffect(id, { points: pts });
  },

  addByKind(kind) {
    const offset = (this.list.length % 5) * 28;
    const offsetPct = (this.list.length % 5) * 3;
    const initial = {
      kind,
      x: 240 + offset,
      y: 180 + offset
    };
    if (kind === 'flow-lite') {
      initial.points = [
        { x: 22 + offsetPct, y: 45 + offsetPct },
        { x: 74 + offsetPct, y: 45 + offsetPct }
      ];
      initial.bends = [0];
      this.activePointIndex = 0;
    }
    const created = this.createLiteEffect(initial);
    this.selectLite(created.id);
    this.renderList();
    this.triggerAppRender();
    if (window.AppCoordinator) window.AppCoordinator.recordHistoryImmediate();
    return created;
  },

  selectLite(id, opts = {}) {
    this.activeId = String(id);
    const le = this.getLiteEffect(id);
    if (le && le.kind === 'flow-lite') {
      document.body.classList.add('editing-flow');
    } else {
      document.body.classList.remove('editing-flow');
    }
    this.syncEditorUI(le);
    this.renderList();
    this.updateUIOverlay(le);
    if (typeof window.syncInspectorLayerSections === 'function') {
      window.syncInspectorLayerSections();
    }
    if (!opts.skipCoordinator && window.AppCoordinator && typeof window.AppCoordinator.selectObject === 'function') {
      window.AppCoordinator.selectObject('lite', String(id), { openAccordionId: 'acc-lite-effects' });
    }
  },

  updateUIOverlay(targetLite) {
    if (typeof document === 'undefined') return;
    const container = document.getElementById('dynamicFlowReticles');
    const pathEl = document.getElementById('flowCurvePath');
    if (!container || !pathEl) return;

    // Do not clobber main DataFlows reticles when a main Flow is active
    if (window.DataFlows && window.DataFlows.activeId !== null) return;

    const le = targetLite !== undefined ? targetLite : (this.activeId ? this.getLiteEffect(this.activeId) : null);
    if (!le || le.kind !== 'flow-lite' || le.visible === false || !Array.isArray(le.points)) {
      container.innerHTML = '';
      pathEl.style.display = 'none';
      return;
    }

    document.body.classList.add('editing-flow');
    const geom = this._computeFlowLiteGeometry(le, 0, 0);
    if (le.points.length >= 2) {
      pathEl.style.display = 'block';
      pathEl.setAttribute('d', geom.pathD);
      pathEl.setAttribute('stroke', le.color || '#38bdf8');
    } else {
      pathEl.style.display = 'none';
    }

    const safeColor = this.sanitizeColor(le.color, '#38bdf8');
    const existingReticles = container.querySelectorAll('[data-lite-flow-pt-idx]');
    if (existingReticles.length === le.points.length) {
      le.points.forEach((pt, idx) => {
        const isActive = idx === this.activePointIndex;
        const el = existingReticles[idx];
        el.className = 'reticle' + (isActive ? ' active-reticle' : '');
        el.style.left = pt.x + '%';
        el.style.top = pt.y + '%';
        el.style.background = isActive ? (safeColor + '4D') : 'rgba(0,0,0,0.65)';
        el.style.borderColor = safeColor;
        el.style.zIndex = isActive ? '12' : '11';
      });
    } else {
      container.innerHTML = '';
      le.points.forEach((pt, idx) => {
        const isActive = idx === this.activePointIndex;
        const ret = document.createElement('div');
        ret.className = 'reticle' + (isActive ? ' active-reticle' : '');
        ret.setAttribute('data-flow-node-idx', String(idx));
        ret.setAttribute('data-lite-flow-pt-idx', String(idx));
        ret.style.left = (Number(pt.x) || 0) + '%';
        ret.style.top = (Number(pt.y) || 0) + '%';
        ret.style.pointerEvents = 'auto';
        ret.style.background = isActive ? (safeColor + '4D') : 'rgba(0,0,0,0.65)';
        ret.style.borderColor = safeColor;
        ret.style.zIndex = isActive ? '12' : '11';
        ret.style.cursor = 'grab';
        ret.textContent = 'P' + (idx + 1);
        container.appendChild(ret);
      });
    }
  },

  handleStageClick(xPct, yPct, dragMode = null) {
    if (!this.activeId) return false;
    const le = this.getLiteEffect(this.activeId);
    if (!le) return false;
    if (le.kind === 'flow-lite' && Array.isArray(le.points) && le.points.length >= 2) {
      if (dragMode === 'translate' && Array.isArray(this._dragStartPoints) && this._dragStartAnchor) {
        const dx = Number(xPct) - this._dragStartAnchor.x;
        const dy = Number(yPct) - this._dragStartAnchor.y;
        const nextPts = this._dragStartPoints.map((p) => ({
          x: Math.round((p.x + dx) * 100) / 100,
          y: Math.round((p.y + dy) * 100) / 100
        }));
        const updated = this.updateLiteEffect(this.activeId, { points: nextPts });
        if (updated) {
          this.syncEditorUI(updated);
          this.updateUIOverlay(updated);
          this.triggerAppRender();
          return true;
        }
        return false;
      }
      const idx = Math.max(0, Math.min(le.points.length - 1, this.activePointIndex));
      const nextPts = le.points.map((p) => ({ x: p.x, y: p.y }));
      nextPts[idx] = {
        x: Math.round(Number(xPct) * 100) / 100,
        y: Math.round(Number(yPct) * 100) / 100
      };
      const updated = this.updateLiteEffect(this.activeId, { points: nextPts });
      if (updated) {
        this.syncEditorUI(updated);
        this.updateUIOverlay(updated);
        this.triggerAppRender();
        return true;
      }
      return false;
    }
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
    document.body.classList.remove('editing-flow');
    this.updateUIOverlay(null);
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
        patch.height = Math.max(5, Math.round(value * (existing.height / existing.width)));
      }
    } else if (key === 'height' && lockAspect) {
      if (isCircleTracer || existing.width === existing.height) {
        patch.width = value;
      } else if (existing.width > 0 && existing.height > 0) {
        patch.width = Math.max(5, Math.round(value * (existing.width / existing.height)));
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
        'flash': ['diamond', 'circle', 'square', 'cross', 'star'],
        'flow-lite': ['dashed', 'dotted', 'single']
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
    setPair('liteArmThickness', le.armThickness != null ? le.armThickness : 3);
    setChk('liteLockAspectChk', le.kind === 'perimeter-tracer' && le.variant === 'circle' ? true : Boolean(le.maintainAspectRatio));
    setChk('liteShowGuideChk', le.showGuide !== false);
    setPair('liteCornerRadius', Math.round(Number(le.cornerRadius != null ? le.cornerRadius : 10)));
    setPair('liteStrokeWidth', le.strokeWidth || 2.5);
    setPair('liteSegments', le.segments || 2);
    const ratioVal = le.lineGapRatio || '1:3';
    const knownRatios = ['1:1', '1:2', '1:3', '1:4', '1:5', '1:6', '2:1', '2:3', '3:1', '3:2'];
    const isKnownRatio = knownRatios.includes(ratioVal);
    setVal('liteRatioPresetSelect', isKnownRatio ? ratioVal : 'custom');
    setVal('liteRatioInput', ratioVal);
    const customRow = document.getElementById('liteRatioCustomRow');
    if (customRow) customRow.style.display = isKnownRatio ? 'none' : '';
    setVal('liteDirectionSelect', le.direction || 'cw');
    setVal('litePlaybackModeSelect', le.playbackMode || 'loop');
    setVal('liteStyleSelect', le.segmentStyle || 'rounded');
    setPair('liteSpeed', le.speed || 3.0);
    setVal('liteBurstCyclesNum', Math.round(Number(le.burstCycles) || 0));
    setVal('litePauseSecondsNum', Number(le.pauseSeconds) || 0);
    setVal('litePauseVisibilitySelect', le.pauseVisibility || 'hide');
    setPair('liteMinScale', le.minScale != null ? le.minScale : 0.85);
    setPair('liteMaxScale', le.maxScale != null ? le.maxScale : 1.15);
    setPair('litePeriod', le.period || 1.8);
    setPair('liteIntensity', le.intensity != null ? le.intensity : (le.kind === 'flash' ? 0.75 : 0.85));
    setPair('liteFlashDur', durMs);
    setChk('liteRepeatChk', le.repeat);
    setPair('liteFlashRepeatInterval', le.repeatInterval || 2.0);
    setChk('liteVisibleChk', le.visible !== false);

    const dimGroup = document.getElementById('liteDimensionsGroup');
    const posRow = document.getElementById('liteXNum') ? document.getElementById('liteXNum').closest('.inspector-prop-row') : null;
    const tracerGroup = document.getElementById('liteTracerGroup');
    const cornerRow = document.getElementById('liteCornerRadiusRow');
    const armRow = document.getElementById('liteArmThicknessRow');
    const pulseGroup = document.getElementById('litePulseGroup');
    const flashGroup = document.getElementById('liteFlashGroup');
    const flowGroup = document.getElementById('liteFlowGroup');
    const intensityRow = document.getElementById('liteIntensityRow');
    if (dimGroup) dimGroup.style.display = le.kind === 'flow-lite' ? 'none' : '';
    if (posRow) posRow.style.display = le.kind === 'flow-lite' ? 'none' : '';
    if (tracerGroup) tracerGroup.style.display = le.kind === 'perimeter-tracer' ? 'block' : 'none';
    if (cornerRow) cornerRow.style.display = (le.kind === 'perimeter-tracer' && le.variant === 'rounded') ? '' : 'none';
    if (armRow) armRow.style.display = ((le.kind === 'pulse' || le.kind === 'flash') && (le.variant === 'cross' || le.variant === 'star')) ? '' : 'none';
    if (pulseGroup) pulseGroup.style.display = le.kind === 'pulse' ? 'block' : 'none';
    if (flashGroup) flashGroup.style.display = le.kind === 'flash' ? 'block' : 'none';
    if (flowGroup) flowGroup.style.display = le.kind === 'flow-lite' ? 'block' : 'none';
    if (intensityRow) intensityRow.style.display = (le.kind === 'pulse' || le.kind === 'flash') ? '' : 'none';

    if (le.kind === 'flow-lite') {
      const pts = Array.isArray(le.points) && le.points.length >= 2 ? le.points : [{ x: 25, y: 50 }, { x: 75, y: 50 }];
      const bends = Array.isArray(le.bends) ? le.bends : new Array(pts.length - 1).fill(0);
      if (this.activePointIndex >= pts.length) this.activePointIndex = pts.length - 1;
      if (this.activePointIndex < 0) this.activePointIndex = 0;
      const activePt = pts[this.activePointIndex] || pts[0];
      const secIdx = Math.min(pts.length - 2, this.activePointIndex);
      const secBend = Number(bends[secIdx]) || 0;

      const ptSlider = document.getElementById('liteFlowActivePtSlider');
      const ptNum = document.getElementById('liteFlowActivePtNum');
      if (ptSlider) { ptSlider.max = String(pts.length); ptSlider.value = String(this.activePointIndex + 1); }
      if (ptNum) { ptNum.max = String(pts.length); ptNum.value = String(this.activePointIndex + 1); }
      const dropBtn = document.getElementById('btnDropLiteFlowNode');
      if (dropBtn) dropBtn.disabled = pts.length <= 2;

      setPair('liteFlowPtX', Math.round(activePt.x * 10) / 10);
      setPair('liteFlowPtY', Math.round(activePt.y * 10) / 10);
      setPair('liteFlowSectionBend', secBend);
      setPair('liteFlowCurvature', Number(le.curvature) || 0);
      setChk('liteFlowShowGuideChk', le.showGuide !== false);
      setPair('liteFlowDashLength', le.dashLength || (le.variant === 'single' ? 36 : 18));
      setPair('liteFlowGapLength', le.gapLength || 14);
      setPair('liteFlowDotSize', le.dotSize || 5);
      setPair('liteFlowStrokeWidth', le.strokeWidth || 3);
      setVal('liteFlowCapStyleSelect', le.segmentStyle || 'rounded');
      setVal('liteFlowStartModeSelect', le.startMode || 'tunnel');
      setVal('liteFlowEndModeSelect', le.endMode || 'tunnel');
      setVal('liteFlowDirectionSelect', le.direction === 'reverse' ? 'reverse' : 'forward');
      setVal('liteFlowPlaybackModeSelect', le.playbackMode || 'loop');
      setPair('liteFlowSpeed', le.speed || 2.0);
      setPair('liteFlowDelay', le.delay || 0);
      setPair('liteFlowEndDelay', le.endDelay || 0);

      const isDotted = le.variant === 'dotted';
      const isSingle = le.variant === 'single';
      const dashRow = document.getElementById('liteFlowDashLengthRow');
      const gapRow = document.getElementById('liteFlowGapLengthRow');
      const dotRow = document.getElementById('liteFlowDotSizeRow');
      const swRow = document.getElementById('liteFlowStrokeWidthRow');
      const capRow = document.getElementById('liteFlowCapStyleRow');
      if (dashRow) dashRow.style.display = isDotted ? 'none' : '';
      if (gapRow) gapRow.style.display = isSingle ? 'none' : '';
      if (dotRow) dotRow.style.display = isDotted ? '' : 'none';
      if (swRow) swRow.style.display = isDotted ? 'none' : '';
      if (capRow) capRow.style.display = isDotted ? 'none' : '';
    }

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
    const iconByKind = { 'perimeter-tracer': '▭', 'pulse': '◎', 'flash': '✦', 'flow-lite': '↝' };
    const kindShort = { 'perimeter-tracer': 'Tracer', 'pulse': 'Pulse', 'flash': 'Flash', 'flow-lite': 'Flow' };
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
