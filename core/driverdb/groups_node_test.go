package driverdb_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

// nodeConfigStores returns the keyword stores of the node and of the cluster
// configurations.
func nodeConfigStores() map[string]keywords.Store {
	return map[string]keywords.Store{
		"node": object.NodeKeywordStore,
		"ccfg": object.KeywordStoreWithDrivers(naming.KindCcfg),
	}
}

// TestKeywordGroupsOfNodeConfig verifies no keyword of the node or cluster
// configuration is left in the group of the keywords not grouped yet, and
// that each names a group of these configurations, with a name and a text.
func TestKeywordGroupsOfNodeConfig(t *testing.T) {
	declared := make(map[string]keywords.Group)
	for _, g := range object.NodeKeywordGroups() {
		declared[g.Driver+" "+g.ID] = g.Group
	}
	for name, store := range nodeConfigStores() {
		t.Run(name, func(t *testing.T) {
			require.NotEmpty(t, store)
			for _, kw := range store {
				assert.NotEqualf(t, keywords.GroupOtherID, kw.GroupID(), "%s is not grouped", kwID(kw))
				_, ok := declared[" "+kw.GroupID()]
				assert.Truef(t, ok, "%s: group %q is not one of the node configuration", kwID(kw), kw.GroupID())
				for _, g := range object.KeywordGroups(keywords.Store{kw}) {
					assert.NotEmptyf(t, g.Name, "%s: group %s has no name", kwID(kw), g.ID)
					assert.NotEmptyf(t, g.Text, "%s: group %s has no text", kwID(kw), g.ID)
					assert.Equalf(t, declared[g.Driver+" "+g.ID], g.Group, "%s: group %s of driver %q is the declared one", kwID(kw), g.ID, g.Driver)
				}
			}
		})
	}
}

// TestKeywordGroupsOfNodeConfigListing verifies the keyword definitions of a
// section type of the node or cluster configuration, which the api lists as
// copies of the keywords, describe the group of each keyword.
func TestKeywordGroupsOfNodeConfigListing(t *testing.T) {
	for name, store := range nodeConfigStores() {
		for index := range store.KeywordsByDriver(naming.KindInvalid) {
			kws, err := store.DriverKeywords(index[0], index[1], naming.KindInvalid)
			require.NoError(t, err, index)
			groups := object.KeywordGroups(kws)
			for _, kw := range kws {
				found := slices.ContainsFunc(groups, func(g keywords.DriverGroup) bool {
					return g.ID == kw.GroupID() && (g.Driver == "" || g.Driver == index.String())
				})
				assert.Truef(t, found, "%s %s: %s: group %q is not listed", name, index, kwID(kw), kw.GroupID())
			}
		}
	}
}

// TestKeywordStrategiesOfNodeConfig verifies the strategies of a group a
// section type of the node configuration declares name keywords of the type,
// and pins the ones of the credentials of a hp3par array.
func TestKeywordStrategiesOfNodeConfig(t *testing.T) {
	for _, g := range object.NodeKeywordGroups() {
		if len(g.Strategies) == 0 {
			continue
		}
		require.NotEmptyf(t, g.Driver, "group %s has strategies, so a section type declares it", g.ID)
		l := keywords.ParseIndex(g.Driver)
		kws, err := object.NodeKeywordStore.DriverKeywords(l[0], l[1], naming.KindInvalid)
		require.NoError(t, err, g.Driver)
		options := make([]string, 0, len(kws))
		for _, kw := range kws {
			options = append(options, kw.Option)
		}
		for _, s := range g.Strategies {
			assert.NotEmptyf(t, s.Name, "%s %s strategy %s has no name", g.Driver, g.ID, s.ID)
			assert.NotEmptyf(t, s.Text, "%s %s strategy %s has no text", g.Driver, g.ID, s.ID)
			assert.NotEmptyf(t, s.Uses, "%s %s strategy %s uses no keyword", g.Driver, g.ID, s.ID)
			for _, option := range slices.Concat(s.Uses, s.Excludes, s.Requires) {
				assert.Containsf(t, options, option, "%s %s strategy %s names %q, no keyword of the type", g.Driver, g.ID, s.ID, option)
			}
			for _, kop := range s.When {
				assert.Containsf(t, options, kop.Key.Option, "%s %s strategy %s is chosen by %q, no keyword of the type", g.Driver, g.ID, s.ID, kop.Key.Option)
			}
		}
	}

	kws, err := object.NodeKeywordStore.DriverKeywords("array", "hp3par", naming.KindInvalid)
	require.NoError(t, err)
	// The keywords of hp3par shared with other array types, as username,
	// list the credentials group of the configuration too.
	var credentials *keywords.DriverGroup
	for _, g := range object.KeywordGroups(kws) {
		if g.ID == "credentials" && g.Driver == "array.hp3par" {
			require.Nil(t, credentials, "one credentials group of array.hp3par")
			credentials = &g
		}
	}
	require.NotNil(t, credentials, "the credentials group of array.hp3par")
	ids := make([]string, 0)
	for _, s := range credentials.Strategies {
		ids = append(ids, s.ID)
	}
	assert.Equal(t, []string{"ssh", "cli"}, ids)
}

// TestKeywordStrategiesOfNodeConfigValidate verifies the validation of the
// cluster configuration warns of a hp3par array setting the credentials of
// the method it does not use, and only then.
func TestKeywordStrategiesOfNodeConfigValidate(t *testing.T) {
	for conf, conflicting := range map[string]bool{
		"method = ssh\nusername = u\nkey = /k\n":            false,
		"method = cli\npwf = /p\n":                          false,
		"method = cli\npwf = /p\nkey = /k\n":                true,
		"method = ssh\nkey = /k\npwf = /p\n":                true,
		"method = cli\npwf = /p\nusername = u\n":            true,
		"key = /k\npwf = /p\n":                              true,
		"method = cli\npwf = /p\nkey =\n":                   false,
		"method = ssh\nusername = u\nkey = /k\ncli = cli\n": false,
	} {
		data := "[array#a1]\ntype = hp3par\nmanager = m\n" + conf
		o, err := object.NewCluster(object.WithConfigData([]byte(data)), object.WithVolatile(true))
		require.NoError(t, err, conf)
		alerts, err := o.Config().Validate()
		require.NoError(t, err, conf)
		found := false
		for _, a := range alerts {
			if a.Kind.String() == "conflicting strategies" {
				found = true
			}
		}
		assert.Equalf(t, conflicting, found, "%s: %s", conf, alerts)
	}
}
