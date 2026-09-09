package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAbbrev is the table of the v2 abbrev() test, so the port renders a
// cluster the way v2 does.
func TestAbbrev(t *testing.T) {
	cases := []struct {
		names    []string
		expected []string
	}{
		{[]string{}, []string{}},
		{[]string{"n1"}, []string{"n1"}},
		{[]string{"n1", "n2"}, []string{"n1", "n2"}},
		{[]string{"n1.org", "n2"}, []string{"n1..", "n2"}},
		{[]string{"n1.org", "n1"}, []string{"n1..", "n1"}},
		{[]string{"n1.org.com", "n2.org.com"}, []string{"n1..", "n2.."}},
		{[]string{"n1.org1.com", "n2.org2.com"}, []string{"n1.org1..", "n2.org2.."}},
		{[]string{"n1.org1.com", "n2"}, []string{"n1..", "n2"}},
		{[]string{"n1.org1.com", "n1"}, []string{"n1..", "n1"}},
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.expected, Abbrev(tc.names), "Abbrev(%v)", tc.names)
	}
}

// TestAbbrevKeepsWhatTellsNamesApart covers the shapes a cluster actually
// takes, beyond the v2 table.
func TestAbbrevKeepsWhatTellsNamesApart(t *testing.T) {
	// The shared domain goes, whatever its depth.
	assert.Equal(t,
		[]string{"node1..", "node2..", "node3.."},
		Abbrev([]string{"node1.prod.example.com", "node2.prod.example.com", "node3.prod.example.com"}))

	// The domain that differs stays, because it is what tells them apart.
	assert.Equal(t,
		[]string{"node1.paris..", "node1.lyon.."},
		Abbrev([]string{"node1.paris.example.com", "node1.lyon.example.com"}))

	// Names differing at their last label share nothing to drop, and are
	// returned untouched rather than marked.
	assert.Equal(t,
		[]string{"n1.example.com", "n2.example.org"},
		Abbrev([]string{"n1.example.com", "n2.example.org"}))

	// A lone name has no second domain to be told apart from, so all of its
	// domain goes.
	assert.Equal(t, []string{"node1.."}, Abbrev([]string{"node1.example.com"}))

	// A cluster of short names is left alone.
	assert.Equal(t, []string{"dev1n1", "dev2n1"}, Abbrev([]string{"dev1n1", "dev2n1"}))
}

// TestAbbrevDoesNotChangeTheCount pins that a renderer may zip the result with
// the names it passed in: one abbreviation comes back per name, in order.
func TestAbbrevDoesNotChangeTheCount(t *testing.T) {
	for _, names := range [][]string{
		{},
		{"n1"},
		{"n1", "n2.example.com", "n3.example.com"},
		{"a.b.c.d.e", "f", "g.b.c.d.e"},
	} {
		assert.Lenf(t, Abbrev(names), len(names), "Abbrev(%v)", names)
	}
}

// TestAbbrevIsStable pins that abbreviating twice changes nothing more, which
// a renderer calling it on every frame relies on.
func TestAbbrevIsStable(t *testing.T) {
	names := []string{"node1.prod.example.com", "node2.prod.example.com"}
	once := Abbrev(names)
	assert.Equal(t, once, Abbrev(append([]string{}, once...)))
}
