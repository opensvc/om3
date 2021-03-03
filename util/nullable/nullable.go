package status

import (
	"database/sql"
	"encoding/json"
)

// NullBool relays the sql package NullBool and adds the json
// marshal and unmarshal methods.
type NullBool struct {
	sql.NullBool
}

// MarshalJSON is the interface method to dump this type to json.
func (nb NullBool) MarshalJSON() ([]byte, error) {
	if nb.Valid {
		return json.Marshal(nb.Bool)
	}
	return json.Marshal(nil)
}

// UnmarshalJSON is the interface method to load a json into this type.
func (nb *NullBool) UnmarshalJSON(data []byte) error {
	var b *bool
	if err := json.Unmarshal(data, &b); err != nil {
		return err
	}
	if b != nil {
		nb.Valid = true
		nb.Bool = *b
	} else {
		nb.Valid = false
	}
	return nil
}
