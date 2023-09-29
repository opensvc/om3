package fqdn

import (
	"fmt"

	"github.com/opensvc/om3/core/naming"
)

type (
	T struct {
		Path    naming.Path
		Cluster string
	}
)

func New(path naming.Path, cluster string) *T {
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
		p         naming.Path
		err       error
	)
	_, err = fmt.Sscanf("%s.%s.%s.%s", s, &name, &namespace, &kind, &cluster)
	if err != nil {
		return nil, err
	}
	p, err = naming.NewPathFromStrings(namespace, kind, name)
	return &T{
		Path:    p,
		Cluster: cluster,
	}, nil
}

func (t T) String() string {
	return fmt.Sprintf("%s.%s.%s.%s", t.Path.Name, t.Path.Namespace, t.Path.Kind, t.Cluster)
}

// Domain returns the domain part of the fqdn
func (t T) Domain() string {
	return fmt.Sprintf("%s.%s.%s", t.Path.Namespace, t.Path.Kind, t.Cluster)
}

// MarshalText implements the json interface
func (t T) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText implements the json interface
func (t *T) UnmarshalText(b []byte) error {
	s := string(b)
	p, err := Parse(s)
	if err != nil {
		return err
	}
	t.Path = p.Path
	t.Cluster = p.Cluster
	return nil
}
