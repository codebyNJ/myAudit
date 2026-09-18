package preview

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// A dev server that dies on startup must be reported immediately. The old
// ProcessState poll never fired, so this took the full 90s bootTimeout.
func TestStartFailsFastWhenServerExitsImmediately(t *testing.T) {
	root := t.TempDir()
	runID := "run-bootfail"
	if err := os.MkdirAll(filepath.Join(root, runID), 0o755); err != nil {
		t.Fatal(err)
	}

	restore := swapLauncherFor(func(dir string) (Launcher, bool) {
		return Launcher{Name: os.Args[0], Args: []string{"-test.run=^TestBootFailHelperProcess$", "--"}}, true
	})
	t.Cleanup(restore)
	t.Setenv("GO_WANT_BOOTFAIL_HELPER", "1")

	m := New(root)
	start := time.Now()
	_, err := m.Start(runID)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an error when the dev server exits during startup")
	}
	if elapsed > 15*time.Second {
		t.Fatalf("took %s to notice the exit; should not wait out bootTimeout (%s)", elapsed, bootTimeout)
	}
}

// The port a failed boot reserved must go back in the pool.
func TestFailedStartReleasesItsPort(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "r1"), 0o755); err != nil {
		t.Fatal(err)
	}
	restore := swapLauncherFor(func(dir string) (Launcher, bool) { return Launcher{}, false })
	t.Cleanup(restore)

	m := New(root)
	if _, err := m.Start("r1"); err == nil {
		t.Fatal("expected Start to fail with no launcher")
	}
	m.mu.Lock()
	held := len(m.reserved)
	m.mu.Unlock()
	if held != 0 {
		t.Fatalf("failed start leaked %d reserved port(s)", held)
	}
}

// Two runs booting at once must not be handed the same port.
func TestReservePortIsUniqueAcrossConcurrentStarts(t *testing.T) {
	m := New(t.TempDir())
	const n = 12
	ports := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			p, err := m.reservePort()
			if err != nil {
				t.Errorf("reservePort: %v", err)
				return
			}
			ports[i] = p
		}(i)
	}
	wg.Wait()

	seen := map[int]bool{}
	for _, p := range ports {
		if p == 0 {
			continue
		}
		if seen[p] {
			t.Fatalf("port %d handed out twice", p)
		}
		seen[p] = true
	}
	if len(seen) != n {
		t.Fatalf("expected %d distinct ports, got %d", n, len(seen))
	}

	for p := range seen {
		m.releasePort(p)
	}
	m.mu.Lock()
	left := len(m.reserved)
	m.mu.Unlock()
	if left != 0 {
		t.Fatalf("releasePort left %d reservations behind", left)
	}
}

// TestBootFailHelperProcess is not a real test; it stands in for a dev server
// that exits without ever binding its port.
func TestBootFailHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_BOOTFAIL_HELPER") != "1" {
		return
	}
	os.Exit(1)
}
