package animlab

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func countRemoteTestChrome(t *testing.T, host string) (int, string) {
	t.Helper()
	out, err := runHostCmd(host, `ps -eo pid=,args= | grep '/tmp/sva-headless-chrome-' | grep -v grep || true`)
	if err != nil {
		t.Fatalf("probing remote processes on %s: %v", host, err)
	}
	if strings.TrimSpace(out) == "" {
		return 0, ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return len(lines), out
}

func TestBrowserSessionLifecycle(t *testing.T) {
	const host = "nigel@192.168.100.40"
	if _, err := runHostCmd(host, "echo ok"); err != nil {
		t.Skipf("Kiwiberry (%s) not reachable via SSH: %v", host, err)
	}

	// Ensure clean starting baseline
	if cnt, raw := countRemoteTestChrome(t, host); cnt != 0 {
		t.Fatalf("expected 0 pre-existing /tmp/sva-headless-chrome-* processes on %s, got %d:\n%s", host, cnt, raw)
	}

	// 1. Start a dummy "unrelated" process with chrome in its command line on Kiwiberry
	// and verify our session lifecycle never touches it.
	unrelOut, err := runHostCmd(host, `python3 -c 'import time; time.sleep(45)' --unrelated-user-chrome-session >/dev/null 2>&1 < /dev/null & echo $!`)
	if err != nil {
		t.Fatalf("starting dummy unrelated process: %v", err)
	}
	unrelPID := strings.TrimSpace(unrelOut)
	defer func() {
		_, _ = runHostCmd(host, "kill -TERM "+unrelPID+" 2>/dev/null || true")
	}()

	// 2. Check 1 & 8: Normal test completion + Test Lab BrowserOwnership & process-tree CPU accounting
	cfg := &Config{
		Version: 1,
		Test:    TestMeta{Name: "lifecycle-verify", Description: "Focused lifecycle & CPU ownership check"},
		Browser: BrowserConfig{
			Engine:            "chromium",
			Headless:          true,
			CDPEndpoint:       "http://127.0.0.1:9222",
			HostBindIP:        "192.168.100.47",
			ProcessCPUHost:    host,
			ViewportWidth:     800,
			ViewportHeight:    600,
			DeviceScaleFactor: 1.0,
		},
		Execution: ExecutionConfig{
			WarmupSeconds:   0.2,
			DurationSeconds: 0.6,
			Repetitions:     1,
			OrderStrategy:   "sequential",
		},
		Instances: InstancesConfig{
			Counts:    []int{10},
			Isolation: "independent-svg-roots",
		},
		Effects: []EffectSpec{{
			Family:        "perimeter-tracer",
			Methods:       []string{"css"},
			Shapes:        []string{"rect"},
			Sizes:         map[string][2]int{"small": {50, 50}},
			Speeds:        map[string]float64{"medium": 1.0},
			SegmentCounts: []int{2},
			LineGapRatios: []string{"25:75"},
		}},
		Workloads:    []WorkloadSpec{{Name: "all-active", Visibility: "all"}},
		Measurements: []string{"process_cpu", "frame_timing", "animation_correctness"},
	}

	res, err := RunSuite(cfg, t.TempDir(), "")
	if err != nil {
		t.Fatalf("RunSuite failed: %v", err)
	}
	if len(res.Scenarios) != 1 || !res.Scenarios[0].CorrectnessPassed {
		t.Fatalf("expected scenario correctness to pass, got: %+v", res.Scenarios)
	}
	rep0 := res.Scenarios[0].Repetitions[0]
	if rep0.ProcessCPUSec == nil || *rep0.ProcessCPUSec <= 0 || rep0.OwnedPIDCount < 2 {
		t.Fatalf("expected valid process-tree CPU accounting (ProcessCPUSec>0, OwnedPIDCount>=2), got cpu=%v pids=%d",
			rep0.ProcessCPUSec, rep0.OwnedPIDCount)
	}

	if cnt, raw := countRemoteTestChrome(t, host); cnt != 0 {
		t.Fatalf("Check 1 FAILED: expected 0 owned Chrome processes after normal RunSuite completion, got %d:\n%s", cnt, raw)
	}

	// 3. Check 2: Test failure path performs full cleanup
	func() {
		sess, err := AcquireBrowserSession(context.Background(), "http://127.0.0.1:9222", host)
		if err != nil {
			t.Fatalf("AcquireBrowserSession (failure check): %v", err)
		}
		defer sess.Close()
		tabCtx, cancelTab := sess.NewTab()
		defer cancelTab()
		// Navigate to about:blank, then simulate an action error / test failure return
		_, _ = chromedp.Run(tabCtx, chromedp.Navigate("about:blank"))
		_, evalErr := chromedp.Run(tabCtx, chromedp.Evaluate[string](`throw new Error("simulated failure")`))
		if evalErr == nil {
			t.Fatalf("expected simulated evaluation failure")
		}
	}()
	if cnt, raw := countRemoteTestChrome(t, host); cnt != 0 {
		t.Fatalf("Check 2 FAILED: expected 0 owned Chrome processes after test failure, got %d:\n%s", cnt, raw)
	}

	// 4. Check 3: Test cancellation / timeout performs full cleanup
	func() {
		sess, err := AcquireBrowserSession(context.Background(), "http://127.0.0.1:9222", host)
		if err != nil {
			t.Fatalf("AcquireBrowserSession (timeout check): %v", err)
		}
		defer sess.Close()
		tCtx, cancelTimeout := context.WithTimeout(sess.AllocCtx, 150*time.Millisecond)
		defer cancelTimeout()
		tabCtx, cancelTab := chromedp.NewContext(tCtx)
		defer cancelTab()
		_, _ = chromedp.Run(tabCtx, chromedp.Navigate("about:blank"))
		_, _ = chromedp.Run(tabCtx, chromedp.Sleep(2*time.Second))
	}()
	if cnt, raw := countRemoteTestChrome(t, host); cnt != 0 {
		t.Fatalf("Check 3 FAILED: expected 0 owned Chrome processes after timeout/cancellation, got %d:\n%s", cnt, raw)
	}

	// 5. Check 4: Repeated browser runs (3 consecutive runs) do not accumulate processes or profiles
	for i := 1; i <= 3; i++ {
		func() {
			sess, err := AcquireBrowserSession(context.Background(), "http://127.0.0.1:9222", host)
			if err != nil {
				t.Fatalf("AcquireBrowserSession (rep %d): %v", i, err)
			}
			defer sess.Close()
			tabCtx, cancelTab := sess.NewTab()
			defer cancelTab()
			if _, err := chromedp.Run(tabCtx, chromedp.Navigate("http://192.168.100.40:8088/text-lite-demo.html")); err != nil {
				t.Fatalf("rep %d navigate failed: %v", i, err)
			}
		}()
		if cnt, raw := countRemoteTestChrome(t, host); cnt != 0 {
			t.Fatalf("Check 4 FAILED after repetition %d: expected 0 owned Chrome processes, got %d:\n%s", i, cnt, raw)
		}
	}

	// Verify temporary profiles in /tmp/sva-headless-chrome-* were removed
	profOut, _ := runHostCmd(host, `ls -d /tmp/sva-headless-chrome-* 2>/dev/null || true`)
	if strings.TrimSpace(profOut) != "" {
		t.Fatalf("expected 0 leftover /tmp/sva-headless-chrome-* directories, found:\n%s", profOut)
	}

	// 6. Check 5 & 6: Verify unrelated session is still alive and all 4 intentional services (:8088, :8091, :8092, :8093) are running
	aliveOut, _ := runHostCmd(host, "ps -p "+unrelPID+" -o pid= || true")
	if strings.TrimSpace(aliveOut) != unrelPID {
		t.Fatalf("Check 6 FAILED: unrelated process PID %s was unexpectedly terminated", unrelPID)
	}
	portsOut, _ := exec.Command("ssh", "-o", "BatchMode=yes", host, "ss -tln | grep -E ':8088|:8091|:8092|:8093'").Output()
	portsStr := string(portsOut)
	for _, p := range []string{":8088", ":8091", ":8092", ":8093"} {
		if !strings.Contains(portsStr, p) {
			t.Fatalf("Check 5 FAILED: intentional service on port %s is not listening:\n%s", p, portsStr)
		}
	}
}
