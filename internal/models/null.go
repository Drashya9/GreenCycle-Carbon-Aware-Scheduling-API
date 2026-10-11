package models

import (
	"database/sql"
	"encoding/json"
)

// Null is a nullable column value. It scans and binds exactly like sql.Null[T], but marshals
// to JSON as the plain value or null — sql.Null[T] has no JSON methods, so on its own it
// leaks its {"V":...,"Valid":...} struct shape into API responses.
type Null[T any] struct {
	sql.Null[T]
}

// NewNull wraps v as a present (non-null) value.
func NewNull[T any](v T) Null[T] {
	return Null[T]{sql.Null[T]{V: v, Valid: true}}
}

func (n Null[T]) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.V)
}

func (n *Null[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*n = Null[T]{}
		return nil
	}
	if err := json.Unmarshal(data, &n.V); err != nil {
		return err
	}
	n.Valid = true
	return nil
}
