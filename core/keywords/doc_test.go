package keywords

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/naming"
)

func docHeadings(t *testing.T, store Store, driver, asked string) []string {
	var b bytes.Buffer
	require.NoError(t, store.Doc(&b, naming.KindSvc, driver, asked, 0, nil))
	return regexp.MustCompile("(?m)^#+ (.*)$").FindAllString(b.String(), -1)
}

// The keywords an option named with no section finds belong to several
// drivers, and their headings say which.
func TestDocHeadingsSayTheDriverOfKeywordsOfSeveralDrivers(t *testing.T) {
	store := Store{
		{Section: "task", Option: "schedule", Types: []string{"host"}},
		{Section: "sync", Option: "schedule", Types: []string{"rsync"}},
		{Section: "DEFAULT", Option: "schedule"},
		{Section: "volume", Option: "schedule", Types: []string{""}},
	}
	assert.Equal(t, []string{
		"## Keyword `schedule` of section `DEFAULT`",
		"## Keyword `schedule` of driver `sync.rsync`",
		"## Keyword `schedule` of driver `task.host`",
		"## Keyword `schedule` of driver `volume`",
	}, docHeadings(t, store, "", "sched*"))
}

// The keywords of a resource asked for all belong to its driver.
func TestDocHeadingsOfTheKeywordsOfOneResourceAreTheOptions(t *testing.T) {
	store := Store{
		{Section: "task", Option: "schedule", Types: []string{"host"}},
		{Section: "task", Option: "stat_timeout"},
	}
	assert.Equal(t, []string{
		"## Keyword `schedule`",
		"## Keyword `stat_timeout`",
	}, docHeadings(t, store, "", "task#1.s*"))
}
