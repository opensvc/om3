package arrayhds

import (
	"bytes"
	"os"
	"strings"
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
		{" resize disk", []string{"devnum", "size", "truncate"}},
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
// collector commands depend on: the collector names a device by the display
// name "add disk" answered, a host names it by its wwid, and the array writes
// it with colons. The manager wants a decimal number.
func TestToDevnumReadsEveryWayADeviceIsNamed(t *testing.T) {
	cases := []struct {
		in       string
		expected string
	}{
		// The array's own notation, both lengths.
		{"00:00:64", "100"},
		{"00:64", "100"},
		{"01:23:45", "74565"},
		{"00:12:34", "4660"},
		{"00:00:c8", "200"},

		// A number already.
		{"100", "100"},
		{"0", "0"},
	}
	for _, tc := range cases {
		got, err := toDevnum(tc.in)
		require.NoErrorf(t, err, "toDevnum(%q)", tc.in)
		assert.Equalf(t, tc.expected, got, "toDevnum(%q)", tc.in)
	}
}

// TestToDevnumRefusesWhatItCannotRead pins that a value it does not read is
// an error, as v2 raised one, rather than passed to the manager: a device
// number misread is another volume deleted or resized.
func TestToDevnumRefusesWhatItCannotRead(t *testing.T) {
	for _, s := range []string{
		"",
		"abc",
		"not:hex",
		"0:1:2",                            // groups of one digit read as another device
		"00:00:00:64",                      // four groups
		"00:0g",                            // not hexadecimal
		"0064",                             // hexadecimal on the array, decimal for the manager
		"-1",                               // not a number
		"60060e80132b3f0050402b3f0000006z", // a wwid that is not hexadecimal
		"x.y",                              // a disk id is resolved through the array
		"60060e80132b3f0050402b3f00000064", // a wwid is resolved through the array
		"360060e80132b3f0050402b3f00000064",
		"210945.4660",
	} {
		_, err := toDevnum(s)
		assert.Errorf(t, err, "toDevnum(%q)", s)
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

// TestCreatedVolumeFindsTheDeviceAtAnyDepth pins how a creation is read: the
// manager answers with the device nested under the group it was carved from.
func TestCreatedVolumeFindsTheDeviceAtAnyDepth(t *testing.T) {
	out := `RESPONSE:
An instance of ArrayGroup
    objectID=ARRAYGROUP.210945.1
    List of 1 Lu elements
        An instance of Lu
            devNum=100
            displayName=00:00:64`
	devNum, displayName, err := createdVolume(out)
	require.NoError(t, err)
	assert.Equal(t, "100", devNum)
	assert.Equal(t, "00:00:64", displayName)
}

// TestCreatedVolumeRefusesAnAmbiguousAnswer pins that the volume the rest of
// "add disk" acts on, and that the collector is told about, is never guessed.
func TestCreatedVolumeRefusesAnAmbiguousAnswer(t *testing.T) {
	for name, out := range map[string]string{
		"no device": "RESPONSE:\nAn instance of ArrayGroup\n    objectID=x",
		"two devices": `RESPONSE:
An instance of ArrayGroup
    List of 2 Lu elements
        An instance of Lu
            devNum=100
            displayName=00:00:64
        An instance of Lu
            devNum=101
            displayName=00:00:65`,
		"no display name": `RESPONSE:
An instance of Lu
    devNum=100`,
		// The collector names the volume by its display name to resize and
		// delete it, so one that reads back to another device is refused.
		"display name of another device": `RESPONSE:
An instance of Lu
    devNum=100
    displayName=00:00:65`,
		"display name with no colon": `RESPONSE:
An instance of Lu
    devNum=100
    displayName=0064`,
		"device number not a number": `RESPONSE:
An instance of Lu
    devNum=0x64
    displayName=00:00:64`,
	} {
		_, _, err := createdVolume(out)
		assert.Errorf(t, err, "%s: %s", name, out)
	}
}

// TestParseKeepsNamesAsText pins that a display name is not read as a number,
// which would drop its leading zeros.
func TestParseKeepsNamesAsText(t *testing.T) {
	data := parse("RESPONSE:\nAn instance of Lu\n    devNum=100\n    displayName=0064\n    label=007")
	require.Len(t, data, 1)
	assert.Equal(t, "0064", data[0]["displayName"])
	assert.Equal(t, "007", data[0]["label"])
	assert.Equal(t, int64(100), data[0]["devNum"])
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

// TestToKBRefusesWhatIsNotAWholeKB pins that a size is never rounded down to
// a volume smaller than asked for.
func TestToKBRefusesWhatIsNotAWholeKB(t *testing.T) {
	for _, tc := range []struct {
		in       string
		expected int64
	}{
		{"1g", 1048576},
		{"10GB", 10485760},
		{"100mib", 102400},
		{"1024", 1},
	} {
		size, err := array.ParseSize(tc.in)
		require.NoErrorf(t, err, "ParseSize(%q)", tc.in)
		got, err := toKB(size.Bytes)
		require.NoErrorf(t, err, "toKB(%q)", tc.in)
		assert.Equalf(t, tc.expected, got, "toKB(%q)", tc.in)
	}
	for _, b := range []int64{0, -1024, 1000, 1025} {
		_, err := toKB(b)
		assert.Errorf(t, err, "toKB(%d)", b)
	}
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

// TestParseSkipsWhatComesBeforeTheAnswer pins that a line before the
// "RESPONSE:" one, as a blank line or a warning, does not hide the answer.
func TestParseSkipsWhatComesBeforeTheAnswer(t *testing.T) {
	b, err := os.ReadFile("testdata/addvirtualvolume.txt")
	require.NoError(t, err)
	for _, head := range []string{"", "\n", "KAIC12345-W A warning.\n\n"} {
		devNum, displayName, err := createdVolume(head + string(b))
		require.NoErrorf(t, err, "%q", head)
		assert.Equal(t, "4660", devNum)
		assert.Equal(t, "00:12:34", displayName)
	}
}

// TestCreatedVolumeTellsVolumesByDeviceNumber pins that an instance nested
// in the volume naming its device number again is not a second volume,
// while a second device number is.
func TestCreatedVolumeTellsVolumesByDeviceNumber(t *testing.T) {
	b, err := os.ReadFile("testdata/addvirtualvolume.txt")
	require.NoError(t, err)
	nested := strings.Replace(string(b), "                    dpPoolID=1", `                    dpPoolID=1
                    List of 1 LDEV elements:
                        An instance of LDEV
                            devNum=4660
                            displayName=00:12:34`, 1)
	devNum, displayName, err := createdVolume(nested)
	require.NoError(t, err)
	assert.Equal(t, "4660", devNum)
	assert.Equal(t, "00:12:34", displayName)

	other := strings.Replace(nested, `                            devNum=4660
                            displayName=00:12:34`, `                            devNum=4661
                            displayName=00:12:35`, 1)
	_, _, err = createdVolume(other)
	require.ErrorContains(t, err, "names 2 volumes")
}
