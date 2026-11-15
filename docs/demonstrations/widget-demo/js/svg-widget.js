/**
 * SVG Animator Studio — Reusable SVG Widget Runtime & Mounting Library (`svg-widget.js`)
 *
 * Stage 16A: Reusable, Self-Contained and Collision-Free SVG Widgets.
 *
 * Responsibilities:
 *   - Parse and namespace SVG documents for safe multi-instance embedding on host pages.
 *   - Scope all element IDs, internal references (url(#...), href="#...", xlink:href="#...").
 *   - Remap SMIL animation targets (<mpath>, event timing begin/end).
 *   - Scope embedded <style> selectors to widget root element.
 *   - Prefix @keyframes and remap animation/animation-name properties to prevent stylesheet collisions.
 *   - Safely preserve data URLs (data:image/...) and text content containing '#'.
 *   - Lightweight DOM mount / unmount lifecycle helper with optional Shadow DOM isolation.
 *
 * Universal UMD module: Browser window global, CommonJS (Node.js), and ES module environments.
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

  const SVGWidget = {
    version: '1.0.0',

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

      const instanceId = options.instanceId || generateInstanceId();
      const details = isolateSVGString(rawSvg, {
        instanceId,
        prefix: options.prefix,
        scopeCss: options.scopeCss !== false,
        rawDetails: true
      });

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

      const handle = {
        instanceId: details.instanceId,
        prefix: details.prefix,
        rootId: details.rootId,
        idMap: details.idMap,
        container: container,
        wrapper: wrapper,
        svgElement: svgEl,
        shadowRoot: shadowRoot,
        unmount() {
          if (wrapper && wrapper.parentNode) {
            wrapper.parentNode.removeChild(wrapper);
          }
        }
      };

      return handle;
    }
  };

  return SVGWidget;
});
