package path

type (
	// Kind is opensvc object kind.
	Kind string

	// Kinds is the result of a binary Or on Kind values
	Kinds map[Kind]any
)

const (
	KindInvalid Kind = ""

	// Svc is the kind of objects containing app, containers, or volumes resources.
	KindSvc Kind = "svc"

	// Vol is the kind of objects containing fs, disk resources. Allocated from Pools.
	KindVol Kind = "vol"

	// Cfg is the kind of objects containing unencrypted key/val pairs used to abstract Svc configurations
	KindCfg Kind = "cfg"

	// Sec is the kind of objects containing encrypted key/val pairs used to abstract Svc configurations
	KindSec Kind = "sec"

	// Usr is the kind of objects containing a API user grants and credentials
	KindUsr Kind = "usr"

	// Ccfg is the kind of objects containing the cluster configuration
	KindCcfg Kind = "ccfg"

	// Nscfg is the kind of objects containing a namespace configuration
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

// New returns a kind struct from its string representation.
func NewKind(s string) Kind {
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
