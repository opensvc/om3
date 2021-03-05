package resource

import (
	"encoding/json"
	"fmt"
	"io"
)

// Loader uses a Reader to load a JSON Resource configuration into
// the resource struct.
type Loader struct {
	r io.Reader
}

// NewLoader allocates a new Loader and returns a reference.
func NewLoader(r io.Reader) *Loader {
	return &Loader{r: r}
}

// Load JSON-decodes data from the Reader and load it at a Resource address
func (l *Loader) Load(v interface{}) error {
	dec := json.NewDecoder(l.r)
	if err := dec.Decode(v); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}
