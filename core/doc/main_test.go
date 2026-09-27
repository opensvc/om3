package doc

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func TestFilterKeywordStoreOptionInAnySection(t *testing.T) {
	store := keywords.Store{
		{Section: "DEFAULT", Option: "nodes"},
		{Section: "volume", Option: "nodes"},
		{Section: "DEFAULT", Option: "orchestrate"},
	}
	option := "nodes"
	got, err := FilterKeywordStore(store, nil, nil, &option, naming.Path{}, nil)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, kw := range got {
		require.Equal(t, "nodes", kw.Option)
	}
}

func TestFindKeywords(t *testing.T) {
	type query struct{ section, option string }
	record := func(asked *[]query, found map[query]api.KeywordDefinitionItems) func(section, option *string) (api.KeywordDefinitionItems, error) {
		return func(section, option *string) (api.KeywordDefinitionItems, error) {
			var q query
			if section != nil {
				q.section = *section
			}
			if option != nil {
				q.option = *option
			}
			*asked = append(*asked, q)
			if items, ok := found[q]; ok {
				return items, nil
			}
			return nil, errors.New("not a section")
		}
	}
	one := api.KeywordDefinitionItems{{}}
	t.Run("a bare word is an option first", func(t *testing.T) {
		var asked []query
		items, err := FindKeywords("pg_cpus", false, record(&asked, map[query]api.KeywordDefinitionItems{{option: "pg_cpus"}: one}))
		require.NoError(t, err)
		require.Len(t, items, 1)
		require.Equal(t, []query{{option: "pg_cpus"}}, asked)
	})
	t.Run("a bare word is a section next", func(t *testing.T) {
		var asked []query
		items, err := FindKeywords("DEFAULT", false, record(&asked, map[query]api.KeywordDefinitionItems{{option: "DEFAULT"}: {}, {section: "DEFAULT"}: one}))
		require.NoError(t, err)
		require.Len(t, items, 1)
		require.Equal(t, []query{{option: "DEFAULT"}, {section: "DEFAULT"}}, asked)
	})
	t.Run("a bare word with a driver is only an option", func(t *testing.T) {
		var asked []query
		_, err := FindKeywords("foo", true, record(&asked, map[query]api.KeywordDefinitionItems{{option: "foo"}: {}}))
		require.EqualError(t, err, "keyword foo not found")
		require.Equal(t, []query{{option: "foo"}}, asked)
	})
	t.Run("a section and an option", func(t *testing.T) {
		var asked []query
		_, err := FindKeywords("app#1.foo", false, record(&asked, map[query]api.KeywordDefinitionItems{{section: "app#1", option: "foo"}: {}}))
		require.EqualError(t, err, "keyword app#1.foo not found")
		require.Equal(t, []query{{section: "app#1", option: "foo"}}, asked)
	})
	t.Run("a word found nowhere", func(t *testing.T) {
		var asked []query
		_, err := FindKeywords("foo", false, record(&asked, map[query]api.KeywordDefinitionItems{{option: "foo"}: {}}))
		require.EqualError(t, err, "keyword foo not found")
	})
	t.Run("the error of the only lookup", func(t *testing.T) {
		var asked []query
		_, err := FindKeywords("app#1.foo", false, record(&asked, nil))
		require.EqualError(t, err, "not a section")
	})
	t.Run("no keyword is every keyword", func(t *testing.T) {
		var asked []query
		_, err := FindKeywords("", false, record(&asked, map[query]api.KeywordDefinitionItems{{}: {}}))
		require.NoError(t, err)
		require.Equal(t, []query{{}}, asked)
	})
}
