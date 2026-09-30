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
