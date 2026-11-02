package table

import (
	"errors"
	"strings"
)

type Table struct {
	Name   string
	As     string
	Schema string
}

func (t *Table) String() (string, error) {
	if t.Name == "" {
		return "", errors.New("krios: table name cannot be empty")
	}

	s := strings.Builder{}

	if t.Schema != "" {
		s.WriteString(t.Schema)
		s.WriteString(".")
	}

	s.WriteString(t.Name)

	if t.As != "" {
		s.WriteString(" AS ")
		s.WriteString(t.As)
	}

	return s.String(), nil
}
