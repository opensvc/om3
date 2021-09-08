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
	// Exist tests the existance of a key
	Exist
	// Equal tests if the current value of the key is equal to the keyop.T value
	Equal
	// NotEqual tests if the current value of the key is not equal to the keyop.T value
	NotEqual
	// GreaterOrEqual tests if the current value of the key is greater or equal to the keyop.T value
	GreaterOrEqual
	// LesserOrEqual tests if the current value of the key is lesser or equal to the keyop.T value
	LesserOrEqual
	// Greater tests if the current value of the key is greater to the keyop.T value
	Greater
	// Lesser tests if the current value of the key is lesser to the keyop.T value
	Lesser
)

var (
	toString = map[Op]string{
		Set:            "=",
		Insert:         "=",
		Append:         "+=",
		Remove:         "-=",
		Merge:          "|=",
		Toggle:         "^=",
		Exist:          ":",
		Equal:          "=",
		NotEqual:       "!=",
		GreaterOrEqual: ">=",
		LesserOrEqual:  "<=",
		Greater:        ">",
		Lesser:         "<",
	}

	toID = map[string]Op{
		"=":  Set,
		"+=": Append,
		"-=": Remove,
		"|=": Merge,
		"^=": Toggle,
		":":  Exist,
		"!=": NotEqual,
		">=": GreaterOrEqual,
		"<=": LesserOrEqual,
		">":  Greater,
		"<":  Lesser,
	}

	regexpIndex = regexp.MustCompile(`(.+)\[(\d+)\]`)
	regexpOp1   = regexp.MustCompile(`(.+)([><=:])(.*)`)
	regexpOp2   = regexp.MustCompile(`(.+)([\-+|^><!]=)(.*)`)
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

func (t Op) Is(op Op) bool {
	return op.String() == t.String()
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
	l := regexpOp2.FindStringSubmatch(s)
	// Example submatch result:
	//   []string{
	//     "env.foo[0]>=1",   /* original string */
	//     "env.foo[0]",      /* key with optional index */
	//     ">",              /* op1 */
	//     "=",              /* op2 */
	//     "1",               /* value */
	//   }
	if len(l) != 4 {
		l = regexpOp1.FindStringSubmatch(s)
	}
	if len(l) != 4 {
		return t
	}
	k := l[1]
	t.Op = ParseOp(l[2])
	t.Value = l[3]

	subs := regexpIndex.FindStringSubmatch(s)
	// Example submatch result:
	//   []string{
	//     "env.foo[0]",   /* original string */
	//     "env.foo",      /* key */
	//     "0",            /* index */
	//   }
	if len(subs) == 3 {
		k = subs[1]
		t.Index, _ = strconv.Atoi(subs[2])
		switch t.Op {
		case Set:
			if t.Value != "" {
				t.Op = Insert
			} else {
				t.Op = Remove
			}
		default:
			// invalid
			return &T{}
		}
	}
	if t.Op == Exist && !strings.Contains(k, ".") {
		//
		// "task" must be interpreted as the section name by the Exist operator
		// instead of DEFAULT.task.
		//
		// Matching DEFAULT options requires a "DEFAULT.<option>:" expression.
		// Note this is a breaking change from b2.1, where we matched if either
		// section or DEFAULT option was found.
		//
		k = k + "."
	}
	t.Key = key.Parse(k)
	return t
}

func (t T) String() string {
	switch t.Op {
	case Exist:
		return fmt.Sprintf("%s:", t.Key)
	case Insert:
		return fmt.Sprintf("%s[%d]=%s", t.Key, t.Index, t.Value)
	default:
		return fmt.Sprintf("%s%s%s", t.Key, t.Op, t.Value)
	}
}
