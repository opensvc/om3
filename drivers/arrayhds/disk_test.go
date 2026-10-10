package arrayhds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/object"
)

// The fixtures in testdata are synthesized, not captured from an array: the
// xml ones say so in a comment, and the text ones, which can not hold one,
// are written in the indented format v2's parser reads, with the keys v2
// reads from them.

const (
	testURL      = "https://hdvm.example:2001/service"
	testUsername = "om"
	testPassword = "s3cret"
	testBin      = "/opt/HiCommand/HiCommandCLI"

	// hba1 sees both ports, through domain 1 of each.
	hba1 = "10000000c9aabb01"
	cl1a = "50060e8007c37500"
	cl2a = "50060e8007c37510"
)

type (
	// step is one manager command a test expects, and what the fake manager
	// answers it with.
	step struct {
		// want is the command, its scope and its xml format checked and left
		// out: "GetStorageArray subtarget=Pool".
		want string

		// fixture is the file of testdata answered on stdout.
		fixture string

		// stderr is what the manager writes on stderr.
		stderr string

		// fail makes the manager exit with an error.
		fail bool
	}

	// fakeCLI is a manager answering a script of commands, in order, and
	// refusing any other.
	fakeCLI struct {
		t     *testing.T
		steps []step
		calls []string
		argv  [][]string
		env   [][]string
	}
)

func (f *fakeCLI) run(_ context.Context, name string, args, env []string) ([]byte, []byte, error) {
	f.t.Helper()
	assert.Equal(f.t, testBin, name, "the bin keyword names the manager cli")
	f.argv = append(f.argv, args)
	f.env = append(f.env, env)
	got := f.command(args)
	f.calls = append(f.calls, got)
	i := len(f.calls) - 1
	if i >= len(f.steps) {
		f.t.Errorf("unexpected command %d: %s", i, got)
		return nil, []byte("unexpected command"), errors.New("exit status 1")
	}
	s := f.steps[i]
	if s.want != got {
		f.t.Errorf("command %d: got %q, want %q", i, got, s.want)
		return nil, []byte("unexpected command"), errors.New("exit status 1")
	}
	var out []byte
	if s.fixture != "" {
		b, err := os.ReadFile(filepath.Join("testdata", s.fixture))
		require.NoError(f.t, err)
		out = b
	}
	if s.fail {
		return out, []byte(s.stderr), errors.New("exit status 1")
	}
	return out, []byte(s.stderr), nil
}

// command renders a manager command line the way the steps are written, after
// checking what every command carries: the url, the credentials, the scope of
// the array, and the xml format for a query and for nothing else.
func (f *fakeCLI) command(args []string) string {
	f.t.Helper()
	require.GreaterOrEqual(f.t, len(args), 6)
	assert.Equal(f.t, testURL, args[0])
	assert.Equal(f.t, []string{"-u", testUsername, "-p", testPassword}, args[2:6])
	cmd, rest := args[1], args[6:]
	if cmd == "GetStorageArray" {
		require.GreaterOrEqual(f.t, len(rest), 2)
		assert.Equal(f.t, []string{"-f", "xml"}, rest[:2], "a query asks for xml")
		rest = rest[2:]
	} else {
		assert.NotContains(f.t, rest, "-f", "a change is answered in text")
	}
	require.GreaterOrEqual(f.t, len(rest), 2)
	assert.Equal(f.t, []string{"serialnum=210945", "model=HUS VM"}, rest[:2], "the command is scoped to the array")
	return strings.Join(append([]string{cmd}, rest[2:]...), " ")
}

// assertDone checks the script was run to its end.
func (f *fakeCLI) assertDone() {
	f.t.Helper()
	want := make([]string, len(f.steps))
	for i, s := range f.steps {
		want[i] = s.want
	}
	assert.Equal(f.t, want, append([]string{}, f.calls...), "the commands run")
}

// assertNoChange checks no command changing the array was run.
func (f *fakeCLI) assertNoChange() {
	f.t.Helper()
	for _, call := range f.calls {
		assert.Truef(f.t, strings.HasPrefix(call, "GetStorageArray "), "a command changing the array was run: %s", call)
	}
}

// newTestArray returns an array configured as a node configuration declares
// one, its manager the fake answering the steps.
func newTestArray(t *testing.T, steps ...step) (*Array, *fakeCLI) {
	t.Helper()
	a := newConfiguredArray(t, testBin)
	fake := &fakeCLI{t: t, steps: steps}
	a.cli = fake.run
	return a, fake
}

// newConfiguredArray returns an array configured as a node configuration
// declares one, running the manager cli the bin keyword names.
func newConfiguredArray(t *testing.T, bin string) *Array {
	t.Helper()
	config := `
[array#hds1]
type = hds
name = HUS VM.210945
url = ` + testURL + `
username = ` + testUsername + `
password = system/sec/hds1
bin = ` + bin + `
jre_path = /opt/java
`
	n, err := object.NewNode(object.WithConfigData([]byte(config)), object.WithVolatile(true))
	require.NoError(t, err)
	a := New()
	a.SetName("array#hds1")
	a.SetConfig(n.MergedConfig())
	a.secret = func() (string, error) { return testPassword, nil }
	return a
}

// runAction runs a command line through the tree the driver declares, as
// "om node array" runs it, and returns what it printed.
func runAction(t *testing.T, a *Array, args ...string) (map[string]any, error) {
	t.Helper()
	var out bytes.Buffer
	err := array.RunActions(context.Background(), a.Actions(), args, &out)
	if err != nil {
		return nil, err
	}
	data := make(map[string]any)
	require.NoErrorf(t, json.Unmarshal(out.Bytes(), &data), "the output is a json dict: %s", out.String())
	return data, nil
}

var (
	queryPools   = step{want: "GetStorageArray subtarget=Pool", fixture: "pools.xml"}
	queryDomains = step{want: "GetStorageArray subtarget=HostStorageDomain hsdsubinfo=WWN,Path", fixture: "domains.xml"}
	queryPorts   = step{want: "GetStorageArray subtarget=Port", fixture: "ports.xml"}
	queryLUs     = step{want: "GetStorageArray subtarget=Logicalunit lusubinfo=Path,LDEV,VolumeConnection", fixture: "lus.xml"}
)

// TestAddDiskAsTheCollectorQueuesIt walks the command the collector queues
// for a new disk: the pool and the mappings are resolved before anything is
// created, the volume is created, labelled and mapped, and read back by the
// display name the creation answered.
func TestAddDiskAsTheCollectorQueuesIt(t *testing.T) {
	a, fake := newTestArray(t,
		queryPools,
		queryDomains,
		queryPorts,
		step{want: "addvirtualvolume capacity=10485760 capacitytype=KB poolid=1", fixture: "addvirtualvolume.txt"},
		step{want: "modifylabel devnums=4660 label=svc1_disk0", fixture: "modifylabel.txt"},
		queryDomains,
		queryPorts,
		// Domain 1 of CL1-A hands out LUNs 0 and 1, so 2 is the first free on
		// both paths.
		step{want: "addlun devnum=4660 portname=CL1-A domain=1 lun=2", fixture: "addlun_cl1a.txt"},
		step{want: "addlun devnum=4660 portname=CL2-A domain=1 lun=2", fixture: "addlun_cl2a.txt"},
		step{want: "GetStorageArray subtarget=Logicalunit lusubinfo=Path,LDEV,VolumeConnection displayname=00:12:34", fixture: "lu_4660_mapped.xml"},
	)
	data, err := runAction(t, a, "add", "disk", "-a", "hds1",
		"--name", "svc1_disk0", "--pool", "dp_gold", "--size", "10GB",
		"--mappings", hba1+":"+cl1a+","+cl2a)
	require.NoError(t, err)
	fake.assertDone()

	// The full command line, once: the password reaches the cli, and the
	// java runtime path its environment.
	assert.Equal(t, []string{
		testURL, "addvirtualvolume", "-u", testUsername, "-p", testPassword,
		"serialnum=210945", "model=HUS VM",
		"capacity=10485760", "capacitytype=KB", "poolid=1",
	}, fake.argv[3])
	assert.Equal(t, []string{"HDVM_CLI_JRE_PATH=/opt/java"}, fake.env[3])

	// What v2 answered: the disk id is the end of the object id, the devid
	// the display name.
	assert.Equal(t, "210945.4660", data["disk_id"])
	assert.Equal(t, "00:12:34", data["disk_devid"])

	lu := data["driver_data"].(map[string]any)["lu"].(map[string]any)
	assert.Equal(t, "4660", lu["devNum"], "the volume read back is the one created, not the first answered")
	assert.Equal(t, "LU.HUSVM.210945.4660", lu["objectID"])
	assert.Equal(t, "10485760", lu["capacityInKB"])
	assert.Equal(t, "1", lu["dpPoolID"])
	assert.Equal(t, "OPEN-V", lu["emulation"], "every attribute is rendered, as v2 rendered them")
	assert.Equal(t, "svc1_disk0", lu["label"])
	paths := lu["Path"].([]any)
	require.Len(t, paths, 2)
	assert.Equal(t, map[string]any{
		"objectID": "PATH.HUSVM.210945.0.1.2", "devNum": "4660", "portName": "CL1-A", "domainID": "1", "LUN": "2",
	}, paths[0])

	// Every initiator of the domains on every port the volume is mapped
	// through.
	assert.Equal(t, map[string]any{
		hba1 + ":" + cl1a:          map[string]any{"hba_id": hba1, "tgt_id": cl1a, "lun": float64(2)},
		hba1 + ":" + cl2a:          map[string]any{"hba_id": hba1, "tgt_id": cl2a, "lun": float64(2)},
		"10000000c9aabb02:" + cl1a: map[string]any{"hba_id": "10000000c9aabb02", "tgt_id": cl1a, "lun": float64(2)},
		"10000000c9aabb02:" + cl2a: map[string]any{"hba_id": "10000000c9aabb02", "tgt_id": cl2a, "lun": float64(2)},
	}, data["mappings"])

	// The log v2 returned: each change with its password masked, and what
	// the manager answered it with.
	log := data["log"].([]any)
	require.NotEmpty(t, log)
	assert.Equal(t, []any{float64(0),
		testBin + " " + testURL + " addvirtualvolume -u " + testUsername + " -p xxxx serialnum=210945 model=HUS VM capacity=10485760 capacitytype=KB poolid=1",
		map[string]any{}}, log[0])
	assert.Equal(t, []any{float64(0), "RESPONSE:", map[string]any{}}, log[1])
	b, _ := json.Marshal(data)
	assert.NotContains(t, string(b), testPassword)
	commands := 0
	for _, entry := range log {
		if strings.HasPrefix(entry.([]any)[1].(string), testBin+" ") {
			commands++
		}
	}
	assert.Equal(t, 4, commands, "the log holds the changes and no query")
}

// TestAddDiskFailingAfterTheCreationNamesTheVolume pins that a volume created
// is never lost track of: the error names it, and it is left in place rather
// than deleted on the way out.
func TestAddDiskFailingAfterTheCreationNamesTheVolume(t *testing.T) {
	a, fake := newTestArray(t,
		queryPools,
		queryDomains,
		queryPorts,
		step{want: "addvirtualvolume capacity=10485760 capacitytype=KB poolid=1", fixture: "addvirtualvolume.txt"},
		step{want: "modifylabel devnums=4660 label=svc1_disk0", fixture: "modifylabel.txt"},
		queryDomains,
		queryPorts,
		step{want: "addlun devnum=4660 portname=CL1-A domain=1 lun=2", fixture: "addlun_cl1a.txt"},
		step{want: "addlun devnum=4660 portname=CL2-A domain=1 lun=2", fail: true, stderr: "KAIC05234-E The LUN is already in use."},
	)
	_, err := runAction(t, a, "add", "disk", "-a", "hds1",
		"--name", "svc1_disk0", "--pool", "dp_gold", "--size", "10g",
		"--mappings", hba1+":"+cl1a+","+cl2a)
	require.Error(t, err)
	fake.assertDone()

	var created *createdError
	require.ErrorAs(t, err, &created)
	assert.Contains(t, err.Error(), "devnum 4660")
	assert.Contains(t, err.Error(), "displayname 00:12:34")
	assert.Contains(t, err.Error(), "left in place")
	assert.Contains(t, err.Error(), "KAIC05234-E", "the manager's message is quoted")
	assert.Contains(t, err.Error(), "port CL1-A domain 1 lun 2", "the path made before the failure is named")
	for _, call := range fake.calls {
		assert.NotContains(t, call, "deletevirtualvolume")
	}
}

// TestAddDiskRefusesAMappingBeforeCreatingAnything pins that a mapping the
// array can not make fails the command before a volume is left behind.
func TestAddDiskRefusesAMappingBeforeCreatingAnything(t *testing.T) {
	for name, mapping := range map[string]string{
		"an unknown target":                        hba1 + ":50060e8007c375ff",
		"an initiator no domain of the port knows": "10000000c9ffff01:" + cl1a,
		"an initiator of another port's domain":    "10000000c9ccdd01:" + cl1a,
	} {
		t.Run(name, func(t *testing.T) {
			a, fake := newTestArray(t, queryPools, queryDomains, queryPorts)
			_, err := runAction(t, a, "add", "disk", "-a", "hds1",
				"--name", "svc1_disk0", "--pool", "dp_gold", "--size", "10g", "--mappings", mapping)
			require.Error(t, err)
			fake.assertDone()
			fake.assertNoChange()
		})
	}
}

// TestAddDiskRefusesWhatItCanNotCreateExactly pins the checks made before
// the creation: the pool exists, and the size is an absolute whole number of
// KB.
func TestAddDiskRefusesWhatItCanNotCreateExactly(t *testing.T) {
	for name, tc := range map[string]struct {
		args  []string
		steps []step
	}{
		"no such pool":  {[]string{"--pool", "dp_bronze", "--size", "10g"}, []step{queryPools}},
		"relative size": {[]string{"--pool", "dp_gold", "--size", "+10g"}, nil},
		"negative size": {[]string{"--pool", "dp_gold", "--size", "-10g"}, nil},
		"partial KB":    {[]string{"--pool", "dp_gold", "--size", "1000"}, nil},
		"no size":       {[]string{"--pool", "dp_gold"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			a, fake := newTestArray(t, tc.steps...)
			_, err := runAction(t, a, append([]string{"add", "disk", "-a", "hds1", "--name", "d"}, tc.args...)...)
			require.Error(t, err)
			fake.assertDone()
			fake.assertNoChange()
		})
	}
}

// TestResizeDiskGrowsTheVolumeOfTheDevnum pins the relative resize the
// collector queues: the volume is picked from the whole list by its device
// number, not taken first from a narrowed query.
func TestResizeDiskGrowsTheVolumeOfTheDevnum(t *testing.T) {
	a, fake := newTestArray(t,
		queryLUs,
		step{want: "modifyvirtualvolume capacity=11534336 capacitytype=KB devnums=4660"},
		step{want: "GetStorageArray subtarget=Logicalunit lusubinfo=Path,LDEV,VolumeConnection displayname=00:12:34", fixture: "lu_4660_grown.xml"},
	)
	data, err := runAction(t, a, "resize", "disk", "-a", "hds1", "--devnum", "00:12:34", "--size", "+1g")
	require.NoError(t, err)
	fake.assertDone()
	lu := data["driver_data"].(map[string]any)["lu"].(map[string]any)
	assert.Equal(t, "4660", lu["devNum"])
	assert.Equal(t, "11534336", lu["capacityInKB"])
	assert.Equal(t, "00:12:34", data["disk_devid"])
}

// TestResizeDiskToAnAbsoluteSize pins that an absolute size is the new size,
// in binary units as v2 and the collector read "GB".
func TestResizeDiskToAnAbsoluteSize(t *testing.T) {
	a, fake := newTestArray(t,
		queryLUs,
		step{want: "modifyvirtualvolume capacity=11534336 capacitytype=KB devnums=4660"},
		step{want: "GetStorageArray subtarget=Logicalunit lusubinfo=Path,LDEV,VolumeConnection displayname=00:12:34", fixture: "lu_4660_grown.xml"},
	)
	_, err := runAction(t, a, "resize", "disk", "-a", "hds1", "--devnum", "4660", "--size", "11GB")
	require.NoError(t, err)
	fake.assertDone()
}

// TestResizeDiskRefusesAShrink pins that a size below the current one, which
// drops the end of the volume, is refused unless truncating is asked for.
func TestResizeDiskRefusesAShrink(t *testing.T) {
	a, fake := newTestArray(t, queryLUs)
	_, err := runAction(t, a, "resize", "disk", "-a", "hds1", "--devnum", "00:12:34", "--size", "5g")
	require.Error(t, err)
	assert.ErrorIs(t, err, array.ErrShrink)
	assert.Contains(t, err.Error(), "4660")
	fake.assertDone()
	fake.assertNoChange()

	// The same, allowed.
	a, fake = newTestArray(t,
		queryLUs,
		step{want: "modifyvirtualvolume capacity=5242880 capacitytype=KB devnums=4660"},
		step{want: "GetStorageArray subtarget=Logicalunit lusubinfo=Path,LDEV,VolumeConnection displayname=00:12:34", fixture: "lus.xml"},
	)
	_, err = runAction(t, a, "resize", "disk", "-a", "hds1", "--devnum", "00:12:34", "--size", "5g", "--truncate")
	require.Error(t, err, "the fake array still reports 10g after the shrink, which is not the size asked for")
	assert.Contains(t, err.Error(), "modifyvirtualvolume to 5242880 KB succeeded")
	fake.assertDone()
}

// TestResizeDiskToTheCurrentSizeChangesNothing pins that a resize to the size
// the volume has is done, without asking the array to change.
func TestResizeDiskToTheCurrentSizeChangesNothing(t *testing.T) {
	a, fake := newTestArray(t, queryLUs)
	_, err := runAction(t, a, "resize", "disk", "-a", "hds1", "--devnum", "00:12:34", "--size", "10g")
	require.NoError(t, err)
	fake.assertDone()
	fake.assertNoChange()
}

// TestResizeDiskOfNoSuchVolume pins that a device number the array does not
// list is an error, whatever the first volume listed.
func TestResizeDiskOfNoSuchVolume(t *testing.T) {
	a, fake := newTestArray(t, queryLUs)
	_, err := runAction(t, a, "resize", "disk", "-a", "hds1", "--devnum", "00:12:35", "--size", "+1g")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "devnum 4661")
	fake.assertDone()
	fake.assertNoChange()
}

// TestCommandsRefuseADevnumTheyCanNotRead pins that a device the driver can
// not read changes nothing, on any of the commands the collector queues.
func TestCommandsRefuseADevnumTheyCanNotRead(t *testing.T) {
	for _, devnum := range []string{"abc", "0064", "0:12:34", "00:12:3g"} {
		for _, args := range [][]string{
			{"del", "disk", "--devnum", devnum},
			{"resize", "disk", "--devnum", devnum, "--size", "+1g"},
			{"add", "map", "--devnum", devnum, "--mappings", hba1 + ":" + cl1a},
		} {
			a, fake := newTestArray(t)
			_, err := runAction(t, a, append(args, "-a", "hds1")...)
			assert.Errorf(t, err, "%v", args)
			assert.Emptyf(t, fake.calls, "%v", args)
		}
	}
}

// TestADiskIDResolvesToItsVolume pins that the disk id "add disk" answers
// resolves back to the volume it was answered for, through its object id,
// and that a disk id of no volume is an error.
func TestADiskIDResolvesToItsVolume(t *testing.T) {
	a, fake := newTestArray(t,
		queryLUs,
		queryLUs,
		step{want: "modifyvirtualvolume capacity=11534336 capacitytype=KB devnums=4660"},
		step{want: "GetStorageArray subtarget=Logicalunit lusubinfo=Path,LDEV,VolumeConnection displayname=00:12:34", fixture: "lu_4660_grown.xml"},
	)
	_, err := runAction(t, a, "resize", "disk", "-a", "hds1", "--devnum", "210945.4660", "--size", "+1g")
	require.NoError(t, err)
	fake.assertDone()

	for _, devnum := range []string{"210945.1234", "999999.4660", "210945.00:12:34"} {
		a, fake = newTestArray(t, queryLUs)
		_, err = runAction(t, a, "del", "disk", "-a", "hds1", "--devnum", devnum)
		assert.Errorf(t, err, devnum)
		fake.assertDone()
		fake.assertNoChange()
	}
}

// TestAWWIDResolvesToItsVolumeOfThisArray pins the wwid a host sees a volume
// as, read as the collector reads it: "60", the world wide name of a port of
// the array, then the ldkc, cu and ldev of the volume. A wwid of another
// array, or of a volume of another ldkc, is refused rather than read as the
// device of a volume of this array.
func TestAWWIDResolvesToItsVolumeOfThisArray(t *testing.T) {
	// cl1a 50060e8007c37500: characters 2 to 12 are 060e8007c3.
	const ofThisArray = "60060e8007c3" + "75000030290000"
	resize := func(devnum string) (*fakeCLI, error) {
		a, fake := newTestArray(t,
			queryPorts,
			queryLUs,
			queryLUs,
			step{want: "modifyvirtualvolume capacity=11534336 capacitytype=KB devnums=4660"},
			step{want: "GetStorageArray subtarget=Logicalunit lusubinfo=Path,LDEV,VolumeConnection displayname=00:12:34", fixture: "lu_4660_grown.xml"},
		)
		_, err := runAction(t, a, "resize", "disk", "-a", "hds1", "--devnum", devnum, "--size", "+1g")
		return fake, err
	}
	for _, wwid := range []string{ofThisArray + "001234", "3" + ofThisArray + "001234"} {
		fake, err := resize(wwid)
		require.NoError(t, err, wwid)
		fake.assertDone()
	}

	// Ldkc 01, cu 12, ldev 34: device 70196, which v2 read as 4660.
	a, fake := newTestArray(t, queryPorts, queryLUs)
	_, err := runAction(t, a, "del", "disk", "-a", "hds1", "--devnum", ofThisArray+"011234")
	require.Error(t, err)
	fake.assertDone()
	fake.assertNoChange()

	// The volume of another array.
	a, fake = newTestArray(t, queryPorts)
	_, err = runAction(t, a, "del", "disk", "-a", "hds1", "--devnum", "60060e8007ff"+"75000030290000"+"001234")
	require.ErrorContains(t, err, "is not of array")
	fake.assertDone()
	fake.assertNoChange()
}

// TestDelDiskUnmapsThenDeletes pins the delete the collector queues: every
// path to the volume is removed, then the volume.
func TestDelDiskUnmapsThenDeletes(t *testing.T) {
	a, fake := newTestArray(t,
		queryDomains,
		step{want: "deletelun devnum=100 portname=CL1-A domain=1"},
		step{want: "deletelun devnum=100 portname=CL2-A domain=1"},
		step{want: "deletevirtualvolume devnums=100"},
	)
	data, err := runAction(t, a, "del", "disk", "-a", "hds1", "--devnum", "00:00:64")
	require.NoError(t, err)
	fake.assertDone()
	assert.Equal(t, "100", data["devnum"])
	assert.Len(t, data["unmapped"], 2)
	assert.Len(t, data["log"], 3)
}

// TestDelDiskRefusesWhenNoDomainIsListed pins that an answer listing no
// domain, which is what an answer this driver can not read looks like, stops
// the delete before anything is changed.
func TestDelDiskRefusesWhenNoDomainIsListed(t *testing.T) {
	a, fake := newTestArray(t,
		step{want: queryDomains.want, fixture: "no_domains.xml"},
	)
	_, err := runAction(t, a, "del", "disk", "-a", "hds1", "--devnum", "00:00:64")
	require.Error(t, err)
	fake.assertDone()
	fake.assertNoChange()
}

// TestDelDiskFailingNamesWhatWasDone pins that a delete failing half way
// says which paths it removed.
func TestDelDiskFailingNamesWhatWasDone(t *testing.T) {
	a, fake := newTestArray(t,
		queryDomains,
		step{want: "deletelun devnum=100 portname=CL1-A domain=1"},
		step{want: "deletelun devnum=100 portname=CL2-A domain=1", fail: true, stderr: "KAIC00000-E denied"},
	)
	_, err := runAction(t, a, "del", "disk", "-a", "hds1", "--devnum", "100")
	require.Error(t, err)
	fake.assertDone()
	assert.Contains(t, err.Error(), "paths removed: port CL1-A domain 1")
	assert.Contains(t, err.Error(), "KAIC00000-E denied")
}

// TestAddMapAsTheCollectorQueuesIt pins the mapping of an existing volume: a
// path it already has is left alone, the others are made with the number
// free in every domain.
func TestAddMapAsTheCollectorQueuesIt(t *testing.T) {
	a, fake := newTestArray(t,
		queryDomains,
		queryPorts,
		// Volume 101 is already mapped through CL1-A domain 1.
		step{want: "addlun devnum=101 portname=CL2-A domain=1 lun=2", fixture: "addlun_cl2a.txt"},
	)
	data, err := runAction(t, a, "add", "map", "-a", "hds1", "--devnum", "00:00:65",
		"--mappings", hba1+":"+cl1a+","+cl2a)
	require.NoError(t, err)
	fake.assertDone()
	assert.Equal(t, "101", data["devnum"])
	require.Len(t, data["paths"], 1)
	assert.Equal(t, "CL2-A", data["paths"].([]any)[0].(map[string]any)["portName"])
	assert.NotEmpty(t, data["log"])
}

// TestAddMapSkipsAPairItCanNotServe pins v2's skip of a pair resolving to
// no port, or to no domain of its port, when the initiator is mapped through
// another: the collector names every target an initiator is zoned to.
func TestAddMapSkipsAPairItCanNotServe(t *testing.T) {
	a, fake := newTestArray(t,
		queryDomains,
		queryPorts,
		step{want: "addlun devnum=4660 portname=CL1-A domain=1 lun=2", fixture: "addlun_cl1a.txt"},
	)
	_, err := runAction(t, a, "add", "map", "-a", "hds1", "--devnum", "00:12:34",
		"--mappings", hba1+":"+cl1a+",50060e8007c375ff")
	require.NoError(t, err)
	fake.assertDone()
}

// TestAddMapRefusesWhatItCanNotMap pins that an initiator mapped through no
// domain at all maps nothing, as a volume reported mapped to a host that can
// not see it is a disk lost for that host.
func TestAddMapRefusesWhatItCanNotMap(t *testing.T) {
	for name, tc := range map[string]struct {
		mappings []string
		steps    []step
	}{
		"unknown target":                    {[]string{hba1 + ":50060e8007c375ff"}, []step{queryDomains, queryPorts}},
		"unknown initiator":                 {[]string{"10000000c9ffff01:" + cl1a}, []step{queryDomains, queryPorts}},
		"one initiator of two in no domain": {[]string{hba1 + ":" + cl1a, "10000000c9ffff01:" + cl1a}, []step{queryDomains, queryPorts}},
		"no domain listed":                  {[]string{hba1 + ":" + cl1a}, []step{{want: queryDomains.want, fixture: "no_domains.xml"}, queryPorts}},
		"no port listed":                    {[]string{hba1 + ":" + cl1a}, []step{queryDomains, {want: queryPorts.want, fixture: "no_domains.xml"}}},
	} {
		t.Run(name, func(t *testing.T) {
			a, fake := newTestArray(t, tc.steps...)
			args := []string{"add", "map", "-a", "hds1", "--devnum", "00:12:34"}
			for _, m := range tc.mappings {
				args = append(args, "--mappings", m)
			}
			_, err := runAction(t, a, args...)
			require.Error(t, err)
			fake.assertDone()
			fake.assertNoChange()
		})
	}
}

// TestAQueryAnswerNotXMLIsAnError pins that an answer this driver can not
// read stops the command, quoting it.
func TestAQueryAnswerNotXMLIsAnError(t *testing.T) {
	a, fake := newTestArray(t, step{want: queryLUs.want, fixture: "modifylabel.txt"})
	_, err := runAction(t, a, "resize", "disk", "-a", "hds1", "--devnum", "00:12:34", "--size", "+1g")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an xml tree")
	fake.assertDone()
	fake.assertNoChange()
}

// TestPortsAreReadByWorldWidePortName pins the attribute v2 and the
// collector read a port's world wide name from, and the one read when it is
// missing.
func TestPortsAreReadByWorldWidePortName(t *testing.T) {
	root, err := decodeXML(`<R><Port displayName="CL1-A" worldWidePortName="50.06.0E.80.07.C3.75.00"/><Port displayName="CL3-A" wwn="50060E8007C37520"/></R>`)
	require.NoError(t, err)
	ports := make([]port, 0)
	for _, e := range root.iter("Port") {
		ports = append(ports, newPort(e))
	}
	require.Len(t, ports, 2)
	assert.Equal(t, cl1a, ports[0].WWPN)
	assert.Equal(t, "50060e8007c37520", ports[1].WWPN)

	b, err := json.Marshal(ports[0])
	require.NoError(t, err)
	assert.JSONEq(t, `{"displayName":"CL1-A","worldWidePortName":"`+cl1a+`"}`, string(b))
}

// TestTheManagerCLIIsRunAsConfigured runs a fake HiCommandCLI, a script the
// bin keyword names, to pin what reaches the process and what is read back
// from it: the arguments, the java runtime path in its environment, and on a
// failure both of its outputs quoted in the error and kept in the log.
func TestTheManagerCLIIsRunAsConfigured(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "HiCommandCLI")
	script := `#!/bin/sh
echo "$@" >"` + filepath.Join(dir, "args") + `"
echo "jre=$HDVM_CLI_JRE_PATH"
echo "KAIC05555-E the volume is in use" >&2
exit 1
`
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o700))
	a := newConfiguredArray(t, bin)
	_, err := runAction(t, a, "del", "map", "-a", "hds1", "--devnum", "100",
		"--mappings", hba1+":"+cl1a)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subtarget=HostStorageDomain")
	assert.Contains(t, err.Error(), "stderr: KAIC05555-E the volume is in use")
	assert.Contains(t, err.Error(), "stdout: jre=/opt/java")
	assert.NotContains(t, err.Error(), testPassword)

	b, err := os.ReadFile(filepath.Join(dir, "args"))
	require.NoError(t, err)
	assert.Equal(t, testURL+" GetStorageArray -u om -p "+testPassword+" -f xml serialnum=210945 model=HUS VM subtarget=HostStorageDomain hsdsubinfo=WWN,Path\n", string(b))

	// A change failing is kept in the log, its stderr at level 1.
	a.journal = nil
	_, err = a.run(context.Background(), false, true, "deletelun", "devnum=100", "portname=CL1-A", "domain=1")
	require.Error(t, err)
	assert.Equal(t, []any{
		[]any{0, bin + " " + testURL + " deletelun -u om -p xxxx serialnum=210945 model=HUS VM devnum=100 portname=CL1-A domain=1", map[string]any{}},
		[]any{0, "jre=/opt/java", map[string]any{}},
		[]any{1, "KAIC05555-E the volume is in use", map[string]any{}},
	}, a.journal)
}
