package worker

import "testing"

func TestParseFindingsIgnoresProseWithBrackets(t *testing.T) {
	// A greedy first-"[" to last-"]" match spans from the citation bracket all
	// the way to the array, parses as nothing, and drops the findings silently.
	summary := "I checked the module [see notes above] and found issues.\n" +
		`[{"title":"SQL injection in search","file":"db.go:42","severity":"high","detail":"user input concatenated"}]`

	got := parseFindings(summary)
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d (%+v)", len(got), got)
	}
	if got[0].Title != "SQL injection in search" || got[0].Severity != "high" {
		t.Fatalf("wrong finding parsed: %+v", got[0])
	}
}

func TestParseFindingsHandlesNestedArrays(t *testing.T) {
	summary := `Findings: [{"title":"a","detail":"uses [1,2,3] as a key"},{"title":"b"}]`
	got := parseFindings(summary)
	if len(got) != 2 {
		t.Fatalf("want 2 findings, got %d (%+v)", len(got), got)
	}
}

func TestParseFindingsEmptyArray(t *testing.T) {
	if got := parseFindings("The module is clean.\n[]"); len(got) != 0 {
		t.Fatalf("want no findings, got %+v", got)
	}
}

func TestParseFindingsNoArray(t *testing.T) {
	if got := parseFindings("I could not finish the review."); got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}

func TestParseFindingsSkipsUntitled(t *testing.T) {
	got := parseFindings(`[{"title":"real"},{"title":"  "},{"detail":"no title"}]`)
	if len(got) != 1 || got[0].Title != "real" {
		t.Fatalf("untitled findings should be dropped, got %+v", got)
	}
}
