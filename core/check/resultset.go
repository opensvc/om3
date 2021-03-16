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

func NewResultSet() *ResultSet {
	rs := &ResultSet{}
	rs.Data = make([]Result, 0)
	return rs
}

func (t *ResultSet) Add(rs *ResultSet) {
	t.Data = append(t.Data, rs.Data...)
}

func (t *ResultSet) Push(r Result) {
	t.Data = append(t.Data, r)
}

func (t ResultSet) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.Data)
}

func (t *ResultSet) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &t.Data)
}
