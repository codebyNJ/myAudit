package api

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestReviewAcceptAndReject(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	postJSON(t, srv.URL+"/api/runs/"+run.String()+"/review", `{"path":"a.js","status":"accepted"}`, 200)
	postJSON(t, srv.URL+"/api/runs/"+run.String()+"/review", `{"path":"b.js","status":"rejected"}`, 200)

	reviews, err := s.Reviews(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if reviews["a.js"] != "accepted" || reviews["b.js"] != "rejected" {
		t.Fatalf("reviews not persisted: %+v", reviews)
	}
}

func TestReviewValidates(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(context.Background(), "acme")
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	postJSON(t, srv.URL+"/api/runs/"+run.String()+"/review", `{"path":"a.js","status":"maybe"}`, 400)
}
