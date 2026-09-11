package arrayfreenas

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
		" add disk", " add zvol", " del disk", " del zvol", " map disk",
		" update dataset", " get pools", " get datasets", " get disk", " get dataset",
		" get system", " add iscsi portal", " add iscsi target", " add iscsi targetgroup",
		" add iscsi initiator", " add iscsi extent", " del iscsi extent", " del iscsi target",
		" del iscsi initiator", " get iscsi portals", " get iscsi targets",
		" get iscsi targetextents", " get iscsi extent", " get iscsi extents",
		" get iscsi initiators", " unmap iscsi zvol",
	} {
		assert.Truef(t, paths[want], "no action named%s", want)
	}
}

// TestTheCollectorCommandsAreAnswered pins the three commands the collector
// runs on a freenas array, with the option names it writes.
//
// "resize zvol" is one of them, and v2 answers to it. This driver answered
// only to "update dataset", so a resize from the collector reached nothing.
func TestTheCollectorCommandsAreAnswered(t *testing.T) {
	actions := (&Array{}).Actions()
	byPath := make(map[string]array.Action)
	for _, action := range actions {
		key := ""
		for _, word := range action.Path {
			key += " " + word
		}
		byPath[key] = action
	}

	for _, tc := range []struct {
		path  string
		flags []string
	}{
		{" add iscsi zvol", []string{"volume", "name", "size", "mappings"}},
		{" resize zvol", []string{"name", "size"}},
		{" del iscsi zvol", []string{"name"}},
	} {
		action, ok := byPath[tc.path]
		require.Truef(t, ok, "no action named%s", tc.path)
		got := make(map[string]bool)
		for _, flag := range action.Flags {
			got[flag.Name] = true
		}
		for _, want := range tc.flags {
			assert.Truef(t, got[want], "%s must take --%s", tc.path, want)
		}
	}

	// The two the collector writes are answered but not advertised.
	assert.True(t, byPath[" add iscsi zvol"].Hidden)
	assert.True(t, byPath[" del iscsi zvol"].Hidden)
	assert.False(t, byPath[" add disk"].Hidden)
}

// TestMappingsAreRepeatableAndWhole is the collector's shape: the option is
// written once per initiator, and each value holds commas of its own.
func TestMappingsAreRepeatableAndWhole(t *testing.T) {
	var got AddDiskOptions
	action := array.Action{
		Path:  []string{"add", "disk"},
		Short: "add",
		Flags: append(append([]array.Flag{}, zvolFlags...), flagInsecureTPC, array.FlagLUN, array.FlagMapping),
		Run: func(_ context.Context, in array.Input) (any, error) {
			got = optAddDisk(in, in.String(array.FlagName.Name))
			return nil, nil
		},
	}
	err := array.RunActions(context.Background(), []array.Action{action},
		[]string{"add", "disk", "--name", "d1", "--size", "1g",
			"--mappings", "iqn.a:iqn.t1,iqn.t2", "--mappings", "iqn.b:iqn.t3"}, &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, []string{"iqn.a:iqn.t1,iqn.t2", "iqn.b:iqn.t3"}, got.Mappings)
	assert.Nil(t, got.LunId, "an unset lun is left for the array to choose")
}

// TestTheDefaultsAreTheOnesThisDriverShipped pins the values a caller that
// names none of them gets.
func TestTheDefaultsAreTheOnesThisDriverShipped(t *testing.T) {
	var got AddZvolOptions
	action := array.Action{
		Path:  []string{"add", "zvol"},
		Short: "add",
		Flags: zvolFlags,
		Run: func(_ context.Context, in array.Input) (any, error) {
			got = optAddZvol(in, in.String(array.FlagName.Name))
			return nil, nil
		},
	}
	require.NoError(t, array.RunActions(context.Background(), []array.Action{action},
		[]string{"add", "zvol", "--name", "d1", "--size", "1g"}, &bytes.Buffer{}))
	assert.Equal(t, "512", got.Blocksize)
	assert.Equal(t, "off", got.Deduplication)
	assert.Equal(t, "inherit", got.Compression)
	assert.False(t, got.Sparse)
}
