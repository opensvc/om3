package arrayhds

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
)

// TestActionsBuildATree pins that what this driver declares is a tree the
// parser can be built from, and that it holds v2's actions, the four the
// collector runs among them.
func TestActionsBuildATree(t *testing.T) {
	actions := (&Array{}).Actions()
	require.NotEmpty(t, actions)
	_, err := array.NewCommand(actions, &bytes.Buffer{})
	require.NoError(t, err)

	paths := make(map[string]array.Action)
	for _, action := range actions {
		require.NotEmptyf(t, action.Short, "action %v has no help", action.Path)
		require.NotNilf(t, action.Run, "action %v does nothing", action.Path)
		key := ""
		for _, word := range action.Path {
			key += " " + word
		}
		_, seen := paths[key]
		assert.Falsef(t, seen, "two actions named%s", key)
		paths[key] = action
	}
	// v2: add_disk add_map del_disk del_map rename_disk resize_disk
	//     list_logicalunits
	for _, want := range []string{
		" add disk", " add map", " del disk", " del map",
		" rename disk", " resize disk", " list logicalunits",
	} {
		_, ok := paths[want]
		assert.Truef(t, ok, "no action named%s", want)
	}

	// v2 declares no lun for add_disk, so neither does this.
	addDiskFlags := make(map[string]bool)
	for _, flag := range paths[" add disk"].Flags {
		addDiskFlags[flag.Name] = true
	}
	assert.Falsef(t, addDiskFlags["lun"], "add disk takes the v2 options and no others: %v", addDiskFlags)
	assert.Len(t, addDiskFlags, 4)

	// The option sets the collector writes.
	for _, tc := range []struct {
		path  string
		flags []string
	}{
		{" add disk", []string{"name", "pool", "size", "mappings"}},
		{" add map", []string{"devnum", "mappings", "lun"}},
		{" resize disk", []string{"devnum", "size"}},
		{" del disk", []string{"devnum"}},
	} {
		got := make(map[string]bool)
		for _, flag := range paths[tc.path].Flags {
			got[flag.Name] = true
		}
		for _, want := range tc.flags {
			assert.Truef(t, got[want], "%s must take --%s", tc.path, want)
		}
	}
}

// TestReportsAreTheV2Sections pins the names the collector reads.
func TestReportsAreTheV2Sections(t *testing.T) {
	keys := make([]string, 0)
	for _, report := range (&Array{}).Reports() {
		require.NotNil(t, report.Get)
		keys = append(keys, report.Key)
	}
	assert.Equal(t, []string{"array", "lu", "arraygroup", "port", "pool"}, keys)
}

// TestToDevnumReadsEveryWayADeviceIsNamed is the conversion three of the four
// collector commands depend on: the collector names a device
// "<serial>.<culd>", a host names it by its wwid, and the array writes it with
// colons. The manager wants a decimal number.
func TestToDevnumReadsEveryWayADeviceIsNamed(t *testing.T) {
	cases := []struct {
		in       string
		expected string
	}{
		// The array's own notation, both lengths.
		{"00:00:64", "100"},
		{"00:64", "100"},
		{"01:23:45", "74565"},

		// The collector inventory, "<serial>.<culd>".
		{"210945.0064", "100"},
		{"210945.00FF", "255"},

		// A wwid, of either length, whose last four characters are the device.
		{"60060e80132b3f0050402b3f00000064", "100"},
		{"360060e80132b3f0050402b3f00000064", "100"},

		// A number already.
		{"100", "100"},
		{"0", "0"},
	}
	for _, tc := range cases {
		assert.Equalf(t, tc.expected, toDevnum(tc.in), "toDevnum(%q)", tc.in)
	}
}

// TestToDevnumLeavesAloneWhatItCannotRead keeps a value it does not recognise
// rather than turning it into a device that exists.
func TestToDevnumLeavesAloneWhatItCannotRead(t *testing.T) {
	for _, s := range []string{"", "not:hex", "x.y", "abc"} {
		assert.Equalf(t, s, toDevnum(s), "toDevnum(%q)", s)
	}
}

// TestParseReadsWhatTheManagerAnswers covers the format the manager answers a
// change with, which is not the xml a query answers: instances opened by a
// line naming their type, and lists opened by a line counting their elements.
func TestParseReadsWhatTheManagerAnswers(t *testing.T) {
	out := `RESPONSE:
An instance of ArrayGroup
    objectID=ARRAYGROUP.210945.1
    chassis=1
    List of 1 Lu elements
        An instance of Lu
            objectID=LU.210945.100
            devNum=100
            displayName=00:00:64
            capacityInKB=10,485,760`

	data := parse(out)
	require.Len(t, data, 1)
	assert.Equal(t, "ARRAYGROUP.210945.1", data[0]["objectID"])
	assert.Equal(t, int64(1), data[0]["chassis"])

	lu, ok := data[0]["Lu"].([]map[string]any)
	require.Truef(t, ok, "the list is read and named by what it holds: %v", data[0])
	require.Len(t, lu, 1)
	assert.Equal(t, int64(100), lu[0]["devNum"])
	assert.Equal(t, "00:00:64", lu[0]["displayName"])
	assert.Equal(t, int64(10485760), lu[0]["capacityInKB"],
		"a capacity is written with commas and read as a number")
}

// TestParseReadsSeveralInstances pins that a marker line repeated is a list of
// instances, not one instance overwriting another.
func TestParseReadsSeveralInstances(t *testing.T) {
	out := `RESPONSE:
An instance of Lu
    devNum=100
An instance of Lu
    devNum=101`
	data := parse(out)
	require.Len(t, data, 2)
	assert.Equal(t, int64(100), data[0]["devNum"])
	assert.Equal(t, int64(101), data[1]["devNum"])
}

// TestParseReadsNothingFromNothing keeps an empty answer from being read as an
// instance with no keys.
func TestParseReadsNothingFromNothing(t *testing.T) {
	assert.Empty(t, parse(""))
	assert.Empty(t, parse("RESPONSE:\n"))
}

// TestDevNumOfAnswerFindsTheDeviceAtAnyDepth pins how a creation is read: the
// manager answers with the device nested under the group it was carved from.
func TestDevNumOfAnswerFindsTheDeviceAtAnyDepth(t *testing.T) {
	out := `RESPONSE:
An instance of ArrayGroup
    objectID=ARRAYGROUP.210945.1
    List of 1 Lu elements
        An instance of Lu
            devNum=100
            displayName=00:00:64`
	devNum, err := devNumOfAnswer(out)
	require.NoError(t, err)
	assert.Equal(t, "100", devNum)

	// An answer naming no device is an error, not an empty device.
	_, err = devNumOfAnswer("RESPONSE:\nAn instance of ArrayGroup\n    objectID=x")
	assert.Error(t, err)
}

// TestModelAndSerialAreReadFromTheName pins that the array is scoped the way
// v2 scopes it: the name is "<model>.<serial>".
func TestModelAndSerialAreReadFromTheName(t *testing.T) {
	a := New()
	a.SetName("array#HUS VM.210945")
	assert.Equal(t, "HUS VM", a.model())
	assert.Equal(t, "210945", a.serial())

	// A name with no dot is both.
	b := New()
	b.SetName("array#210945")
	assert.Equal(t, "210945", b.model())
	assert.Equal(t, "210945", b.serial())
}

// TestSizeKBIsWhatTheManagerReads pins the unit the manager is told a capacity
// in.
func TestSizeKBIsWhatTheManagerReads(t *testing.T) {
	for _, tc := range []struct{ in, expected string }{
		{"1g", "1048576"},
		{"100mib", "102400"},
		{"1024", "1"},
	} {
		got, err := sizeKB(tc.in)
		require.NoErrorf(t, err, "sizeKB(%q)", tc.in)
		assert.Equalf(t, tc.expected, got, "sizeKB(%q)", tc.in)
	}
	_, err := sizeKB("not a size")
	assert.Error(t, err)
}

// TestNormalizedWWNMatchesHowTheArrayStoresOne pins that a name written with
// dots, or in upper case, matches the one the array holds.
func TestNormalizedWWNMatchesHowTheArrayStoresOne(t *testing.T) {
	assert.Equal(t, "210000e08b0d1a2b", normalizedWWN("21.00.00.E0.8B.0D.1A.2B"))
	assert.Equal(t, "210000e08b0d1a2b", normalizedWWN("210000E08B0D1A2B"))
}

// TestFreeLUNIsTheLowestNoneOfTheDomainsHandsOut pins that a volume answers to
// the same number on every path to it.
func TestFreeLUNIsTheLowestNoneOfTheDomainsHandsOut(t *testing.T) {
	assert.Equal(t, 0, freeLUN(map[int]bool{}))
	assert.Equal(t, 2, freeLUN(map[int]bool{0: true, 1: true}))
	assert.Equal(t, 1, freeLUN(map[int]bool{0: true, 2: true}))
}
