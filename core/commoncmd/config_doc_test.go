package commoncmd

import (
	"bytes"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/doc"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"

	_ "github.com/opensvc/om3/v3/drivers/ressyncrsync"
	_ "github.com/opensvc/om3/v3/drivers/ressyncstore"
)

// The documentation of a keyword of a driver a user may not choose says the
// grant choosing it needs, as the api refuses the keyword to that user: the
// sentence the api answers, not the one of the keyword alone.
func TestDocSaysTheGrantOfTheDriver(t *testing.T) {
	all := doc.ConvertKeywordStore(object.KeywordStoreWithDrivers(naming.KindSvc))
	// The keywords the api answers for a driver and a keyword asked: the
	// one of that driver, whose section the driver names.
	asked := func(typ, option string) api.KeywordDefinitionItems {
		l := make(api.KeywordDefinitionItems, 0)
		for _, item := range all {
			if item.Option == option && slices.Equal(item.Types, []string{typ}) {
				item.Section = "sync"
				l = append(l, item)
			}
		}
		require.Len(t, l, 1)
		return l
	}
	var b bytes.Buffer
	require.NoError(t, Doc(&b, asked("rsync", "schedule"), naming.KindSvc, "sync.rsync", "schedule", 0))
	assert.Contains(t, b.String(), "A keyword of the rsync driver, which needs the root grant to choose.")

	b.Reset()
	require.NoError(t, Doc(&b, asked("store", "schedule"), naming.KindSvc, "sync.store", "schedule", 0))
	assert.NotContains(t, b.String(), "needs the root grant to choose", "the store driver is a namespace administrator's")
}
