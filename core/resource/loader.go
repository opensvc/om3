package resource

import (
	"encoding/json"
	"fmt"
	"io"
)

type Loader struct {
	r io.Reader
}

func NewLoader(r io.Reader) *Loader {
	return &Loader{r: r}
}

func (l *Loader) Load(v interface{}) error {
	dec := json.NewDecoder(l.r)
	if err := dec.Decode(v); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

