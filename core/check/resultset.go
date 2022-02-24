package check

import (
	"encoding/json"
)

type (
	// ResultSet holds the list of all evaluated check instances.
	ResultSet struct {
		Data []Result
	}
)

// NewResultSet allocates and returns a ResultSet.
func NewResultSet() *ResultSet {
	rs := &ResultSet{}
	rs.Data = make([]Result, 0)
	return rs
}

// Len returns the number of results in the set.
func (t *ResultSet) Len() int {
	return len(t.Data)
}

// Add appends another ResultSet to this ResultSet
func (t *ResultSet) Add(rs *ResultSet) {
	t.Data = append(t.Data, rs.Data...)
}

// Push adds a Result to this ResultSet.
func (t *ResultSet) Push(r Result) {
	t.Data = append(t.Data, r)
}

// MarshalJSON turns this ResultSet into a byte slice.
func (t ResultSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Data)
}

// UnmarshalJSON parses a byte slice and loads this ResultSet.
func (t *ResultSet) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &t.Data)
}
