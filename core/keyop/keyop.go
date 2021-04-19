package keyop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
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
		Index int
	}
)

const (
	// Invalid is for invalid operator
	Invalid Op = iota
	// Set overwrites the value
	Set
	// Append appends an element, even if already present
	Append
	// Remove removes an element if present, do nothing if not present
	Remove
	// Merge adds an element if not present, do nothing if present
	Merge
	// Toggle adds an element if not present, removes it if present
	Toggle
	// Insert adds an element at the position specified by Index
	Insert
)

var (
	toString = map[Op]string{
		Set:    "=",
		Insert: "=",
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

	regexpIndex = regexp.MustCompile(`(.+)\[(\d+)\]`)
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
	subs := regexpIndex.FindStringSubmatch(k)
	// Example subs:
	//   env.foo[0] => {"env.foo[0]", "env.foo", "0"}
	if len(subs) == 3 {
		k = subs[1]
		t.Index, _ = strconv.Atoi(subs[2])
		switch t.Op {
		case Set:
			t.Op = Insert
		default:
			// invalid
			return &T{}
		}
	}
	t.Key = key.Parse(k)
	return t
}

func (t T) String() string {
	switch t.Op {
	case Insert:
		return fmt.Sprintf("%s[%d]=%s", t.Key, t.Index, t.Value)
	default:
		return fmt.Sprintf("%s%s%s", t.Key, t.Op, t.Value)
	}
}
