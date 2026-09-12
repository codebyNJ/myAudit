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
	root     string
}

func New(workspaceRoot string) *Manager {
	return &Manager{live: map[string]*Server{}, starting: map[string]*startWait{}, crashes: map[string]crashInfo{}, root: workspaceRoot}
}

// launcherFor resolves the Launcher for a run's directory. It is a variable
// (rather than a direct call to Command) so tests can substitute a fake,
// slow-to-become-reachable launcher to exercise the in-flight start guard.
var launcherFor = Command

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

func freePort() (int, error) {
	for p := portLow; p <= portHigh; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			l.Close()
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port in %d-%d", portLow, portHigh)
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
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	l, ok := launcherFor(dir)
	if !ok {
		return nil, fmt.Errorf("no dev server detected for this project")
	}
	if l.Static {
		return m.startStatic(runID, dir, port)
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
		return nil, err
	}
	// The child has its own handle to the log file once started; close ours
	// so the parent process doesn't hold the file open for the run's
	// lifetime (Windows locks the file for as long as any handle is open).
	if lf != nil {
		lf.Close()
	}

	deadline := time.Now().Add(bootTimeout)
	for time.Now().Before(deadline) {
		if reachable(port) {
			s := &Server{RunID: runID, URL: fmt.Sprintf("http://localhost:%d", port), Cmd: c, Log: logPath}
			m.mu.Lock()
			m.live[runID] = s
			m.mu.Unlock()
			writeLive(dir, s.URL)
			go m.watch(runID, s)
			return s, nil
		}
		if c.ProcessState != nil && c.ProcessState.Exited() {
			return nil, fmt.Errorf("dev server exited during startup (see .myaudit/preview.log)")
		}
		time.Sleep(250 * time.Millisecond)
	}
	_ = proc.KillTree(c)
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

	s := &Server{RunID: runID, URL: fmt.Sprintf("http://localhost:%d", port), HTTPServer: srv}
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
func (m *Manager) watch(runID string, s *Server) {
	err := s.Cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.live[runID] != s {
		return
	}
	delete(m.live, runID)
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
