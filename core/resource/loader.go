package resource

import (
	"encoding/json"
	"fmt"
	"io"
)

// Loader uses a Reader to load a Resource configuration into the Resource struct
type Loader struct {
	r io.Reader
}

// NewLoader allocates a new Loader struct
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
