package arrayhoc

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
)

// TestActionsBuildATree pins that what this driver declares is a tree the
// parser can be built from, which a malformed path or a duplicated option
// would break.
func TestActionsBuildATree(t *testing.T) {
	actions := (&Array{}).Actions()
	require.NotEmpty(t, actions)

	root, err := array.NewCommand(actions, &bytes.Buffer{})
	require.NoError(t, err)

	paths := make(map[string]bool)
	for _, action := range actions {
		require.NotEmptyf(t, action.Path, "an action has no path")
		require.NotEmptyf(t, action.Short, "action %v has no help", action.Path)
		require.NotNilf(t, action.Run, "action %v does nothing", action.Path)
		key := ""
		for _, word := range action.Path {
			key += " " + word
		}
		assert.Falsef(t, paths[key], "two actions named%s", key)
		paths[key] = true
	}

	// The verbs a pool driver and an operator reach for.
	for _, want := range []string{" add disk", " del disk", " resize disk", " map disk", " unmap disk", " get volumes"} {
		assert.Truef(t, paths[want], "no action named%s", want)
	}

	// The tree hangs them off one word each, not one branch per action.
	var names []string
	for _, cmd := range root.Commands() {
		names = append(names, cmd.Name())
	}
	assert.Contains(t, names, "add")
	assert.Contains(t, names, "get")
}

// TestAMappingReachesTheDriverWhole is the collector's shape: several
// mappings, each holding commas of its own.
func TestAMappingReachesTheDriverWhole(t *testing.T) {
	var got []string
	action := array.Action{
		Path:  []string{"add", "disk"},
		Short: "add",
		Flags: []array.Flag{array.FlagMapping},
		Run: func(_ context.Context, in array.Input) (any, error) {
			got = in.StringSlice(array.FlagMapping.Name)
			return nil, nil
		},
	}
	err := array.RunActions(context.Background(), []array.Action{action},
		[]string{"add", "disk", "--mappings", "iqn.a:t1,t2", "--mappings", "iqn.b:t3"}, &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, []string{"iqn.a:t1,t2", "iqn.b:t3"}, got)
}

// TestTheV2AddDiskSyntaxIsRead pins that the add_disk command line of the v2
// agent runs here unchanged: the same option names, and the same option set.
func TestTheV2AddDiskSyntaxIsRead(t *testing.T) {
	var got OptAddDisk
	var addDisk array.Action
	for _, action := range (&Array{}).Actions() {
		if len(action.Path) == 2 && action.Path[0] == "add" && action.Path[1] == "disk" {
			addDisk = action
		}
	}
	require.NotNil(t, addDisk.Run, "no add disk action")

	// Replace the body, so this reads what the parser produced rather than
	// talking to an array.
	addDisk.Run = func(_ context.Context, in array.Input) (any, error) {
		got = OptAddDisk{
			Volume: OptAddVolume{
				Name:          in.String(array.FlagName.Name),
				Size:          in.String(array.FlagSize.Name),
				PoolId:        in.String(array.FlagPool.Name),
				Compression:   in.Bool(flagCompression.Name),
				Deduplication: in.Bool(flagDeduplication.Name),
			},
			Mapping: OptMapping{
				Mappings:          in.StringSlice(array.FlagMapping.Name),
				HostGroupNames:    in.StringSlice(array.FlagTarget.Name),
				LUN:               in.Int(array.FlagLUN.Name),
				VolumeIdRangeFrom: in.Int(flagVolumeIDRangeFrom.Name),
				VolumeIdRangeTo:   in.Int(flagVolumeIDRangeTo.Name),
			},
		}
		return nil, nil
	}

	// The option names and set of the v2 hcs add_disk action.
	args := []string{"add", "disk",
		"--name", "d1",
		"--pool", "p1",
		"--size", "1g",
		"--target", "tgt1",
		"--mappings", "iqn.a:t1,t2",
		"--lun", "3",
		"--compression",
		"--dedup",
		"--start-ldev-id", "10",
		"--end-ldev-id", "20",
		"--resource-group", "rg1",
	}
	require.NoError(t, array.RunActions(context.Background(), []array.Action{addDisk}, args, &bytes.Buffer{}))

	assert.Equal(t, "d1", got.Volume.Name)
	assert.Equal(t, "p1", got.Volume.PoolId)
	assert.Equal(t, "1g", got.Volume.Size)
	assert.True(t, got.Volume.Compression)
	assert.True(t, got.Volume.Deduplication)
	assert.Equal(t, []string{"tgt1"}, got.Mapping.HostGroupNames)
	assert.Equal(t, []string{"iqn.a:t1,t2"}, got.Mapping.Mappings)
	assert.Equal(t, 3, got.Mapping.LUN)
	assert.Equal(t, 10, got.Mapping.VolumeIdRangeFrom)
	assert.Equal(t, 20, got.Mapping.VolumeIdRangeTo)
}

// TestNoOptionV2DoesNotHave pins that this driver reads the v2 names and no
// others: an option v2 never had is refused rather than quietly accepted.
func TestNoOptionV2DoesNotHave(t *testing.T) {
	actions := (&Array{}).Actions()
	for _, args := range [][]string{
		{"add", "disk", "--pool-id", "p1"},
		{"add", "disk", "--deduplication"},
		{"add", "disk", "--from", "10"},
		{"add", "disk", "--to", "20"},
		{"add", "disk", "--mapping", "iqn.a:t1"},
		{"add", "disk", "--hostGroup", "hg1"},
		{"del", "disk", "--serial", "s1"},
		{"resize", "disk", "--serial", "s1"},
	} {
		err := array.RunActions(context.Background(), actions, args, &bytes.Buffer{})
		assert.Errorf(t, err, "args %v must be refused", args)
	}
}

// TestAnUnsetVSMIDAsksWhatV2Asks pins the answer to "does the default of an
// option v2 does not have change what add disk does".
//
// It does not: the option adds a field to the volume creation request only
// when it is set, so an add disk that leaves it alone sends the body v2 sends.
func TestAnUnsetVSMIDAsksWhatV2Asks(t *testing.T) {
	// The options a v2 add_disk command line can set, and nothing else.
	v2 := OptAddVolume{Name: "d1", Size: "1g", PoolId: "p1"}

	data := addVolumeData(v2, 1024)
	assert.NotContains(t, data, "virtualStorageMachineId",
		"an unset vsm-id must add nothing to the request")
	assert.Equal(t, map[string]string{
		"poolId":          "p1",
		"capacityInBytes": "1024",
		"label":           "d1",
	}, data)

	// The same, with the compression and dedup v2 also has.
	withSaving := v2
	withSaving.Compression = true
	withSaving.Deduplication = true
	data = addVolumeData(withSaving, 1024)
	assert.NotContains(t, data, "virtualStorageMachineId")
	assert.Equal(t, "DEDUPLICATION_AND_COMPRESSION", data["dkcDataSavingType"])

	// Set, it is the one thing that changes.
	withVSM := v2
	withVSM.VirtualStorageMachineId = "vsm1"
	data = addVolumeData(withVSM, 1024)
	assert.Equal(t, "vsm1", data["virtualStorageMachineId"])
	delete(data, "virtualStorageMachineId")
	assert.Equal(t, addVolumeData(v2, 1024), data, "nothing else moves")
}
