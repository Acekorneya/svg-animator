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

      // Remap @keyframes animation-name references inside inline style="..." attributes
      if (Object.keys(keyframeMap).length > 0 && /\bstyle\s*=/i.test(tagAttrs)) {
        tagAttrs = tagAttrs.replace(/\bstyle\s*=\s*(["'])([^"']*)\1/gi, (match, q, styleVal) => {
          let updatedStyle = styleVal;
          for (const [oldKF, newKF] of Object.entries(keyframeMap)) {
            const reInlineAnim = new RegExp(`(\\b(?:animation|animation-name)\\s*:\\s*[^;]*?)\\b${escapeRegex(oldKF)}\\b`, 'g');
            updatedStyle = updatedStyle.replace(reInlineAnim, `$1${newKF}`);
          }
          return `style=${q}${updatedStyle}${q}`;
        });
      }

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
   * Handles authored IDs, isolated IDs (prefix__id), semantic targets and stable entity IDs.
   */
  function resolveTargetElement(svgEl, idMap, targetName) {
    if (!svgEl) return null;
    if (!targetName || targetName === 'root' || targetName === 'svg' || targetName === 'global') {
      return svgEl;
    }

    // 1. Exact data-semantic-target match (e.g. data-semantic-target="flow-1")
    let el = svgEl.querySelector(`[data-semantic-target="${targetName}"]`);
    if (el) return el;

    // 2. Exact data-semantic-name match (e.g. data-semantic-name="mail-flow")
    el = svgEl.querySelector(`[data-semantic-name="${targetName}"]`);
    if (el) return el;

    // 3. idMap direct lookup (authored ID -> isolated ID)
    if (idMap && idMap[targetName]) {
      const mappedId = idMap[targetName];
      el = svgEl.querySelector(`#${mappedId}`) || (svgEl.ownerDocument && svgEl.ownerDocument.getElementById(mappedId));
      if (el) return el;
    }

    // 4. idMap suffix lookup (handles multi-stage isolation e.g. target "flow-1" matching idMap key "mail__flow-1")
    if (idMap) {
      for (const [authoredKey, isolatedVal] of Object.entries(idMap)) {
        if (authoredKey === targetName || authoredKey.endsWith('__' + targetName) || authoredKey.endsWith('-' + targetName)) {
          el = svgEl.querySelector(`#${isolatedVal}`) || (svgEl.ownerDocument && svgEl.ownerDocument.getElementById(isolatedVal));
          if (el) return el;
        }
      }
    }

    // 5. Stable entity data attributes (data-node-id, data-flow-id, data-image-id, data-glow-id, data-ripple-id, data-lite-id)
    el = svgEl.querySelector(`[data-node-id="${targetName}"], [data-flow-id="${targetName}"], [data-image-id="${targetName}"], [data-glow-id="${targetName}"], [data-ripple-id="${targetName}"], [data-lite-id="${targetName}"]`);
    if (el) return el;

    // Stripped prefix fallback (e.g. "node-1" -> "1")
    const strippedId = String(targetName).replace(/^(node|flow|glow|ripple|image|lite)-/, '');
    if (strippedId !== targetName) {
      el = svgEl.querySelector(`[data-node-id="${strippedId}"], [data-flow-id="${strippedId}"], [data-image-id="${strippedId}"], [data-glow-id="${strippedId}"], [data-ripple-id="${strippedId}"], [data-lite-id="${strippedId}"]`);
      if (el) return el;
    }

    // 6. Direct or isolated ID query (exact match or endsWith __targetName)
    try {
      el = svgEl.querySelector(`#${targetName}`);
      if (el) return el;
      el = svgEl.querySelector(`[id$="__${targetName}"], [id$="-${targetName}"]`);
      if (el) return el;
    } catch (e) {}

    // 7. CSS selector fallback
    try {
      el = svgEl.querySelector(targetName);
      if (el) return el;
    } catch (e) {}

    return null;
  }

  /**
   * Applies targeted animation speed multiplier to element sub-animations,
   * respecting the formula: Effective Speed = Master Playback Rate × Targeted Flow Speed.
   */
  function applyTargetedSpeed(el, localRate, masterRate) {
    if (!el) return;
    const lRate = Number(localRate);
    const mRate = Number(masterRate);
    const validLocal = (!isNaN(lRate) && lRate > 0) ? lRate : 1.0;
    const validMaster = (!isNaN(mRate) && mRate >= 0) ? mRate : 1.0;
    const effectiveSpeed = Math.max(0.001, validMaster * validLocal);

    // 1. Web Animations API within target element subtree only
    if (typeof el.getAnimations === 'function') {
      const anims = el.getAnimations({ subtree: true });
      if (anims && anims.length > 0) {
        anims.forEach((a) => {
          try { a.playbackRate = effectiveSpeed; } catch (err) {}
        });
      }
    }

    // 2. SMIL animations (<animate>, <animateMotion>, <animateTransform>) in target element subtree
    if (typeof el.querySelectorAll === 'function') {
      el.querySelectorAll('animate, animateMotion, animateTransform').forEach((animEl) => {
        let baseDur = animEl.getAttribute('data-base-dur');
        if (!baseDur) {
          const curDurStr = animEl.getAttribute('dur') || '2s';
          const curDur = parseFloat(curDurStr) || 2;
          baseDur = String(curDur);
          animEl.setAttribute('data-base-dur', baseDur);
        }
        const newDur = (parseFloat(baseDur) / effectiveSpeed).toFixed(3);
        animEl.setAttribute('dur', `${newDur}s`);
      });

      // 3. CSS @keyframes animations on Lite effect payloads
      el.querySelectorAll('.lite-payload').forEach((payEl) => {
        if (!payEl.style) return;
        let baseDur = payEl.getAttribute('data-base-dur') || (payEl.style.getPropertyValue && payEl.style.getPropertyValue('--sva-base-dur'));
        if (!baseDur) {
          baseDur = payEl.style.animationDuration || '3s';
        }
        const parsed = parseFloat(baseDur);
        if (!isNaN(parsed) && parsed > 0) {
          payEl.setAttribute('data-base-dur', String(parsed));
          payEl.style.animationDuration = `${(parsed / effectiveSpeed).toFixed(3)}s`;
        }
      });
    }
  }

  /**
   * Applies validated property to the resolved DOM element.
   */
  function applyPropertyToElement(el, property, value, paramDef, instance) {
    const svgEl = instance ? (instance.svgElement || (instance.wrapper && instance.wrapper.querySelector('svg'))) : null;

    // Detect specialized entity types
    const isFlow = el && (Boolean(el.classList?.contains('diagram-flow-instance')) ||
                   Boolean(typeof el.hasAttribute === 'function' && el.hasAttribute('data-flow-id')) ||
                   Boolean(el.querySelector?.('.flow-track-path')));
    const isGlow = el && (Boolean(el.classList?.contains('diagram-glow-instance')) ||
                   Boolean(typeof el.hasAttribute === 'function' && el.hasAttribute('data-glow-id')) ||
                   Boolean(el.querySelector?.('.glow-hit-target')));
    const isRipple = el && (Boolean(el.classList?.contains('diagram-ripple-instance')) ||
                     Boolean(typeof el.hasAttribute === 'function' && el.hasAttribute('data-ripple-id')) ||
                     Boolean(el.querySelector?.('.ripple-hit-target')));
    const isLite = el && (Boolean(el.classList?.contains('diagram-lite-instance')) ||
                   Boolean(typeof el.hasAttribute === 'function' && el.hasAttribute('data-lite-id')) ||
                   Boolean(el.querySelector?.('.lite-payload')));

    switch (property) {
      case 'stroke':
      case 'color':
      case 'fill': {
        if (!el) break;

        if (isFlow) {
          // Flow: color changes the moving payload only.
          // The invisible guide path remains stroke="none".
          // Do not recolour unrelated path geometry.
          const payloadGroup = el.querySelector('.flow-payload-group') || el.querySelector('.flow-payload-wrapper');
          if (payloadGroup) {
            payloadGroup.querySelectorAll('circle, path, rect, ellipse, line, polygon').forEach(c => {
              const curStroke = c.getAttribute('stroke');
              const curFill = c.getAttribute('fill');
              if (curStroke && curStroke !== 'none') {
                c.setAttribute('stroke', value);
                c.style.stroke = value;
              }
              if (curFill && curFill !== 'none') {
                c.setAttribute('fill', value);
                c.style.fill = value;
              }
              if (!curStroke && !curFill) {
                c.setAttribute('fill', value);
                c.style.fill = value;
              }
            });
          }
          // Check motion guide track paths: if stroke="none", NEVER paint it!
          el.querySelectorAll('path.flow-track-path, .flow-track-path').forEach(p => {
            const trackStroke = p.getAttribute('stroke');
            if (trackStroke === 'none' || p.style.stroke === 'none') {
              p.setAttribute('stroke', 'none');
              p.style.stroke = 'none';
            } else if (property === 'stroke') {
              // Only recolour if it was authored as a visible track line and property is explicitly stroke
              p.setAttribute('stroke', value);
              p.style.stroke = value;
            }
          });
        } else if (isGlow) {
          // Glow: color changes the illuminated effect only.
          // NEVER recolour transparent hit-test elements (.glow-hit-target) or elements with fill="transparent".
          const elId = el.getAttribute('id') || (el.id && el.id.baseVal) || String(el.id || '');
          const glowId = el.getAttribute('data-glow-id') || elId.replace(/^.*__glow-/, '').replace(/^.*glow-/, '');
          if (svgEl) {
            const radGrads = [];
            if (glowId) {
              svgEl.querySelectorAll(`radialGradient[id*="glowGrad-${glowId}"], radialGradient[id*="glow-grad-${glowId}"], radialGradient[id*="glowGrad"], radialGradient[id*="glow"]`).forEach(rg => {
                if (!radGrads.includes(rg)) radGrads.push(rg);
              });
            }
            // Inspect child elements of the glow for referenced gradient URLs
            el.querySelectorAll('circle, path, rect, ellipse').forEach(c => {
              const fill = c.getAttribute('fill') || (c.style && c.style.fill) || '';
              const urlMatch = fill.match(/url\(['"]?#([^'")]+)['"]?\)/);
              if (urlMatch) {
                const gradEl = svgEl.querySelector(`#${urlMatch[1]}`);
                if (gradEl && !radGrads.includes(gradEl)) radGrads.push(gradEl);
              }
            });
            radGrads.forEach(rg => {
              rg.querySelectorAll('stop').forEach(st => {
                st.setAttribute('stop-color', value);
                st.style.stopColor = value;
              });
            });
            const filters = svgEl.querySelectorAll(`filter[id*="glowBlur-${glowId}"] feFlood, filter[id*="glowBlur-${glowId}"] feDropShadow, filter[id*="glow"] feFlood`);
            filters.forEach(f => {
              if (f && typeof f.hasAttribute === 'function' && f.hasAttribute('flood-color')) f.setAttribute('flood-color', value);
            });
          }
          el.querySelectorAll('circle, path, rect, ellipse').forEach(c => {
            if (c.classList?.contains('glow-hit-target') || c.getAttribute('fill') === 'transparent' || c.getAttribute('pointer-events') === 'all') {
              return; // NEVER touch transparent hit-test elements
            }
            const curStroke = c.getAttribute('stroke');
            const curFill = c.getAttribute('fill');
            if (curStroke && curStroke !== 'none' && curStroke !== 'transparent') {
              c.setAttribute('stroke', value);
              c.style.stroke = value;
            }
            if (curFill && curFill !== 'none' && curFill !== 'transparent' && !curFill.startsWith('url(')) {
              c.setAttribute('fill', value);
              c.style.fill = value;
            }
          });
        } else if (isRipple) {
          // Ripple: Colour must target the actual visible ripple output only.
          // Hit-test elements (.ripple-hit-target), shadow rings (black), and image/mask helpers remain untouched.
          el.querySelectorAll('circle, path').forEach(c => {
            if (c.classList?.contains('ripple-hit-target') || c.getAttribute('fill') === 'transparent' || c.getAttribute('pointer-events') === 'all') {
              return; // NEVER touch hit-test element
            }
            const curStroke = c.getAttribute('stroke');
            if (curStroke === 'black' || curStroke === '#000' || curStroke === '#000000' || c.closest?.('g[transform*="scale(0.92)"]')) {
              return; // Keep shadow rings original black semantics
            }
            if (c.closest?.('mask') || c.tagName?.toLowerCase() === 'image') {
              return; // Do not touch mask or raster image elements
            }
            c.setAttribute('stroke', value);
            c.style.stroke = value;
          });
        } else if (isLite) {
          // Lite effects: recolour visible .lite-payload & .lite-guide-track elements, preserving .lite-hit-target
          el.querySelectorAll('.lite-payload').forEach(c => {
            const curStroke = c.getAttribute('stroke');
            const curFill = c.getAttribute('fill');
            if (curStroke && curStroke !== 'none') {
              c.setAttribute('stroke', value);
              if (c.style) c.style.stroke = value;
            }
            if (curFill && curFill !== 'none') {
              c.setAttribute('fill', value);
              if (c.style) c.style.fill = value;
            }
          });
          el.querySelectorAll('.lite-guide-track').forEach(g => {
            g.setAttribute('stroke', value);
            if (g.style) g.style.stroke = value;
          });
        } else {
          // Generic Node or Element
          if (property === 'fill') {
            el.setAttribute('fill', value);
            el.style.fill = value;
            if (el.tagName && el.tagName.toLowerCase() === 'g') {
              el.querySelectorAll('rect, circle, path, polygon, ellipse').forEach(c => {
                if (!c.classList?.contains('glow-hit-target') && !c.classList?.contains('ripple-hit-target') && c.getAttribute('fill') !== 'transparent' && c.getAttribute('fill') !== 'none') {
                  c.setAttribute('fill', value);
                  c.style.fill = value;
                }
              });
            }
          } else {
            el.setAttribute('stroke', value);
            el.style.stroke = value;
            if (el.tagName && el.tagName.toLowerCase() === 'g') {
              el.querySelectorAll('rect, circle, path, polygon, ellipse, line').forEach(c => {
                if (!c.classList?.contains('glow-hit-target') && !c.classList?.contains('ripple-hit-target') && !c.classList?.contains('flow-track-path')) {
                  const curStroke = c.getAttribute('stroke');
                  if (curStroke !== 'none') {
                    c.setAttribute('stroke', value);
                    c.style.stroke = value;
                  }
                }
              });
            }
          }
        }
        break;
      }
      case 'visibility': {
        const isVisible = value !== false && value !== 'hidden' && value !== 0 && value !== '0';
        const visStr = isVisible ? 'visible' : 'hidden';
        if (el) {
          el.style.visibility = visStr;
          el.setAttribute('visibility', visStr);
        }
        break;
      }
      case 'opacity': {
        if (!el) break;
        const num = Math.max(0, Math.min(1, parseFloat(value) || 0));
        if (isRipple) {
          // Ripple opacity: target only visible wave crests, never hit-target or background
          el.querySelectorAll('circle, path').forEach(c => {
            if (c.classList?.contains('ripple-hit-target') || c.getAttribute('fill') === 'transparent') return;
            if (c.closest?.('mask') || c.tagName?.toLowerCase() === 'image') return;
            c.style.opacity = String(num);
          });
        } else if (isGlow) {
          // Glow intensity: apply opacity to graphic container
          const gfx = el.querySelector('g[pointer-events="none"]') || el;
          gfx.style.opacity = String(num);
          gfx.setAttribute('opacity', String(num));
        } else if (isLite) {
          el.setAttribute('opacity', String(num));
          if (el.style) el.style.opacity = String(num);
          el.querySelectorAll('.lite-payload').forEach(c => {
            c.removeAttribute('opacity');
            c.removeAttribute('stroke-opacity');
            if (c.style) {
              c.style.opacity = '';
            }
          });
        } else {
          el.setAttribute('opacity', String(num));
          el.style.opacity = String(num);
          if (el.tagName && el.tagName.toLowerCase() === 'g') {
            el.querySelectorAll('rect, circle, path, polygon, ellipse, image').forEach(c => {
              if (!c.classList?.contains('glow-hit-target') && !c.classList?.contains('ripple-hit-target')) {
                c.style.opacity = String(num);
              }
            });
          }
        }
        break;
      }
      case 'text':
      case 'textContent':
        if (el) {
          const valStr = String(value);
          if (el.tagName && el.tagName.toLowerCase() === 'text') {
            el.textContent = valStr;
          } else {
            const textEls = el.querySelectorAll('text');
            if (textEls.length > 0) {
              textEls[0].textContent = valStr;
            } else {
              el.textContent = valStr;
            }
          }
        }
        break;
      case 'playbackRate':
      case 'speed':
        if (el && paramDef && paramDef.target && paramDef.target !== 'root' && paramDef.target !== 'global') {
          // Object-specific targeted animation speed adjustment
          const localRate = Number(value);
          if (!isNaN(localRate) && localRate > 0) {
            const masterRate = (instance && typeof instance.getSpeed === 'function') ? instance.getSpeed() : 1.0;
            applyTargetedSpeed(el, localRate, masterRate);
          }
        } else if (instance && typeof instance.setSpeed === 'function') {
          // Global widget instance playback rate
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
      case 'trigger': {
        if (el) {
          el.querySelectorAll('.lite-flash-anim, .sva-lite-flash-anim').forEach(node => {
            node.setAttribute('data-trigger-nonce', String(value));
            if (node.style) {
              const prevAnim = node.style.animationName || '';
              node.style.animationName = 'none';
              try { void node.getBoundingClientRect(); } catch (e) {}
              node.style.animationName = prevAnim;
            }
          });
        }
        break;
      }
      case 'scale': {
        if (el) {
          const sVal = Math.max(0.2, Math.min(4.0, parseFloat(value) || 1.15));
          el.querySelectorAll('.lite-payload').forEach(node => {
            node.setAttribute('data-pulse-scale', String(sVal));
            if (node.style && typeof node.style.setProperty === 'function') {
              node.style.setProperty('--sva-pulse-max', String(sVal));
            }
          });
        }
        break;
      }
      case 'duration': {
        if (el) {
          const msVal = Math.max(10, Math.min(5000, parseFloat(value) || 100));
          const secVal = (msVal / 1000).toFixed(3);
          el.querySelectorAll('.lite-payload').forEach(node => {
            node.setAttribute('data-flash-duration-ms', String(Math.round(msVal)));
            if (node.style) {
              node.style.animationDuration = `${secVal}s`;
              if (typeof node.style.setProperty === 'function') {
                node.style.setProperty('--sva-base-dur', `${secVal}s`);
              }
            }
          });
        }
        break;
      }
      case 'strokeWidth': {
        if (el) {
          const sw = Math.max(0.5, Math.min(20, parseFloat(value) || 2.5));
          el.querySelectorAll('.lite-payload').forEach(node => {
            node.setAttribute('stroke-width', String(sw));
            if (node.style && typeof node.style.setProperty === 'function') {
              node.style.setProperty('--sva-base-sw', `${sw}px`);
            }
          });
        }
        break;
      }
      case 'segments':
      case 'lineGapRatio': {
        if (el) {
          if (property === 'segments') {
            el.setAttribute('data-lite-segments', String(Math.max(1, Math.min(16, parseInt(value, 10) || 2))));
          } else {
            el.setAttribute('data-lite-ratio', String(value || '1:3'));
          }
          const segs = Math.max(1, Math.min(16, parseInt(el.getAttribute('data-lite-segments') || '2', 10) || 2));
          const ratioStr = el.getAttribute('data-lite-ratio') || '1:3';
          const parts = String(ratioStr).split(':');
          const lPart = Math.max(0.1, parseFloat(parts[0]) || 1);
          const gPart = Math.max(0.1, parseFloat(parts[1]) || 3);
          const unit = Math.round((1000 / segs) * 1000) / 1000;
          const dashLen = Math.round((unit * (lPart / (lPart + gPart))) * 1000) / 1000;
          const gapLen = Math.round((unit - dashLen) * 1000) / 1000;
          el.querySelectorAll('.lite-payload').forEach(node => {
            node.setAttribute('stroke-dasharray', `${dashLen} ${gapLen}`);
            if (node.style && typeof node.style.setProperty === 'function') {
              node.style.setProperty('--sva-unit', `${unit}px`);
              node.style.setProperty('--sva-dash', `${dashLen}px`);
              node.style.setProperty('--sva-gap', `${gapLen}px`);
            }
          });
        }
        break;
      }
      case 'direction': {
        if (el) {
          const dir = String(value).toLowerCase() === 'ccw' ? 'ccw' : 'cw';
          el.querySelectorAll('.lite-payload').forEach(node => {
            const curAnim = (node.style && node.style.animationName) || '';
            const pfxMatch = curAnim.match(/^(.*__)sva-lite-trace-/);
            const pfx = pfxMatch ? pfxMatch[1] : '';
            const entryExit = node.getAttribute('data-entry-exit') || 'none';
            const suffix = entryExit === 'fade' ? '-fade' : (entryExit === 'grow' ? '-grow' : (entryExit === 'fade-grow' ? '-fadegrow' : ''));
            const newTraceName = `${pfx}sva-lite-trace-${dir}${suffix}`;
            if (node.classList) {
              Array.from(node.classList).forEach(cls => {
                if (cls.includes('sva-lite-trace-')) node.classList.remove(cls);
              });
              node.classList.add(newTraceName);
            }
            if (node.style) {
              node.style.animationName = newTraceName;
            }
            node.setAttribute('data-direction', dir);
          });
        }
        break;
      }
      case 'startOffset': {
        if (el) {
          const pct = Math.max(0, Math.min(100, parseFloat(value) || 0));
          const offsetUnits = -(pct / 100) * 1000;
          el.querySelectorAll('.lite-payload').forEach(node => {
            node.setAttribute('data-start-offset', String(pct));
            if (node.style && typeof node.style.setProperty === 'function') {
              node.style.setProperty('--sva-start-offset', `${offsetUnits}px`);
            }
          });
        }
        break;
      }
      case 'playbackMode': {
        if (el) {
          const raw = String(value).toLowerCase();
          const mode = raw === 'pingpong' ? 'pingpong' : (raw === 'once' ? 'once' : 'loop');
          el.querySelectorAll('.lite-payload').forEach(node => {
            const curAnim = (node.style && node.style.animationName) || '';
            const pfxMatch = curAnim.match(/^(.*__)sva-lite-trace-/);
            const pfx = pfxMatch ? pfxMatch[1] : '';
            if (node.classList) {
              Array.from(node.classList).forEach(cls => {
                if (cls.includes('sva-lite-mode-')) node.classList.remove(cls);
              });
              node.classList.add(`${pfx}sva-lite-mode-${mode}`);
            }
            if (node.style) {
              node.style.animationIterationCount = mode === 'once' ? '1' : 'infinite';
              node.style.animationDirection = mode === 'pingpong' ? 'alternate' : 'normal';
              node.style.animationFillMode = mode === 'once' ? 'forwards' : 'none';
            }
            node.setAttribute('data-playback-mode', mode);
          });
        }
        break;
      }
      case 'showGuide': {
        if (el) {
          const show = value !== false && value !== 'false' && value !== 0 && value !== '0';
          el.querySelectorAll('.lite-guide-track').forEach(g => {
            g.style.visibility = show ? 'visible' : 'hidden';
            g.setAttribute('visibility', show ? 'visible' : 'hidden');
          });
        }
        break;
      }
      case 'period': {
        if (el) {
          const sec = Math.max(0.2, Math.min(20, parseFloat(value) || 1.8));
          el.querySelectorAll('.lite-payload').forEach(node => {
            node.setAttribute('data-pulse-period', String(sec));
            if (node.style) {
              node.style.animationDuration = `${sec}s`;
              if (typeof node.style.setProperty === 'function') {
                node.style.setProperty('--sva-base-dur', `${sec}s`);
              }
            }
          });
        }
        break;
      }
      case 'intensity': {
        if (el) {
          const amp = Math.max(0, Math.min(1, parseFloat(value) || 0.85));
          el.querySelectorAll('.lite-payload').forEach(node => {
            node.setAttribute('data-intensity', String(amp));
            if (node.style && typeof node.style.setProperty === 'function') {
              node.style.setProperty('--sva-pulse-amp', String(amp));
              node.style.setProperty('--sva-flash-int', String(amp));
            }
          });
        }
        break;
      }
      case 'repeat':
      case 'repeatInterval': {
        if (el) {
          el.querySelectorAll('.lite-payload').forEach(node => {
            if (property === 'repeat') {
              const rep = value === true || value === 'true' || value === 1 || value === '1';
              node.setAttribute('data-repeat', rep ? 'true' : 'false');
            } else {
              const iv = Math.max(0.2, Math.min(60, parseFloat(value) || 2.0));
              node.setAttribute('data-repeat-interval', String(iv));
            }
            const rep = node.getAttribute('data-repeat') === 'true';
            const iv = Math.max(0.2, parseFloat(node.getAttribute('data-repeat-interval') || '2.0') || 2.0);
            const durMs = parseFloat(node.getAttribute('data-flash-duration-ms') || '100') || 100;
            const durSec = durMs / 1000;
            if (rep) {
              const pauseSec = Math.max(0.05, iv - durSec);
              node.setAttribute('data-burst-cycles', '1');
              node.setAttribute('data-pause-seconds', pauseSec.toFixed(3));
              node.setAttribute('data-pause-visibility', 'hide');
              if (node.style) {
                node.style.animationIterationCount = '1';
                if (property === 'repeat') {
                  const prevAnim = node.style.animationName || '';
                  node.style.animationName = 'none';
                  try { void node.getBoundingClientRect(); } catch (e) {}
                  node.style.visibility = '';
                  node.style.animationName = prevAnim;
                }
              }
            } else {
              node.removeAttribute('data-burst-cycles');
              node.removeAttribute('data-pause-seconds');
              node.removeAttribute('data-burst-paused');
              if (node.style) {
                node.style.animationIterationCount = '1';
                node.style.visibility = '';
              }
            }
          });
        }
        break;
      }
      case 'burstCycles':
      case 'pauseSeconds':
      case 'pauseVisibility': {
        if (el) {
          el.querySelectorAll('.lite-payload').forEach(node => {
            if (property === 'burstCycles') {
              const cyc = Math.max(0, Math.min(100, parseInt(value, 10) || 0));
              node.setAttribute('data-burst-cycles', String(cyc));
            } else if (property === 'pauseSeconds') {
              const pSec = Math.max(0, Math.min(300, parseFloat(value) || 0));
              node.setAttribute('data-pause-seconds', String(pSec));
            } else {
              const vis = String(value).toLowerCase() === 'hold' ? 'hold' : 'hide';
              node.setAttribute('data-pause-visibility', vis);
            }
            const cyc = parseInt(node.getAttribute('data-burst-cycles') || '0', 10) || 0;
            const pSec = parseFloat(node.getAttribute('data-pause-seconds') || '0') || 0;
            const mode = node.getAttribute('data-playback-mode') || 'loop';
            const hasBurst = mode !== 'once' && cyc > 0 && pSec > 0;
            if (node.style) {
              node.style.animationIterationCount = mode === 'once' ? '1' : (hasBurst ? String(cyc) : 'infinite');
              node.style.animationFillMode = (mode === 'once' || hasBurst) ? 'forwards' : 'none';
              if (!hasBurst) {
                node.removeAttribute('data-burst-paused');
                node.style.visibility = '';
              }
            }
          });
        }
        break;
      }
      case 'armThickness': {
        if (el) {
          const thick = Math.max(1, Math.min(20, parseFloat(value) || 3));
          el.querySelectorAll('.lite-payload').forEach(node => {
            node.setAttribute('data-arm-thickness', String(thick));
            if (typeof node.getBBox === 'function') {
              try {
                const bb = node.getBBox();
                const cx = bb.x + bb.width / 2;
                const cy = bb.y + bb.height / 2;
                const rx = bb.width / 2;
                const ry = bb.height / 2;
                if (rx > 0 && ry > 0) {
                  if (node.tagName && node.tagName.toLowerCase() === 'path') {
                    const ha = Math.max(0.5, Math.min(rx * 0.9, ry * 0.9, thick / 2));
                    const d = `M ${(cx - ha).toFixed(2)},${(cy - ry).toFixed(2)} L ${(cx + ha).toFixed(2)},${(cy - ry).toFixed(2)} L ${(cx + ha).toFixed(2)},${(cy - ha).toFixed(2)} L ${(cx + rx).toFixed(2)},${(cy - ha).toFixed(2)} L ${(cx + rx).toFixed(2)},${(cy + ha).toFixed(2)} L ${(cx + ha).toFixed(2)},${(cy + ha).toFixed(2)} L ${(cx + ha).toFixed(2)},${(cy + ry).toFixed(2)} L ${(cx - ha).toFixed(2)},${(cy + ry).toFixed(2)} L ${(cx - ha).toFixed(2)},${(cy + ha).toFixed(2)} L ${(cx - rx).toFixed(2)},${(cy + ha).toFixed(2)} L ${(cx - rx).toFixed(2)},${(cy - ha).toFixed(2)} L ${(cx - ha).toFixed(2)},${(cy - ha).toFixed(2)} Z`;
                    node.setAttribute('d', d);
                  } else if (node.tagName && node.tagName.toLowerCase() === 'polygon') {
                    const pts = [];
                    const waist = Math.max(0.5, Math.min(rx * 0.45, ry * 0.45, thick * 0.65));
                    for (let i = 0; i < 8; i++) {
                      const ang = (Math.PI / 4) * i - Math.PI / 2;
                      const radX = i % 2 === 0 ? rx : waist;
                      const radY = i % 2 === 0 ? ry : waist;
                      pts.push(`${(cx + Math.cos(ang) * radX).toFixed(2)},${(cy + Math.sin(ang) * radY).toFixed(2)}`);
                    }
                    node.setAttribute('points', pts.join(' '));
                  }
                }
              } catch (e) {}
            }
          });
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

      // 6. Animation & Visibility-Aware Suspension State
      let isPaused = false;
      let isVisible = true;
      let isReducedMotion = false;
      let isSuspended = false;
      let currentSpeed = 1.0;
      let visibilityObserver = null;
      let reducedMotionQuery = null;
      let reducedMotionHandler = null;

      const visibilityAware = options.visibilityAware !== false;
      const reducedMotionMode = options.reducedMotion || 'auto'; // 'auto' | 'reduce' | 'ignore'

      // Ensure global paused/suspended CSS helper exists once in document
      if (typeof document !== 'undefined' && typeof document.getElementById === 'function' && !document.getElementById('sva-widget-global-style')) {
        const styleEl = document.createElement('style');
        styleEl.id = 'sva-widget-global-style';
        styleEl.textContent = `
          .sva-widget-wrapper.sva-paused *,
          .sva-widget-wrapper.sva-paused,
          .sva-widget-wrapper.sva-suspended *,
          .sva-widget-wrapper.sva-suspended {
            animation-play-state: paused !important;
          }
        `;
        const targetHost = document.head || document.body;
        if (targetHost && typeof targetHost.appendChild === 'function') {
          targetHost.appendChild(styleEl);
        }
      }

      function setSMILSuspended(svgRoot, suspended) {
        if (!svgRoot || typeof svgRoot.querySelectorAll !== 'function') return;
        const smilEls = svgRoot.querySelectorAll('animate, animateMotion, animateTransform');
        if (!smilEls || smilEls.length === 0) return;
        smilEls.forEach((animEl) => {
          if (suspended) {
            if (animEl.dataset && animEl.dataset.svaOrigBegin === undefined) {
              animEl.dataset.svaOrigBegin = animEl.getAttribute('begin') || '0s';
            }
            animEl.setAttribute('begin', 'indefinite');
            if (typeof animEl.endElement === 'function') {
              try { animEl.endElement(); } catch (e) {}
            }
          } else {
            const origBegin = (animEl.dataset && animEl.dataset.svaOrigBegin !== undefined)
              ? animEl.dataset.svaOrigBegin
              : '0s';
            animEl.setAttribute('begin', origBegin);
            if (animEl.dataset) delete animEl.dataset.svaOrigBegin;
            if (typeof animEl.beginElement === 'function') {
              try { animEl.beginElement(); } catch (e) {}
            }
          }
        });
      }

      const burstTimers = new Set();
      function clearBurstTimers() {
        burstTimers.forEach((tid) => clearTimeout(tid));
        burstTimers.clear();
      }

      function restartBurstNode(node) {
        if (!node || !node.style || handle.isDestroyed) return;
        node.removeAttribute('data-burst-paused');
        const prevAnim = node.style.animationName || '';
        node.style.animationName = 'none';
        try { void node.getBoundingClientRect(); } catch (e) {}
        node.style.visibility = '';
        node.style.animationName = prevAnim;
      }

      if (svgEl && typeof svgEl.addEventListener === 'function') {
        svgEl.setAttribute('data-sva-runtime-managed', '1');
        svgEl.addEventListener('animationend', (e) => {
          const node = e && e.target;
          if (!node || !node.getAttribute) return;
          const cyc = parseInt(node.getAttribute('data-burst-cycles') || '0', 10);
          const pSec = parseFloat(node.getAttribute('data-pause-seconds') || '0');
          if (!(cyc > 0 && pSec > 0)) return;
          const vis = node.getAttribute('data-pause-visibility') || 'hide';
          if (vis === 'hide' && node.style) {
            node.style.visibility = 'hidden';
          }
          node.setAttribute('data-burst-paused', '1');
          if (isSuspended) return;
          const tid = setTimeout(() => {
            burstTimers.delete(tid);
            if (isSuspended || handle.isDestroyed) return;
            restartBurstNode(node);
          }, Math.max(10, pSec * 1000));
          burstTimers.add(tid);
        });
      }

      function applySuspensionState() {
        const shouldSuspend = isPaused || isReducedMotion || (visibilityAware && !isVisible);
        if (shouldSuspend === isSuspended) return;
        isSuspended = shouldSuspend;
        if (wrapper && wrapper.classList) {
          if (isSuspended) wrapper.classList.add('sva-suspended');
          else wrapper.classList.remove('sva-suspended');
        }
        setSMILSuspended(svgEl, isSuspended);
        if (isSuspended) {
          clearBurstTimers();
        } else if (svgEl && typeof svgEl.querySelectorAll === 'function') {
          svgEl.querySelectorAll('[data-burst-paused="1"]').forEach((node) => {
            restartBurstNode(node);
          });
        }
        if (typeof handle !== 'undefined' && handle._emit) {
          handle._emit('suspensionChange', {
            instanceId: handle.instanceId,
            suspended: isSuspended,
            visible: isVisible,
            reducedMotion: isReducedMotion
          });
        }
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
            const isGlobalTarget = !paramDef.target || paramDef.target === 'root' || paramDef.target === 'global';
            if (!targetEl && !isGlobalTarget) {
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
          applySuspensionState();
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
          applySuspensionState();
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

        isSuspended() {
          return isSuspended;
        },

        isVisibleInViewport() {
          return isVisible;
        },

        suspendAnimations() {
          isPaused = true;
          applySuspensionState();
          return handle;
        },

        resumeAnimations() {
          isPaused = false;
          applySuspensionState();
          return handle;
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
          // Re-evaluate targeted speeds so: Effective Speed = Master Playback Rate × Targeted Flow Speed
          if (manifest && Array.isArray(manifest.parameters)) {
            for (const p of manifest.parameters) {
              if ((p.property === 'speed' || p.property === 'playbackRate') && p.target && p.target !== 'root' && p.target !== 'global') {
                const targetSpeed = Number(currentValues[p.name]);
                if (!isNaN(targetSpeed) && targetSpeed > 0) {
                  const targetEl = resolveTargetElement(svgEl, details.idMap, p.target);
                  if (targetEl) {
                    applyTargetedSpeed(targetEl, targetSpeed, rate);
                  }
                }
              }
            }
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
          clearBurstTimers();
          if (visibilityObserver) {
            try { visibilityObserver.disconnect(); } catch (e) {}
            visibilityObserver = null;
          }
          if (reducedMotionQuery && reducedMotionHandler) {
            try {
              if (typeof reducedMotionQuery.removeEventListener === 'function') {
                reducedMotionQuery.removeEventListener('change', reducedMotionHandler);
              } else if (typeof reducedMotionQuery.removeListener === 'function') {
                reducedMotionQuery.removeListener(reducedMotionHandler);
              }
            } catch (e) {}
            reducedMotionQuery = null;
            reducedMotionHandler = null;
          }
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

      // 9. Initialize Reduced-Motion preference & IntersectionObserver visibility-aware suspension
      if (reducedMotionMode === 'reduce') {
        isReducedMotion = true;
      } else if (reducedMotionMode !== 'ignore' && typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
        try {
          reducedMotionQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
          isReducedMotion = Boolean(reducedMotionQuery && reducedMotionQuery.matches);
          reducedMotionHandler = (ev) => {
            isReducedMotion = Boolean(ev && ev.matches);
            applySuspensionState();
          };
          if (typeof reducedMotionQuery.addEventListener === 'function') {
            reducedMotionQuery.addEventListener('change', reducedMotionHandler);
          } else if (typeof reducedMotionQuery.addListener === 'function') {
            reducedMotionQuery.addListener(reducedMotionHandler);
          }
        } catch (e) {}
      }

      if (visibilityAware && typeof IntersectionObserver === 'function' && wrapper) {
        try {
          visibilityObserver = new IntersectionObserver((entries) => {
            for (const entry of entries) {
              if (entry.target === wrapper) {
                isVisible = Boolean(entry.isIntersecting || entry.intersectionRatio > 0);
                applySuspensionState();
              }
            }
          }, {
            root: options.visibilityRoot || null,
            rootMargin: options.visibilityMargin || '50px',
            threshold: 0.01
          });
          visibilityObserver.observe(wrapper);
        } catch (e) {}
      }

      applySuspensionState();

      return handle;
    },

    createInstance(target, rawSvg, options = {}) {
      return SVGWidget.mount(target, rawSvg, options);
    }
  };

  return SVGWidget;
});
