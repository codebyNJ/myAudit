package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

func registerExport(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/runs/{id}/findings.json", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		cards, err := s.NodeDetailsForRun(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		type finding struct {
			Title string `json:"title"`
			File  string `json:"file,omitempty"`
			// Class distinguishes a defect from a suggestion; without it a
			// consumer of this file cannot tell a naming nit from an injection.
			Class      string   `json:"class,omitempty"`
			Severity   string   `json:"severity"`
			Priority   string   `json:"priority"`
			Confidence string   `json:"confidence,omitempty"`
			Category   string   `json:"category,omitempty"`
			Status     string   `json:"status"`
			Detail     string   `json:"detail,omitempty"`
			Tags       []string `json:"tags"`
			// Where the work for this finding lives. Without these the export
			// says what is wrong but not what was done about it.
			CommitSHA string `json:"commit_sha,omitempty"`
			Branch    string `json:"branch,omitempty"`
			PRURL     string `json:"pr_url,omitempty"`
		}
		out := []finding{}
		for _, c := range cards {
			if c.Type != "bug" {
				continue
			}
			out = append(out, finding{c.Title, c.File, c.Class, c.Severity, c.Priority, c.Confidence, c.Category, c.Status, c.Detail, c.Tags, c.CommitSHA, c.Branch, c.PRURL})
		}
		w.Header().Set("content-type", "application/json")
		w.Header().Set("content-disposition", `attachment; filename="findings.json"`)
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	})

	mux.HandleFunc("GET /api/runs/{id}/report.md", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		cards, _ := s.NodeDetailsForRun(r.Context(), id)
		run, _ := s.GetRun(r.Context(), id)
		cost, _ := s.RunCostUSD(r.Context(), id)
		notes, _ := s.GetNotes(r.Context(), id)

		w.Header().Set("content-type", "text/markdown; charset=utf-8")
		w.Header().Set("content-disposition", `attachment; filename="report.md"`)
		_, _ = w.Write([]byte(buildReport(run, cards, cost, notes)))
	})

	mux.HandleFunc("GET /api/runs/{id}/patch.diff", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		ws := sandbox.Workspace{Dir: filepath.Join("runs", id.String())}
		diff, err := ws.DiffFromBaseline(r.Context(), "")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("content-type", "text/x-diff; charset=utf-8")
		w.Header().Set("content-disposition", `attachment; filename="fixes.patch"`)
		_, _ = w.Write([]byte(diff))
	})
}

func buildReport(run store.RunSummary, cards []store.NodeDetail, cost float64, notes string) string {
	var bugs []store.NodeDetail
	counts := map[string]int{}
	for _, c := range cards {
		if c.Type == "bug" {
			bugs = append(bugs, c)
			counts[c.Status]++
		}
	}
	var b strings.Builder
	title := run.Project
	if title == "" {
		title = run.ID.String()
	}
	fmt.Fprintf(&b, "# Audit report — %s\n\n", title)
	fmt.Fprintf(&b, "- Run: `%s`\n", run.ID)
	if run.Status != "" {
		fmt.Fprintf(&b, "- Status: %s\n", run.Status)
	}
	fmt.Fprintf(&b, "- Model cost: $%.2f\n", cost)
	fmt.Fprintf(&b, "- Findings: %d (%d fixed, %d in review, %d failed, %d open, %d dismissed)\n\n",
		len(bugs), counts["done"], counts["in_review"], counts["failed"], counts["open"], counts["dismissed"])

	if len(bugs) > 0 {
		b.WriteString("## Findings\n\n| Class | Severity | Priority | Status | Title | File |\n|---|---|---|---|---|---|\n")
		for _, f := range bugs {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
				dash(classOf(f)), dash(f.Severity), dash(f.Priority), dash(f.Status), mdCell(f.Title), mdCell(f.File))
		}
		b.WriteString("\n## Details\n\n")
		for _, f := range bugs {
			fmt.Fprintf(&b, "### %s — %s / %s (%s)\n", f.Title, dash(f.Severity), dash(f.Priority), dash(f.Status))
			if f.File != "" {
				fmt.Fprintf(&b, "`%s`\n\n", f.File)
			}
			if strings.TrimSpace(f.Detail) != "" {
				b.WriteString(f.Detail + "\n\n")
			}
			if git := gitTrail(f); git != "" {
				b.WriteString(git + "\n\n")
			}
		}
	}
	if strings.TrimSpace(notes) != "" {
		b.WriteString("## Notes log\n\n" + notes + "\n")
	}
	return b.String()
}

// classOf defaults to "bug" so findings filed before classification still read
// correctly in a report rather than showing a blank column.
func classOf(f store.NodeDetail) string {
	if f.Class == "" {
		return "bug"
	}
	return f.Class
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func mdCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", "\\|")
}

// gitTrail renders what was done about a finding: the commit, the branch it is
// labelled with, and the PR if one was opened.
func gitTrail(f store.NodeDetail) string {
	var parts []string
	if f.CommitSHA != "" {
		parts = append(parts, "commit `"+shortSHA(f.CommitSHA)+"`")
	}
	if f.Branch != "" {
		parts = append(parts, "branch `"+f.Branch+"`")
	}
	if f.PRURL != "" {
		parts = append(parts, "PR "+f.PRURL)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " · ")
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
