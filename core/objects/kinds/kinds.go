package kinds

import (
	"bytes"
	"encoding/json"
)

// Type is an integer representing the opensvc object kind.
type Type int

const (
	// Svc is the kind of objects containing app, containers, or volumes resources.
	Svc Type = iota
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

var toString = map[Type]string{
	Svc:   "svc",
	Vol:   "vol",
	Cfg:   "cfg",
	Sec:   "sec",
	Usr:   "usr",
	Ccfg:  "ccfg",
	Nscfg: "nscfg",
}

var toID = map[string]Type{
	"svc":   Svc,
	"vol":   Vol,
	"cfg":   Cfg,
	"sec":   Sec,
	"usr":   Usr,
	"ccfg":  Ccfg,
	"nscfg": Nscfg,
}

func (t Type) String() string {
	return toString[t]
}

// MarshalJSON marshals the enum as a quoted json string
func (t Type) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(toString[t])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *Type) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value, 'Created' in this case.
	*t = toID[j]
	return nil
}
