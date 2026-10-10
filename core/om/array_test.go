package om

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArrayNameFromArgs(t *testing.T) {
	isArray := func(name string) bool {
		return name == "freenas" || name == "array#freenas"
	}
	cases := map[string]struct {
		args []string
		name string
		rest []string
		err  string
	}{
		"named first": {
			args: []string{"freenas", "add", "disk", "--size", "1g"},
			name: "freenas",
			rest: []string{"add", "disk", "--size", "1g"},
		},
		"named with -a after the action, as the collector queues it": {
			args: []string{"add", "iscsi", "zvol", "-a", "freenas", "--name", "d1"},
			name: "freenas",
			rest: []string{"add", "iscsi", "zvol", "-a", "freenas", "--name", "d1"},
		},
		"named with --array first": {
			args: []string{"--array", "freenas", "del", "disk"},
			name: "freenas",
			rest: []string{"--array", "freenas", "del", "disk"},
		},
		"named twice": {
			args: []string{"freenas", "add", "disk", "-a", "freenas"},
			err:  "named twice",
		},
		"not named": {
			args: []string{},
			rest: []string{},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, rest, err := arrayNameFromArgs(c.args, isArray)
			if c.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.name, got)
			assert.Equal(t, c.rest, rest)
		})
	}
}

// The collector queues its array actions as v2 node actions, which the
// proxy node runs as "om node array ...".
func TestNodeArrayResolves(t *testing.T) {
	requireResolves(t, []string{"node", "array", "add", "disk", "-a", "a1", "--name", "d1", "--size", "1g"})
	assert.True(t, cmdNodeArray.Hidden, "a v2 form kept for the collector is not offered")
}

func TestArraySection(t *testing.T) {
	sections := []string{"node", "array#freenas", "array#vmax1", "array#vmax2", "array#dup1", "array#dup2"}
	names := map[string]string{
		"array#vmax1": "000297600001",
		"array#vmax2": "000297600002",
		"array#dup1":  "same",
		"array#dup2":  "same",
	}
	nameOf := func(section string) string { return names[section] }

	got, err := arraySection(sections, "freenas", nameOf)
	require.NoError(t, err)
	assert.Equal(t, "array#freenas", got, "by section name")

	got, err = arraySection(sections, "array#freenas", nameOf)
	require.NoError(t, err)
	assert.Equal(t, "array#freenas", got, "by section name, prefixed")

	got, err = arraySection(sections, "000297600002", nameOf)
	require.NoError(t, err)
	assert.Equal(t, "array#vmax2", got, "by name keyword, as a symmetrix serial")

	_, err = arraySection(sections, "same", nameOf)
	require.Error(t, err, "two sections with that name")
	assert.Contains(t, err.Error(), "ambiguous")

	_, err = arraySection(sections, "nothing", nameOf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no section found")
}
