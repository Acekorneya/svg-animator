# Stage 16B: Public Widget Runtime API Reference

## 1. Top-Level Library Interface (`SVGWidget`)

The library is distributed as a universal UMD module (`internal/web/static/js/svg-widget.js`).

### `SVGWidget.mount(target, rawSvg, options)`

Mounts an isolated SVG document into a DOM container element.

- **Parameters:**
  - `target` (`HTMLElement | string`): Container element or CSS selector.
  - `rawSvg` (`string`): Full SVG source markup.
  - `options` (`object`, optional):
    - `instanceId` (`string`): Unique identifier for this instance (defaults to auto-generated `sva-w-<id>`).
    - `prefix` (`string`): Prefix used for all namespaced IDs (defaults to `${instanceId}__`).
    - `scopeCss` (`boolean`): Whether to namespace `<style>` rules (default `true`).
    - `shadowDOM` (`boolean`): If `true`, mounts inside a Shadow Root for complete isolation (default `false`).
    - `manifest` (`object`): Optional parameter manifest schema overriding or supplementing embedded manifest.
    - `initialParameters` (`object`): Initial parameter key-value overrides applied immediately on mount.
    - `width` (`number | string`): Wrapper width (e.g. `'100%'`, `400`).
    - `height` (`number | string`): Wrapper height (e.g. `'240px'`).
- **Returns:** `WidgetInstanceHandle`

---

## 2. Widget Instance Handle (`instance`)

### Parameter Methods

#### `instance.getParameter(name: string): any`
Returns the current value of the named parameter, or `undefined` if not defined.

#### `instance.getParameters(): Record<string, any>`
Returns a shallow clone of all current parameter values.

#### `instance.setParameter(name: string, value: any): WidgetInstanceHandle`
Sets a single parameter value. Validates type and constraints before applying to DOM. Throws an Error if validation fails. Returns the instance handle for method chaining.

#### `instance.setParameters(batch: Record<string, any>): WidgetInstanceHandle`
Sets multiple parameter values atomically. If any parameter fails validation, the entire batch is rejected with an Error and zero DOM mutations occur.

#### `instance.resetParameters(): WidgetInstanceHandle`
Resets all controllable parameters to their default values declared in the manifest.

#### `instance.getManifest(): WidgetManifest`
Returns the widget's parameter manifest schema.

---

### Playback & Animation Methods

#### `instance.play(): WidgetInstanceHandle`
Resumes playback of all animations (SMIL, CSS, Web Animations API) within this widget.

#### `instance.pause(): WidgetInstanceHandle`
Pauses all animations within this widget without affecting other widgets on the page.

#### `instance.resume(): WidgetInstanceHandle`
Alias for `instance.play()`.

#### `instance.stop(): WidgetInstanceHandle`
Pauses animations and resets current playback time to 0.

#### `instance.setSpeed(rate: number): WidgetInstanceHandle`
Sets the playback rate for animations within this instance (`1.0` = normal, `2.0` = double speed).

#### `instance.getSpeed(): number`
Returns the current playback speed rate.

#### `instance.isPaused(): boolean`
Returns `true` if this instance is currently paused.

---

### Event System Methods

#### `instance.on(event: string, callback: Function): WidgetInstanceHandle`
Registers an event listener. Supported events:
- `'change'`: Fired when parameters update. Payload: `{ instanceId, parameters: changedKeyValues }`.
- `'parametersChanged'`: Fired after batch updates. Payload: `{ instanceId, parameters: allValues }`.
- `'play'`: Fired when instance playback starts or resumes.
- `'pause'`: Fired when instance playback pauses.
- `'stop'`: Fired when instance playback stops.
- `'destroy'`: Fired when instance is destroyed.

#### `instance.off(event: string, callback?: Function): WidgetInstanceHandle`
Unregisters event listener(s).

---

### Lifecycle Methods

#### `instance.destroy(): WidgetInstanceHandle`
Stops playback, unmounts the wrapper and SVG from the DOM, clears internal listeners, and marks the instance as destroyed (`instance.isDestroyed === true`).

#### `instance.unmount(): WidgetInstanceHandle`
Removes the widget element from the DOM without destroying event listeners.
