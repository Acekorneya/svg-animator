package animlab

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/systeminfo"
	"github.com/chromedp/chromedp"
)

// BrowserOwnership records how the runner established ownership of a specific Chromium instance.
type BrowserOwnership struct {
	Mode        string `json:"mode"` // "external-cdp-isolated-tree" | "unavailable"
	RootPID     int    `json:"rootPid,omitempty"`
	UserDataDir string `json:"userDataDir,omitempty"`
	CDPPort     int    `json:"cdpPort,omitempty"`
	ClkTck      int    `json:"clkTck,omitempty"`
	Host        string `json:"host,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

func runHostCmd(sshHost, script string) (string, error) {
	var out []byte
	var err error
	if sshHost != "" {
		out, err = exec.Command("ssh", "-o", "ConnectTimeout=5", "-o", "BatchMode=yes", sshHost, script).Output()
	} else {
		out, err = exec.Command("bash", "-c", script).Output()
	}
	return strings.TrimSpace(string(out)), err
}

func parseCDPPort(cdpEndpoint string) int {
	u, err := url.Parse(cdpEndpoint)
	if err != nil || u.Port() == "" {
		return 9222
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil || p <= 0 {
		return 9222
	}
	return p
}

// EstablishBrowserOwnership identifies the specific root Chromium PID attached via CDP
// and verifies access to its Linux /proc process tree and CLK_TCK.
func EstablishBrowserOwnership(ctx context.Context, cdpEndpoint, sshHost string) BrowserOwnership {
	port := parseCDPPort(cdpEndpoint)
	own := BrowserOwnership{
		Mode:    "unavailable",
		CDPPort: port,
		Host:    sshHost,
	}
	if own.Host == "" {
		own.Host = "localhost"
	}

	// 1. Try CDP SystemInfo.getProcessInfo to get the authoritative browser PID
	var browserPID int
	if si, err := chromedp.Call(ctx, systeminfo.GetProcessInfo, cdp.Empty{}); err == nil {
		for _, p := range si.ProcessInfo {
			if p.Type == "browser" && p.ID > 0 {
				browserPID = int(p.ID)
				break
			}
		}
	}

	// 2. Verify/discover root PID + CLK_TCK + user-data-dir on browser host
	discoverScript := fmt.Sprintf(`
CLK=$(getconf CLK_TCK 2>/dev/null || echo 100)
BPID="%d"
if [ -z "$BPID" ] || [ "$BPID" = "0" ] || [ ! -d "/proc/$BPID" ]; then
  BPID=$(ps -eo pid=,args= | awk -v port="--remote-debugging-port=%d" 'index($0, port) && $0 !~ /--type=/ {print $1; exit}')
fi
if [ -z "$BPID" ] || [ "$BPID" = "0" ] || [ ! -d "/proc/$BPID" ]; then
  echo "ERR|no-root-pid|$CLK"
  exit 0
fi
ARGS=$(tr '\0' ' ' < "/proc/$BPID/cmdline" 2>/dev/null)
UDIR=$(echo "$ARGS" | grep -oE '\-\-user-data-dir=[^ ]+' | head -n1 | cut -d= -f2-)
echo "OK|$BPID|$CLK|$UDIR"
`, browserPID, port)

	out, err := runHostCmd(sshHost, discoverScript)
	if err != nil || out == "" {
		own.Reason = fmt.Sprintf("host %s unreachable or /proc probe failed: %v", own.Host, err)
		return own
	}
	parts := strings.SplitN(out, "|", 4)
	if len(parts) < 3 || parts[0] != "OK" {
		own.Reason = fmt.Sprintf("could not identify Chromium root PID on %s for port %d (%s)", own.Host, port, out)
		return own
	}
	pid, err1 := strconv.Atoi(parts[1])
	clk, err2 := strconv.Atoi(parts[2])
	if err1 != nil || pid <= 0 {
		own.Reason = fmt.Sprintf("invalid root PID %q", parts[1])
		return own
	}
	if err2 != nil || clk <= 0 {
		clk = 100
	}
	own.Mode = "external-cdp-isolated-tree"
	own.RootPID = pid
	own.ClkTck = clk
	if len(parts) >= 4 {
		own.UserDataDir = parts[3]
	}
	own.Reason = fmt.Sprintf("verified Chromium root PID %d (port=%d, clk_tck=%d)", pid, port, clk)
	return own
}

// SampleOwnedProcessTreeCPU sums utime+stime (in seconds) strictly for own.RootPID and its recursive descendants.
// Returns nil (unavailable) if ownership is not established or the root PID no longer exists.
func SampleOwnedProcessTreeCPU(own BrowserOwnership, sshHost string) (*float64, int, error) {
	if own.Mode == "unavailable" || own.RootPID <= 0 || own.ClkTck <= 0 {
		return nil, 0, fmt.Errorf("browser ownership unavailable: %s", own.Reason)
	}

	// Single fast shell invocation:
	// 1. Verify root PID still exists.
	// 2. Walk ps -eo pid=,ppid= to collect rootPID + all recursive descendants.
	// 3. Sum /proc/<pid>/stat fields 14 (utime) + 15 (stime) for those PIDs only, divide by own.ClkTck.
	script := fmt.Sprintf(`
ROOT=%d
CLK=%d
if [ ! -r "/proc/$ROOT/stat" ]; then
  echo "GONE"
  exit 0
fi
PIDS=$(ps -eo pid=,ppid= | awk -v root="$ROOT" '
  { pid=$1; ppid=$2; children[ppid]=children[ppid]" "pid }
  END {
    q[1]=root; head=1; tail=1; seen[root]=1;
    while (head<=tail) {
      cur=q[head++];
      print cur;
      n=split(children[cur], arr, " ");
      for (i=1; i<=n; i++) {
        c=arr[i];
        if (c!="" && !seen[c]) { seen[c]=1; q[++tail]=c; }
      }
    }
  }')
CNT=0
TICKS=0
for p in $PIDS; do
  if [ -r "/proc/$p/stat" ]; then
    # Field 2 (comm) can contain spaces inside parentheses; strip up to last ')'
    T=$(sed 's/.*) //' "/proc/$p/stat" 2>/dev/null | awk '{print $12+$13}')
    if [ -n "$T" ]; then
      TICKS=$((TICKS + T))
      CNT=$((CNT + 1))
    fi
  fi
done
if [ "$CNT" -eq 0 ]; then
  echo "GONE"
  exit 0
fi
awk -v t="$TICKS" -v c="$CLK" -v n="$CNT" 'BEGIN { printf "OK|%%.6f|%%d\n", t/c, n }'
`, own.RootPID, own.ClkTck)

	out, err := runHostCmd(sshHost, script)
	if err != nil {
		return nil, 0, fmt.Errorf("cpu probe error: %w", err)
	}
	if out == "" || strings.HasPrefix(out, "GONE") {
		return nil, 0, fmt.Errorf("owned browser root PID %d terminated or unreadable", own.RootPID)
	}
	parts := strings.Split(out, "|")
	if len(parts) != 3 || parts[0] != "OK" {
		return nil, 0, fmt.Errorf("unexpected cpu probe output: %q", out)
	}
	sec, err1 := strconv.ParseFloat(parts[1], 64)
	cnt, err2 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || cnt <= 0 {
		return nil, 0, fmt.Errorf("failed parsing cpu probe output: %q", out)
	}
	return &sec, cnt, nil
}

// CollectHostHardwareInfo gathers OS, CPU model, cores, and memory from the browser host.
func CollectHostHardwareInfo(sshHost string) map[string]any {
	script := `
OS=$(uname -srm 2>/dev/null)
CPU=$(awk -F: '/model name/ {gsub(/^[ \t]+/, "", $2); print $2; exit}' /proc/cpuinfo 2>/dev/null)
CORES=$(nproc 2>/dev/null || echo 0)
MEM_KB=$(awk '/MemTotal/ {print $2; exit}' /proc/meminfo 2>/dev/null)
CLK=$(getconf CLK_TCK 2>/dev/null || echo 100)
echo "$OS|$CPU|$CORES|$MEM_KB|$CLK"
`
	out, err := runHostCmd(sshHost, script)
	if err != nil || out == "" {
		return map[string]any{"targetHostProbe": "unavailable"}
	}
	parts := strings.SplitN(out, "|", 5)
	res := map[string]any{}
	if len(parts) >= 1 && parts[0] != "" {
		res["targetOS"] = parts[0]
	}
	if len(parts) >= 2 && parts[1] != "" {
		res["targetCPUModel"] = parts[1]
	}
	if len(parts) >= 3 {
		if c, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil && c > 0 {
			res["targetCPUCores"] = c
		}
	}
	if len(parts) >= 4 {
		if kb, err := strconv.Atoi(strings.TrimSpace(parts[3])); err == nil && kb > 0 {
			res["targetMemoryMB"] = kb / 1024
		}
	}
	if len(parts) >= 5 {
		if clk, err := strconv.Atoi(strings.TrimSpace(parts[4])); err == nil && clk > 0 {
			res["clkTck"] = clk
		}
	}
	return res
}
