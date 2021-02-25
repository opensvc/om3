package object

import (
	"bytes"
	"encoding/json"
)

// Kind is an integer representing the opensvc object kind.
type Kind int

const (
	// KindInvalid is for invalid kinds
	KindInvalid Kind = iota
	// KindSvc is the kind of objects containing app, containers, or volumes resources.
	KindSvc
	// KindVol is the kind of objects containing fs, disk resources. Allocated from Pools.
	KindVol
	// KindCfg is the kind of objects containing unencrypted key/val pairs used to abstract Svc configurations
	KindCfg
	// KindSec is the kind of objects containing encrypted key/val pairs used to abstract Svc configurations
	KindSec
	// KindUsr is the kind of objects containing a API user grants and credentials
	KindUsr
	// KindCcfg is the kind of objects containing the cluster configuration
	KindCcfg
	// KindNscfg is the kind of objects containing a namespace configuration
	KindNscfg
)

var (
	kindID2String = map[Kind]string{
		KindSvc:   "svc",
		KindVol:   "vol",
		KindCfg:   "cfg",
		KindSec:   "sec",
		KindUsr:   "usr",
		KindCcfg:  "ccfg",
		KindNscfg: "nscfg",
	}

	kindStringToID = map[string]Kind{
		"svc":   KindSvc,
		"vol":   KindVol,
		"cfg":   KindCfg,
		"sec":   KindSec,
		"usr":   KindUsr,
		"ccfg":  KindCcfg,
		"nscfg": KindNscfg,
	}
)

func (t Kind) String() string {
	return kindID2String[t]
}

// NewKind returns a Kind struct from a kind string.
func NewKind(kind string) Kind {
	t, ok := kindStringToID[kind]
	if ok {
		return t
	}
	return KindInvalid
}

// MarshalJSON marshals the enum as a quoted json string
func (t Kind) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(kindID2String[t])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *Kind) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value, 'Created' in this case.
	*t = kindStringToID[j]
	return nil
}
