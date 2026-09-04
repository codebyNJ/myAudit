-- +goose Up
CREATE TABLE doc_briefs (
  provider    TEXT NOT NULL,
  version     TEXT NOT NULL,
  source_hash TEXT NOT NULL,
  content     JSONB NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (provider, version)
);

-- +goose Down
DROP TABLE doc_briefs;
