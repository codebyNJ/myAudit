package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"myaudit/internal/store"
)

// registerExport adds downloadable audit artifacts: a human report (report.md)
// and machine-readable findings (findings.json), so results can leave the tool
// into a tracker / PR / CI.
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
			Title    string   `json:"title"`
			File     string   `json:"file,omitempty"`
			Severity string   `json:"severity"`
			Priority string   `json:"priority"`
			Status   string   `json:"status"`
			Detail   string   `json:"detail,omitempty"`
			Tags     []string `json:"tags"`
		}
		out := []finding{}
		for _, c := range cards {
			if c.Type != "bug" {
				continue
			}
			out = append(out, finding{c.Title, c.File, c.Severity, c.Priority, c.Status, c.Detail, c.Tags})
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
}

// buildReport renders a self-contained markdown audit report: header, a findings
// table + details, and the running notes log.
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
		b.WriteString("## Findings\n\n| Severity | Priority | Status | Title | File |\n|---|---|---|---|---|\n")
		for _, f := range bugs {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
				dash(f.Severity), dash(f.Priority), dash(f.Status), mdCell(f.Title), mdCell(f.File))
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
		}
	}
	if strings.TrimSpace(notes) != "" {
		b.WriteString("## Notes log\n\n" + notes + "\n")
	}
	return b.String()
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// mdCell keeps a value safe inside a markdown table cell (no raw pipes/newlines).
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", "\\|")
}
