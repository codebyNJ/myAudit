package worker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A model that omits `class`, or invents one, must not be able to quietly
// downgrade a real defect into a note.
func TestNormClassDefaultsToBug(t *testing.T) {
	for _, in := range []string{"", "   ", "nonsense", "BUG", "defect"} {
		if got := normClass(in); got != "bug" {
			t.Errorf("normClass(%q) = %q, want bug", in, got)
		}
	}
	for _, in := range []string{"improvement", "Improvement", " STYLE ", "question"} {
		if got := normClass(in); got == "bug" {
			t.Errorf("normClass(%q) = bug, want the class the agent chose", in)
		}
	}
}

// The whole point of the class field: a naming nit must not arrive as a P0,
// whatever severity the agent claimed for it.
func TestPriorityForKeepsNonDefectsOutOfP0(t *testing.T) {
	for _, class := range []string{"improvement", "style", "question"} {
		if got := priorityFor(class, "high"); got != "P2" {
			t.Errorf("priorityFor(%q, high) = %q, want P2", class, got)
		}
	}
	// Real defects still map straight off severity.
	for sev, want := range map[string]string{"high": "P0", "medium": "P1", "low": "P2"} {
		if got := priorityFor("bug", sev); got != want {
			t.Errorf("priorityFor(bug, %q) = %q, want %q", sev, got, want)
		}
	}
}

// Findings outside the audited module are tagged, never dropped — a
// cross-module problem is real, and the module boundary is ours rather than
// the codebase's.
func TestScopeTag(t *testing.T) {
	cases := []struct{ file, module, want string }{
		{"internal/api/api.go:12", "internal/api", "scope:in"},
		{"./internal/api/api.go", "internal/api", "scope:in"},
		{"internal/api", "internal/api", "scope:in"},
		{"internal/store/read.go:40", "internal/api", "scope:adjacent"},
		{"go.mod", "internal/api", "scope:adjacent"},
		// A root-scoped audit owns everything.
		{"anything/at/all.go", ".", "scope:in"},
		// Nothing to compare against.
		{"", "internal/api", "scope:in"},
	}
	for _, c := range cases {
		if got := scopeTag(c.file, c.module); got != c.want {
			t.Errorf("scopeTag(%q, %q) = %q, want %q", c.file, c.module, got, c.want)
		}
	}
}

// A near-miss prefix must not count as inside the module.
func TestScopeTagDoesNotMatchSiblingPrefix(t *testing.T) {
	if got := scopeTag("internal/apiserver/main.go", "internal/api"); got != "scope:adjacent" {
		t.Fatalf("internal/apiserver is not inside internal/api, got %q", got)
	}
}

func TestNormConfidence(t *testing.T) {
	for _, in := range []string{"high", "MEDIUM", " low "} {
		if normConfidence(in) == "" {
			t.Errorf("normConfidence(%q) should be kept", in)
		}
	}
	for _, in := range []string{"", "pretty sure", "80%"} {
		if got := normConfidence(in); got != "" {
			t.Errorf("normConfidence(%q) = %q, want empty rather than a guess", in, got)
		}
	}
}

func TestParseFindingsReadsClassAndConfidence(t *testing.T) {
	got := parseFindings(`[{"title":"naming is inconsistent","class":"style","severity":"low","confidence":"high","category":"maintainability"}]`)
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d", len(got))
	}
	if got[0].Class != "style" || got[0].Confidence != "high" || got[0].Category != "maintainability" {
		t.Fatalf("class/confidence/category not parsed: %+v", got[0])
	}
}

// Nothing told the agent how this codebase prefers to be written, so a pattern
// used deliberately came back as a finding.
func TestReadConventionsCollectsRepoFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "AGENTS.md", "Always return errors, never panic.")
	write(t, dir, "CONTRIBUTING.md", "Table-driven tests only.")

	got := readConventions(dir)
	for _, want := range []string{"Repo conventions", "AGENTS.md", "never panic", "CONTRIBUTING.md", "Table-driven"} {
		if !strings.Contains(got, want) {
			t.Fatalf("conventions missing %q, got:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "not a defect") {
		t.Fatal("conventions should tell the agent to treat these as intentional")
	}
}

func TestReadConventionsEmptyWhenRepoStatesNone(t *testing.T) {
	if got := readConventions(t.TempDir()); got != "" {
		t.Fatalf("want no section when the repo has no convention files, got:\n%s", got)
	}
}

func TestReadConventionsTruncatesLongFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "CLAUDE.md", strings.Repeat("x", conventionBudget*2))

	got := readConventions(dir)
	if !strings.Contains(got, "truncated") {
		t.Fatal("an oversized convention file should be truncated, not pasted whole into every prompt")
	}
	if len(got) > conventionBudget*2 {
		t.Fatalf("truncation did not bound the output: %d bytes", len(got))
	}
}

// The prompt is the actual cause of every complaint in #25, so assert on it.
func TestQATaskAsksForClassAndJudgement(t *testing.T) {
	got := qaTask("backend", "backend", "", "")

	for _, want := range []string{
		`"class":"bug|improvement|style|question"`,
		`"confidence":"high|medium|low"`,
		`"category":`,
		"is a convention, not a defect",
		"do not silently reattribute it to this module",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("qaTask missing %q", want)
		}
	}

	// It used to instruct the agent to hunt style gaps and then filed the
	// results as bugs; that instruction is what made style compete with
	// security for attention.
	if strings.Contains(got, "style/best-practice gaps") {
		t.Error("qaTask still asks for style gaps as part of the defect hunt")
	}
	if strings.Contains(got, "best-practice violations") {
		t.Error("qaTask still asks for best-practice violations alongside real bugs")
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
