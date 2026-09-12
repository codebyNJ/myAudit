package preview

import "testing"

func TestCommandNodeUnchanged(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"scripts":{"dev":"vite"},"dependencies":{"react":"^19"}}`)
	write(t, dir, "node_modules/.keep", "")
	l, ok := Command(dir)
	if !ok || l.Name != "npm" || len(l.Args) != 2 || l.Args[1] != "dev" || l.Static {
		t.Fatalf("Command() = %+v ok=%v, want npm run dev", l, ok)
	}
}

func TestCommandStaticFallback(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", "<!doctype html><div id=root></div>")
	l, ok := Command(dir)
	if !ok || !l.Static {
		t.Fatalf("Command() = %+v ok=%v, want static launcher", l, ok)
	}
}

func TestCommandNoneWhenDetectNone(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "README.md", "just docs\n")
	if _, ok := Command(dir); ok {
		t.Fatal("Command should refuse when Detect is none")
	}
}
