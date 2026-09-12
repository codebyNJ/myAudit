package preview

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, rel, body string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectCLIProject(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"name":"cli","scripts":{"start":"node index.js","test":"node --test tests/t.js"}}`)
	write(t, dir, "index.js", "console.log('hi')")
	if got := Detect(dir); got != KindNone {
		t.Fatalf("cli project: got %q, want none", got)
	}
}

func TestDetectWebReact(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"dependencies":{"react":"^19","react-dom":"^19"},"scripts":{"dev":"vite"}}`)
	write(t, dir, "node_modules/.keep", "")
	if got := Detect(dir); got != KindWeb {
		t.Fatalf("react project: got %q, want web", got)
	}
}

func TestDetectWebByIndexHTML(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", "<!doctype html><div id=root></div>")
	if got := Detect(dir); got != KindWeb {
		t.Fatalf("index.html: got %q, want web", got)
	}
}

func TestDetectDesktopTauri(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "src-tauri/tauri.conf.json", `{}`)
	if got := Detect(dir); got != KindDesktop {
		t.Fatalf("tauri: got %q, want desktop", got)
	}
}

func TestCommandSkipsNonUI(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"scripts":{"dev":"vite"}}`)
	write(t, dir, "node_modules/.keep", "")
	if _, ok := Command(dir); ok {
		t.Fatal("Command should refuse when Detect is none")
	}
}

func TestCommandFindsWebDev(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"scripts":{"dev":"vite"},"dependencies":{"react":"^19"}}`)
	write(t, dir, "node_modules/.keep", "")
	l, ok := Command(dir)
	if !ok || l.Name != "npm" || len(l.Args) != 2 || l.Args[1] != "dev" {
		t.Fatalf("Command() = %+v ok=%v, want npm run dev", l, ok)
	}
}
