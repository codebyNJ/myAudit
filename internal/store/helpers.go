package store

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

func marshalIDs(ids []uuid.UUID) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

func scanIDs(s string) []uuid.UUID {
	if s == "" || s == "null" {
		return []uuid.UUID{}
	}
	var v []uuid.UUID
	if json.Unmarshal([]byte(s), &v) != nil {
		return []uuid.UUID{}
	}
	return v
}

func scanTags(s string) []string {
	if s == "" || s == "null" {
		return []string{}
	}
	var v []string
	if json.Unmarshal([]byte(s), &v) != nil {
		return []string{}
	}
	return v
}

func nullUUID(ns sql.NullString) *uuid.UUID {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	id, err := uuid.Parse(ns.String)
	if err != nil {
		return nil
	}
	return &id
}
