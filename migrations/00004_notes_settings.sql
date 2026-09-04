-- +goose Up
CREATE TABLE notes (
  run_id     UUID PRIMARY KEY REFERENCES runs(id),
  content    TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- single-row app settings (id is always 1)
CREATE TABLE settings (
  id         INT PRIMARY KEY DEFAULT 1,
  data       JSONB NOT NULL DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT settings_singleton CHECK (id = 1)
);
INSERT INTO settings(id, data) VALUES (1, '{"model_tier":"opus-4.8","token_budget":100000,"theme":"dark"}');

-- +goose Down
DROP TABLE notes;
DROP TABLE settings;
