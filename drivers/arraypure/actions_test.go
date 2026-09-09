package arraypure

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
)

// TestActionsBuildATree pins that what this driver declares is a tree the
// parser can be built from, and that it holds the verbs it shipped.
func TestActionsBuildATree(t *testing.T) {
	actions := (&Array{}).Actions()
	require.NotEmpty(t, actions)
	_, err := array.NewCommand(actions, &bytes.Buffer{})
	require.NoError(t, err)

	paths := make(map[string]bool)
	for _, action := range actions {
		require.NotEmptyf(t, action.Short, "action %v has no help", action.Path)
		require.NotNilf(t, action.Run, "action %v does nothing", action.Path)
		key := ""
		for _, word := range action.Path {
			key += " " + word
		}
		assert.Falsef(t, paths[key], "two actions named%s", key)
		paths[key] = true
	}
	for _, want := range []string{
		" add disk", " del disk", " resize disk", " map disk", " unmap disk",
		" get hosts", " get connections", " get volumes", " get controllers",
		" get drives", " get pods", " get ports", " get interfaces",
		" get volumegroups", " get hostgroups", " get arrays", " get hardware",
	} {
		assert.Truef(t, paths[want], "no action named%s", want)
	}
}

// TestTheOptionsKeepTheirTypes pins the two that a shared catalogue could
// change under this driver: it names one host group where another names
// several, and its mappings are repeatable and taken whole.
func TestTheOptionsKeepTheirTypes(t *testing.T) {
	var mapDisk array.Action
	for _, action := range (&Array{}).Actions() {
		if len(action.Path) == 2 && action.Path[0] == "map" {
			mapDisk = action
		}
	}
	require.NotNil(t, mapDisk.Run)

	kinds := make(map[string]array.Kind)
	for _, flag := range mapDisk.Flags {
		kinds[flag.Name] = flag.Kind
	}
	assert.Equal(t, array.String, kinds["hostgroup"], "one host group, named")
	assert.Equal(t, array.RawStringSlice, kinds["mappings"], "several mappings, taken whole")
	assert.Equal(t, array.String, kinds["id"])
	assert.Equal(t, array.Int, kinds["lun"])
}

// TestAMappingReachesTheDriverWhole covers the value the collector writes,
// which holds commas of its own.
func TestAMappingReachesTheDriverWhole(t *testing.T) {
	var got OptMapping
	action := array.Action{
		Path:  []string{"map", "disk"},
		Short: "map",
		Flags: []array.Flag{array.FlagMapping, flagHost, flagHostGroup, array.FlagLUN},
		Run: func(_ context.Context, in array.Input) (any, error) {
			got = optMapping(in)
			return nil, nil
		},
	}
	err := array.RunActions(context.Background(), []array.Action{action},
		[]string{"map", "disk", "--mappings", "iqn.a:t1,t2", "--mappings", "iqn.b:t3", "--hostgroup", "hg1"},
		&bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, []string{"iqn.a:t1,t2", "iqn.b:t3"}, got.Mappings)
	assert.Equal(t, "hg1", got.HostGroupName)
	assert.Equal(t, -1, got.LUN, "an unset lun is the sentinel this driver reads")
}
