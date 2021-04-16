package keyop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/xmap"
)

type (
	// Op is an integer representing an operation on a configuration key value
	Op int

	// T defines a parsed key operation
	T struct {
		Key   key.T
		Op    Op
		Value string
	}
)

const (
	// Invalid is for invalid operator
	Invalid Op = iota
	// Svc is the kind of objects containing app, containers, or volumes resources.
	Set
	// Vol is the kind of objects containing fs, disk resources. Allocated from Pools.
	Append
	// Cfg is the kind of objects containing unencrypted key/val pairs used to abstract Svc configurations
	Remove
	// Sec is the kind of objects containing encrypted key/val pairs used to abstract Svc configurations
	Merge
	// Usr is the kind of objects containing a API user grants and credentials
	Toggle
)

var (
	toString = map[Op]string{
		Set:    "=",
		Append: "+=",
		Remove: "-=",
		Merge:  "|=",
		Toggle: "^=",
	}

	toID = map[string]Op{
		"=":  Set,
		"+=": Append,
		"-=": Remove,
		"|=": Merge,
		"^=": Toggle,
	}
)

func (t Op) String() string {
	return toString[t]
}

// ParseOp returns an operator from its string representation.
func ParseOp(s string) Op {
	t, ok := toID[s]
	if ok {
		return t
	}
	return Set
}

// MarshalJSON marshals the enum as a quoted json string
func (t Op) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(toString[t])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *Op) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value, 'Created' in this case.
	*t = toID[j]
	return nil
}

func Ops() []string {
	return xmap.Keys(toID)
}

func (t T) IsZero() bool {
	return t.Op == Invalid
}

func Parse(s string) *T {
	t := &T{}
	l := strings.SplitN(s, "=", 2)
	if len(l) != 2 {
		return t
	}
	k := l[0]
	t.Value = l[1]
	end := len(k) - 1
	opStr := fmt.Sprintf("%c=", k[end])
	t.Op = ParseOp(opStr)
	if t.Op != Set {
		k = k[:end]
	}
	t.Key = key.Parse(k)
	return t
}
