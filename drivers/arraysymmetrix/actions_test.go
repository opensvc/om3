package arraysymmetrix

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
)

// TestActionsBuildATree pins that what this driver declares is a tree the
// parser can be built from, and that it holds the verbs it shipped, the two
// the collector runs on a powermax among them.
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
		" add disk", " add tdev", " del disk", " del tdev", " resize disk",
		" rename disk", " map disk", " unmap disk", " add masking",
		" createpair", " deletepair", " set mode",
		" get pools", " get sgs", " get srps", " get directors", " get tdevs", " get views",
	} {
		assert.Truef(t, paths[want], "no action named%s", want)
	}
}

// TestTheSRDFOptionsAreTheirOwn is the bug the declaration fixes: three
// options were bound to one variable, so --srdf-type and --srdf-mode set the
// storage resource pool instead of themselves, and the type and the mode were
// whatever their zero value was.
func TestTheSRDFOptionsAreTheirOwn(t *testing.T) {
	var got OptAddDisk
	var addDisk array.Action
	for _, action := range (&Array{}).Actions() {
		if len(action.Path) == 2 && action.Path[0] == "add" && action.Path[1] == "disk" {
			addDisk = action
		}
	}
	require.NotNil(t, addDisk.Run)
	addDisk.Run = func(_ context.Context, in array.Input) (any, error) {
		got = OptAddDisk{
			SRP:      in.String(flagSRP.Name),
			SRDFMode: in.String(flagSRDFMode.Name),
			SRDFType: in.String(flagSRDFType.Name),
		}
		return nil, nil
	}

	require.NoError(t, array.RunActions(context.Background(), []array.Action{addDisk},
		[]string{"add", "disk", "--srp", "SRP_1", "--srdf-mode", "acp_wp", "--srdf-type", "R2"},
		&bytes.Buffer{}))
	assert.Equal(t, "SRP_1", got.SRP, "the pool is not moved by the srdf options")
	assert.Equal(t, "acp_wp", got.SRDFMode)
	assert.Equal(t, "R2", got.SRDFType)

	// The defaults v2 declares, for a caller naming neither.
	require.NoError(t, array.RunActions(context.Background(), []array.Action{addDisk},
		[]string{"add", "disk"}, &bytes.Buffer{}))
	assert.Equal(t, "sync", got.SRDFMode)
	assert.Equal(t, "R1", got.SRDFType)
	assert.Equal(t, "", got.SRP)
}

// TestMappingsAreReadInTheCollectorGrammar pins that a mapping holding several
// targets becomes one path per target.
func TestMappingsAreReadInTheCollectorGrammar(t *testing.T) {
	var got array.Mappings
	action := array.Action{
		Path:  []string{"map", "disk"},
		Short: "map",
		Flags: []array.Flag{array.FlagMapping},
		Run: func(_ context.Context, in array.Input) (any, error) {
			var err error
			got, err = optMappings(in)
			return nil, err
		},
	}
	require.NoError(t, array.RunActions(context.Background(), []array.Action{action},
		[]string{"map", "disk", "--mappings", "hba1:tgt1,tgt2", "--mappings", "hba2:tgt3"},
		&bytes.Buffer{}))
	assert.Len(t, got, 3)
	assert.Contains(t, got, "hba1:tgt1")
	assert.Contains(t, got, "hba1:tgt2")
	assert.Contains(t, got, "hba2:tgt3")
}
