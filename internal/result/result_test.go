package result

import (
	"database/sql"
	"errors"
	"testing"
)

type testResult struct {
	value int64
	err   error
}

func (r testResult) LastInsertId() (int64, error) { return r.value, r.err }
func (r testResult) RowsAffected() (int64, error) { return r.value, r.err }

func TestResultAvailability(t *testing.T) {
	for _, tc := range []struct {
		name      string
		result    sql.Result
		value     int64
		available bool
	}{
		{"zero", testResult{}, 0, true}, {"value", testResult{value: 7}, 7, true},
		{"unsupported", testResult{err: errors.New("unsupported")}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Wrap(tc.result)
			for _, method := range []func() (int64, bool){r.LastInsertId, r.RowsAffected} {
				if n, ok := method(); n != tc.value || ok != tc.available {
					t.Fatalf("result = %d, %v", n, ok)
				}
			}
		})
	}
}
