package naming

import "strings"

type (
	// Kind is opensvc object kind.
	Kind string

	// Kinds is the result of a binary Or on Kind values
	Kinds map[Kind]any
)

const (
	KindInvalid Kind = ""

	// KindSvc is the kind of objects containing app, containers, or volumes resources.
	KindSvc Kind = "svc"

	// KindVol is the kind of objects containing fs, disk resources. Allocated from Pools.
	KindVol Kind = "vol"

	// KindCfg is the kind of objects containing unencrypted key/val pairs used to abstract Svc configurations
	KindCfg Kind = "cfg"

	// KindSec is the kind of objects containing encrypted key/val pairs used to abstract Svc configurations
	KindSec Kind = "sec"

	// KindUsr is the kind of objects containing a API user grants and credentials
	KindUsr Kind = "usr"

	// KindCcfg is the kind of objects containing the cluster configuration
	KindCcfg Kind = "ccfg"

	// KindNscfg is the kind of objects containing a namespace configuration
	KindNscfg Kind = "nscfg"
)

var (
	kindMap = map[string]any{
		string(KindSvc):   nil,
		string(KindVol):   nil,
		string(KindCfg):   nil,
		string(KindSec):   nil,
		string(KindUsr):   nil,
		string(KindCcfg):  nil,
		string(KindNscfg): nil,
	}

	KindKVStore = []Kind{
		KindCfg,
		KindSec,
		KindUsr,
	}

	KindAll = []Kind{
		KindSvc,
		KindVol,
		KindCfg,
		KindSec,
		KindUsr,
		KindCcfg,
		KindNscfg,
	}

	KindStrings = []string{
		string(KindSvc),
		string(KindVol),
		string(KindCfg),
		string(KindSec),
		string(KindUsr),
		string(KindCcfg),
		string(KindNscfg),
	}
)

func (t Kind) String() string {
	return string(t)
}

// ParseKind returns a Kind from its string representation.
func ParseKind(s string) Kind {
	if _, ok := kindMap[s]; ok {
		return Kind(s)
	} else {
		return KindInvalid
	}
}

func NewKinds(kinds ...Kind) Kinds {
	m := make(Kinds)
	for _, kind := range kinds {
		m[kind] = nil
	}
	return m
}

func (t Kinds) Has(kind Kind) bool {
	if kind == KindInvalid {
		return true
	}
	if t == nil {
		return true
	}
	_, ok := t[kind]
	return ok
}

func (t Kind) Or(kinds ...Kind) Kinds {
	m := NewKinds(kinds...)
	m[t] = nil
	return m
}

func (t Kinds) Or(kinds ...Kind) Kinds {
	for _, kind := range kinds {
		t[kind] = nil
	}
	return t
}

func (t Kinds) String() string {
	l := make([]string, len(t))
	i := 0
	for key := range t {
		l[i] = key.String()
	}
	return strings.Join(l, "|")
}
