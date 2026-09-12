package agent

// Result is the provider-neutral outcome of one agent invocation.
type Result struct {
	OK      bool
	Summary string
	CostUSD float64
	Tokens  int
	Err     string
}

// Mode controls how much tool access the agent has for a node.
type Mode int

const (
	ReadOnly Mode = iota
	Live
)

const (
	ProviderClaude   = "claude"
	ProviderOpenCode = "opencode"
)

var ReadOnlyAllow = []string{
	"Read", "Glob", "Grep",
}

var ReadOnlyDeny = []string{
	"Write", "Edit", "MultiEdit", "NotebookEdit", "Bash", "WebFetch", "WebSearch",
}

var LiveAllow = []string{
	"Read", "Glob", "Grep", "Write", "Edit", "MultiEdit", "Bash",
}

var LiveDeny = []string{
	"WebFetch", "WebSearch",
}

func PolicyFor(m Mode) (allow, deny []string) {
	if m == Live {
		return LiveAllow, LiveDeny
	}
	return ReadOnlyAllow, ReadOnlyDeny
}
