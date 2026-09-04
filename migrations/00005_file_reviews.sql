-- +goose Up
CREATE TABLE file_reviews (
  run_id  UUID NOT NULL REFERENCES runs(id),
  path    TEXT NOT NULL,
  status  TEXT NOT NULL,               -- accepted | rejected
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, path)
);

-- +goose Down
DROP TABLE file_reviews;
