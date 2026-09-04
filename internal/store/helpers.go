package store

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

// marshalIDs renders a node-id slice as a JSON array for the deps TEXT column.
func marshalIDs(ids []uuid.UUID) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

// scanIDs parses the deps TEXT column (JSON array) back into ids.
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

// scanTags parses a JSON string array (from json_group_array) into []string.
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

// uuidArg passes a nullable uuid to SQL: nil pointer → NULL.
func uuidArg(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return p.String()
}

// nullUUID turns a nullable TEXT id column into *uuid.UUID.
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
