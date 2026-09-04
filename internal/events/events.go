package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
)

// Event is a single typed, correlated log entry. Kind comes from a fixed
// vocabulary (run.start, node.start, understand.done, finding, ...).
type Event struct {
	RunID  uuid.UUID
	NodeID *uuid.UUID
	SpanID string
	Level  string
	Kind   string
	Msg    string
	Attrs  map[string]any
}

// Logger writes typed events to SQLite and mirrors them to slog.
type Logger struct {
	db *sql.DB
}

// New builds a Logger over the given DB handle.
func New(db *sql.DB) *Logger { return &Logger{db: db} }

// Log persists one event. Insert failures never block the caller — they are
// logged and swallowed.
func (l *Logger) Log(ctx context.Context, e Event) {
	if e.Level == "" {
		e.Level = "info"
	}
	attrs, _ := json.Marshal(e.Attrs)
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO events(run_id, node_id, span_id, level, kind, msg, attrs)
		 VALUES(?,?,?,?,?,?,?)`,
		e.RunID, nodeArg(e.NodeID), nullStr(e.SpanID), e.Level, e.Kind, e.Msg, string(attrs))
	if err != nil {
		slog.Error("event insert failed", "err", err)
	}
	slog.Info(e.Kind, "run", e.RunID, "msg", e.Msg)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nodeArg passes a nullable node id to SQL: nil pointer → NULL.
func nodeArg(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return p.String()
}
