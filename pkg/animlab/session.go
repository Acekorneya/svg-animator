package animlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
)

// BrowserSession manages a chromedp browser session that either connects to an existing
// CDP endpoint or spawns an ephemeral, isolated headless Chrome instance (locally or via SSH on
// a target host) with its own temporary user-data-dir and process group.
//
// When Close() is called (or on SIGINT/SIGTERM), BrowserSession closes all CDP targets,
// gracefully terminates any browser process and SSH tunnel it spawned, and removes the
// temporary profile directory. It never terminates a pre-existing external browser process.
type BrowserSession struct {
	CDPEndpoint string
	CPUHost     string
	AllocCtx    context.Context

	closeOnce   sync.Once
	cancelAlloc context.CancelFunc
	sigStop     func()

	managedRemote bool
	remoteHost    string
	remotePID     int
	remotePort    int
	remoteProfile string
	tunnelCmd     *exec.Cmd

	managedLocal bool
	localCmd     *exec.Cmd
	localProfile string
}

// AcquireBrowserSession connects to cdpEndpoint if reachable; otherwise it launches an
// ephemeral headless Chrome instance (on cpuHost via SSH if cpuHost is non-empty and not
// localhost, or locally otherwise) with a dedicated temporary profile and signal-safe cleanup.
func AcquireBrowserSession(parentCtx context.Context, cdpEndpoint, cpuHost string) (*BrowserSession, error) {
	if cdpEndpoint == "" {
		cdpEndpoint = "http://127.0.0.1:9222"
	}
	s := &BrowserSession{
		CDPEndpoint: cdpEndpoint,
		CPUHost:     cpuHost,
	}

	if isCDPReachable(cdpEndpoint) {
		allocCtx, cancel := remote.NewAllocator(parentCtx, cdpEndpoint)
		s.AllocCtx = allocCtx
		s.cancelAlloc = cancel
		s.installSignalHandler()
		return s, nil
	}

	// No pre-existing CDP server at cdpEndpoint; launch an ephemeral managed browser.
	isRemoteHost := cpuHost != "" && cpuHost != "localhost" && cpuHost != "127.0.0.1"
	if isRemoteHost {
		if err := s.startManagedRemote(cdpEndpoint, cpuHost); err != nil {
			s.Close()
			return nil, err
		}
	} else {
		if err := s.startManagedLocal(cdpEndpoint); err != nil {
			s.Close()
			return nil, err
		}
	}

	allocCtx, cancel := remote.NewAllocator(parentCtx, s.CDPEndpoint)
	s.AllocCtx = allocCtx
	s.cancelAlloc = cancel
	s.installSignalHandler()
	return s, nil
}

// NewTab creates a new isolated browser tab (CDP target) on the session's allocator.
// Canceling the returned CancelFunc closes and detaches the tab in Chrome.
func (s *BrowserSession) NewTab(opts ...chromedp.ContextOption) (context.Context, context.CancelFunc) {
	return chromedp.NewContext(s.AllocCtx, opts...)
}

func (s *BrowserSession) installSignalHandler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	s.sigStop = func() {
		signal.Stop(sigCh)
		close(done)
	}
	go func() {
		select {
		case <-sigCh:
			s.Close()
			os.Exit(130)
		case <-done:
		}
	}()
}

// Close closes all CDP contexts/tabs created under this session and tears down any
// ephemeral browser process, SSH port-forward tunnel, and temporary profile directory
// owned by this session. Safe to call multiple times.
func (s *BrowserSession) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.sigStop != nil {
			s.sigStop()
		}
		if s.cancelAlloc != nil {
			s.cancelAlloc()
		}

		if s.managedRemote {
			s.stopManagedRemote()
		}
		if s.managedLocal {
			s.stopManagedLocal()
		}
	})
}

func isCDPReachable(cdpEndpoint string) bool {
	verURL := strings.TrimRight(cdpEndpoint, "/") + "/json/version"
	if strings.HasPrefix(verURL, "ws://") {
		verURL = "http://" + strings.TrimPrefix(verURL, "ws://")
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get(verURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var info map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return false
	}
	_, ok := info["webSocketDebuggerUrl"]
	return ok
}

func allocateLocalPort(preferred int) (int, error) {
	if preferred > 0 {
		if ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferred)); err == nil {
			_ = ln.Close()
			return preferred, nil
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func (s *BrowserSession) startManagedRemote(cdpEndpoint, sshHost string) error {
	prefPort := parseCDPPort(cdpEndpoint)
	localPort, err := allocateLocalPort(prefPort)
	if err != nil {
		return fmt.Errorf("allocating local CDP forward port: %w", err)
	}

	launchScript := fmt.Sprintf(`
set -e
PORT=%d
if ss -tln 2>/dev/null | grep -q ":${PORT} "; then
  PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
fi
UDIR=$(mktemp -d /tmp/sva-headless-chrome-XXXXXX)
CHROME_BIN=""
for b in /opt/google/chrome/chrome /usr/bin/google-chrome /usr/bin/google-chrome-stable /usr/bin/chromium /usr/bin/chromium-browser; do
  if [ -x "$b" ]; then CHROME_BIN="$b"; break; fi
done
if [ -z "$CHROME_BIN" ]; then
  echo "ERR|no-chrome-binary"
  exit 1
fi
setsid "$CHROME_BIN" \
  --headless \
  --no-sandbox \
  --disable-gpu \
  --remote-debugging-port="$PORT" \
  --remote-debugging-address=127.0.0.1 \
  --incognito \
  --noerrdialogs \
  --no-first-run \
  --disable-dev-shm-usage \
  --user-data-dir="$UDIR" \
  --ozone-platform=headless \
  --ozone-override-screen-size=800,600 \
  --use-angle=swiftshader-webgl \
  about:blank >/dev/null 2>&1 < /dev/null &
BPID=$!
for i in $(seq 1 40); do
  if curl -fsS "http://127.0.0.1:${PORT}/json/version" >/dev/null 2>&1; then
    echo "OK|${BPID}|${PORT}|${UDIR}"
    exit 0
  fi
  sleep 0.1
done
kill -TERM -"$BPID" 2>/dev/null || kill -TERM "$BPID" 2>/dev/null || true
rm -rf "$UDIR" 2>/dev/null || true
echo "ERR|timeout-waiting-for-cdp"
exit 1
`, prefPort)

	out, err := runHostCmd(sshHost, launchScript)
	if err != nil || !strings.HasPrefix(out, "OK|") {
		return fmt.Errorf("failed launching ephemeral headless Chrome on %s: %v (%s)", sshHost, err, out)
	}
	parts := strings.SplitN(out, "|", 4)
	if len(parts) != 4 {
		return fmt.Errorf("unexpected launch output from %s: %q", sshHost, out)
	}
	rPID, _ := strconv.Atoi(parts[1])
	rPort, _ := strconv.Atoi(parts[2])
	rProfile := strings.TrimSpace(parts[3])

	s.managedRemote = true
	s.remoteHost = sshHost
	s.remotePID = rPID
	s.remotePort = rPort
	s.remoteProfile = rProfile

	// Start dedicated SSH local port-forward tunnel (foreground child process, no -f)
	tunnelSpec := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", localPort, rPort)
	tCmd := exec.Command("ssh",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-o", "ExitOnForwardFailure=yes",
		"-N", "-L", tunnelSpec,
		sshHost,
	)
	var stderr bytes.Buffer
	tCmd.Stderr = &stderr
	if err := tCmd.Start(); err != nil {
		return fmt.Errorf("starting SSH CDP tunnel to %s: %w", sshHost, err)
	}
	s.tunnelCmd = tCmd
	s.CDPEndpoint = fmt.Sprintf("http://127.0.0.1:%d", localPort)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if isCDPReachable(s.CDPEndpoint) {
			return nil
		}
		time.Sleep(80 * time.Millisecond)
	}
	return fmt.Errorf("SSH CDP tunnel to %s (%s) did not become reachable: %s", sshHost, s.CDPEndpoint, stderr.String())
}

func (s *BrowserSession) stopManagedRemote() {
	if s.tunnelCmd != nil && s.tunnelCmd.Process != nil {
		_ = s.tunnelCmd.Process.Kill()
		_, _ = s.tunnelCmd.Process.Wait()
		s.tunnelCmd = nil
	}
	if s.remoteHost == "" || s.remotePID <= 0 {
		return
	}
	// Verify the remote PID / user-data-dir still belongs to our ephemeral session before signaling.
	cleanupScript := fmt.Sprintf(`
BPID=%d
UDIR=%q
if [ -n "$BPID" ] && [ "$BPID" -gt 1 ] && [ -d "/proc/$BPID" ]; then
  CMD=$(tr '\0' ' ' < "/proc/$BPID/cmdline" 2>/dev/null || true)
  if echo "$CMD" | grep -qF -- "$UDIR"; then
    kill -TERM -"$BPID" 2>/dev/null || kill -TERM "$BPID" 2>/dev/null || true
    for i in $(seq 1 25); do
      if [ ! -d "/proc/$BPID" ]; then break; fi
      sleep 0.1
    done
    if [ -d "/proc/$BPID" ]; then
      kill -KILL -"$BPID" 2>/dev/null || kill -KILL "$BPID" 2>/dev/null || true
    fi
  fi
fi
# Ensure all processes using the dedicated temporary profile exit before removing the directory
if [ -n "$UDIR" ] && echo "$UDIR" | grep -q '^/tmp/sva-headless-chrome-'; then
  for attempt in 1 2 3 4 5; do
    REMAIN=$(ps -eo pid=,args= | grep -F -- "$UDIR" | grep -v "rm -rf" | awk '{print $1}' || true)
    if [ -z "$REMAIN" ]; then break; fi
    if [ "$attempt" -le 3 ]; then
      kill -TERM $REMAIN 2>/dev/null || true
    else
      kill -KILL $REMAIN 2>/dev/null || true
    fi
    sleep 0.15
  done
  rm -rf "$UDIR" 2>/dev/null || true
  if [ -d "$UDIR" ]; then
    sleep 0.2
    rm -rf "$UDIR" 2>/dev/null || true
  fi
fi
`, s.remotePID, s.remoteProfile)
	_, _ = runHostCmd(s.remoteHost, cleanupScript)
}

func (s *BrowserSession) startManagedLocal(cdpEndpoint string) error {
	prefPort := parseCDPPort(cdpEndpoint)
	localPort, err := allocateLocalPort(prefPort)
	if err != nil {
		return fmt.Errorf("allocating local CDP port: %w", err)
	}
	uDir, err := os.MkdirTemp("", "sva-headless-chrome-*")
	if err != nil {
		return fmt.Errorf("creating temporary Chrome user-data-dir: %w", err)
	}
	chromeBin := ""
	for _, b := range []string{"/opt/google/chrome/chrome", "/usr/bin/google-chrome", "/usr/bin/google-chrome-stable", "/usr/bin/chromium", "/usr/bin/chromium-browser"} {
		if st, err := os.Stat(b); err == nil && !st.IsDir() {
			chromeBin = b
			break
		}
	}
	if chromeBin == "" {
		_ = os.RemoveAll(uDir)
		return fmt.Errorf("no local Chrome/Chromium executable found")
	}

	cmd := exec.Command(chromeBin,
		"--headless",
		"--no-sandbox",
		"--disable-gpu",
		fmt.Sprintf("--remote-debugging-port=%d", localPort),
		"--remote-debugging-address=127.0.0.1",
		"--incognito",
		"--noerrdialogs",
		"--no-first-run",
		"--disable-dev-shm-usage",
		"--user-data-dir="+uDir,
		"--ozone-platform=headless",
		"--ozone-override-screen-size=800,600",
		"--use-angle=swiftshader-webgl",
		"about:blank",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(uDir)
		return fmt.Errorf("starting local headless Chrome: %w", err)
	}
	s.managedLocal = true
	s.localCmd = cmd
	s.localProfile = uDir
	s.CDPEndpoint = fmt.Sprintf("http://127.0.0.1:%d", localPort)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if isCDPReachable(s.CDPEndpoint) {
			return nil
		}
		time.Sleep(80 * time.Millisecond)
	}
	return fmt.Errorf("local headless Chrome on %s did not become reachable", s.CDPEndpoint)
}

func (s *BrowserSession) stopManagedLocal() {
	if s.localCmd != nil && s.localCmd.Process != nil {
		pgid, err := syscall.Getpgid(s.localCmd.Process.Pid)
		if err == nil && pgid > 1 {
			_ = syscall.Kill(-pgid, syscall.SIGTERM)
		} else {
			_ = s.localCmd.Process.Signal(syscall.SIGTERM)
		}
		done := make(chan struct{})
		go func() {
			_, _ = s.localCmd.Process.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2500 * time.Millisecond):
			if err == nil && pgid > 1 {
				_ = syscall.Kill(-pgid, syscall.SIGKILL)
			} else {
				_ = s.localCmd.Process.Kill()
			}
			<-done
		}
		s.localCmd = nil
	}
	if s.localProfile != "" && strings.Contains(s.localProfile, "sva-headless-chrome-") {
		_ = os.RemoveAll(s.localProfile)
		s.localProfile = ""
	}
}

// ParseCDPHost extracts the hostname from a URL if needed.
func ParseCDPHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
