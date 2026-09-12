package preview

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStartConcurrentSingleSpawn proves that two Start(runID) calls racing
// against the same run while the boot is still in flight collapse into one
// boot attempt: only one subprocess is spawned, and both callers observe the
// same *Server once it becomes reachable. Without the in-flight guard, both
// calls see runID absent from m.live (registration happens only after the
// port answers) and each spawns its own subprocess.
func TestStartConcurrentSingleSpawn(t *testing.T) {
	root := t.TempDir()
	runID := "run-concurrent"
	if err := os.MkdirAll(filepath.Join(root, runID), 0o755); err != nil {
		t.Fatal(err)
	}

	invocations := filepath.Join(root, "invocations.log")

	origLauncher := launcherFor
	launcherFor = func(dir string) (Launcher, bool) {
		return Launcher{Name: os.Args[0], Args: []string{"-test.run=TestHelperProcess", "--", invocations}}, true
	}
	t.Cleanup(func() { launcherFor = origLauncher })

	if err := os.Setenv("GO_WANT_HELPER_PROCESS", "1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("GO_WANT_HELPER_PROCESS") })

	m := New(root)

	const n = 2
	results := make([]*Server, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = m.Start(runID)
		}(i)
	}
	wg.Wait()
	t.Cleanup(func() {
		if s := results[0]; s != nil && s.Cmd != nil && s.Cmd.Process != nil {
			_ = s.Cmd.Process.Kill()
			_, _ = s.Cmd.Process.Wait()
		}
		m.Stop(runID)
	})

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Start[%d] returned error: %v", i, err)
		}
	}
	if results[0] == nil || results[1] == nil {
		t.Fatalf("expected non-nil servers, got %+v", results)
	}
	if results[0] != results[1] {
		t.Fatalf("expected both concurrent Start calls to return the same *Server, got %p and %p", results[0], results[1])
	}

	b, err := os.ReadFile(invocations)
	if err != nil {
		t.Fatalf("reading invocation log: %v", err)
	}
	count := len(strings.Split(strings.TrimSpace(string(b)), "\n"))
	if count != 1 {
		t.Fatalf("expected exactly 1 subprocess spawn for concurrent Start calls, got %d (log: %q)", count, string(b))
	}
}

// TestHelperProcess is not a real test; it is re-executed as a subprocess by
// TestStartConcurrentSingleSpawn to stand in for a slow-to-boot dev server.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		os.Exit(2)
	}
	invocations := args[1]

	f, err := os.OpenFile(invocations, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		os.Exit(1)
	}
	_, _ = f.WriteString("1\n")
	_ = f.Close()

	// Simulate a slow-booting dev server so concurrent Start calls overlap
	// while this process is still not reachable on PORT.
	time.Sleep(500 * time.Millisecond)

	ln, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("PORT"))
	if err != nil {
		os.Exit(1)
	}
	defer ln.Close()
	time.Sleep(time.Hour)
}
