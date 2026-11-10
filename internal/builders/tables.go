package builders

import (
	"errors"
	"strings"
)

type Tables []Table

func (t *Tables) Add(tb Table) {
	*t = append(*t, tb)
}

func (t *Tables) Validate() error {
	if len(*t) == 0 {
		return errors.New("Krios - Table Builder: at least one table is required!")
	}

	for _, tb := range *t {
		if err := tb.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (t *Tables) Build() string {
	var tbs []string

	for _, tb := range *t {
		tbs = append(tbs, tb.Build())
	}

	return strings.Join(tbs, ", ")
}
