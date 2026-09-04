-- +goose Up
CREATE TABLE runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'running',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE nodes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID NOT NULL REFERENCES runs(id),
  type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  deps UUID[] NOT NULL DEFAULT '{}',
  input_snapshot JSONB,
  output JSONB,
  attempts INT NOT NULL DEFAULT 0,
  claimed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_nodes_run_status ON nodes(run_id, status);

CREATE TABLE events (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID NOT NULL,
  node_id UUID,
  span_id TEXT,
  ts TIMESTAMPTZ NOT NULL DEFAULT now(),
  level TEXT NOT NULL,
  kind TEXT NOT NULL,
  msg TEXT,
  attrs JSONB
);
CREATE INDEX idx_events_run ON events(run_id, ts);

-- +goose Down
DROP TABLE events;
DROP TABLE nodes;
DROP TABLE runs;
