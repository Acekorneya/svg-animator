/**
 * SVG Animator Studio — Reusable SVG Widget Runtime & Mounting Library (`svg-widget.js`)
 *
 * Stage 16B: External Widget Controls, Runtime Parameters and Independent Host Integration.
 *
 * Responsibilities:
 *   - Parse and namespace SVG documents for safe multi-instance embedding on host pages.
 *   - Scope all element IDs, internal references (url(#...), href="#...", xlink:href="#...").
 *   - Remap SMIL animation targets (<mpath>, event timing begin/end).
 *   - Scope embedded <style> selectors to widget root element.
 *   - Prefix @keyframes and remap animation/animation-name properties to prevent stylesheet collisions.
 *   - Safely preserve data URLs (data:image/...) and text content containing '#'.
 *   - Manifest parsing and runtime parameter binding with atomic validation.
 *   - Semantic target resolution (semantic target name -> scoped element in instance DOM).
 *   - Per-instance independent animation control (play, pause, resume, stop, speed).
 *   - Universal UMD module: Browser window global, CommonJS (Node.js), and ES module environments.
 */

(function(root, factory) {
  if (typeof define === 'function' && define.amd) {
    define([], factory);
  } else if (typeof module === 'object' && module.exports) {
    module.exports = factory();
  } else {
    root.SVGWidget = factory();
  }
})(typeof self !== 'undefined' ? self : this, function() {
  'use strict';

  let widgetCounter = 0;

  function generateInstanceId() {
    widgetCounter += 1;
    const rnd = Math.random().toString(36).slice(2, 7);
    const ts = (Date.now() % 1000000).toString(36);
    return `sva-w-${ts}-${rnd}-${widgetCounter}`;
  }

  function escapeRegex(str) {
    return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  const COLOR_REGEX = /^(?:#[0-9a-fA-F]{3,8}|(?:rgb|hsl)a?\([^)]+\)|[a-zA-Z]+|transparent|currentColor)$/;

  /**
   * Scopes CSS rules to the widget root ID and prefixes keyframes.
   */
  function scopeCSSRules(css, rootId, idMap, keyframeMap) {
    let transformedCSS = css;

    // 1. Rename @keyframes definitions and usages
    for (const [oldKF, newKF] of Object.entries(keyframeMap)) {
      const reKF = new RegExp(`@keyframes\\s+${escapeRegex(oldKF)}\\b`, 'g');
      transformedCSS = transformedCSS.replace(reKF, `@keyframes ${newKF}`);

      const reAnim = new RegExp(`\\b(animation(?:-name)?\\s*:\\s*[^;}]*?)\\b${escapeRegex(oldKF)}\\b`, 'g');
      transformedCSS = transformedCSS.replace(reAnim, `$1${newKF}`);
    }

    // 2. Rewrite url(#oldId) inside CSS rules
    for (const [oldId, newId] of Object.entries(idMap)) {
      const reUrl = new RegExp(`url\\(\\s*["']?#${escapeRegex(oldId)}["']?\\s*\\)`, 'g');
      transformedCSS = transformedCSS.replace(reUrl, `url(#${newId})`);
    }

    // 3. Temporarily stash @keyframes blocks so their nested rules are not mangled
    const kfBlocks = [];
    const reKFBlock = /@keyframes\s+[^{]+\{(?:[^{}]*\{[^{}]*\}[^{}]*)*\}/g;
    const cssWithoutKF = transformedCSS.replace(reKFBlock, (m) => {
      kfBlocks.push(m);
      return `/*__SVA_KF_BLOCK_${kfBlocks.length - 1}__*/`;
    });

    // 4. Scope remaining rule selectors to #rootId
    const reRule = /([^{}/]+)\{([^{}]+)\}/g;
    let scoped = cssWithoutKF.replace(reRule, (match, rawSel, body) => {
      const sel = rawSel.trim();
      if (sel.startsWith('@') || sel.startsWith('/*')) return match;

      const parts = sel.split(',');
      const scopedParts = [];
      for (let s of parts) {
        let clean = s.trim();
        if (!clean) continue;

        // Replace any #oldId in selector
        for (const [oldId, newId] of Object.entries(idMap)) {
          clean = clean.replace(new RegExp(`#${escapeRegex(oldId)}\\b`, 'g'), `#${newId}`);
        }
        scopedParts.push(`#${rootId} ${clean}`);
      }

      if (scopedParts.length === 0) return match;
      return `\n    ${scopedParts.join(', ')} {${body}}`;
    });

    // 5. Restore @keyframes blocks
    kfBlocks.forEach((kf, idx) => {
      scoped = scoped.replace(`/*__SVA_KF_BLOCK_${idx}__*/`, kf);
    });

    return scoped;
  }

  /**
   * Core string/regex-based SVG widget isolator.
   * Works identically in browser DOM and Node.js headless environments.
   */
  function isolateSVGString(rawSvg, options = {}) {
    if (!rawSvg || typeof rawSvg !== 'string') {
      throw new Error('SVGWidget.isolate requires a non-empty SVG string.');
    }

    let instanceId = (options.instanceId && String(options.instanceId).trim()) || generateInstanceId();
    instanceId = instanceId.replace(/[^a-zA-Z0-9_-]/g, '-');
    const prefix = (options.prefix && String(options.prefix).trim()) || `${instanceId}__`;
    const rootId = `sva-widget-${instanceId}`;
    const scopeCss = options.scopeCss !== false;

    // 1. Scan and collect declared IDs: id="VALUE" or id='VALUE'
    const idMap = {};
    const reIDDecl = /\bid\s*=\s*(["'])([^"']+)\1/g;
    let m;
    while ((m = reIDDecl.exec(rawSvg)) !== null) {
      const oldId = m[2];
      if (oldId && !idMap[oldId]) {
        idMap[oldId] = `${prefix}${oldId}`;
      }
    }

    // 2. Scan and collect @keyframes in <style>
    const keyframeMap = {};
    const reKF = /@keyframes\s+([a-zA-Z0-9_-]+)/g;
    while ((m = reKF.exec(rawSvg)) !== null) {
      const oldKF = m[1];
      if (oldKF && !keyframeMap[oldKF]) {
        keyframeMap[oldKF] = `${prefix}${oldKF}`;
      }
    }

    // 3. Transform CSS in <style> blocks
    let transformed = rawSvg.replace(/<style\b([^>]*)>([\s\S]*?)<\/style>/gi, (match, attrs, css) => {
      let scopedCSS = css;
      if (scopeCss) {
        scopedCSS = scopeCSSRules(css, rootId, idMap, keyframeMap);
      }
      return `<style${attrs}>${scopedCSS}</style>`;
    });

    // 4. Transform tag attributes
    let isFirstSVG = true;
    const reTag = /<([a-zA-Z0-9:_-]+)((?:\s+[^>]+)?)>/g;

    transformed = transformed.replace(reTag, (tagMatch, tagName, tagAttrs) => {
      if (tagName.startsWith('/')) return tagMatch;

      if (tagName.toLowerCase() === 'svg' && isFirstSVG) {
        isFirstSVG = false;
        // Set root ID and instance attribute
        if (/\bid\s*=\s*["'][^"']*["']/i.test(tagAttrs)) {
          tagAttrs = tagAttrs.replace(/\bid\s*=\s*["'][^"']*["']/i, `id="${rootId}"`);
        } else {
          tagAttrs = ` id="${rootId}"` + tagAttrs;
        }

        if (!/data-sva-widget-instance/i.test(tagAttrs)) {
          tagAttrs += ` data-sva-widget-instance="${instanceId}"`;
        }
      } else {
        // Remap declared id attributes on non-root elements
        tagAttrs = tagAttrs.replace(/\bid\s*=\s*(["'])([^"']+)\1/g, (match, q, oldId) => {
          if (idMap[oldId]) {
            return `id=${q}${idMap[oldId]}${q}`;
          }
          return match;
        });
      }

      // Remap href and xlink:href if starting with # (fragment reference only)
      tagAttrs = tagAttrs.replace(/\b(xlink:href|href)\s*=\s*(["'])(#)([^"']+)\2/g, (match, attr, q, hash, targetId) => {
        if (idMap[targetId]) {
          return `${attr}=${q}#${idMap[targetId]}${q}`;
        }
        return match;
      });

      // Remap url(#oldId) in attributes (fill, stroke, filter, clip-path, mask, marker-*, style)
      tagAttrs = tagAttrs.replace(/url\(\s*["']?#([a-zA-Z0-9:_-]+)["']?\s*\)/g, (match, targetId) => {
        if (idMap[targetId]) {
          return `url(#${idMap[targetId]})`;
        }
        return match;
      });

      // Remap aria-labelledby and aria-describedby
      tagAttrs = tagAttrs.replace(/\b(aria-labelledby|aria-describedby)\s*=\s*(["'])([^"']+)\2/g, (match, attr, q, rawTokens) => {
        const tokens = rawTokens.split(/\s+/).filter(Boolean);
        const mapped = tokens.map((t) => idMap[t] || t);
        return `${attr}=${q}${mapped.join(' ')}${q}`;
      });

      // Remap SMIL begin/end event targets: begin="anim1.begin" / end="anim1.end+2s"
      tagAttrs = tagAttrs.replace(/\b(begin|end)\s*=\s*(["'])([^"']+)\2/g, (match, attr, q, val) => {
        let timing = val;
        for (const [oldId, newId] of Object.entries(idMap)) {
          timing = timing.replace(new RegExp(`\\b${escapeRegex(oldId)}\\.`, 'g'), `${newId}.`);
        }
        return `${attr}=${q}${timing}${q}`;
      });

      return `<${tagName}${tagAttrs}>`;
    });

    // 5. Inject manifest if provided and not already present
    if (options.manifest && Array.isArray(options.manifest.parameters) && options.manifest.parameters.length > 0) {
      const reExistingManifest = new RegExp(/<script\b[^>]*\bid=["'](?:[^"']*__)?sva-widget-manifest["']/i.source, 'i');
      if (!reExistingManifest.test(transformed)) {
        const manifestJson = JSON.stringify(options.manifest);
        const manifestTag = '\n  <script type="application/json" id="' + prefix + 'sva-widget-manifest">' + manifestJson + '<' + '/script>\n';
        const lastSvgIdx = transformed.lastIndexOf('</svg>');
        if (lastSvgIdx !== -1) {
          transformed = transformed.slice(0, lastSvgIdx) + manifestTag + transformed.slice(lastSvgIdx);
        } else {
          transformed += manifestTag;
        }
      }
    }

    if (options.rawDetails) {
      return {
        success: true,
        isolatedSvg: transformed,
        instanceId,
        prefix,
        rootId,
        idMap
      };
    }

    return transformed;
  }

  /**
   * Extracts embedded widget manifest from raw SVG string.
   */
  function extractManifestFromSVG(rawSvg) {
    if (!rawSvg || typeof rawSvg !== 'string') return null;
    const re = new RegExp(/<script\b[^>]*\bid=["'](?:[^"']*__)?sva-widget-manifest["'][^>]*>([\s\S]*?)<\//.source + 'script>', 'i');
    const match = re.exec(rawSvg);
    if (match && match[1]) {
      try {
        const parsed = JSON.parse(match[1]);
        if (parsed && Array.isArray(parsed.parameters)) {
          return parsed;
        }
      } catch (e) {
        console.warn('[SVGWidget] Failed to parse embedded manifest JSON:', e);
      }
    }
    return null;
  }

  /**
   * Validates a candidate value against a parameter definition.
   */
  function validateParameterValue(paramDef, value) {
    if (value === undefined || value === null) {
      return { valid: false, error: `Value for parameter '${paramDef.name}' cannot be null or undefined.` };
    }
    switch (paramDef.type) {
      case 'color': {
        const str = String(value).trim();
        if (!COLOR_REGEX.test(str)) {
          return { valid: false, error: `Invalid color value '${value}' for parameter '${paramDef.name}'. Must be a valid hex, rgb, hsl, or named color.` };
        }
        return { valid: true, value: str };
      }
      case 'number':
      case 'speed': {
        const num = Number(value);
        if (isNaN(num) || !isFinite(num)) {
          return { valid: false, error: `Value '${value}' for parameter '${paramDef.name}' must be a finite number.` };
        }
        if (paramDef.min !== undefined && paramDef.min !== null && num < paramDef.min) {
          return { valid: false, error: `Value ${num} is below minimum allowed (${paramDef.min}) for parameter '${paramDef.name}'.` };
        }
        if (paramDef.max !== undefined && paramDef.max !== null && num > paramDef.max) {
          return { valid: false, error: `Value ${num} is above maximum allowed (${paramDef.max}) for parameter '${paramDef.name}'.` };
        }
        return { valid: true, value: num };
      }
      case 'boolean': {
        return { valid: true, value: Boolean(value) };
      }
      case 'enum': {
        const strVal = String(value);
        if (Array.isArray(paramDef.options) && paramDef.options.length > 0) {
          if (!paramDef.options.includes(strVal)) {
            return {
              valid: false,
              error: `Value '${strVal}' is not a permitted option for enum parameter '${paramDef.name}'. Permitted options: [${paramDef.options.join(', ')}]`
            };
          }
        }
        return { valid: true, value: strVal };
      }
      case 'text': {
        return { valid: true, value: String(value) };
      }
      default:
        return { valid: true, value };
    }
  }

  /**
   * Resolves semantic target name to a DOM element inside the mounted SVG.
   */
  function resolveTargetElement(svgEl, idMap, targetName) {
    if (!svgEl) return null;
    if (!targetName || targetName === 'root' || targetName === 'svg') {
      return svgEl;
    }

    // 1. data-semantic-target
    let el = svgEl.querySelector(`[data-semantic-target="${targetName}"]`);
    if (el) return el;

    // 2. idMap lookup (authored ID -> isolated ID)
    const mappedId = idMap && idMap[targetName];
    if (mappedId) {
      el = svgEl.querySelector(`#${mappedId}`) || (svgEl.ownerDocument && svgEl.ownerDocument.getElementById(mappedId));
      if (el) return el;
    }

    // 3. data-node-id, data-flow-id, data-image-id, data-glow-id, data-ripple-id
    el = svgEl.querySelector(`[data-node-id="${targetName}"], [data-flow-id="${targetName}"], [data-image-id="${targetName}"], [data-glow-id="${targetName}"], [data-ripple-id="${targetName}"]`);
    if (el) return el;

    // Stripped prefix fallback (e.g. "node-1" -> "1")
    const strippedId = String(targetName).replace(/^(node|flow|glow|ripple|image)-/, '');
    if (strippedId !== targetName) {
      el = svgEl.querySelector(`[data-node-id="${strippedId}"], [data-flow-id="${strippedId}"], [data-image-id="${strippedId}"], [data-glow-id="${strippedId}"], [data-ripple-id="${strippedId}"]`);
      if (el) return el;
    }

    // 4. Scoped direct ID query
    try {
      el = svgEl.querySelector(`#${targetName}`);
      if (el) return el;
    } catch (e) {}

    // 5. CSS selector fallback
    try {
      el = svgEl.querySelector(targetName);
      if (el) return el;
    } catch (e) {}

    return null;
  }

  /**
   * Applies validated property to the resolved DOM element.
   */
  function applyPropertyToElement(el, property, value, paramDef, instance) {
    switch (property) {
      case 'fill':
        if (el) {
          el.setAttribute('fill', value);
          el.style.fill = value;
          if (el.tagName && el.tagName.toLowerCase() === 'g') {
            el.querySelectorAll('rect, circle, path, polygon, ellipse').forEach(c => {
              c.setAttribute('fill', value);
              c.style.fill = value;
            });
          }
        }
        break;
      case 'stroke':
        if (el) {
          el.setAttribute('stroke', value);
          el.style.stroke = value;
          if (el.tagName && el.tagName.toLowerCase() === 'g') {
            el.querySelectorAll('rect, circle, path, polygon, ellipse, line').forEach(c => {
              c.setAttribute('stroke', value);
              c.style.stroke = value;
            });
          }
        }
        break;
      case 'color':
        if (el) {
          el.style.color = value;
          el.setAttribute('fill', value);
          el.style.fill = value;
          if (el.tagName && el.tagName.toLowerCase() === 'g') {
            el.querySelectorAll('rect, circle, path, polygon, ellipse').forEach(c => {
              c.setAttribute('fill', value);
              c.style.fill = value;
            });
          }
        }
        break;
      case 'visibility': {
        const isVisible = value !== false && value !== 'hidden' && value !== 0;
        const visStr = isVisible ? 'visible' : 'hidden';
        if (el) {
          el.style.visibility = visStr;
          el.setAttribute('visibility', visStr);
        }
        break;
      }
      case 'opacity':
        if (el) {
          el.setAttribute('opacity', String(value));
          el.style.opacity = String(value);
        }
        break;
      case 'text':
      case 'textContent':
        if (el) {
          const textEl = (el.tagName && el.tagName.toLowerCase() === 'text') ? el : el.querySelector('text');
          if (textEl) {
            textEl.textContent = String(value);
          } else {
            el.textContent = String(value);
          }
        }
        break;
      case 'playbackRate':
      case 'speed':
        if (instance && typeof instance.setSpeed === 'function') {
          instance.setSpeed(value);
        }
        break;
      case 'state':
      case 'operatingState': {
        const stateStr = String(value);
        if (el) el.setAttribute('data-state', stateStr);
        if (instance && instance.wrapper) instance.wrapper.setAttribute('data-state', stateStr);

        // State enum actions
        if (stateStr === 'paused') {
          if (instance) instance.pause();
        } else if (stateStr === 'stopped') {
          if (instance) instance.stop();
        } else if (stateStr === 'running') {
          if (instance) instance.resume();
        }
        break;
      }
      default:
        if (el) {
          try { el.setAttribute(property, String(value)); } catch (e) {}
          try { el.style[property] = String(value); } catch (e) {}
          if (el.tagName && el.tagName.toLowerCase() === 'g') {
            const primary = el.querySelector('rect, circle, path, polygon, ellipse');
            if (primary) {
              try { primary.setAttribute(property, String(value)); } catch (e) {}
              try { primary.style[property] = String(value); } catch (e) {}
            }
          }
        }
        break;
    }
  }

  const SVGWidget = {
    version: '1.2.0',

    generateInstanceId,

    /**
     * Isolates an SVG string so all IDs, references, CSS, and animations
     * are strictly scoped to the given instanceId.
     */
    isolate(rawSvg, options = {}) {
      return isolateSVGString(rawSvg, options);
    },

    /**
     * Parses an isolated SVG string into a live SVGElement.
     */
    create(rawSvg, options = {}) {
      const isolatedSvg = isolateSVGString(rawSvg, options);
      if (typeof DOMParser !== 'undefined') {
        const parser = new DOMParser();
        const doc = parser.parseFromString(isolatedSvg, 'image/svg+xml');
        const err = doc.querySelector('parsererror');
        if (err) {
          throw new Error('SVGWidget.create XML parser error: ' + err.textContent);
        }
        return doc.documentElement;
      }
      throw new Error('SVGWidget.create requires a browser DOM environment with DOMParser.');
    },

    /**
     * Mounts an SVG widget instance into a DOM container element.
     * Supports optional Shadow DOM isolation for total encapsulation.
     * Returns an authoritative, fully interactive widget instance handle.
     */
    mount(target, rawSvg, options = {}) {
      if (typeof document === 'undefined') {
        throw new Error('SVGWidget.mount requires a browser document environment.');
      }

      let container = target;
      if (typeof target === 'string') {
        container = document.querySelector(target);
      }
      if (!container || !container.appendChild) {
        throw new Error(`SVGWidget.mount target container not found: ${target}`);
      }

      // 1. Resolve manifest (options.manifest or embedded script in rawSvg)
      let manifest = options.manifest;
      if (!manifest) {
        manifest = extractManifestFromSVG(rawSvg);
      }
      if (!manifest || !Array.isArray(manifest.parameters)) {
        manifest = { schemaVersion: 1, parameters: [] };
      }

      // 2. Isolate SVG markup
      const instanceId = options.instanceId || generateInstanceId();
      const details = isolateSVGString(rawSvg, {
        instanceId,
        prefix: options.prefix,
        scopeCss: options.scopeCss !== false,
        manifest: manifest,
        rawDetails: true
      });

      // 3. Mount in container (Light DOM or Shadow DOM)
      let mountRoot = container;
      let shadowRoot = null;

      if (options.shadowDOM) {
        try {
          shadowRoot = container.attachShadow({ mode: 'open' });
          mountRoot = shadowRoot;
        } catch (err) {
          console.warn('[SVGWidget] Failed to attach shadow root, mounting in light DOM:', err);
          mountRoot = container;
        }
      }

      // Create a wrapper div to contain the SVG
      const wrapper = document.createElement('div');
      wrapper.className = 'sva-widget-wrapper';
      wrapper.setAttribute('data-sva-widget-instance', instanceId);
      wrapper.style.display = 'block';
      wrapper.style.width = options.width ? (typeof options.width === 'number' ? `${options.width}px` : options.width) : '100%';
      wrapper.style.height = options.height ? (typeof options.height === 'number' ? `${options.height}px` : options.height) : '100%';
      wrapper.style.position = 'relative';
      wrapper.style.overflow = 'hidden';

      wrapper.innerHTML = details.isolatedSvg;
      mountRoot.appendChild(wrapper);

      const svgEl = wrapper.querySelector('svg');

      // 4. Parameter state mapping
      const paramMap = {};
      const currentValues = {};
      for (const p of manifest.parameters) {
        if (p && p.name) {
          paramMap[p.name] = p;
          if (p.default !== undefined) {
            currentValues[p.name] = p.default;
          }
        }
      }

      // 5. Event system
      const eventListeners = {};

      // 6. Animation state
      let isPaused = false;
      let currentSpeed = 1.0;

      // Ensure global paused CSS helper exists once in document
      if (typeof document.getElementById === 'function' && !document.getElementById('sva-widget-global-style')) {
        const styleEl = document.createElement('style');
        styleEl.id = 'sva-widget-global-style';
        styleEl.textContent = `
          .sva-widget-wrapper.sva-paused *,
          .sva-widget-wrapper.sva-paused {
            animation-play-state: paused !important;
          }
        `;
        document.head.appendChild(styleEl);
      }

      // 7. Construct instance handle
      const handle = {
        instanceId: details.instanceId,
        prefix: details.prefix,
        rootId: details.rootId,
        idMap: details.idMap,
        container: container,
        wrapper: wrapper,
        svgElement: svgEl,
        shadowRoot: shadowRoot,
        isDestroyed: false,

        // Parameter schema
        getManifest() {
          return JSON.parse(JSON.stringify(manifest));
        },

        getParameter(name) {
          if (currentValues.hasOwnProperty(name)) {
            return currentValues[name];
          }
          return undefined;
        },

        getParameters() {
          return Object.assign({}, currentValues);
        },

        setParameter(name, value) {
          const batch = {};
          batch[name] = value;
          return handle.setParameters(batch);
        },

        setParameters(batch) {
          if (handle.isDestroyed) {
            throw new Error(`Cannot setParameters on destroyed widget instance '${handle.instanceId}'.`);
          }
          if (!batch || typeof batch !== 'object') {
            throw new Error('setParameters expects a key-value object of parameter names and values.');
          }

          // Phase 1: Full validation check (atomic)
          const validatedUpdates = [];
          for (const [key, val] of Object.entries(batch)) {
            const paramDef = paramMap[key];
            if (!paramDef) {
              throw new Error(`Unknown widget parameter: '${key}'`);
            }
            const valRes = validateParameterValue(paramDef, val);
            if (!valRes.valid) {
              throw new Error(valRes.error);
            }
            const targetEl = resolveTargetElement(svgEl, details.idMap, paramDef.target);
            if (!targetEl && paramDef.property !== 'speed' && paramDef.property !== 'playbackRate') {
              throw new Error(`Target element '${paramDef.target}' not found in widget for parameter '${key}'.`);
            }
            validatedUpdates.push({ paramDef, targetEl, cleanVal: valRes.value });
          }

          // Phase 2: Atomic DOM and state application
          const changed = {};
          for (const { paramDef, targetEl, cleanVal } of validatedUpdates) {
            currentValues[paramDef.name] = cleanVal;
            changed[paramDef.name] = cleanVal;
            applyPropertyToElement(targetEl, paramDef.property, cleanVal, paramDef, handle);
          }

          handle._emit('change', { instanceId: handle.instanceId, parameters: changed });
          handle._emit('parametersChanged', { instanceId: handle.instanceId, parameters: handle.getParameters() });
          return handle;
        },

        resetParameters() {
          const defaults = {};
          for (const p of manifest.parameters) {
            if (p.default !== undefined) {
              defaults[p.name] = p.default;
            }
          }
          return handle.setParameters(defaults);
        },

        // Animation playback controls
        play() {
          if (handle.isDestroyed) return handle;
          isPaused = false;
          if (wrapper && wrapper.classList && typeof wrapper.classList.remove === 'function') wrapper.classList.remove('sva-paused');
          if (svgEl && typeof svgEl.unpauseAnimations === 'function') {
            try { svgEl.unpauseAnimations(); } catch (e) {}
          }
          if (svgEl && typeof svgEl.getAnimations === 'function') {
            try {
              const anims = svgEl.getAnimations({ subtree: true });
              anims.forEach((a) => { try { a.play(); } catch (err) {} });
            } catch (e) {}
          }
          handle._emit('play', { instanceId: handle.instanceId });
          return handle;
        },

        pause() {
          if (handle.isDestroyed) return handle;
          isPaused = true;
          if (wrapper && wrapper.classList && typeof wrapper.classList.add === 'function') wrapper.classList.add('sva-paused');
          if (svgEl && typeof svgEl.pauseAnimations === 'function') {
            try { svgEl.pauseAnimations(); } catch (e) {}
          }
          if (svgEl && typeof svgEl.getAnimations === 'function') {
            try {
              const anims = svgEl.getAnimations({ subtree: true });
              anims.forEach((a) => { try { a.pause(); } catch (err) {} });
            } catch (e) {}
          }
          handle._emit('pause', { instanceId: handle.instanceId });
          return handle;
        },

        resume() {
          return handle.play();
        },

        stop() {
          handle.pause();
          if (svgEl && typeof svgEl.setCurrentTime === 'function') {
            try { svgEl.setCurrentTime(0); } catch (e) {}
          }
          handle._emit('stop', { instanceId: handle.instanceId });
          return handle;
        },

        isPaused() {
          return isPaused;
        },

        setSpeed(speed) {
          const rate = Number(speed);
          if (isNaN(rate) || rate < 0) {
            throw new Error(`Invalid speed value: ${speed}. Must be a non-negative number.`);
          }
          currentSpeed = rate;
          if (wrapper && wrapper.style && typeof wrapper.style.setProperty === 'function') {
            wrapper.style.setProperty('--sva-speed', String(rate));
          }
          if (svgEl && typeof svgEl.getAnimations === 'function') {
            try {
              const anims = svgEl.getAnimations({ subtree: true });
              anims.forEach((a) => { try { a.playbackRate = rate; } catch (err) {} });
            } catch (e) {}
          }
          handle._emit('speedChange', { instanceId: handle.instanceId, speed: rate });
          return handle;
        },

        getSpeed() {
          return currentSpeed;
        },

        // Events
        on(event, callback) {
          if (typeof callback !== 'function') return handle;
          if (!eventListeners[event]) eventListeners[event] = [];
          eventListeners[event].push(callback);
          return handle;
        },

        off(event, callback) {
          if (!eventListeners[event]) return handle;
          if (!callback) {
            delete eventListeners[event];
          } else {
            eventListeners[event] = eventListeners[event].filter((cb) => cb !== callback);
          }
          return handle;
        },

        _emit(event, data) {
          if (eventListeners[event]) {
            eventListeners[event].forEach((cb) => {
              try { cb(data); } catch (e) { console.error('[SVGWidget listener error]', e); }
            });
          }
        },

        // Unmount & destruction
        unmount() {
          if (wrapper && wrapper.parentNode) {
            wrapper.parentNode.removeChild(wrapper);
          }
          handle.isDestroyed = true;
          return handle;
        },

        destroy() {
          handle.stop();
          handle.unmount();
          handle._emit('destroy', { instanceId: handle.instanceId });
          for (const k in eventListeners) delete eventListeners[k];
          return handle;
        }
      };

      // 8. Apply initial parameters if provided in options or manifest defaults
      const initialParams = Object.assign({}, currentValues, options.parameters || options.initialParameters || {});
      if (Object.keys(initialParams).length > 0) {
        try {
          handle.setParameters(initialParams);
        } catch (err) {
          console.warn('[SVGWidget] Failed applying initial parameters on mount:', err);
        }
      }

      return handle;
    },

    createInstance(target, rawSvg, options = {}) {
      return SVGWidget.mount(target, rawSvg, options);
    }
  };

  return SVGWidget;
});
