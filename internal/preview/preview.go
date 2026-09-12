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

type Manager struct {
	mu   sync.Mutex
	live map[string]*Server
	root string
}

func New(workspaceRoot string) *Manager {
	return &Manager{live: map[string]*Server{}, root: workspaceRoot}
}

func Command(dir string, port int) (Launcher, bool) {
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
	m.mu.Unlock()

	dir := filepath.Join(m.root, runID)
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	l, ok := Command(dir, port)
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

	deadline := time.Now().Add(bootTimeout)
	for time.Now().Before(deadline) {
		if reachable(port) {
			s := &Server{RunID: runID, URL: fmt.Sprintf("http://localhost:%d", port), Cmd: c, Log: logPath}
			m.mu.Lock()
			m.live[runID] = s
			m.mu.Unlock()
			writeLive(dir, s.URL)
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
		go func() { _ = s.Cmd.Wait() }()
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
