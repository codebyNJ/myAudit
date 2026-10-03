package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalOnly(t *testing.T) {
	s := newStore(t)
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	cases := []struct {
		name, host, origin string
		want               int
	}{
		{"the UI itself", "", "", 200},
		{"vite dev server proxy", "", "http://localhost:5173", 200},
		{"ipv6 loopback origin", "", "http://[::1]:7788", 200},
		{"another website", "", "https://attacker.example", 403},
		{"opaque origin", "", "null", 403},
		{"dns rebinding", "attacker.example:7788", "", 403},
		{"lan address", "192.168.1.20:7788", "", 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", srv.URL+"/api/runs", nil)
			if tc.host != "" {
				req.Host = tc.host
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("want %d, got %d", tc.want, resp.StatusCode)
			}
		})
	}
}

// The attack that worked: a page posts text/plain JSON, which needs no CORS
// preflight, and the handler never looked at the content type.
func TestCrossOriginSimplePostCannotStartARun(t *testing.T) {
	s := newStore(t)
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	body := `{"repo_path":"` + t.TempDir() + `"}`
	req, _ := http.NewRequest("POST", srv.URL+"/api/runs", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	req.Header.Set("Origin", "https://attacker.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
	if runs, _ := s.ListRuns(req.Context(), 10); len(runs) != 0 {
		t.Fatalf("a run was created: %v", runs)
	}
}
