package worker

import "testing"

func TestParseFlowsPlainJSON(t *testing.T) {
	doc, err := parseFlows(`{"persistence":"none — static JSON","data_flows":[{"name":"Playlist","store":"static file","steps":[{"label":"load","file":"lib/p.ts:3","kind":"source"}]}],"product_flows":[{"name":"Play","steps":[{"label":"click","file":"app/page.tsx:9"}]}]}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Persistence == "" || len(doc.DataFlows) != 1 || len(doc.ProductFlows) != 1 {
		t.Fatalf("bad parse: %+v", doc)
	}
	if doc.DataFlows[0].Steps[0].File != "lib/p.ts:3" {
		t.Fatalf("step file lost: %+v", doc.DataFlows[0])
	}
}

func TestParseFlowsFencedAndProse(t *testing.T) {
	in := "Here is the map:\n```json\n{\"data_flows\":[{\"name\":\"A\"}]}\n```\nHope that helps."
	doc, err := parseFlows(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.DataFlows) != 1 || doc.DataFlows[0].Name != "A" {
		t.Fatalf("bad parse: %+v", doc)
	}
}

func TestParseFlowsRejectsNonJSON(t *testing.T) {
	if _, err := parseFlows("[stub] node skipped (dev mode)"); err == nil {
		t.Fatal("expected an error for non-JSON agent output")
	}
}

func TestParseFlowsRejectsEmptyDoc(t *testing.T) {
	if _, err := parseFlows(`{"persistence":"sqlite"}`); err == nil {
		t.Fatal("expected an error when no flows were identified")
	}
}
