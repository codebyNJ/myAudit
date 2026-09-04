package events

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event is a single typed, correlated log entry. Kind comes from a fixed
// vocabulary (run.start, node.start, claude.request, gate.red, ...).
type Event struct {
	RunID  uuid.UUID
	NodeID *uuid.UUID
	SpanID string
	Level  string
	Kind   string
	Msg    string
	Attrs  map[string]any
}

// Logger writes typed events to Postgres and mirrors them to slog.
type Logger struct {
	pool *pgxpool.Pool
}

// New builds a Logger over the given pool.
func New(p *pgxpool.Pool) *Logger { return &Logger{pool: p} }

// Log persists one event. Insert failures never block the caller — they are
// logged and swallowed.
func (l *Logger) Log(ctx context.Context, e Event) {
	if e.Level == "" {
		e.Level = "info"
	}
	attrs, _ := json.Marshal(e.Attrs)
	_, err := l.pool.Exec(ctx,
		`INSERT INTO events(run_id, node_id, span_id, level, kind, msg, attrs)
		 VALUES($1,$2,$3,$4,$5,$6,$7)`,
		e.RunID, e.NodeID, nullStr(e.SpanID), e.Level, e.Kind, e.Msg, attrs)
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
