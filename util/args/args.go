//
// Parses a posix arguments string using shlex.Split() and proposes
// methods to drop options and option-values.
//
// Example:
//
//   args, _ := Parse("-f -c /tmp/foo --comment foo --comment 'foo bar'")
//   args.DropOption("-f")
//   args.DropOptionAndAnyValue("-c")
//   args.DropOptionAndAnyValue("--comment")
//   args.Get()
//
package args

import (
	"regexp"

	"github.com/anmitsu/go-shlex"
)

type (
	T struct {
		args []string
	}
	dropType int
	dropOpt  struct {
		Type   dropType
		Value  string
		Option string
	}
)

const (
	dropNone dropType = iota
	dropAny
	dropExact
	dropMatching
)

// Parse splits the string using the shlex splitter and store the
// resulting string slice.
func Parse(s string) (*T, error) {
	l, err := shlex.Split(s, true)
	if err != nil {
		return nil, err
	}
	t := &T{
		args: l,
	}
	return t, nil
}

// Set stores the string slice, which must have been formatted by the
// caller as a shlex splitted string slice.
func (t *T) Set(args []string) {
	t.args = args
}

// Get returns the arguments string slice installed by Parse() or
// T.Set()
func (t T) Get() []string {
	l := make([]string, 0)
	return append(l, t.args...)
}

// DropOption removes from args the elements matching s. If multiple
// elements match, they are all removed.
func (t *T) DropOption(s string) {
	t.dropOption(dropOpt{
		Option: s,
	})
}

// DropOptionAndValue removes from args the elements matching s and the
// following element, considered the value of the option. If multiple
// elements match, they are all removed.
func (t *T) DropOptionAndAnyValue(s string) {
	t.dropOption(dropOpt{
		Option: s,
		Type:   dropAny,
	})
}

// DropOptionAndExactValue removes from args the elements matching s
// and the following element exactly matching v.
func (t *T) DropOptionAndExactValue(s, v string) {
	t.dropOption(dropOpt{
		Option: s,
		Type:   dropExact,
		Value:  v,
	})
}

// DropOptionAndMatchingValue removes from args the elements matching s
// and the following element matching the v regular expression.
func (t *T) DropOptionAndMatchingValue(s, v string) {
	t.dropOption(dropOpt{
		Option: s,
		Type:   dropMatching,
		Value:  v,
	})
}

func (t *T) dropOption(opt dropOpt) {
	var (
		undecidedArg string
		r            *regexp.Regexp
	)
	l := make([]string, 0)
	if opt.Type == dropMatching {
		r = regexp.MustCompile(opt.Value)
	}
	match := func(arg string) bool {
		switch opt.Type {
		case dropAny:
			return true
		case dropExact:
			if arg == opt.Value {
				return true
			}
		case dropMatching:
			if r.Match([]byte(arg)) {
				return true
			}
		}
		return false
	}
	for _, arg := range t.args {
		if undecidedArg != "" {
			if !match(arg) {
				l = append(l, undecidedArg, arg)
			}
			undecidedArg = ""
		} else if arg == opt.Option {
			// arm to be able to drop depending on the value during the next loop iteration
			undecidedArg = arg
		} else {
			l = append(l, arg)
		}
	}
	t.args = l
}
