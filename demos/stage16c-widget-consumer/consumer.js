/**
 * Stage 16C: Independent Reference Widget Consumer (`consumer.js`)
 *
 * Demonstrates an external application consuming complete Stage 16B exported
 * SVG widget packages using ONLY the public runtime (`svg-widget.js`) and
 * `widget-manifest.json` contract. Zero dependencies on SVG Animator Studio.
 */
(function (root, factory) {
  if (typeof module === 'object' && module.exports) {
    module.exports = factory();
  } else {
    root.Stage16CConsumer = factory();
  }
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  const DEFAULT_HOSTS = [
    { id: 'host-a', hostname: 'pve01', online: true, cpu: 25 },
    { id: 'host-b', hostname: 'pve02', online: false, cpu: 0 },
    { id: 'host-c', hostname: 'pve03', online: true, cpu: 68 }
  ];

  // Built-in mapping profiles matching the two Studio-authored packages
  const MAPPING_PROFILES = {
    'host-monitor-v1': {
      label: 'v1 Contract (hostname + heartbeat-ripple-visibility)',
      textParam: 'hostname-node-text',
      fillParam: 'hostname-node-fill',
      strokeParam: 'hostname-node-stroke',
      flowColorParam: 'traffic-flow-color',
      flowSpeedParam: 'traffic-flow-speed',
      flowVisibilityParam: 'traffic-flow-visibility',
      glowColorParam: 'status-glow-color',
      glowOpacityParam: 'status-glow-opacity',
      rippleOpacityParam: 'heartbeat-ripple-opacity',
      rippleVisibilityParam: 'heartbeat-ripple-visibility'
    },
    'host-monitor-v2': {
      label: 'v2 Contract (server node, no ripple visibility)',
      textParam: 'server-node-text',
      fillParam: 'server-node-fill',
      strokeParam: 'server-node-stroke',
      flowColorParam: 'traffic-flow-color',
      flowSpeedParam: 'traffic-flow-speed',
      flowVisibilityParam: 'traffic-flow-visibility',
      glowColorParam: 'status-glow-color',
      glowOpacityParam: 'status-glow-opacity',
      rippleOpacityParam: 'heartbeat-ripple-opacity'
    }
  };

  function extractEmbeddedManifest(svgText) {
    if (!svgText || typeof svgText !== 'string') return null;
    const m = /<script\b[^>]*\bid=["'](?:[^"']*__)?sva-widget-manifest["'][^>]*>([\s\S]*?)<\/script>/i.exec(svgText);
    if (!m || !m[1]) return null;
    try {
      return JSON.parse(m[1].trim());
    } catch (e) {
      return null;
    }
  }

  /**
   * Validates that all files in a loaded widget package form a single,
   * consistent, non-mixed release.
   */
  function validateWidgetPackageConsistency(pkg) {
    if (!pkg || typeof pkg !== 'object') {
      throw new Error('Package validation failed: package object is missing.');
    }
    if (!pkg.manifest || typeof pkg.manifest !== 'object') {
      throw new Error('Package validation failed: widget-manifest.json is missing or invalid.');
    }
    if (pkg.manifest.schemaVersion !== 1 || !Array.isArray(pkg.manifest.parameters)) {
      throw new Error('Package validation failed: manifest must define schemaVersion=1 and a parameters array.');
    }
    if (!pkg.svgText || typeof pkg.svgText !== 'string' || !pkg.svgText.includes('<svg')) {
      throw new Error('Package validation failed: SVG artwork file is missing or malformed.');
    }
    if (!pkg.widgetRuntime || typeof pkg.widgetRuntime.mount !== 'function') {
      throw new Error('Package validation failed: public SVGWidget runtime (js/svg-widget.js) is missing.');
    }

    const embedded = extractEmbeddedManifest(pkg.svgText);
    if (!embedded) {
      throw new Error('Package consistency error: SVG artwork does not contain an embedded sva-widget-manifest.');
    }

    if (pkg.manifest.packageId && embedded.packageId && pkg.manifest.packageId !== embedded.packageId) {
      throw new Error(
        `Package consistency error: manifest packageId "${pkg.manifest.packageId}" does not match SVG embedded packageId "${embedded.packageId}". Mixed package files are rejected.`
      );
    }

    if (pkg.manifest.widgetName && embedded.widgetName && pkg.manifest.widgetName !== embedded.widgetName) {
      throw new Error(
        `Package consistency error: manifest widgetName "${pkg.manifest.widgetName}" does not match SVG embedded widgetName "${embedded.widgetName}".`
      );
    }

    const manifestNames = pkg.manifest.parameters.map((p) => p.name).sort().join(',');
    const embeddedNames = (embedded.parameters || []).map((p) => p.name).sort().join(',');
    if (manifestNames !== embeddedNames) {
      throw new Error(
        'Package consistency error: widget-manifest.json parameter list differs from SVG embedded manifest.'
      );
    }

    return {
      valid: true,
      packageId: pkg.manifest.packageId || embedded.packageId || 'unversioned',
      packageVersion: pkg.manifest.packageVersion || '1.0.0',
      widgetName: pkg.manifest.widgetName || pkg.baseName || 'widget',
      parameterCount: pkg.manifest.parameters.length
    };
  }

  /**
   * Validates that every parameter required by the consumer mapping profile
   * actually exists in the loaded package's manifest.
   */
  function validateMappingsAgainstManifest(mappingSpec, manifest) {
    if (!mappingSpec || typeof mappingSpec !== 'object') {
      throw new Error('Mapping validation failed: mapping configuration is missing.');
    }
    if (!manifest || !Array.isArray(manifest.parameters)) {
      throw new Error('Mapping validation failed: manifest parameters unavailable.');
    }

    const available = new Set(manifest.parameters.map((p) => p.name));
    const missing = [];
    for (const [role, paramName] of Object.entries(mappingSpec)) {
      if (role === 'label' || !paramName) continue;
      if (!available.has(paramName)) {
        missing.push(`${role} -> "${paramName}"`);
      }
    }

    if (missing.length > 0) {
      throw new Error(
        `Invalid parameter mapping for package "${manifest.widgetName || 'widget'}" (${manifest.packageId || 'v1'}): ` +
        `manifest does not expose ${missing.join(', ')}.`
      );
    }
    return true;
  }

  /**
   * Pure consumer-side mapping from a host record to widget parameters.
   * Explicitly distinguishes valid numeric 0 from missing/unavailable values.
   */
  function mapHostRecordToWidgetParameters(host, mappingSpec, manifest, options = {}) {
    const missingStrategy = options.missingStrategy || 'warn';
    const available = new Set((manifest && manifest.parameters ? manifest.parameters : []).map((p) => p.name));
    const out = {};

    const setIfMapped = (roleKey, value) => {
      const paramName = mappingSpec[roleKey];
      if (paramName && available.has(paramName)) {
        out[paramName] = value;
      }
    };

    const isOnline = host.online === true;
    const hasCpu = host.cpu !== null && host.cpu !== undefined && host.cpu !== '' && !Number.isNaN(Number(host.cpu));
    const cpuNum = hasCpu ? Math.max(0, Math.min(100, Number(host.cpu))) : null;

    // 1. Hostname -> Node text
    const displayHost = (host.hostname && String(host.hostname).trim())
      ? String(host.hostname).trim()
      : `${host.id || 'host'} (no name)`;
    setIfMapped('textParam', isOnline ? displayHost : `${displayHost} [OFF]`);

    // 2. Online / Offline & CPU -> Visual & animation controls
    if (!isOnline) {
      setIfMapped('flowVisibilityParam', false);
      setIfMapped('rippleVisibilityParam', false);
      setIfMapped('flowColorParam', '#64748b');
      setIfMapped('flowSpeedParam', 0.5);
      setIfMapped('strokeParam', '#ef4444');
      setIfMapped('fillParam', '#2a1215');
      setIfMapped('glowColorParam', '#ef4444');
      setIfMapped('glowOpacityParam', 0.9);
      setIfMapped('rippleOpacityParam', 0.0);
      return out;
    }

    // Online host with MISSING CPU metric
    if (!hasCpu) {
      if (missingStrategy === 'treat-as-zero') {
        setIfMapped('flowVisibilityParam', true);
        setIfMapped('rippleVisibilityParam', true);
        setIfMapped('flowColorParam', '#38bdf8');
        setIfMapped('flowSpeedParam', 0.5);
        setIfMapped('strokeParam', '#38bdf8');
        setIfMapped('fillParam', '#1e293b');
        setIfMapped('glowColorParam', '#38bdf8');
        setIfMapped('glowOpacityParam', 0.35);
        setIfMapped('rippleOpacityParam', 0.3);
      } else if (missingStrategy === 'hide') {
        setIfMapped('flowVisibilityParam', false);
        setIfMapped('rippleVisibilityParam', false);
        setIfMapped('strokeParam', '#64748b');
        setIfMapped('fillParam', '#1e293b');
        setIfMapped('glowColorParam', '#64748b');
        setIfMapped('glowOpacityParam', 0.15);
        setIfMapped('rippleOpacityParam', 0.15);
      } else {
        // Default 'warn' strategy: amber telemetry warning indicator
        setIfMapped('flowVisibilityParam', true);
        setIfMapped('rippleVisibilityParam', true);
        setIfMapped('flowColorParam', '#f59e0b');
        setIfMapped('flowSpeedParam', 0.4);
        setIfMapped('strokeParam', '#f59e0b');
        setIfMapped('fillParam', '#2d2210');
        setIfMapped('glowColorParam', '#f59e0b');
        setIfMapped('glowOpacityParam', 0.9);
        setIfMapped('rippleOpacityParam', 0.65);
        setIfMapped('textParam', `${displayHost} [NO CPU]`);
      }
      return out;
    }

    // Online host with VALID CPU === 0 (idle)
    if (cpuNum === 0) {
      setIfMapped('flowVisibilityParam', true);
      setIfMapped('rippleVisibilityParam', true);
      setIfMapped('flowColorParam', '#38bdf8');
      setIfMapped('flowSpeedParam', 0.5);
      setIfMapped('strokeParam', '#38bdf8');
      setIfMapped('fillParam', '#1e293b');
      setIfMapped('glowColorParam', '#38bdf8');
      setIfMapped('glowOpacityParam', 0.35);
      setIfMapped('rippleOpacityParam', 0.3);
      return out;
    }

    // Online host with VALID nonzero CPU (1..100)
    const accent = cpuNum >= 80 ? '#ef4444' : (cpuNum >= 50 ? '#f59e0b' : '#22c55e');
    const fill = cpuNum >= 80 ? '#3b1518' : '#1e293b';
    const speed = Number((0.5 + (cpuNum / 100) * 3.5).toFixed(2));
    const glowOp = Number((0.35 + (cpuNum / 100) * 0.65).toFixed(2));
    const rippleOp = Number((0.3 + (cpuNum / 100) * 0.7).toFixed(2));

    setIfMapped('flowVisibilityParam', true);
    setIfMapped('rippleVisibilityParam', true);
    setIfMapped('flowColorParam', accent);
    setIfMapped('flowSpeedParam', speed);
    setIfMapped('strokeParam', accent);
    setIfMapped('fillParam', fill);
    setIfMapped('glowColorParam', accent);
    setIfMapped('glowOpacityParam', glowOp);
    setIfMapped('rippleOpacityParam', rippleOp);
    return out;
  }

  class SimulatedHostDataSource {
    constructor(initialHosts) {
      this.hosts = (initialHosts || DEFAULT_HOSTS).map((h) => Object.assign({}, h));
      this._tick = 0;
    }

    getHosts() {
      return this.hosts.map((h) => Object.assign({}, h));
    }

    getHost(id) {
      const h = this.hosts.find((item) => item.id === id);
      return h ? Object.assign({}, h) : null;
    }

    updateHost(id, patch) {
      const idx = this.hosts.findIndex((item) => item.id === id);
      if (idx === -1) throw new Error(`Unknown simulated host "${id}"`);
      this.hosts[idx] = Object.assign({}, this.hosts[idx], patch);
      return Object.assign({}, this.hosts[idx]);
    }

    stepSimulation() {
      this._tick += 1;
      for (const h of this.hosts) {
        if (!h.online) continue;
        if (h.cpu === null || h.cpu === undefined) continue;
        const delta = ((this._tick % 2 === 0 ? 1 : -1) * (7 + (h.id.charCodeAt(h.id.length - 1) % 5)));
        h.cpu = Math.max(0, Math.min(100, Number(h.cpu) + delta));
      }
      return this.getHosts();
    }

    reset() {
      this.hosts = DEFAULT_HOSTS.map((h) => Object.assign({}, h));
      this._tick = 0;
      return this.getHosts();
    }
  }

  class WidgetInstanceManager {
    constructor() {
      this.activePackage = null;
      this.activeMapping = null;
      this.instances = new Map(); // hostId -> { handle, container, mountCount }
      this.missingStrategy = 'warn';
    }

    loadPackage(pkg, mappingSpec) {
      const info = validateWidgetPackageConsistency(pkg);
      validateMappingsAgainstManifest(mappingSpec, pkg.manifest);
      this.unmountAll();
      this.activePackage = pkg;
      this.activeMapping = Object.assign({}, mappingSpec);
      return info;
    }

    mountHostInstance(host, container) {
      if (!this.activePackage) {
        throw new Error('Cannot mount widget instance: no validated widget package is loaded.');
      }
      if (!container) {
        throw new Error(`Cannot mount instance for "${host.id}": container element is required.`);
      }

      const existing = this.instances.get(host.id);
      const prevMountCount = existing ? existing.mountCount : 0;
      if (existing && existing.handle && !existing.handle.isDestroyed) {
        existing.handle.destroy();
      }

      const initialParams = mapHostRecordToWidgetParameters(
        host,
        this.activeMapping,
        this.activePackage.manifest,
        { missingStrategy: this.missingStrategy }
      );

      const handle = this.activePackage.widgetRuntime.mount(container, this.activePackage.svgText, {
        instanceId: `consumer-${host.id}-${prevMountCount + 1}`,
        manifest: this.activePackage.manifest,
        parameters: initialParams
      });

      this.instances.set(host.id, {
        handle,
        container,
        mountCount: prevMountCount + 1
      });
      return handle;
    }

    updateHostInstance(host) {
      const entry = this.instances.get(host.id);
      if (!entry || !entry.handle || entry.handle.isDestroyed) {
        return null;
      }
      const params = mapHostRecordToWidgetParameters(
        host,
        this.activeMapping,
        this.activePackage.manifest,
        { missingStrategy: this.missingStrategy }
      );
      entry.handle.setParameters(params);
      return params;
    }

    unmountHostInstance(hostId) {
      const entry = this.instances.get(hostId);
      if (!entry) return false;
      if (entry.handle && !entry.handle.isDestroyed) {
        entry.handle.destroy();
      }
      this.instances.delete(hostId);
      return true;
    }

    unmountAll() {
      for (const hostId of Array.from(this.instances.keys())) {
        this.unmountHostInstance(hostId);
      }
    }

    getInstanceInfo(hostId) {
      const entry = this.instances.get(hostId);
      if (!entry || !entry.handle) return null;
      return {
        instanceId: entry.handle.instanceId,
        isDestroyed: entry.handle.isDestroyed,
        mountCount: entry.mountCount,
        parameters: entry.handle.getParameters()
      };
    }
  }

  return {
    DEFAULT_HOSTS,
    MAPPING_PROFILES,
    extractEmbeddedManifest,
    validateWidgetPackageConsistency,
    validateMappingsAgainstManifest,
    mapHostRecordToWidgetParameters,
    SimulatedHostDataSource,
    WidgetInstanceManager
  };
});
