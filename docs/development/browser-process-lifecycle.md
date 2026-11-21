# Browser and Test-Process Lifecycle Operations Guide

## 1. Purpose

Automated browser verification and SVG Animation Test Lab benchmarks execute real headless Chromium/Chrome sessions (typically on Kiwiberry `192.168.100.40` or locally). Because animated SVG pages and WebAssembly studio tabs continuously run CSS/SMIL/JS animations, an orphaned headless browser or unclosed CDP tab will consume CPU and memory indefinitely—causing severe CPU pressure stalls on a two-vCPU guest.

This guide defines the operational procedure for identifying, attributing, safely terminating, and verifying browser test processes while protecting intentional development services and user sessions.

---

## 2. Intentional Development Services (Protected)

The following services on Kiwiberry (`192.168.100.40`) are intentional, persistent development servers and must **never** be terminated during browser cleanup:

| Port | Service | Process Pattern | Purpose |
| :--- | :--- | :--- | :--- |
| `:8088` | SVG Animator Studio | `/home/nigel/svg-animator -serve :8088` | Primary Go/WASM Studio server & Lite Effect demos |
| `:8091` | Stage 16C Widget Consumer | `python3 -m http.server 8091` | External data-driven SVG widget consumer app |
| `:8092` | Stage 16D Benchmark / Demo | `python3 -m http.server 8092` | Stage 16D scalability benchmark harness |
| `:8093` | SVG Animation Test Lab Viewer | `/home/nigel/anim-test-lab/anim-test-lab serve ... -addr :8093` | Permanent Test Lab catalogue & report server |

Always distinguish a **web server** serving a port from a **browser renderer process** viewing a page served by that port.

---

## 3. How to Identify Browser Test Processes

Inspect the browser host (e.g., `ssh nigel@192.168.100.40`) using `ps`, `pstree`, and `ss`:

```bash
# 1. List all Chrome/Chromium processes with PID, PPID, PGID, start time, elapsed time, CPU, RSS, and full command line
ps -eo pid,ppid,pgid,user,lstart,etime,time,%cpu,%mem,rss,args --sort=pid | grep -E "PID|chrome|chromium" | grep -v grep

# 2. Check listening Chrome DevTools Protocol (CDP) debugging ports (e.g. 9222 or ephemeral ports)
ss -tulpn | grep -E "chrome|chromium|9222"

# 3. View the process hierarchy rooted at a specific browser PID
pstree -aps <ROOT_PID>
```

Key attributes to record before taking any action:
- **Root Browser PID & PGID:** The top-level browser process has no `--type=` flag in its command line; child zygote, GPU (`--type=gpu-process`), utility (`--type=utility`), and tab renderer (`--type=renderer`) processes descend from it.
- **Debugging Port:** `--remote-debugging-port=<PORT>`.
- **Profile Directory:** `--user-data-dir=<PATH>`. Ephemeral sessions spawned by `animlab.AcquireBrowserSession` use `/tmp/sva-headless-chrome-XXXXXX`.

---

## 4. How to Determine the Owning Harness and Distinguish Active vs. Abandoned Sessions

Before terminating any browser process, determine what created it and whether a test or user is actively connected:

```bash
# 1. Check active TCP connections to the CDP port (e.g. 9222)
ss -tanp | grep :<CDP_PORT>

# 2. Check if any test runner is currently executing on the test runner host or browser host
ps -ef | grep -E "anim-test-lab|verify-|benchmark-|go test" | grep -v grep

# 3. Query open CDP targets (tabs/pages) to see which URLs and titles are loaded
curl -s http://127.0.0.1:<CDP_PORT>/json/list | python3 -m json.tool
```

### Distinguishing Sessions
- **Active Test Session:** An `anim-test-lab` or `verify-*` process is currently running, or an active `ESTABLISHED` TCP connection exists on `<CDP_PORT>` from a running test binary. **Do not terminate.**
- **Interactive User Browser Session:** Runs without `--headless` / `--ozone-platform=headless` or uses the user's primary desktop profile (`/home/nigel/.config/google-chrome/Default`). **Never terminate.**
- **Abandoned Test Workload:** Runs with `--headless` and a temporary `--user-data-dir` (e.g. `/tmp/sva-headless-chrome-*` or `.config/google-chrome-headless/scoped_dir*`), has no running `verify-*` or `anim-test-lab` test process on either host, and has accumulated stale test tabs in `/json/list`.

---

## 5. How to Terminate Owned Test Browsers Safely

Never run indiscriminate commands such as `killall chrome` or `pkill chrome`. Instead, clean up a confirmed abandoned headless test browser by its specific CDP endpoint, PID/PGID, and dedicated `--user-data-dir`:

1. **Close open CDP page targets gracefully** (when the CDP port is responsive):
   ```bash
   python3 -c '
   import json, urllib.request
   for t in json.loads(urllib.request.urlopen("http://127.0.0.1:9222/json/list").read()):
       if t.get("type") == "page":
           urllib.request.urlopen("http://127.0.0.1:9222/json/close/" + t["id"]).read()
   '
   ```
2. **Send `SIGTERM` to the specific root browser PID (or process group) after verifying its `--user-data-dir` in `/proc/<PID>/cmdline`:**
   ```bash
   kill -TERM <ROOT_PID>
   ```
3. **Wait up to 2–3 seconds** for child renderer, GPU, zygote, and `chrome_crashpad_handler` subprocesses to exit cleanly.
4. **Escalate to `SIGKILL` only if specific processes from that verified `--user-data-dir` fail to exit.**
5. **Remove the temporary profile directory** only after confirming the process has exited and the directory is a test-generated temporary profile (e.g. `/tmp/sva-headless-chrome-*` or `/home/nigel/.config/google-chrome-headless/scoped_dir*`).
6. **Terminate any orphaned local SSH port-forward tunnel** (`ssh -N -L 9222:127.0.0.1:9222 ...`) by its specific PID.

---

## 6. How to Verify Cleanup and Inspect Pressure-Stall Measurements

After terminating an abandoned browser workload (or after completing a browser verification suite), verify that both processes and system pressure have recovered:

```bash
# 1. Confirm zero unexpected Chrome processes remain
ps -ef | grep -E "chrome|chromium" | grep -v grep

# 2. Check instantaneous CPU and memory availability
top -b -n 2 -d 1 | head -n 25
free -m
vmstat 1 3

# 3. Inspect Linux Pressure Stall Information (PSI)
cat /proc/pressure/cpu
cat /proc/pressure/memory
cat /proc/pressure/io

# 4. Verify all four protected development services remain healthy
ss -tln | grep -E ':8088|:8091|:8092|:8093'
```

- On an idle two-vCPU guest, `/proc/pressure/cpu` `some avg10` should drop to near `0.00–2.00%` within 15–30 seconds (`avg60` and `avg300` decay exponentially over 1 and 5 minutes).
- `vmstat` `si` / `so` (swap in/out) should be `0`, and CPU `id` (idle) should be `95–100%` when no test is running.

---

## 7. Automated Lifecycle Management (`animlab.AcquireBrowserSession`)

All Go browser verification harnesses (`scripts/cmd/verify-*/main.go`, `scripts/cmd/benchmark-stage16d/main.go`) and the SVG Animation Test Lab (`pkg/animlab/runner.go`) use `animlab.AcquireBrowserSession` (`pkg/animlab/session.go`):

- **Self-Managed Ephemeral Browser:** If no CDP server is running at `127.0.0.1:9222`, `AcquireBrowserSession` automatically launches an isolated headless Chrome process on the target host in its own session (`setsid`) with a unique `/tmp/sva-headless-chrome-XXXXXX` profile and a tracked foreground SSH port-forward tunnel.
- **Deterministic Teardown:** `defer sess.Close()` (plus an `os.Interrupt` / `SIGTERM` signal handler) closes all CDP tabs, terminates the owned remote/local Chrome process tree, closes the SSH tunnel, and deletes `/tmp/sva-headless-chrome-XXXXXX`.
- **Safe External Attach:** If an intentionally persistent external CDP browser is already listening at `cdpEndpoint`, `AcquireBrowserSession` attaches to it and closes only the tabs created during the test run without killing the external browser.
- **No `log.Fatal` / `os.Exit` Bypass:** Harnesses use a `func main() { os.Exit(run()) }` pattern so `defer sess.Close()` and `defer cancelTask()` always execute on test failure or timeout.

---

## 8. Handling Intentionally Persistent Browser Sessions

If a developer deliberately starts a persistent headless browser session for interactive debugging:
1. Launch it with a clearly named profile directory (e.g. `--user-data-dir=/tmp/sva-persistent-debug-<owner>`).
2. Document its PID, port, owner, and purpose in the active session notes.
3. Shut it down explicitly (`kill -TERM <PID>` and remove its profile directory) as soon as the debugging session finishes.
