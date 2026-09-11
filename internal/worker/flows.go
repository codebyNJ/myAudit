package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"myaudit/internal/agent"
	"myaudit/internal/queue"
	"myaudit/internal/sandbox"
)

const flowsTimeout = 12 * time.Minute

// FlowStep is one hop in a flow, optionally anchored to a file:line so the UI
// can jump straight to the code.
type FlowStep struct {
	Label string `json:"label"`
	File  string `json:"file,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

// DataFlow describes how one piece of state is produced, moved and persisted.
type DataFlow struct {
	Name    string     `json:"name"`
	Entity  string     `json:"entity,omitempty"`
	Store   string     `json:"store,omitempty"`
	Steps   []FlowStep `json:"steps,omitempty"`
	Note    string     `json:"note,omitempty"`
	Concern string     `json:"concern,omitempty"`
}

// ProductFlow describes one user-facing journey through the product.
type ProductFlow struct {
	Name    string     `json:"name"`
	Trigger string     `json:"trigger,omitempty"`
	Steps   []FlowStep `json:"steps,omitempty"`
	Outcome string     `json:"outcome,omitempty"`
	Concern string     `json:"concern,omitempty"`
}

// FlowsDoc is the persisted result of the flows node.
type FlowsDoc struct {
	Persistence  string        `json:"persistence,omitempty"`
	DataFlows    []DataFlow    `json:"data_flows,omitempty"`
	ProductFlows []ProductFlow `json:"product_flows,omitempty"`
}

func (d Deps) doFlows(ctx context.Context, c *queue.ClaimedNode, ws sandbox.Workspace) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, flowsTimeout)
	defer cancel()

	// Routed through runAgent (like qa/bug) so an agent-level failure (r.OK
	// false, or an infra err) gets the same bounded retry-then-checkpoint
	// treatment instead of silently falling through to parseFlows("") and
	// reporting a misleading "unexpected end of JSON input".
	r, ok := d.runAgent(ctx, c, ws, flowsTask, agent.ReadOnly)
	if !ok {
		return true, nil
	}

	doc, perr := parseFlows(r.Summary)
	if perr != nil {
		snippet := strings.TrimSpace(r.Summary)
		if len(snippet) > 240 {
			snippet = snippet[:240] + "…"
		}
		d.fail(ctx, c, "flows parse: "+perr.Error()+" | agent said: "+snippet)
		return true, nil
	}

	raw, _ := json.Marshal(doc)
	d.complete(ctx, c, nodeOutput{
		Kind:    "flows",
		Summary: fmt.Sprintf("%d data flow(s), %d product flow(s)", len(doc.DataFlows), len(doc.ProductFlows)),
		CostUSD: r.CostUSD,
		Tokens:  r.Tokens,
		Flows:   raw,
	})
	return true, nil
}

// parseFlows tolerates the fenced / prose-wrapped JSON agents sometimes emit.
func parseFlows(s string) (FlowsDoc, error) {
	var doc FlowsDoc
	txt := strings.TrimSpace(s)
	if i := strings.Index(txt, "```"); i >= 0 {
		rest := txt[i+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if j := strings.Index(rest, "```"); j >= 0 {
			rest = rest[:j]
		}
		txt = strings.TrimSpace(rest)
	}
	if i := strings.IndexByte(txt, '{'); i > 0 {
		txt = txt[i:]
	}
	if j := strings.LastIndexByte(txt, '}'); j >= 0 && j < len(txt)-1 {
		txt = txt[:j+1]
	}
	if err := json.Unmarshal([]byte(txt), &doc); err != nil {
		return doc, err
	}
	if len(doc.DataFlows) == 0 && len(doc.ProductFlows) == 0 {
		return doc, fmt.Errorf("no flows identified")
	}
	return doc, nil
}

const flowsTask = "Map how this product actually works, for an engineer who has never seen it.\n\n" +
	"1. DATA / DATABASE FLOWS — for every meaningful piece of state: where it originates, how it is " +
	"transformed, and where it is persisted or read back (DB table, ORM model, API route, cache, " +
	"localStorage, file, in-memory store, external service). Name the real storage technology. If the " +
	"product has no database, say so explicitly in `persistence` and describe what it uses instead.\n" +
	"2. PRODUCT FLOWS — the main user-facing journeys end to end: what the user does, which components / " +
	"handlers / hooks run in order, and what the user ends up with.\n\n" +
	"Read the code to confirm each step; do not guess. Anchor steps to real `path:line` locations. " +
	"Do NOT modify any files.\n\n" +
	"Your FINAL message must be ONLY JSON (no prose, no fences):\n" +
	`{"persistence":"<one line: the storage tech, or 'none — <what it uses>'>",` +
	`"data_flows":[{"name":"<short>","entity":"<table/model/key>","store":"<where it lives>",` +
	`"steps":[{"label":"<what happens>","file":"<path:line>","kind":"source|transform|store|read"}],` +
	`"note":"<optional>","concern":"<optional risk you noticed>"}],` +
	`"product_flows":[{"name":"<short>","trigger":"<what starts it>",` +
	`"steps":[{"label":"<what happens>","file":"<path:line>"}],` +
	`"outcome":"<what the user gets>","concern":"<optional>"}]}` +
	"\nMax 6 data flows and 6 product flows, most important first."
