package worker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChangedFiles(t *testing.T) {
	d, err := os.ReadFile("testdata/two_file.diff")
	if err != nil {
		t.Fatal(err)
	}
	f := changedFiles(string(d))
	if len(f) != 2 || f[0] != "src/x.js" || f[1] != "src/y.js" {
		t.Fatalf("changedFiles=%v", f)
	}
}

func TestParseFindings(t *testing.T) {

	raw := "Here are the issues:\n```json\n" +
		`[{"title":"SQL injection","file":"db.go:10","severity":"high","detail":"unsanitized"},` +
		`{"title":"no timeout","file":"http.go:5","severity":"low","detail":"add ctx"}]` +
		"\n```\n"
	fs := parseFindings(raw)
	if len(fs) != 2 || fs[0].Title != "SQL injection" || fs[1].Severity != "low" {
		t.Fatalf("parse: %+v", fs)
	}
	if parseFindings("no json here") != nil {
		t.Fatal("non-json should parse to nil")
	}
	if len(parseFindings("[]")) != 0 {
		t.Fatal("empty array → no findings")
	}
}

func TestParseFindingsCapturesCategoryAndConfidence(t *testing.T) {
	raw := `[{"title":"SQL injection","file":"db.go:10","severity":"high","category":"security","confidence":"high","detail":"unsanitized"}]`
	fs := parseFindings(raw)
	if len(fs) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(fs))
	}
	if fs[0].Category != "security" || fs[0].Confidence != "high" {
		t.Fatalf("category/confidence not parsed: %+v", fs[0])
	}
}

func TestDetectTestCmd(t *testing.T) {
	node := t.TempDir()
	os.WriteFile(filepath.Join(node, "package.json"), []byte("{}"), 0o644)
	if name, _, ok := detectTestCmd(node); !ok || name != "npm" {
		t.Fatalf("npm: %s ok=%v", name, ok)
	}
	golang := t.TempDir()
	os.WriteFile(filepath.Join(golang, "go.mod"), []byte("module x"), 0o644)
	if name, _, ok := detectTestCmd(golang); !ok || name != "go" {
		t.Fatalf("go: %s ok=%v", name, ok)
	}
	if _, _, ok := detectTestCmd(t.TempDir()); ok {
		t.Fatal("empty dir should have no runner")
	}
}
