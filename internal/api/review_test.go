package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReviewAcceptAndReject(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	nid, _ := s.AddNode(ctx, run, "implement", nil)
	// node output with two files
	s.Pool().Exec(ctx, `UPDATE nodes SET status='done', output='{"summary":"x","files":[{"path":"a.js","content":"1","action":"create"},{"path":"b.js","content":"2","action":"create"}]}' WHERE id=$1`, nid)

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	// accept a.js
	post(t, srv.URL+"/api/runs/"+run.String()+"/review", `{"path":"a.js","status":"accepted"}`, 200)
	// reject b.js
	post(t, srv.URL+"/api/runs/"+run.String()+"/review", `{"path":"b.js","status":"rejected"}`, 200)

	// detail: a.js present + accepted, b.js gone
	var d RunDetail
	resp, _ := http.Get(srv.URL + "/api/runs/" + run.String())
	json.NewDecoder(resp.Body).Decode(&d)
	var aReview string
	var hasB bool
	for _, f := range d.Files {
		if f.Path == "a.js" {
			aReview = f.Review
		}
		if f.Path == "b.js" {
			hasB = true
		}
	}
	if aReview != "accepted" {
		t.Fatalf("a.js should be accepted, got %q", aReview)
	}
	if hasB {
		t.Fatal("b.js was rejected and should be removed from the file list")
	}
}

func TestReviewValidates(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(context.Background(), "acme")
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	post(t, srv.URL+"/api/runs/"+run.String()+"/review", `{"path":"a.js","status":"maybe"}`, 400)
}

func post(t *testing.T, url, body string, want int) {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("POST %s: want %d, got %d", url, want, resp.StatusCode)
	}
}
