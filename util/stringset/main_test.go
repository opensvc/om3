package stringset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOps(t *testing.T) {
	set := New()
	assert.Lenf(t, set, 0, "len is zero after new")
	set.Add("a", "ab", "a")
	assert.Lenf(t, set, 2, "len is two after adding a,ab,a")
	assert.Truef(t, set.Contains("a"), "set contains a")
	assert.Truef(t, set.Contains("ab"), "set contains a")
	assert.Falsef(t, set.Contains("b"), "set does not contain b")
	l := set.Slice()
	assert.Contains(t, l, "a", "set slice contains a")
	assert.Contains(t, l, "ab", "set slice contains a")
}
