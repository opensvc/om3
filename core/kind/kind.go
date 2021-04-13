package kind

import (
	"bytes"
	"encoding/json"

	"opensvc.com/opensvc/util/xmap"
)

// T is an integer representing the opensvc object kind.
type T int

const (
	// Invalid is for invalid kinds
	Invalid T = iota
	// Svc is the kind of objects containing app, containers, or volumes resources.
	Svc
	// Vol is the kind of objects containing fs, disk resources. Allocated from Pools.
	Vol
	// Cfg is the kind of objects containing unencrypted key/val pairs used to abstract Svc configurations
	Cfg
	// Sec is the kind of objects containing encrypted key/val pairs used to abstract Svc configurations
	Sec
	// Usr is the kind of objects containing a API user grants and credentials
	Usr
	// Ccfg is the kind of objects containing the cluster configuration
	Ccfg
	// Nscfg is the kind of objects containing a namespace configuration
	Nscfg
)

var (
	toString = map[T]string{
		Svc:   "svc",
		Vol:   "vol",
		Cfg:   "cfg",
		Sec:   "sec",
		Usr:   "usr",
		Ccfg:  "ccfg",
		Nscfg: "nscfg",
	}

	toID = map[string]T{
		"svc":   Svc,
		"vol":   Vol,
		"cfg":   Cfg,
		"sec":   Sec,
		"usr":   Usr,
		"ccfg":  Ccfg,
		"nscfg": Nscfg,
	}
)

func (t T) String() string {
	return toString[t]
}

// New returns a kind struct from its string representation.
func New(s string) T {
	t, ok := toID[s]
	if ok {
		return t
	}
	return Invalid
}

// MarshalJSON marshals the enum as a quoted json string
func (t T) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(toString[t])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *T) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value, 'Created' in this case.
	*t = toID[j]
	return nil
}

func Names() []string {
	return xmap.Keys(toID)
}
