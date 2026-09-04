package api

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"myintern/internal/store"
)

// skipTreeDir are directories excluded from the workspace file tree.
var skipTreeDir = map[string]bool{".git": true, "node_modules": true, "dist": true, "build": true, ".next": true}

// listWorkspaceFiles walks a run's on-disk workspace and returns every file
// (scaffold + generated), sorted by path, flagging feature-changed files and
// carrying review status. Rejected files are omitted. Content is not read here.
func listWorkspaceFiles(runID string, changed map[string]bool, reviews map[string]string) []store.FileEntry {
	root := filepath.Join("runs", runID)
	out := []store.FileEntry{}
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if p != root && skipTreeDir[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || reviews[rel] == "rejected" {
			return nil
		}
		out = append(out, store.FileEntry{Path: rel, Changed: changed[rel], Review: reviews[rel]})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Schema + OpenAPI views. The template baseline is fixed; a run's own resources
// are layered on so the Schema/Swagger tabs reflect what was actually generated.

type schemaField struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	Ref  string `json:"ref,omitempty"`
}
type schemaEntity struct {
	Name   string        `json:"name"`
	Fields []schemaField `json:"fields"`
}
type apiRoute struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Summary string `json:"summary"`
}

func baselineSchema() []schemaEntity {
	return []schemaEntity{
		{"users", []schemaField{{Name: "_id", Type: "ObjectId"}, {Name: "email", Type: "unique"}, {Name: "passwordHash"}, {Name: "currentWorkspaceId", Ref: "workspaces"}}},
		{"workspaces", []schemaField{{Name: "_id", Type: "ObjectId"}, {Name: "name"}, {Name: "ownerId", Ref: "users"}}},
		{"workspace_members", []schemaField{{Name: "userId", Ref: "users"}, {Name: "workspaceId", Ref: "workspaces"}, {Name: "role", Type: "ADMIN|VIEWER"}}},
		{"items", []schemaField{{Name: "_id", Type: "ObjectId"}, {Name: "workspaceId", Ref: "workspaces"}, {Name: "name"}}},
	}
}

func baselineOpenAPI() []apiRoute {
	return []apiRoute{
		{"POST", "/v1/auth/login", "login"},
		{"POST", "/v1/auth/reset", "reset password"},
		{"GET", "/v1/me", "current user"},
		{"POST", "/v1/workspace", "create workspace"},
		{"GET", "/v1/item", "list items (workspace-scoped)"},
		{"DELETE", "/v1/workspace/{id}", "delete workspace"},
	}
}

// schemaWith appends one entity per resource (workspace-scoped, with its fields).
func schemaWith(resources []store.Resource) []schemaEntity {
	out := baselineSchema()
	for _, r := range resources {
		fields := []schemaField{{Name: "_id", Type: "ObjectId"}, {Name: "workspaceId", Ref: "workspaces"}}
		for _, f := range r.Fields {
			fields = append(fields, schemaField{Name: f.Name, Type: f.Type})
		}
		out = append(out, schemaEntity{Name: strings.ToLower(r.Name) + "s", Fields: fields})
	}
	return out
}

// openapiWith appends the standard CRUD routes for each resource.
func openapiWith(resources []store.Resource) []apiRoute {
	out := baselineOpenAPI()
	for _, r := range resources {
		p := "/v1/" + strings.ToLower(r.Name)
		lower := strings.ToLower(r.Name)
		out = append(out,
			apiRoute{"GET", p, "list " + lower + "s (workspace-scoped)"},
			apiRoute{"POST", p, "create " + lower},
			apiRoute{"GET", p + "/{id}", "get " + lower},
			apiRoute{"PATCH", p + "/{id}", "update " + lower},
			apiRoute{"DELETE", p + "/{id}", "delete " + lower},
		)
	}
	return out
}
