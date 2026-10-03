package api

import "testing"

func TestIsFixIntentIsTheBareCommand(t *testing.T) {
	for _, m := range []string{"fix", "Fix", "  fix all  "} {
		if !isFixIntent(m) {
			t.Errorf("%q should queue the open tickets", m)
		}
	}
	for _, m := range []string{"fix the typo in README", "fixtures — where are they?", "fixed?", "fix this one function"} {
		if isFixIntent(m) {
			t.Errorf("%q is a request for the chat agent, not a command to queue every ticket", m)
		}
	}
}
