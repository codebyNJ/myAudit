package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// localOnly refuses any request that is not from this machine's own UI.
//
// The API drives agents that have a shell, so reaching it is reaching the
// host. A Host header that is not loopback is another machine on the network
// or a DNS-rebinding page; an Origin that is not loopback is another website's
// script. Browsers send Origin on every cross-origin request and every POST —
// including the text/plain kind that skips the CORS preflight — so checking it
// is what stops a page the user merely visits from starting a run.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackHost(r.Host) {
			http.Error(w, "myAudit only answers requests addressed to localhost", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			if u, err := url.Parse(o); err != nil || !loopbackHost(u.Host) {
				http.Error(w, "cross-origin request refused", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// loopbackHost reports whether a Host-style "host[:port]" names this machine.
func loopbackHost(hostport string) bool {
	h := hostport
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		h = host
	}
	return loopbackOnly(strings.Trim(h, "[]"))
}
