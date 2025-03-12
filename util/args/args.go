// Package args parses a posix arguments string using shlex.Split() and proposes
// methods to drop options and option-values.
//
// Example:
//
//	args, _ := Parse("-f -c /tmp/foo --comment foo --comment 'foo bar'")
//	args.DropOption("-f")
//	args.DropOptionAndAnyValue("-c")
//	args.DropOptionAndAnyValue("--comment")
//	args.Get()
package args

import (
	"regexp"

	"github.com/anmitsu/go-shlex"
)

type (
	T struct {
		args []string
	}
	matchType int
	matchOpt  struct {
		Type   matchType
		Value  string
		Option string
	}
)

const (
	matchNone matchType = iota
	matchAny
	matchExact
	matchRegexp
)

// New allocates a new T and returns its address
func New(s ...string) *T {
	t := &T{}
	t.args = append([]string{}, s...)
	return t
}

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
// caller as a shlex split string slice.
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
	t.dropOption(matchOpt{
		Option: s,
	})
}

// DropOptionAndAnyValue removes from args the elements matching s and the
// following element, considered the value of the option. If multiple
// elements match, they are all removed.
func (t *T) DropOptionAndAnyValue(s string) {
	t.dropOption(matchOpt{
		Option: s,
		Type:   matchAny,
	})
}

// DropOptionAndExactValue removes from args the elements matching s
// and the following element exactly matching v.
func (t *T) DropOptionAndExactValue(s, v string) {
	t.dropOption(matchOpt{
		Option: s,
		Type:   matchExact,
		Value:  v,
	})
}

// DropOptionAndMatchingValue removes from args the elements matching s
// and the following element matching the v regular expression.
func (t *T) DropOptionAndMatchingValue(s, v string) {
	t.dropOption(matchOpt{
		Option: s,
		Type:   matchRegexp,
		Value:  v,
	})
}

// HasOption returns true if any of the elements is matching s
func (t *T) HasOption(s string) bool {
	return t.hasOption(matchOpt{
		Option: s,
	})
}

// HasOptionAndAnyValue returns true if any of the elements is matching s with any value
func (t *T) HasOptionAndAnyValue(s string) bool {
	return t.hasOption(matchOpt{
		Option: s,
		Type:   matchAny,
	})
}

// HasOptionAndMatchingValue returns true if any of the elements is matching s
// and the following element is matching the v regular expression.
func (t *T) HasOptionAndMatchingValue(s, v string) bool {
	return t.hasOption(matchOpt{
		Option: s,
		Type:   matchRegexp,
		Value:  v,
	})
}
func strMatchArg(arg string, opt matchOpt, r *regexp.Regexp) bool {
	switch opt.Type {
	case matchAny:
		return true
	case matchExact:
		if arg == opt.Value {
			return true
		}
	case matchRegexp:
		if r.Match([]byte(arg)) {
			return true
		}
	}
	return false
}

func (t *T) hasOption(opt matchOpt) bool {
	var (
		undecidedArg string
		r            *regexp.Regexp
	)
	if opt.Type == matchRegexp {
		r = regexp.MustCompile(opt.Value)
	}
	for _, arg := range t.args {
		if undecidedArg != "" {
			if strMatchArg(arg, opt, r) {
				return true
			}
			undecidedArg = ""
		} else if arg == opt.Option {
			// arm to be able to drop depending on the value during the next loop iteration
			undecidedArg = arg
		}
	}
	return false
}

func (t *T) dropOption(opt matchOpt) {
	var (
		undecidedArg string
		r            *regexp.Regexp
	)
	l := make([]string, 0)
	if opt.Type == matchRegexp {
		r = regexp.MustCompile(opt.Value)
	}
	for _, arg := range t.args {
		if undecidedArg != "" {
			if !strMatchArg(arg, opt, r) {
				l = append(l, undecidedArg, arg)
			}
			undecidedArg = ""
		} else if arg == opt.Option {
			if opt.Type != matchNone {
				// arm to be able to drop depending on the value during the next loop iteration
				undecidedArg = arg
			}
		} else {
			l = append(l, arg)
		}
	}
	t.args = l
}

func (t *T) Append(s ...string) {
	t.args = append(t.args, s...)
}
