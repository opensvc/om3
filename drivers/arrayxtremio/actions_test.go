package arrayxtremio

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
)

// TestActionsBuildATree pins that what this driver declares is a tree the
// parser can be built from, and that it holds the verbs the collector runs.
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

	// The verbs the collector runs on an xtremio array.
	for _, want := range []string{" add disk", " resize disk", " del disk"} {
		assert.Truef(t, paths[want], "no action named%s", want)
	}
	// And the rest of what v2 answers to.
	for _, want := range []string{" add map", " del map", " list volumes", " list initiators",
		" list initiator-groups", " list targets", " list target-groups", " list mappings"} {
		assert.Truef(t, paths[want], "no action named%s", want)
	}
}

// TestAddDiskTakesTheV2Options pins the option set of the action the collector
// runs, which is the one v2 declares for add_disk and no other.
func TestAddDiskTakesTheV2Options(t *testing.T) {
	var addDisk array.Action
	for _, action := range (&Array{}).Actions() {
		if len(action.Path) == 2 && action.Path[0] == "add" && action.Path[1] == "disk" {
			addDisk = action
		}
	}
	require.NotNil(t, addDisk.Run)

	got := make(map[string]bool)
	for _, flag := range addDisk.Flags {
		got[flag.Name] = true
	}
	// v2: name size blocksize tags alignment_offset small_io_alerts
	//     unaligned_io_alerts vaai_tp_alerts access mappings
	for _, want := range []string{"name", "size", "blocksize", "tag", "alignment-offset",
		"small-io-alerts", "unaligned-io-alerts", "vaai-tp-alerts", "access", "mappings"} {
		assert.Truef(t, got[want], "add disk must take --%s", want)
	}
	assert.Lenf(t, got, 10, "add disk must take the v2 options and no others: %v", got)
}

// TestReportsAreTheV2Sections pins the names the collector reads.
func TestReportsAreTheV2Sections(t *testing.T) {
	keys := make([]string, 0)
	for _, report := range (&Array{}).Reports() {
		require.NotNil(t, report.Get)
		keys = append(keys, report.Key)
	}
	assert.Equal(t, []string{"clusters_details", "volumes_details", "targets_details"}, keys)
}

// TestConvertHBAID pins how a port name is rendered for the array: a wwn is
// stored with a colon between each byte, an iqn is stored as it is.
func TestConvertHBAID(t *testing.T) {
	assert.Equal(t, "21:00:00:24:ff:0d:e4:4a", convertHBAID("210000 24ff0de44a"[0:6]+"24ff0de44a"))
	assert.Equal(t, "iqn.1993-08.org.debian:01:a1b2", convertHBAID("iqn.1993-08.org.debian:01:a1b2"))
	assert.Equal(t, "short", convertHBAID("short"))
}

// TestVolumePathNamesAVolumeAsTheArrayReadsIt pins that an index is a path
// element and a name is a parameter, which the array does not read the same
// way.
func TestVolumePathNamesAVolumeAsTheArrayReadsIt(t *testing.T) {
	path, params := volumePath("12")
	assert.Equal(t, "/volumes/12", path)
	assert.Empty(t, params)

	path, params = volumePath("d1")
	assert.Equal(t, "/volumes", path)
	assert.Equal(t, map[string]string{"name": "d1"}, params)
}

// TestConvertIDsSendsANumberedIDAsANumber pins that an id that is a number is
// sent as one, and one that is a name is left alone.
func TestConvertIDsSendsANumberedIDAsANumber(t *testing.T) {
	got := convertIDs(map[string]any{"vol-id": "12", "ig-id": "grp1", "vol-name": "42"})
	assert.Equal(t, 12, got["vol-id"])
	assert.Equal(t, "grp1", got["ig-id"])
	assert.Equal(t, "42", got["vol-name"], "only an id is converted")
}
