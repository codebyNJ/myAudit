package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
)

type Event struct {
	RunID  uuid.UUID
	NodeID *uuid.UUID
	SpanID string
	Level  string
	Kind   string
	Msg    string
	Attrs  map[string]any
}

type Logger struct {
	db *sql.DB
}

func New(db *sql.DB) *Logger { return &Logger{db: db} }

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

func nodeArg(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return p.String()
}
