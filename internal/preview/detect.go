package preview

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Kind string

const (
	KindWeb     Kind = "web"
	KindDesktop Kind = "desktop"
	KindNone    Kind = "none"
)

var uiExt = map[string]bool{
	".tsx": true, ".jsx": true, ".vue": true, ".svelte": true, ".html": true,
}

var webDeps = []string{
	"react", "react-dom", "vue", "svelte", "@angular/core", "next", "nuxt",
	"@remix-run/react", "expo", "@tauri-apps/api",
}

var webConfigs = []string{
	"vite.config.js", "vite.config.ts", "vite.config.mjs",
	"next.config.js", "next.config.mjs", "next.config.ts",
	"nuxt.config.ts", "nuxt.config.js", "angular.json", "svelte.config.js",
}

var uiDirs = []string{"web", "ui", "frontend", "client", "app", "src", "pages"}

func Detect(dir string) Kind {
	if fileExists(filepath.Join(dir, "src-tauri", "tauri.conf.json")) ||
		fileExists(filepath.Join(dir, "tauri.conf.json")) {
		return KindDesktop
	}
	if pkg, ok := readPackageJSON(dir); ok {
		if hasDep(pkg, "electron") {
			return KindDesktop
		}
		for _, d := range webDeps {
			if hasDep(pkg, d) {
				return KindWeb
			}
		}
	}
	for _, name := range webConfigs {
		if fileExists(filepath.Join(dir, name)) {
			return KindWeb
		}
	}
	for _, rel := range []string{"index.html", "public/index.html", "static/index.html", "web/index.html"} {
		if fileExists(filepath.Join(dir, rel)) {
			return KindWeb
		}
	}
	for _, d := range uiDirs {
		if hasUIFiles(filepath.Join(dir, d)) {
			return KindWeb
		}
	}
	return KindNone
}

func readPackageJSON(dir string) (map[string]any, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return nil, false
	}
	return m, true
}

func hasDep(pkg map[string]any, name string) bool {
	for _, key := range []string{"dependencies", "devDependencies", "peerDependencies"} {
		deps, _ := pkg[key].(map[string]any)
		if deps == nil {
			continue
		}
		if _, ok := deps[name]; ok {
			return true
		}
	}
	return false
}

func hasUIFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == "node_modules" || base == "dist" || base == "build" || base == ".git" {
				return filepath.SkipDir
			}

			rel, _ := filepath.Rel(dir, p)
			if strings.Count(rel, string(os.PathSeparator)) > 4 {
				return filepath.SkipDir
			}
			return nil
		}
		if uiExt[strings.ToLower(filepath.Ext(p))] {
			found = true
		}
		return nil
	})
	return found
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
