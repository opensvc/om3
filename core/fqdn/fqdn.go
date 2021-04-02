package fqdn

import (
	"bytes"
	"encoding/json"
	"fmt"

	"opensvc.com/opensvc/core/path"
)

type (
	T struct {
		Path    path.T
		Cluster string
	}
)

func New(path path.T, cluster string) *T {
	return &T{
		Path:    path,
		Cluster: cluster,
	}
}

func Parse(s string) (*T, error) {
	var (
		name      string
		namespace string
		kind      string
		cluster   string
		p         path.T
		err       error
	)
	_, err = fmt.Sscanf("%s.%s.%s.%s", s, &name, &namespace, &kind, &cluster)
	if err != nil {
		return nil, err
	}
	p, err = path.New(name, namespace, kind)
	return &T{
		Path:    p,
		Cluster: cluster,
	}, nil
}

func (t T) String() string {
	return fmt.Sprintf("%s.%s.%s.%s", t.Path.Name, t.Path.Namespace, t.Path.Kind, t.Cluster)
}

// MarshalJSON implements the json interface
func (t T) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(t.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON implements the json interface
func (t *T) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	p, err := Parse(s)
	if err != nil {
		return err
	}
	t.Path = p.Path
	t.Cluster = p.Cluster
	return nil
}
