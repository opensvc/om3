package keywords

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
)

// A section with no type is of the default driver of its group, as the
// validation takes it: its keywords are those of that driver.
func TestDriverKeywordsOfASectionWithNoTypeAreTheDefaultDriverOnes(t *testing.T) {
	group := driver.NewGroup("ip")
	name := driver.NewID(group, "").Name
	require.NotEmpty(t, name, "ip has a default driver")
	store := Store{
		{Section: "ip", Option: "netns", Types: []string{name}, Kind: naming.NewKinds(naming.KindSvc)},
	}
	l, err := store.DriverKeywords("ip", "", naming.KindSvc)
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, "netns", l[0].Option)

	_, err = store.DriverKeywords("nosuchgroup", "", naming.KindSvc)
	assert.ErrorIs(t, err, ErrDriverNotFound)
}
