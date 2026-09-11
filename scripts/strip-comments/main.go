package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
)

func isGoDirective(group *ast.CommentGroup) bool {
	if group == nil {
		return false
	}
	for _, c := range group.List {
		text := strings.TrimSpace(c.Text)
		if strings.HasPrefix(text, "//go:") || strings.HasPrefix(text, "// +build") {
			return true
		}
	}
	return false
}

func clearComments(n ast.Node) {
	ast.Inspect(n, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.File:
			kept := make([]*ast.CommentGroup, 0, len(x.Comments))
			for _, g := range x.Comments {
				if isGoDirective(g) {
					kept = append(kept, g)
				}
			}
			x.Comments = kept
			if !isGoDirective(x.Doc) {
				x.Doc = nil
			}
		case *ast.GenDecl:
			if !isGoDirective(x.Doc) {
				x.Doc = nil
			}
		case *ast.FuncDecl:
			x.Doc = nil
		case *ast.TypeSpec:
			x.Doc = nil
			x.Comment = nil
		case *ast.ValueSpec:
			if !isGoDirective(x.Doc) {
				x.Doc = nil
			}
			x.Comment = nil
		case *ast.Field:
			x.Doc = nil
			x.Comment = nil
		case *ast.ImportSpec:
			x.Doc = nil
			x.Comment = nil
		}
		return true
	})
}

func stripFile(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	clearComments(f)
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.TabIndent | printer.UseSpaces, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, f); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	out := buf.Bytes()
	if !bytes.HasSuffix(out, []byte("\n")) {
		out = append(out, '\n')
	}
	return os.WriteFile(path, out, 0)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run . <file.go> ...")
		os.Exit(1)
	}
	var failed bool
	for _, path := range os.Args[1:] {
		if err := stripFile(path); err != nil {
			fmt.Fprintln(os.Stderr, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
