package preview

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchRecordsCrashAfterUnexpectedExit(t *testing.T) {
	root := t.TempDir()
	runID := "run-crash"
	if err := os.MkdirAll(filepath.Join(root, runID), 0o755); err != nil {
		t.Fatal(err)
	}

	origLauncher := launcherFor
	launcherFor = func(dir string) (Launcher, bool) {
		return Launcher{Name: os.Args[0], Args: []string{"-test.run=TestCrashHelperProcess", "--"}}, true
	}
	t.Cleanup(func() { launcherFor = origLauncher })

	if err := os.Setenv("GO_WANT_CRASH_HELPER", "1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("GO_WANT_CRASH_HELPER") })

	m := New(root)
	if _, err := m.Start(runID); err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	var reason string
	var ok bool
	for time.Now().Before(deadline) {
		reason, ok = m.LastCrash(runID)
		if ok {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ok {
		t.Fatal("expected LastCrash to be populated after the process exited unexpectedly")
	}
	if reason == "" {
		t.Fatal("expected a non-empty crash reason")
	}
	if _, stillLive := m.Get(runID); stillLive {
		t.Fatal("expected the dead server to be removed from live once crashed")
	}
}

func TestRestartClearsCrashAndBootsFresh(t *testing.T) {
	root := t.TempDir()
	runID := "run-restart"
	if err := os.MkdirAll(filepath.Join(root, runID), 0o755); err != nil {
		t.Fatal(err)
	}

	origLauncher := launcherFor
	invocations := filepath.Join(root, "invocations.log")
	launcherFor = func(dir string) (Launcher, bool) {
		return Launcher{Name: os.Args[0], Args: []string{"-test.run=TestHelperProcess", "--", invocations}}, true
	}
	t.Cleanup(func() { launcherFor = origLauncher })

	if err := os.Setenv("GO_WANT_HELPER_PROCESS", "1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("GO_WANT_HELPER_PROCESS") })

	m := New(root)
	first, err := m.Start(runID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Stop(runID) })

	second, err := m.Restart(runID)
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if second == first {
		t.Fatal("expected Restart to boot a fresh server, not return the stale one")
	}
	if _, ok := m.LastCrash(runID); ok {
		t.Fatal("expected Restart to clear any stale crash record")
	}
}

// TestCrashHelperProcess is not a real test; it's re-executed as a subprocess
// to simulate a dev server that boots successfully then exits on its own.
func TestCrashHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_CRASH_HELPER") != "1" {
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+os.Getenv("PORT"))
	if err != nil {
		os.Exit(1)
	}
	time.Sleep(300 * time.Millisecond)
	ln.Close()
	os.Exit(1)
}
