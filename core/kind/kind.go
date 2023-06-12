package kind

import (
	"github.com/opensvc/om3/util/xmap"
	"github.com/pkg/errors"
)

type (
	// T is opensvc object kind.
	T uint

	// Mask is the result of a binary Or on T values
	Mask uint
)

const (
	// Invalid is for invalid kinds
	Invalid T = 0
	// Svc is the kind of objects containing app, containers, or volumes resources.
	Svc T = 1 << iota
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

// MarshalText marshals the enum as a string
func (t T) MarshalText() ([]byte, error) {
	if s, ok := toString[t]; !ok {
		return nil, errors.Errorf("unknown kind %v", t)
	} else {
		return []byte(s), nil
	}
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *T) UnmarshalText(b []byte) error {
	s := string(b)
	if k, ok := toID[s]; !ok {
		return errors.Errorf("unknown kind %s", s)
	} else {
		*t = k
		return nil
	}
}

func Names() []string {
	return xmap.Keys(toID)
}

func (t Mask) Has(kind T) bool {
	if t == 0 {
		return true
	}
	return int(t)&int(kind) != 0
}

func (t T) Or(ts ...T) Mask {
	m := Mask(t)
	return or(m, ts...)
}

func (t Mask) Or(ts ...T) Mask {
	return or(t, ts...)
}

func Or(ts ...T) Mask {
	return or(Mask(0), ts...)
}

func or(m Mask, ts ...T) Mask {
	i := int(m)
	for _, t := range ts {
		i |= int(t)
	}
	return Mask(i)
}
