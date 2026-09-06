-- myAudit schema (SQLite). Applied idempotently on store.Open — no migration
-- tool: this is a single-file local DB, CREATE TABLE IF NOT EXISTS is enough.
-- UUIDs are TEXT (generated in Go via google/uuid). Timestamps are declared
-- TIMESTAMP so the modernc driver scans them back into time.Time. JSON columns
-- (deps, input_snapshot, output, attrs, options, settings.data) are TEXT holding
-- JSON, read with SQLite's json_extract/json_each.

CREATE TABLE IF NOT EXISTS runs (
  id         TEXT PRIMARY KEY,
  project    TEXT NOT NULL,
  status     TEXT NOT NULL DEFAULT 'running',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS nodes (
  id             TEXT PRIMARY KEY,
  run_id         TEXT NOT NULL REFERENCES runs(id),
  type           TEXT NOT NULL,
  status         TEXT NOT NULL DEFAULT 'pending',
  deps           TEXT NOT NULL DEFAULT '[]',   -- JSON array of node ids
  input_snapshot TEXT,                          -- JSON
  output         TEXT,                          -- JSON
  attempts       INTEGER NOT NULL DEFAULT 0,
  claimed_at     TIMESTAMP,
  created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_nodes_run_status ON nodes(run_id, status);

CREATE TABLE IF NOT EXISTS events (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id  TEXT NOT NULL,
  node_id TEXT,
  span_id TEXT,
  ts      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  level   TEXT NOT NULL,
  kind    TEXT NOT NULL,
  msg     TEXT,
  attrs   TEXT
);
CREATE INDEX IF NOT EXISTS idx_events_run ON events(run_id, ts);

CREATE TABLE IF NOT EXISTS checkpoints (
  id          TEXT PRIMARY KEY,
  run_id      TEXT NOT NULL REFERENCES runs(id),
  node_id     TEXT NOT NULL REFERENCES nodes(id),
  question    TEXT NOT NULL,
  options     TEXT,
  answer      TEXT,
  resolved    INTEGER NOT NULL DEFAULT 0,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_checkpoints_open ON checkpoints(node_id) WHERE resolved = 0;

CREATE TABLE IF NOT EXISTS notes (
  run_id     TEXT PRIMARY KEY REFERENCES runs(id),
  content    TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- single-row app settings (id is always 1)
CREATE TABLE IF NOT EXISTS settings (
  id         INTEGER PRIMARY KEY CHECK (id = 1),
  data       TEXT NOT NULL DEFAULT '{}',
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT OR IGNORE INTO settings(id, data)
  VALUES (1, '{"model_tier":"haiku-4.5","token_budget":100000,"theme":"dark"}');

CREATE TABLE IF NOT EXISTS file_reviews (
  run_id     TEXT NOT NULL REFERENCES runs(id),
  path       TEXT NOT NULL,
  status     TEXT NOT NULL,               -- accepted | rejected
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (run_id, path)
);
