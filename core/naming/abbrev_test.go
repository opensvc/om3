package naming

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAbbrev is the table of the v2 abbrev() test. The shapes v2 covers are
// all shared-tail ones, and they render the same here.
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

	// A label the names share in the middle goes the way one at the end
	// does. v2 returns these untouched, having no shared tail to look at.
	assert.Equal(t,
		[]string{"n1..com", "n2..org"},
		Abbrev([]string{"n1.example.com", "n2.example.org"}))

	// Names sharing nothing past their first label are returned untouched:
	// nothing was dropped, so nothing says it was.
	assert.Equal(t,
		[]string{"n1.example.com", "n2.other.org"},
		Abbrev([]string{"n1.example.com", "n2.other.org"}))

	// A lone name has no second domain to be told apart from, so all of its
	// domain goes.
	assert.Equal(t, []string{"node1.."}, Abbrev([]string{"node1.example.com"}))

	// A cluster of short names is left alone.
	assert.Equal(t, []string{"dev1n1", "dev2n1"}, Abbrev([]string{"dev1n1", "dev2n1"}))
}

// TestAbbrevTrimsSharedLabelsInTheMiddle is the shape that motivated looking
// past the shared tail: a cloud name carrying the region between two domains
// the whole fleet shares.
func TestAbbrevTrimsSharedLabelsInTheMiddle(t *testing.T) {
	assert.Equal(t,
		[]string{"ip-xxx-xxx-xxx..eu-fr-paris..", "ip-yyy-yyy-zzz..eu-fr-north.."},
		Abbrev([]string{
			"ip-xxx-xxx-xxx.commondomain1.eu-fr-paris.commondomain2.etc",
			"ip-yyy-yyy-zzz.commondomain1.eu-fr-north.commondomain2.etc",
		}))
}

// TestAbbrevMarksEachRunOnce pins that a run of dropped labels is one marker:
// the marker says a name was cut here, and a column header is not the place to
// count how many labels that was.
func TestAbbrevMarksEachRunOnce(t *testing.T) {
	assert.Equal(t,
		[]string{"n1..a..", "n2..b.."},
		Abbrev([]string{"n1.x.y.a.p.q.r", "n2.x.y.b.p.q.r"}))
}

// TestAbbrevKeepsTheFirstLabel pins that the node's own name survives even
// when every node shares it: a column headed "..paris.." names nothing.
func TestAbbrevKeepsTheFirstLabel(t *testing.T) {
	assert.Equal(t,
		[]string{"web.paris..", "web.lyon.."},
		Abbrev([]string{"web.paris.example.com", "web.lyon.example.com"}))
}

// TestAbbrevIgnoresBareHostnames pins that a name with no domain does not get
// a say in what the others share: beside n2, the domain of n1 is still the
// only domain there is, and all of it goes.
func TestAbbrevIgnoresBareHostnames(t *testing.T) {
	assert.Equal(t,
		[]string{"n1..", "n2", "n3.."},
		Abbrev([]string{"n1.prod.example.com", "n2", "n3.prod.example.com"}))
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
