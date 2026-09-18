package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/store"
)

var errBadPath = errors.New("bad path")

// resolveWorkspacePath maps a caller-supplied relative path to a location
// inside runs/<id>. It is the single guard for every workspace file endpoint:
// each one used to repeat a `..`-prefix check, which let two things through —
// "." (which resolves to the run root, so DELETE erased the whole workspace)
// and symlinks, which an imported repo can carry and the agent can create, and
// which point wherever they like. Returns the absolute path to operate on and
// the cleaned relative path for tools that want one (git, event logs).
func resolveWorkspacePath(runID uuid.UUID, rel string) (abs, clean string, err error) {
	clean = filepath.Clean(rel)
	sep := string(filepath.Separator)
	if rel == "" || clean == "." || clean == ".." || clean == sep ||
		strings.HasPrefix(clean, ".."+sep) || filepath.IsAbs(clean) {
		return "", "", errBadPath
	}
	root, err := filepath.Abs(filepath.Join("runs", runID.String()))
	if err != nil {
		return "", "", errBadPath
	}
	abs = filepath.Join(root, clean)
	if !within(root, abs) {
		return "", "", errBadPath
	}
	return abs, clean, nil
}

// wsPathFrom resolves the path and writes the 400 itself, so handlers stay flat.
func wsPathFrom(w http.ResponseWriter, r *http.Request, rel string) (id uuid.UUID, abs, clean string, ok bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return id, "", "", false
	}
	abs, clean, err = resolveWorkspacePath(id, rel)
	if err != nil {
		http.Error(w, "bad path", 400)
		return id, "", "", false
	}
	return id, abs, clean, true
}

// within reports whether p stays inside root once symlinks are followed.
func within(root, p string) bool {
	realP, err := resolveExisting(p)
	if err != nil {
		return false
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	rel, err := filepath.Rel(realRoot, realP)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolveExisting evaluates symlinks on the longest existing prefix of p and
// re-attaches the remainder, so a path whose leaf does not exist yet (a new
// file, a rename target) can still be checked.
func resolveExisting(p string) (string, error) {
	cur, rest := p, ""
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			return filepath.Join(resolved, rest), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return filepath.Join(cur, rest), nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// nodeInRun loads the node named by {nid} and confirms it belongs to the run
// named by {id}. The node handlers used to ignore {id} entirely, so a node id
// could be mutated through any run's URL.
func nodeInRun(w http.ResponseWriter, r *http.Request, s *store.Store) (store.Node, bool) {
	runID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad run id", 400)
		return store.Node{}, false
	}
	nodeID, err := uuid.Parse(r.PathValue("nid"))
	if err != nil {
		http.Error(w, "bad node id", 400)
		return store.Node{}, false
	}
	n, err := s.GetNode(r.Context(), nodeID)
	if err != nil || n.RunID != runID {
		http.Error(w, "node not found in this run", 404)
		return store.Node{}, false
	}
	return n, true
}
