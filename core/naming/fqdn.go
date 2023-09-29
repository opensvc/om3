package naming

import (
	"fmt"
	"regexp"
)

const regexStringRFC1123 = `^([a-zA-Z0-9]{1}[a-zA-Z0-9_-]{0,62})(\.[a-zA-Z0-9_]{1}[a-zA-Z0-9_-]{0,62})*?(\.[a-zA-Z]{1}[a-zA-Z0-9]{0,62})\.?$`

var regexRFC1123 = regexp.MustCompile(regexStringRFC1123)

type (
	FQDN struct {
		Path    Path
		Cluster string
	}
)

// IsValidFQDN verifies the string meets the RFC1123 requirements
func IsValidFQDN(s string) bool {
	return regexRFC1123.MatchString(s)
}

func NewFQDN(path Path, cluster string) *FQDN {
	return &FQDN{
		Path:    path,
		Cluster: cluster,
	}
}

func ParseFQDN(s string) (*FQDN, error) {
	var (
		name      string
		namespace string
		kind      string
		cluster   string
		p         Path
		err       error
	)
	_, err = fmt.Sscanf("%s.%s.%s.%s", s, &name, &namespace, &kind, &cluster)
	if err != nil {
		return nil, err
	}
	p, err = NewPathFromStrings(namespace, kind, name)
	return &FQDN{
		Path:    p,
		Cluster: cluster,
	}, nil
}

func (t FQDN) String() string {
	return fmt.Sprintf("%s.%s.%s.%s", t.Path.Name, t.Path.Namespace, t.Path.Kind, t.Cluster)
}

// Domain returns the domain part of the fqdn
func (t FQDN) Domain() string {
	return fmt.Sprintf("%s.%s.%s", t.Path.Namespace, t.Path.Kind, t.Cluster)
}

// MarshalText implements the json interface
func (t FQDN) MarshalText() ([]byte, error) {
	return []byte(t.String()), nil
}

// UnmarshalText implements the json interface
func (t *FQDN) UnmarshalText(b []byte) error {
	s := string(b)
	p, err := ParseFQDN(s)
	if err != nil {
		return err
	}
	t.Path = p.Path
	t.Cluster = p.Cluster
	return nil
}
