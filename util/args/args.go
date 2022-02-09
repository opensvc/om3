//
// Parses a posix arguments string using shlex.Split() and proposes
// methods to drop options and option-values.
//
// Example:
//
//   args, _ := Parse("-f -c /tmp/foo --comment foo --comment 'foo bar'")
//   args.DropOption("-f")
//   args.DropOptionWithValue("-c")
//   args.DropOptionWithValue("--comment")
//   args.Get()
//
package args

import "github.com/anmitsu/go-shlex"

type (
	T struct {
		args []string
	}
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
	t.dropOption(s, false)
}

// DropOptionAndValue removes from args the elements matching s and the
// following element, considered the value of the option. If multiple
// elements match, they are all removed.
func (t *T) DropOptionAndValue(s string) {
	t.dropOption(s, true)
}

func (t *T) dropOption(s string, withValue bool) {
	l := make([]string, 0)
	prevWasDroppedOption := false
	for _, arg := range t.args {
		if prevWasDroppedOption {
			prevWasDroppedOption = false
			continue
		}
		if arg == s {
			if withValue {
				// arm to drop the value on next loop iteration
				prevWasDroppedOption = true
			}
			continue
		}
		l = append(l, arg)
	}
	t.args = l
}
