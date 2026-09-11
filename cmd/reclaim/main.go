package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/codebyNJ/myAudit/internal/sandbox"
)

func main() {
	root := "runs"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no run workspaces:", err)
		os.Exit(1)
	}

	var total int64
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		freed, rerr := sandbox.Reclaim(dir)
		if rerr != nil {
			fmt.Printf("  %s: %v\n", e.Name(), rerr)
			continue
		}
		if freed > 0 {
			fmt.Printf("  %s: freed %.0f MB\n", e.Name()[:8], float64(freed)/(1<<20))
		}
		total += freed
	}
	fmt.Printf("reclaimed %.2f GB\n", float64(total)/(1<<30))
}
