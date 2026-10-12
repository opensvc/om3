package resourceid

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsExact(t *testing.T) {
	for s, expected := range map[string]bool{
		// a driver group filters on the resources of that group
		"sync": false,
		"fs":   false,
		"task": false,
		// a pattern filters on the resources it matches
		"fs#d*":   false,
		"task#2*": false,
		"ip#?":    false,
		"fs#[12]": false,
		"*":       false,
		// a resource id names one resource
		"fs#1":        true,
		"ip#12":       true,
		"ip#backend3": true,
		"container#0": true,
		// not a resource id at all
		"":        false,
		"DEFAULT": false,
		"env":     false,
	} {
		t.Run(s, func(t *testing.T) {
			assert.Equal(t, expected, IsExact(s))
		})
	}
}

// A resource id names paths and is written in commands: one leading out of the
// var directory of its object, or into a shell, is refused, while a pattern of
// ids is not.
func TestParseRefusesWhatIsNoResourceIdCharacter(t *testing.T) {
	for _, s := range []string{
		"fs#x/../../etc/cron.d",
		"fs#/etc",
		`fs#a\b`,
		"fs#$(touch /x)",
		"fs#`id`",
		"fs#a;b",
		"fs#a b",
		"fs#a\nb",
		"fs#a\x00",
		"fs#'a'",
		"fs#a|b",
		"fs#a&b",
		"fs#a>b",
		"fs#{env.x}",
	} {
		_, err := Parse(s)
		assert.Error(t, err, "%q", s)
	}
	for _, s := range []string{"fs#1", "container#web-1", "task#a_b", "app#a.b", "foo#bar", "node", "fs#d*", "ip#?", "fs#[12]", "fs#[!1]", "*"} {
		_, err := Parse(s)
		assert.NoError(t, err, "%q", s)
	}
}

// A resource section is <group>#<index>: a bare name, or half of the pair,
// is not one.
func TestIsIndexed(t *testing.T) {
	for s, want := range map[string]bool{
		"fs#1":    true,
		"foo#bar": true,
		"node":    false,
		"fs":      false,
		"fs#":     false,
		"#1":      false,
	} {
		rid, err := Parse(s)
		if err != nil {
			t.Fatalf("%s: %s", s, err)
		}
		if got := rid.IsIndexed(); got != want {
			t.Errorf("%s: got %v, want %v", s, got, want)
		}
	}
}
