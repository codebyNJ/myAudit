package preview

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/codebyNJ/myAudit/internal/proc"
)

const (
	bootTimeout = 90 * time.Second
	portLow     = 41000
	portHigh    = 41200
)

type Launcher struct {
	Name   string
	Args   []string
	Static bool
}

type Server struct {
	RunID      string
	URL        string
	Cmd        *exec.Cmd
	HTTPServer *http.Server
	Log        string
	port       int
}

// startWait lets concurrent Start(runID) callers for the same run share one
// boot attempt: result fields are written by the booting goroutine only
// before it closes ch, so readers that received from the closed channel may
// read them without holding m.mu.
type startWait struct {
	ch     chan struct{}
	server *Server
	err    error
}

type crashInfo struct {
	reason string
}

type Manager struct {
	mu       sync.Mutex
	live     map[string]*Server
	starting map[string]*startWait
	crashes  map[string]crashInfo
	reserved map[int]bool
	root     string
}

func New(workspaceRoot string) *Manager {
	return &Manager{live: map[string]*Server{}, starting: map[string]*startWait{}, crashes: map[string]crashInfo{}, reserved: map[int]bool{}, root: workspaceRoot}
}

// reservePort hands out a port no other in-flight start is using. Probing with
// a listener alone is not enough: the listener must be closed before the child
// can bind it, so two runs booting at the same moment both probed the same
// free port and the second dev server failed to bind.
func (m *Manager) reservePort() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for p := portLow; p <= portHigh; p++ {
		if m.reserved[p] {
			continue
		}
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			continue
		}
		l.Close()
		m.reserved[p] = true
		return p, nil
	}
	return 0, fmt.Errorf("no free port in %d-%d", portLow, portHigh)
}

func (m *Manager) releasePort(p int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.reserved, p)
}

// launcherFor resolves the Launcher for a run's directory. It is a variable
// (rather than a direct call to Command) so tests can substitute a fake,
// slow-to-become-reachable launcher to exercise the in-flight start guard.
var launcherFor = Command
var launcherForMu sync.Mutex

func swapLauncherFor(fn func(string) (Launcher, bool)) func() {
	launcherForMu.Lock()
	prev := launcherFor
	launcherFor = fn
	launcherForMu.Unlock()
	return func() {
		launcherForMu.Lock()
		launcherFor = prev
		launcherForMu.Unlock()
	}
}

func Command(dir string) (Launcher, bool) {
	if Detect(dir) == KindNone {
		return Launcher{}, false
	}
	if l, ok := nodeLauncher(dir); ok {
		return l, true
	}
	if fileExists(filepath.Join(dir, "index.html")) {
		return Launcher{Static: true}, true
	}
	return Launcher{}, false
}

func nodeLauncher(dir string) (Launcher, bool) {
	pkg := filepath.Join(dir, "package.json")
	b, err := os.ReadFile(pkg)
	if err != nil {
		return Launcher{}, false
	}
	var m struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(b, &m) != nil {
		return Launcher{}, false
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); err != nil {
		return Launcher{}, false
	}
	for _, s := range []string{"dev", "start", "serve", "tauri"} {
		if _, has := m.Scripts[s]; has {
			return Launcher{Name: "npm", Args: []string{"run", s}}, true
		}
	}
	return Launcher{}, false
}

func reachable(port int) bool {
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func (m *Manager) Get(runID string) (*Server, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.live[runID]
	return s, ok
}

func (m *Manager) Start(runID string) (*Server, error) {
	m.mu.Lock()
	if s, ok := m.live[runID]; ok {
		m.mu.Unlock()
		return s, nil
	}
	if w, ok := m.starting[runID]; ok {
		m.mu.Unlock()
		<-w.ch
		return w.server, w.err
	}
	w := &startWait{ch: make(chan struct{})}
	m.starting[runID] = w
	m.mu.Unlock()

	server, err := m.start(runID)

	m.mu.Lock()
	delete(m.starting, runID)
	m.mu.Unlock()
	w.server, w.err = server, err
	close(w.ch)

	return server, err
}

func (m *Manager) start(runID string) (*Server, error) {
	m.mu.Lock()
	delete(m.crashes, runID)
	m.mu.Unlock()

	dir := filepath.Join(m.root, runID)
	port, err := m.reservePort()
	if err != nil {
		return nil, err
	}
	released := false
	releaseUnlessLive := func() {
		if !released {
			released = true
			m.releasePort(port)
		}
	}
	launcherForMu.Lock()
	pick := launcherFor
	launcherForMu.Unlock()
	l, ok := pick(dir)
	if !ok {
		releaseUnlessLive()
		return nil, fmt.Errorf("no dev server detected for this project")
	}
	if l.Static {
		s, err := m.startStatic(runID, dir, port)
		if err != nil {
			releaseUnlessLive()
		}
		return s, err
	}

	logPath := filepath.Join(dir, ".myaudit", "preview.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	lf, _ := os.Create(logPath)

	c := exec.Command(l.Name, l.Args...)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"PORT="+fmt.Sprint(port),
		"BROWSER=none",
		"NEXT_TELEMETRY_DISABLED=1",
	)
	if lf != nil {
		c.Stdout, c.Stderr = lf, lf
	}
	proc.SetGroup(c)

	if err := c.Start(); err != nil {
		if lf != nil {
			lf.Close()
		}
		releaseUnlessLive()
		return nil, err
	}
	// The child has its own handle to the log file once started; close ours
	// so the parent process doesn't hold the file open for the run's
	// lifetime (Windows locks the file for as long as any handle is open).
	if lf != nil {
		lf.Close()
	}

	// Wait must be called exactly once, so the boot loop and watch() share this
	// channel. Polling c.ProcessState here instead never reported an exit:
	// ProcessState stays nil until Wait returns, so a dev server that died on
	// startup burned the whole bootTimeout before failing with the wrong error.
	exited := make(chan error, 1)
	go func() { exited <- c.Wait() }()

	deadline := time.Now().Add(bootTimeout)
	for time.Now().Before(deadline) {
		if reachable(port) {
			s := &Server{RunID: runID, URL: fmt.Sprintf("http://localhost:%d", port), Cmd: c, Log: logPath, port: port}
			m.mu.Lock()
			m.live[runID] = s
			m.mu.Unlock()
			released = true // the live server owns the port until Stop
			writeLive(dir, s.URL)
			go m.watch(runID, s, exited)
			return s, nil
		}
		select {
		case err := <-exited:
			releaseUnlessLive()
			return nil, fmt.Errorf("dev server exited during startup (%v) — see .myaudit/preview.log", err)
		case <-time.After(250 * time.Millisecond):
		}
	}
	_ = proc.KillTree(c)
	releaseUnlessLive()
	return nil, fmt.Errorf("dev server did not answer on port %d within %s", port, bootTimeout)
}

func (m *Manager) startStatic(runID, dir string, port int) (*Server, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Addr: addr, Handler: http.FileServer(http.Dir(dir))}
	go func() { _ = srv.Serve(ln) }()

	s := &Server{RunID: runID, URL: fmt.Sprintf("http://localhost:%d", port), HTTPServer: srv, port: port}
	m.mu.Lock()
	m.live[runID] = s
	m.mu.Unlock()
	writeLive(dir, s.URL)
	return s, nil
}

// watch reaps the subprocess and, if it exits without Stop having already
// removed it, records the exit as a crash. Comparing m.live[runID] == s (not
// just presence) means a Stop or a newer boot racing this exit is not
// mistaken for a crash.
func (m *Manager) watch(runID string, s *Server, exited <-chan error) {
	err := <-exited
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.live[runID] != s {
		return
	}
	delete(m.live, runID)
	delete(m.reserved, s.port)
	reason := "dev server exited"
	if err != nil {
		reason = err.Error()
	}
	m.crashes[runID] = crashInfo{reason: reason}
}

func (m *Manager) LastCrash(runID string) (reason string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.crashes[runID]
	return c.reason, ok
}

func (m *Manager) Restart(runID string) (*Server, error) {
	m.Stop(runID)
	return m.Start(runID)
}

func (m *Manager) Stop(runID string) {
	m.mu.Lock()
	s, ok := m.live[runID]
	delete(m.live, runID)
	m.mu.Unlock()
	if !ok {
		return
	}
	if s.HTTPServer != nil {
		_ = s.HTTPServer.Close()
	} else {
		_ = proc.KillTree(s.Cmd)
	}
	m.releasePort(s.port)
	clearLive(filepath.Join(m.root, runID))
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.live))
	for id := range m.live {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Stop(id)
	}
}

func writeLive(dir, url string) {
	b, _ := json.Marshal(map[string]string{"url": url, "title": "dev server (auto-started)"})
	_ = os.WriteFile(filepath.Join(dir, ".myaudit", "live.json"), b, 0o644)
}

func clearLive(dir string) {
	os.Remove(filepath.Join(dir, ".myaudit", "live.json"))
}
