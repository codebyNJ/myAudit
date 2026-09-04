-- +goose Up
CREATE TABLE checkpoints (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID NOT NULL REFERENCES runs(id),
  node_id UUID NOT NULL REFERENCES nodes(id),
  question TEXT NOT NULL,
  options JSONB,
  answer TEXT,
  resolved BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);
CREATE INDEX idx_checkpoints_open ON checkpoints(node_id) WHERE NOT resolved;

-- +goose Down
DROP TABLE checkpoints;
