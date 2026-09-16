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
