package arraysymmetrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/object"
)

const (
	testSID  = "000197600001"
	testRSID = "000197600002"
	testWWN  = "60000970000197600001533030414243"
)

type (
	// fakeSymcli answers the symcli commands of the driver as an array
	// would, and records them, so a test pins the exact commands an action
	// sends to the array.
	fakeSymcli struct {
		t       *testing.T
		answers map[string][]fakeAnswer
		calls   []string

		// pairFiles is the content of the pair files the commands named,
		// read while the command ran, before the driver removed them.
		pairFiles []string
	}
	fakeAnswer struct {
		out  string
		err  string
		code int
	}
)

func newFakeSymcli(t *testing.T) *fakeSymcli {
	return &fakeSymcli{t: t, answers: make(map[string][]fakeAnswer)}
}

// on sets the answers to a command, written as "<bin> <args>" with the pair
// file path written <pairfile>. The answers are given in turn, the last
// one to every call after it.
func (f *fakeSymcli) on(cmd string, answers ...fakeAnswer) *fakeSymcli {
	f.answers[cmd] = append(f.answers[cmd], answers...)
	return f
}

func (f *fakeSymcli) ok(cmd, out string) *fakeSymcli {
	return f.on(cmd, fakeAnswer{out: out})
}

func (f *fakeSymcli) run(_ context.Context, bin string, args []string) ([]byte, []byte, int, error) {
	words := []string{bin}
	for i, arg := range args {
		if i > 0 && args[i-1] == "-f" {
			b, err := os.ReadFile(arg)
			require.NoError(f.t, err, "the pair file is there while the command runs")
			f.pairFiles = append(f.pairFiles, string(b))
			arg = "<pairfile>"
		}
		words = append(words, arg)
	}
	cmd := strings.Join(words, " ")
	f.calls = append(f.calls, cmd)
	l, ok := f.answers[cmd]
	if !ok || len(l) == 0 {
		f.t.Errorf("unexpected command: %s", cmd)
		return nil, []byte("unexpected command"), 99, nil
	}
	answer := l[0]
	if len(l) > 1 {
		f.answers[cmd] = l[1:]
	}
	return []byte(answer.out), []byte(answer.err), answer.code, nil
}

// newTestArray returns an array configured the way a node configuration
// configures one, whose symcli commands the fake answers.
func newTestArray(t *testing.T, fake *fakeSymcli) *Array {
	t.Helper()
	config := fmt.Sprintf(`
[array#sym1]
type = symmetrix
name = %s
symcli_path = /nonexistent/symcli
`, testSID)
	n, err := object.NewNode(object.WithConfigData([]byte(config)), object.WithVolatile(true))
	require.NoError(t, err)
	a := New()
	a.SetName("array#sym1")
	a.SetConfig(n.MergedConfig())
	if fake != nil {
		a.runner = fake.run
	}
	return a
}

// runAction runs a command line through the command tree of the array, as
// "om node array" does, and returns what it rendered.
func runAction(t *testing.T, a *Array, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := array.RunActions(context.Background(), a.Actions(), args, &buf)
	return buf.String(), err
}

func xmlDoc(body string) string {
	return `<?xml version="1.0" standalone="yes" ?>
<SymCLI_ML>
  <Symmetrix>
    <Symm_Info>
      <symid>` + testSID + `</symid>
    </Symm_Info>
` + body + `
  </Symmetrix>
</SymCLI_ML>
`
}

// rdfXML is the SRDF pairing of a R1 device 0ABCD with the R2 device 0BCDE
// of the remote array, in rdf group 1.
const rdfXML = `
      <RDF>
        <RDF_Info>
          <pair_state>Synchronized</pair_state>
          <suspend_state>N/A</suspend_state>
          <r1_invalids>0</r1_invalids>
          <WPace_Info>
            <pacing_capable>Yes</pacing_capable>
          </WPace_Info>
        </RDF_Info>
        <Mode>
          <mode>Synchronous</mode>
          <adaptive_copy_skew>65535</adaptive_copy_skew>
        </Mode>
        <Link>
          <configuration>Fibre</configuration>
        </Link>
        <Status>
          <rdf>Ready</rdf>
        </Status>
        <Local>
          <dev_name>0ABCD</dev_name>
          <type>R1</type>
          <ra_group_num>1</ra_group_num>
          <state>Ready</state>
        </Local>
        <Remote>
          <dev_name>0BCDE</dev_name>
          <remote_symid>` + testRSID + `</remote_symid>
          <wwn>60000970000197600002533030424344</wwn>
          <state>Write Disabled</state>
        </Remote>
      </RDF>`

// devShowXML is the symdev show output of a device of cyl cylinders.
func devShowXML(dev string, cyl int64, snapvxSource string, rdf string) string {
	return xmlDoc(fmt.Sprintf(`
    <Device>
      <Dev_Info>
        <dev_name>%s</dev_name>
        <configuration>TDEV</configuration>
        <status>Ready</status>
        <snapvx_source>%s</snapvx_source>
        <snapvx_target>False</snapvx_target>
        <SRP_name>SRP_1</SRP_name>
      </Dev_Info>
      <Product>
        <vendor>EMC</vendor>
        <symid>%s</symid>
        <wwn>%s</wwn>
      </Product>
      <Flags>
        <worm_protected>N/A</worm_protected>
        <aclx>False</aclx>
        <meta>None</meta>
      </Flags>
      <Capacity>
        <block_size>512</block_size>
        <cylinders>%d</cylinders>
        <kilobytes>%d</kilobytes>
        <megabytes>%d</megabytes>
        <gigabytes>10.0</gigabytes>
        <terabytes>0.01</terabytes>
      </Capacity>%s
    </Device>`, dev, snapvxSource, testSID, testWWN, cyl, cyl*cylinderKB, cyl*cylinderKB/1024, rdf))
}

func devWWNXML(dev string) string {
	return xmlDoc(fmt.Sprintf(`
    <Device>
      <symid>%s</symid>
      <dev_name>%s</dev_name>
      <pd_name>Not Visible</pd_name>
      <configuration>TDEV</configuration>
      <flags>
        <meta>None</meta>
      </flags>
      <wwn>%s</wwn>
    </Device>`, testSID, dev, testWWN))
}

// initiatorXML is the initiator group of the hba h1, in the views MV_A and
// MV_B.
var initiatorXML = xmlDoc(`
    <Initiator_Group>
      <Group_Info>
        <group_name>IG_H1</group_name>
        <init_count>N/A</init_count>
        <view_count>N/A</view_count>
        <consistent_lun>No</consistent_lun>
        <Mask_View_Names>
          <view_name>MV_B</view_name>
          <view_name>MV_A *</view_name>
        </Mask_View_Names>
      </Group_Info>
    </Initiator_Group>`)

// viewAXML presents the children SG_A and SG_A_GK of CSG_A to h1 and h9
// on the target t1, port 8, and holds the device 0ABCD at lun 00A.
var viewAXML = xmlDoc(`
    <Masking_View>
      <View_Info>
        <view_name>MV_A</view_name>
        <init_grpname>IG_A</init_grpname>
        <Initiator_List>
          <Initiator>
            <wwn>h1</wwn>
          </Initiator>
          <Initiator>
            <wwn>h9</wwn>
          </Initiator>
        </Initiator_List>
        <port_grpname>PG_A</port_grpname>
        <port_info>
          <Director_Identification>
            <dir>FA-1D</dir>
            <port>8</port>
            <port_wwn>t1</port_wwn>
          </Director_Identification>
        </port_info>
        <stor_grpname>CSG_A</stor_grpname>
        <SG_Child_info>
          <child_count>2</child_count>
          <SG>
            <group_name>SG_A_GK</group_name>
            <Status>IsChild</Status>
          </SG>
          <SG>
            <group_name>SG_A</group_name>
            <Status>IsChild</Status>
          </SG>
        </SG_Child_info>
        <Device>
          <dev_name>0ABCD</dev_name>
          <dev_port_info>
            <director>01D</director>
            <port>8</port>
            <host_lun>00A</host_lun>
          </dev_port_info>
        </Device>
      </View_Info>
    </Masking_View>`)

// viewBXML presents SG_B to h1 alone, on the target t2 only. Its storage
// group is narrower than those of MV_A, and it must not be chosen for a
// mapping on t1.
var viewBXML = xmlDoc(`
    <Masking_View>
      <View_Info>
        <view_name>MV_B</view_name>
        <init_grpname>IG_B</init_grpname>
        <Initiator_List>
          <Initiator>
            <wwn>h1</wwn>
          </Initiator>
        </Initiator_List>
        <port_grpname>PG_B</port_grpname>
        <port_info>
          <Director_Identification>
            <dir>FA-2D</dir>
            <port>9</port>
            <port_wwn>t2</port_wwn>
          </Director_Identification>
        </port_info>
        <stor_grpname>SG_B</stor_grpname>
      </View_Info>
    </Masking_View>`)

func sgShowXML(name, srp, slo string, gks int, devs string) string {
	return fmt.Sprintf(`<?xml version="1.0" standalone="yes" ?>
<SymCLI_ML>
  <SG>
    <SG_Info>
      <name>%s</name>
      <symid>%s</symid>
      <SLO_name>%s</SLO_name>
      <SRP_name>%s</SRP_name>
      <Num_of_GKS>%d</Num_of_GKS>
    </SG_Info>%s
  </SG>
</SymCLI_ML>
`, name, testSID, slo, srp, gks, devs)
}

// devSGsXML lists SG_A as the storage group of the device, the child of
// CSG_A, which MV_A presents.
var devSGsXML = xmlDoc(`
    <Storage_Group>
      <Group_Info>
        <group_name>CSG_A</group_name>
        <dev_count>N/A</dev_count>
        <Mask_View_Names>
          <view_name>MV_A</view_name>
        </Mask_View_Names>
        <Status>IsParent</Status>
      </Group_Info>
    </Storage_Group>
    <Storage_Group>
      <Group_Info>
        <group_name>SG_A</group_name>
        <dev_count>1</dev_count>
        <Mask_View_Names>
          <view_count>0</view_count>
        </Mask_View_Names>
        <Cascaded_View_Names>
          <view_count>1</view_count>
          <view_name>MV_A</view_name>
        </Cascaded_View_Names>
        <Status>IsChild</Status>
      </Group_Info>
    </Storage_Group>`)

func createdXML(dev string) string {
	return fmt.Sprintf(`STARTING a TDEV Create Device operation on Symm %s.
 The TDEV Create Device operation SUCCESSFULLY COMPLETED: 1 devices created.
1 TDEVs create requested in request 1 and devices created are 1[ %s ]
`, testSID, dev)
}

// onMappingResolution sets the answers of the commands resolving the
// storage group of the mapping h1:t1, with the pool filter of SRP_1.
func (f *fakeSymcli) onMappingResolution() *fakeSymcli {
	return f.
		ok("symaccess -sid "+testSID+" -output xml_e list -type initiator -wwn h1", initiatorXML).
		ok("symaccess -sid "+testSID+" -output xml_e show view MV_A -detail", viewAXML).
		ok("symaccess -sid "+testSID+" -output xml_e show view MV_B -detail", viewBXML).
		ok("symsg -sid "+testSID+" -output xml_e show SG_A", sgShowXML("SG_A", "SRP_1", "Diamond", 0, "")).
		ok("symsg -sid "+testSID+" -output xml_e show SG_A_GK", sgShowXML("SG_A_GK", "none", "none", 6, ""))
}

var mappingResolutionCalls = []string{
	"symaccess -sid " + testSID + " -output xml_e list -type initiator -wwn h1",
	"symaccess -sid " + testSID + " -output xml_e show view MV_A -detail",
	"symaccess -sid " + testSID + " -output xml_e show view MV_B -detail",
	"symsg -sid " + testSID + " -output xml_e show SG_A",
	"symsg -sid " + testSID + " -output xml_e show SG_A_GK",
}

// onDiskResult sets the answers of the commands reading the result of a
// disk added or mapped.
func (f *fakeSymcli) onDiskResult(dev string) *fakeSymcli {
	return f.
		ok("symdev -sid "+testSID+" -output xml_e list -devs "+dev+" -wwn", devWWNXML(dev)).
		ok("symaccess -sid "+testSID+" -output xml_e list -type storage -devs "+dev, devSGsXML)
}

func diskResultCalls(dev string) []string {
	return []string{
		"symdev -sid " + testSID + " -output xml_e list -devs " + dev + " -wwn",
		"symaccess -sid " + testSID + " -output xml_e list -type storage -devs " + dev,
		"symaccess -sid " + testSID + " -output xml_e show view MV_A -detail",
	}
}

// diskResultJSON is the result of an action adding or mapping 0ABCD, in
// v2's shape: the device is reached on t1 by the two initiators of MV_A.
const diskResultJSON = `{
	"disk_id": "` + testWWN + `",
	"disk_devid": "0ABCD",
	"dev_id": "0ABCD",
	"mappings": {
		"h1:t1": {"sg": "SG_A", "view_name": "MV_A", "hba_id": "h1", "tgt_id": "t1", "lun": "00A"},
		"h9:t1": {"sg": "SG_A", "view_name": "MV_A", "hba_id": "h9", "tgt_id": "t1", "lun": "00A"}
	},
	"driver_data": {
		"dev": {
			"symid": "` + testSID + `",
			"dev_name": "0ABCD",
			"pd_name": "Not Visible",
			"configuration": "TDEV",
			"flags": {"meta": "None"},
			"wwn": "` + testWWN + `"
		}
	}
}`

func calls(groups ...[]string) []string {
	var l []string
	for _, g := range groups {
		l = append(l, g...)
	}
	return l
}

// TestAddMapIsV2AddMap pins the hidden "add map" the collector queues: the
// device is put into the storage group the mapping reaches through a view
// presenting on the requested target, by its resolved name, and the result
// is v2's.
func TestAddMapIsV2AddMap(t *testing.T) {
	fake := newFakeSymcli(t).
		ok("symdev -sid "+testSID+" -output xml_e show -wwn "+testWWN, devShowXML("0ABCD", 5461, "False", "")).
		onMappingResolution().
		ok("symaccess -sid "+testSID+" -name SG_A -type storage add dev 0ABCD", "").
		onDiskResult("0ABCD")
	a := newTestArray(t, fake)

	out, err := runAction(t, a, "add", "map", "-a", testSID, "--dev", testWWN, "--mappings", "h1:t1", "--srp", "SRP_1")
	require.NoError(t, err)
	assert.Equal(t, calls(
		[]string{"symdev -sid " + testSID + " -output xml_e show -wwn " + testWWN},
		mappingResolutionCalls,
		[]string{"symaccess -sid " + testSID + " -name SG_A -type storage add dev 0ABCD"},
		diskResultCalls("0ABCD"),
	), fake.calls)
	assert.JSONEq(t, diskResultJSON, out)
}

// TestMappingStorageGroupIsChosenPerView is the bug the per-view port check
// fixes: the ports of the views of the initiator were gathered across the
// views, so MV_B, presenting on t2 only, offered its narrower SG_B to a
// mapping on t1. With no pool to filter on, the two groups of MV_A are as
// narrow, and the first by name is chosen, every time.
func TestMappingStorageGroupIsChosenPerView(t *testing.T) {
	for i := 0; i < 10; i++ {
		fake := newFakeSymcli(t).onMappingResolution()
		a := newTestArray(t, fake)
		mappings, err := array.ParseMappings([]string{"h1:t1"})
		require.NoError(t, err)
		sg, err := a.bestSG(context.Background(), testSID, mappings, "", "")
		require.NoError(t, err)
		assert.Equal(t, "SG_A", sg)
	}
}

// TestAddDiskWithSRDF pins the commands of a SRDF disk added the way the
// collector asks for it, and its result.
func TestAddDiskWithSRDF(t *testing.T) {
	fake := newFakeSymcli(t).
		ok("symcfg -sid "+testSID+" -output xml_e -rdfg 7 list", xmlDoc(`
    <RdfGroup>
      <ra_group_num>7</ra_group_num>
      <remote_ra_group_num>7</remote_ra_group_num>
      <remote_symid>`+testRSID+`</remote_symid>
    </RdfGroup>`)).
		onMappingResolution().
		ok("symdev -sid "+testSID+" create -tdev -N 1 -cap 5461 -captype cyl -emulation FBA -device_name d1 -noprompt -v", createdXML("0ABCD")).
		ok("symdev -sid "+testRSID+" create -tdev -N 1 -cap 5461 -captype cyl -emulation FBA -device_name d1 -noprompt -v", createdXML("0BCDE")).
		ok("symdev -sid "+testSID+" -output xml_e show 0ABCD", devShowXML("0ABCD", 5461, "False", "")).
		ok("symdev -sid "+testRSID+" -output xml_e show 0BCDE", devShowXML("0BCDE", 5461, "False", "")).
		ok("symrdf -sid "+testSID+" -f <pairfile> -rdfg 7 createpair -noprompt -rdf_mode sync -type R1 -establish", "").
		ok("symaccess -sid "+testSID+" -name SG_A -type storage add dev 0ABCD", "").
		onDiskResult("0ABCD")
	a := newTestArray(t, fake)

	out, err := runAction(t, a, "add", "disk", "-a", testSID, "--name", "d1", "--size", "10g",
		"--mappings", "h1:t1", "--srp", "SRP_1", "--srdf", "--rdfg", "7")
	require.NoError(t, err)
	assert.Equal(t, calls(
		[]string{"symcfg -sid " + testSID + " -output xml_e -rdfg 7 list"},
		mappingResolutionCalls,
		[]string{
			"symdev -sid " + testSID + " create -tdev -N 1 -cap 5461 -captype cyl -emulation FBA -device_name d1 -noprompt -v",
			"symdev -sid " + testSID + " -output xml_e show 0ABCD",
			"symdev -sid " + testRSID + " create -tdev -N 1 -cap 5461 -captype cyl -emulation FBA -device_name d1 -noprompt -v",
			"symdev -sid " + testRSID + " -output xml_e show 0BCDE",
			"symdev -sid " + testSID + " -output xml_e show 0ABCD",
			"symrdf -sid " + testSID + " -f <pairfile> -rdfg 7 createpair -noprompt -rdf_mode sync -type R1 -establish",
			"symaccess -sid " + testSID + " -name SG_A -type storage add dev 0ABCD",
		},
		diskResultCalls("0ABCD"),
	), fake.calls)
	assert.Equal(t, []string{"0ABCD 0BCDE\n"}, fake.pairFiles)
	assert.JSONEq(t, diskResultJSON, out)
}

// TestAddDiskCreatesNothingForAnUnservedMapping pins that the storage group
// is resolved before any device is created: a mapping no view serves
// creates nothing.
func TestAddDiskCreatesNothingForAnUnservedMapping(t *testing.T) {
	fake := newFakeSymcli(t).onMappingResolution()
	a := newTestArray(t, fake)

	out, err := runAction(t, a, "add", "disk", "-a", testSID, "--name", "d1", "--size", "10g", "--mappings", "h1:t3")
	require.ErrorContains(t, err, "no storage group found for the requested mappings h1:t3")
	assert.Empty(t, out)
	assert.Equal(t, mappingResolutionCalls[:3], fake.calls)
	for _, call := range fake.calls {
		assert.NotContains(t, call, " create ")
	}
}

func TestAddDiskRefusesSRDFWithoutRDFG(t *testing.T) {
	fake := newFakeSymcli(t)
	a := newTestArray(t, fake)
	_, err := runAction(t, a, "add", "disk", "-a", testSID, "--name", "d1", "--size", "10g", "--mappings", "h1:t1", "--srdf")
	require.ErrorContains(t, err, "--srdf is specified but --rdfg is not")
	assert.Empty(t, fake.calls)
}

// TestAddDiskNamesTheDevicesItCreated pins that a failure after the devices
// were created deletes none and names them all, so the operator can clean
// up.
func TestAddDiskNamesTheDevicesItCreated(t *testing.T) {
	fake := newFakeSymcli(t).
		ok("symcfg -sid "+testSID+" -output xml_e -rdfg 7 list", xmlDoc(`
    <RdfGroup>
      <ra_group_num>7</ra_group_num>
      <remote_symid>`+testRSID+`</remote_symid>
    </RdfGroup>`)).
		onMappingResolution().
		ok("symdev -sid "+testSID+" create -tdev -N 1 -cap 5461 -captype cyl -emulation FBA -device_name d1 -noprompt -v", createdXML("0ABCD")).
		ok("symdev -sid "+testRSID+" create -tdev -N 1 -cap 5461 -captype cyl -emulation FBA -device_name d1 -noprompt -v", createdXML("0BCDE")).
		ok("symdev -sid "+testSID+" -output xml_e show 0ABCD", devShowXML("0ABCD", 5461, "False", "")).
		ok("symdev -sid "+testRSID+" -output xml_e show 0BCDE", devShowXML("0BCDE", 5461, "False", "")).
		on("symrdf -sid "+testSID+" -f <pairfile> -rdfg 7 createpair -noprompt -rdf_mode sync -type R1 -establish", fakeAnswer{err: "The RDF group is offline", code: 1})
	a := newTestArray(t, fake)

	_, err := runAction(t, a, "add", "disk", "-a", testSID, "--name", "d1", "--size", "10g",
		"--mappings", "h1:t1", "--srp", "SRP_1", "--srdf", "--rdfg", "7")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "The RDF group is offline")
	assert.Contains(t, err.Error(), "R1 dev 0ABCD on array "+testSID+" and R2 dev 0BCDE on array "+testRSID)
	for _, call := range fake.calls {
		assert.NotContains(t, call, " delete ")
		assert.NotContains(t, call, " add dev ")
	}
}

func TestResize(t *testing.T) {
	show := "symdev -sid " + testSID + " -output xml_e show 0ABCD"
	t.Run("grow", func(t *testing.T) {
		fake := newFakeSymcli(t).
			ok(show, devShowXML("0ABCD", 5461, "False", "")).
			ok("symdev -sid "+testSID+" modify 0ABCD -tdev -cap 6007 -captype cyl -noprompt", "")
		out, err := runAction(t, newTestArray(t, fake), "resize", "disk", "-a", testSID, "--dev", "0ABCD", "--size", "+1g")
		require.NoError(t, err)
		assert.Equal(t, []string{show, "symdev -sid " + testSID + " modify 0ABCD -tdev -cap 6007 -captype cyl -noprompt"}, fake.calls)
		assert.JSONEq(t, `{"driver_data": {"pair_deleted": false}}`, out)
	})
	t.Run("refused shrink", func(t *testing.T) {
		fake := newFakeSymcli(t).ok(show, devShowXML("0ABCD", 5461, "False", ""))
		out, err := runAction(t, newTestArray(t, fake), "resize", "disk", "-a", testSID, "--dev", "0ABCD", "--size", "5g")
		require.ErrorIs(t, err, array.ErrShrink)
		assert.Contains(t, err.Error(), "--truncate")
		assert.Empty(t, out)
		assert.Equal(t, []string{show}, fake.calls)
	})
	for _, flag := range []string{"--truncate", "--force"} {
		t.Run("shrink with "+flag, func(t *testing.T) {
			fake := newFakeSymcli(t).
				ok(show, devShowXML("0ABCD", 5461, "False", "")).
				ok("symdev -sid "+testSID+" modify 0ABCD -tdev -cap 2730 -captype cyl -noprompt", "")
			_, err := runAction(t, newTestArray(t, fake), "resize", "disk", "-a", testSID, "--dev", "0ABCD", "--size", "5g", flag)
			require.NoError(t, err)
			assert.Equal(t, []string{show, "symdev -sid " + testSID + " modify 0ABCD -tdev -cap 2730 -captype cyl -noprompt"}, fake.calls)
		})
	}
	t.Run("a cylinder of another size is refused", func(t *testing.T) {
		doc := strings.Replace(devShowXML("0ABCD", 5461, "False", ""), fmt.Sprintf("<kilobytes>%d</kilobytes>", 5461*cylinderKB), fmt.Sprintf("<kilobytes>%d</kilobytes>", 5461*960), 1)
		fake := newFakeSymcli(t).ok(show, doc)
		_, err := runAction(t, newTestArray(t, fake), "resize", "disk", "-a", testSID, "--dev", "0ABCD", "--size", "+1g")
		require.ErrorContains(t, err, "KB per cylinder")
		assert.Equal(t, []string{show}, fake.calls)
	})
	t.Run("paired on a powermax", func(t *testing.T) {
		fake := newFakeSymcli(t).
			ok(show, devShowXML("0ABCD", 5461, "False", rdfXML)).
			ok("symcfg -sid "+testSID+" -output xml_e list", symcfgListXML("PowerMax_8000")).
			ok("symdev -sid "+testSID+" modify 0ABCD -tdev -cap 6007 -captype cyl -noprompt -rdfg 1", "")
		out, err := runAction(t, newTestArray(t, fake), "resize", "disk", "-a", testSID, "--dev", "0ABCD", "--size", "+1g")
		require.NoError(t, err)
		assert.Equal(t, []string{
			show,
			"symcfg -sid " + testSID + " -output xml_e list",
			"symdev -sid " + testSID + " modify 0ABCD -tdev -cap 6007 -captype cyl -noprompt -rdfg 1",
		}, fake.calls)
		var data map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &data))
		driverData := data["driver_data"].(map[string]any)
		assert.Equal(t, false, driverData["pair_deleted"])
		assertRDFStrings(t, driverData["rdf"])
	})
	t.Run("paired on a vmax", func(t *testing.T) {
		fake := newFakeSymcli(t).
			ok(show, devShowXML("0ABCD", 5461, "False", rdfXML)).
			ok("symcfg -sid "+testSID+" -output xml_e list", symcfgListXML("VMAX250F")).
			ok("symrdf -sid "+testSID+" -f <pairfile> -rdfg 1 suspend -noprompt", "").
			ok("symrdf -sid "+testSID+" -f <pairfile> -rdfg 1 deletepair -noprompt -force", "").
			ok("symdev -sid "+testSID+" modify 0ABCD -tdev -cap 6007 -captype cyl -noprompt", "")
		out, err := runAction(t, newTestArray(t, fake), "resize", "disk", "-a", testSID, "--dev", "0ABCD", "--size", "+1g")
		require.NoError(t, err)
		assert.Equal(t, []string{
			show,
			"symcfg -sid " + testSID + " -output xml_e list",
			"symrdf -sid " + testSID + " -f <pairfile> -rdfg 1 suspend -noprompt",
			"symrdf -sid " + testSID + " -f <pairfile> -rdfg 1 deletepair -noprompt -force",
			"symdev -sid " + testSID + " modify 0ABCD -tdev -cap 6007 -captype cyl -noprompt",
		}, fake.calls)
		assert.Equal(t, []string{"0ABCD 0BCDE\n", "0ABCD 0BCDE\n"}, fake.pairFiles)
		var data map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &data))
		driverData := data["driver_data"].(map[string]any)
		assert.Equal(t, true, driverData["pair_deleted"])
		assertRDFStrings(t, driverData["rdf"])
	})
}

func symcfgListXML(model string) string {
	return `<?xml version="1.0" standalone="yes" ?>
<SymCLI_ML>
  <Symmetrix>
    <Symm_Info>
      <symid>` + testSID + `</symid>
      <attachment>Local</attachment>
      <model>` + model + `</model>
      <microcode_version>5978</microcode_version>
    </Symm_Info>
  </Symmetrix>
</SymCLI_ML>
`
}

// assertRDFStrings pins the pairing the collector reads back: the values it
// joins into the commands it chains are the strings the array printed.
func assertRDFStrings(t *testing.T, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"RDF_Info": {"pair_state": "Synchronized", "suspend_state": "N/A", "r1_invalids": "0", "WPace_Info": {"pacing_capable": "Yes"}},
		"Mode": {"mode": "Synchronous", "adaptive_copy_skew": "65535"},
		"Link": {"configuration": "Fibre"},
		"Status": {"rdf": "Ready"},
		"Local": {"dev_name": "0ABCD", "type": "R1", "ra_group_num": "1", "state": "Ready"},
		"Remote": {"dev_name": "0BCDE", "remote_symid": "`+testRSID+`", "wwn": "60000970000197600002533030424344", "state": "Write Disabled"}
	}`, string(b))
}

func TestCreatePair(t *testing.T) {
	show := "symdev -sid " + testSID + " -output xml_e show 0ABCD"
	t.Run("establish", func(t *testing.T) {
		fake := newFakeSymcli(t).
			ok(show, devShowXML("0ABCD", 5461, "False", "")).
			ok("symrdf -sid "+testSID+" -f <pairfile> -rdfg 7 createpair -noprompt -rdf_mode sync -type R1 -establish", "")
		out, err := runAction(t, newTestArray(t, fake), "createpair", "-a", testSID, "--pair", "0ABCD:0BCDE", "--rdfg", "7", "--srdf-type", "R1", "--srdf-mode", "sync")
		require.NoError(t, err)
		assert.Empty(t, out)
		assert.Equal(t, []string{show, "symrdf -sid " + testSID + " -f <pairfile> -rdfg 7 createpair -noprompt -rdf_mode sync -type R1 -establish"}, fake.calls)
		assert.Equal(t, []string{"0ABCD 0BCDE\n"}, fake.pairFiles)
	})
	t.Run("invalidate", func(t *testing.T) {
		fake := newFakeSymcli(t).
			ok(show, devShowXML("0ABCD", 5461, "False", "")).
			ok("symrdf -sid "+testSID+" -f <pairfile> -rdfg 7 createpair -noprompt -rdf_mode sync -type R1 -invalidate R2", "")
		_, err := runAction(t, newTestArray(t, fake), "createpair", "-a", testSID, "--pair", "0ABCD:0BCDE", "--rdfg", "7", "--srdf-type", "R1", "--srdf-mode", "sync", "--invalidate", "R2")
		require.NoError(t, err)
		assert.Len(t, fake.calls, 2)
	})
	t.Run("already paired", func(t *testing.T) {
		fake := newFakeSymcli(t).ok(show, devShowXML("0ABCD", 5461, "False", rdfXML))
		_, err := runAction(t, newTestArray(t, fake), "createpair", "-a", testSID, "--pair", "0ABCD:0BCDE", "--rdfg", "7", "--srdf-type", "R1", "--srdf-mode", "sync")
		require.ErrorContains(t, err, "already in a RDF relation")
		assert.Equal(t, []string{show}, fake.calls)
	})
	for _, pair := range []string{"0ABCD", "0ABCD:", ":0BCDE", "0ABCD:0BCDE:0CDEF"} {
		t.Run("misformatted "+pair, func(t *testing.T) {
			fake := newFakeSymcli(t)
			_, err := runAction(t, newTestArray(t, fake), "createpair", "-a", testSID, "--pair", pair, "--rdfg", "7")
			require.ErrorContains(t, err, "misformatted pair")
			assert.Empty(t, fake.calls)
		})
	}
}

func TestDelDisk(t *testing.T) {
	show := "symdev -sid " + testSID + " -output xml_e show 0ABCD"
	head := []string{
		show,
		"symdev -sid " + testSID + " write_disable 0ABCD -noprompt",
		"symaccess -sid " + testSID + " -output xml_e list -type storage -devs 0ABCD",
		"symaccess -sid " + testSID + " -name SG_A -type storage remove dev 0ABCD -unmap",
	}
	onHead := func(f *fakeSymcli, rdf string) *fakeSymcli {
		return f.
			ok(show, devShowXML("0ABCD", 5461, "False", rdf)).
			ok("symdev -sid "+testSID+" write_disable 0ABCD -noprompt", "").
			ok("symaccess -sid "+testSID+" -output xml_e list -type storage -devs 0ABCD", devSGsXML).
			ok("symaccess -sid "+testSID+" -name SG_A -type storage remove dev 0ABCD -unmap", "")
	}
	symcli := func(version string) string {
		return "Symmetrix Command Line Interface (SYMCLI) Version " + version + " (Edit Level: 2752)\n"
	}
	t.Run("unpaired", func(t *testing.T) {
		fake := onHead(newFakeSymcli(t), "").
			ok("symcli", symcli("V10.1.0.0")).
			ok("symdev -sid "+testSID+" delete 0ABCD -noprompt", "")
		out, err := runAction(t, newTestArray(t, fake), "del", "disk", "-a", testSID, "--dev", "0ABCD")
		require.NoError(t, err)
		assert.Equal(t, calls(head, []string{"symcli", "symdev -sid " + testSID + " delete 0ABCD -noprompt"}), fake.calls)
		var data map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &data))
		assert.Equal(t, testWWN, data["disk_id"])
		assert.Equal(t, "0ABCD", data["disk_devid"])
		driverData := data["driver_data"].(map[string]any)
		assert.NotContains(t, driverData, "rdf", "no remote device for the collector to delete")
	})
	t.Run("paired", func(t *testing.T) {
		fake := onHead(newFakeSymcli(t), rdfXML).
			ok("symrdf -sid "+testSID+" -f <pairfile> -rdfg 1 suspend -noprompt", "").
			ok("symrdf -sid "+testSID+" -f <pairfile> -rdfg 1 deletepair -noprompt -force", "").
			ok("symcli", symcli("V10.1.0.0")).
			ok("symdev -sid "+testSID+" delete 0ABCD -noprompt", "")
		out, err := runAction(t, newTestArray(t, fake), "del", "disk", "-a", testSID, "--dev", "0ABCD")
		require.NoError(t, err)
		assert.Equal(t, calls(head, []string{
			"symrdf -sid " + testSID + " -f <pairfile> -rdfg 1 suspend -noprompt",
			"symrdf -sid " + testSID + " -f <pairfile> -rdfg 1 deletepair -noprompt -force",
			"symcli",
			"symdev -sid " + testSID + " delete 0ABCD -noprompt",
		}), fake.calls)
		var data map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &data))
		assertRDFStrings(t, data["driver_data"].(map[string]any)["rdf"])
	})
	t.Run("an old symcli frees, and a refused delete is retried once", func(t *testing.T) {
		defer func(d time.Duration) { retryDelay = d }(retryDelay)
		retryDelay = time.Millisecond
		free := []string{
			"symdev -sid " + testSID + " free -devs 0ABCD -all -noprompt",
			"symcfg -sid " + testSID + " -output xml_e list -tdevs -devs 0ABCD",
			"symcfg -sid " + testSID + " verify -tdevs -devs 0ABCD -deallocating",
			"symcfg -sid " + testSID + " verify -tdevs -devs 0ABCD -freeingall",
		}
		fake := onHead(newFakeSymcli(t), "").
			ok("symcli", symcli("V9.1.0.0")).
			ok(free[0], "").
			ok(free[1], xmlDoc(`<ThinDevs><Device><dev_name>0ABCD</dev_name><shared_tracks>FALSE</shared_tracks><written_tracks>N/A</written_tracks><alloc_tracks>0</alloc_tracks></Device></ThinDevs>`)).
			ok(free[2], "None\n").
			ok(free[3], "None\n").
			on("symdev -sid "+testSID+" delete 0ABCD -noprompt",
				fakeAnswer{err: "A free of all allocations is required", code: 1},
				fakeAnswer{})
		_, err := runAction(t, newTestArray(t, fake), "del", "disk", "-a", testSID, "--dev", "0ABCD")
		require.NoError(t, err)
		assert.Equal(t, calls(head, []string{"symcli"},
			free, []string{"symdev -sid " + testSID + " delete 0ABCD -noprompt"},
			free, []string{"symdev -sid " + testSID + " delete 0ABCD -noprompt"},
		), fake.calls, "the device is deleted once, and nothing runs on it after")
	})
	t.Run("a snapvx source is refused", func(t *testing.T) {
		fake := newFakeSymcli(t).ok(show, devShowXML("0ABCD", 5461, "True", ""))
		_, err := runAction(t, newTestArray(t, fake), "del", "disk", "-a", testSID, "--dev", "0ABCD")
		require.ErrorContains(t, err, "snapvx_source")
		assert.Equal(t, []string{show}, fake.calls)
	})
	t.Run("a failure says what was done", func(t *testing.T) {
		fake := onHead(newFakeSymcli(t), rdfXML).
			on("symrdf -sid "+testSID+" -f <pairfile> -rdfg 1 suspend -noprompt", fakeAnswer{err: "link down", code: 1})
		_, err := runAction(t, newTestArray(t, fake), "del", "disk", "-a", testSID, "--dev", "0ABCD")
		require.ErrorContains(t, err, "link down: dev 0ABCD was write disabled, unmapped, and is not deleted")
	})
}

// TestAddMasking pins the commands of a v2 masking plan, the plan going on
// past a failed step, and the results recorded once per step.
func TestAddMasking(t *testing.T) {
	plan := `{
		"array_id": "` + testSID + `",
		"proxy": {"nodename": "n1"},
		"ig": [{"name": "IG1", "hba_ids": ["h1"], "ig": ["IGC"]}],
		"sg": [{"name": "SG1", "srp": "SRP_1", "slo": "Diamond", "sg": ["SGC"]}],
		"gk": [{"sg": "SG1", "count": 6}],
		"dev": [{"name": "d1", "size": "10g", "sg": "SG1"}],
		"mv": [{"name": "MV1", "pg": ["T2", "t1"], "sg": ["SG1"], "ig": ["IG1"]}]
	}`
	cmds := []string{
		"symaccess -sid " + testSID + " -name IG1 -type initiator -consistent_lun create",
		"symaccess -sid " + testSID + " -name IG1 -type initiator -ig IGC add",
		"symaccess -sid " + testSID + " -name IG1 -type initiator -wwn h1 add",
		"symsg -sid " + testSID + " create SG1 -srp SRP_1 -slo Diamond",
		"symsg -sid " + testSID + " -sg SG1 add sg SGC",
		"symsg -sid " + testSID + " -output xml_e show SG1",
		"symdev -sid " + testSID + " create -gk -N 4 -sg SG1 -noprompt",
		"symsg -sid " + testSID + " -output xml_e show SG1",
		"symdev -sid " + testSID + " create -tdev -N 1 -cap 5461 -captype cyl -sg SG1 -emulation FBA -device_name d1 -noprompt -v",
		"symaccess -sid " + testSID + " -output xml_e list -type port",
		"symaccess -sid " + testSID + " -output xml_e show PG2 -type port",
		"symaccess -sid " + testSID + " -output xml_e show PG1 -type port",
		"symaccess -sid " + testSID + " create view -name MV1 -pg PG2 -sg SG1 -ig IG1",
	}
	pgShow := func(name string, wwns ...string) string {
		s := ""
		for _, wwn := range wwns {
			s += "<Director_Identification><dir>FA-1D</dir><port>8</port><port_wwn>" + wwn + "</port_wwn></Director_Identification>"
		}
		return xmlDoc("<Port_Group><Group_Info><group_name>" + name + "</group_name>" + s + "</Group_Info></Port_Group>")
	}
	fake := newFakeSymcli(t).
		on(cmds[0], fakeAnswer{err: "The initiator group already exists", code: 1}).
		ok(cmds[1], "").
		ok(cmds[2], "").
		ok(cmds[3], "").
		on(cmds[4], fakeAnswer{err: "The group is currently within device masking view", code: 1}).
		ok(cmds[5], sgShowXML("SG1", "SRP_1", "Diamond", 2, "")).
		ok(cmds[6], "").
		ok(cmds[8], createdXML("0ABCD")).
		ok(cmds[9], xmlDoc(`
    <Port_Group><Group_Info><group_name>PG2</group_name><port_count>N/A</port_count></Group_Info></Port_Group>
    <Port_Group><Group_Info><group_name>PG1</group_name></Group_Info></Port_Group>`)).
		ok(cmds[10], pgShow("PG2", "t1", "t2")).
		ok(cmds[11], pgShow("PG1", "t1")).
		ok(cmds[12], "")
	a := newTestArray(t, fake)

	out, err := runAction(t, a, "add", "masking", "-a", testSID, "--data", plan)
	require.NoError(t, err, "a failed step is a result, not an error")
	assert.Equal(t, cmds, fake.calls)

	var data MaskingDump
	require.NoError(t, json.Unmarshal([]byte(out), &data))
	require.Len(t, data.InitiatorGroups, 1)
	igResults := data.InitiatorGroups[0].Results
	require.Len(t, igResults, 3)
	assert.Equal(t, 1, igResults[0].Ret)
	assert.Equal(t, "The initiator group already exists", igResults[0].Err)
	assert.Equal(t, strings.Fields(cmds[0]), igResults[0].Cmd)
	require.Len(t, data.StorageGroups[0].Results, 2)
	assert.Equal(t, 0, data.StorageGroups[0].Results[1].Ret, "a child already in its parent is done")
	require.Len(t, data.Gatekeepers[0].Results, 1)
	assert.Equal(t, strings.Fields(cmds[6]), data.Gatekeepers[0].Results[0].Cmd)
	require.Len(t, data.Devices[0].Results, 1)
	assert.Equal(t, strings.Fields(cmds[8]), data.Devices[0].Results[0].Cmd)
	require.Len(t, data.Views[0].Results, 1, "a view result is recorded once")
	assert.Equal(t, strings.Fields(cmds[12]), data.Views[0].Results[0].Cmd)

	t.Run("a storage group holding devices gets none", func(t *testing.T) {
		fake := newFakeSymcli(t).ok("symsg -sid "+testSID+" -output xml_e show SG1", sgShowXML("SG1", "SRP_1", "Diamond", 6, "<Device><dev_name>0ABCD</dev_name></Device>"))
		out, err := runAction(t, newTestArray(t, fake), "add", "masking", "-a", testSID, "--data", `{"dev": [{"size": "10g", "sg": "SG1"}], "gk": [{"sg": "SG1"}]}`)
		require.NoError(t, err)
		assert.Equal(t, []string{"symsg -sid " + testSID + " -output xml_e show SG1", "symsg -sid " + testSID + " -output xml_e show SG1"}, fake.calls)
		assert.JSONEq(t, `{"gk": [{"sg": "SG1", "result": []}], "dev": [{"size": "10g", "sg": "SG1", "result": []}]}`, out)
	})
	t.Run("a view with no port group runs nothing", func(t *testing.T) {
		fake := newFakeSymcli(t).
			ok(cmds[9], xmlDoc(`<Port_Group><Group_Info><group_name>PG1</group_name></Group_Info></Port_Group>`)).
			ok(cmds[11], pgShow("PG1", "t1"))
		out, err := runAction(t, newTestArray(t, fake), "add", "masking", "-a", testSID, "--data", `{"mv": [{"name": "MV1", "pg": ["t3"]}]}`)
		require.NoError(t, err)
		var data MaskingDump
		require.NoError(t, json.Unmarshal([]byte(out), &data))
		require.Len(t, data.Views[0].Results, 1)
		assert.Equal(t, 1, data.Views[0].Results[0].Ret)
		assert.Equal(t, []string{}, data.Views[0].Results[0].Cmd)
		assert.Contains(t, data.Views[0].Results[0].Err, "no pg with port ids")
	})
	for name, plan := range map[string]string{
		"not json":         `{`,
		"empty":            ``,
		"a misspelled key": `{"ig": [{"name": "IG1", "igs": ["IGC"]}]}`,
		"an ig of no name": `{"ig": [{"hba_ids": ["h1"]}]}`,
		"a bad size":       `{"dev": [{"size": "10x", "sg": "SG1"}]}`,
		"a view of no pg":  `{"mv": [{"name": "MV1"}]}`,
	} {
		t.Run("refuse "+name, func(t *testing.T) {
			fake := newFakeSymcli(t)
			_, err := runAction(t, newTestArray(t, fake), "add", "masking", "-a", testSID, "--data", plan)
			require.Error(t, err)
			assert.Empty(t, fake.calls, "an invalid plan runs nothing")
		})
	}
}

// TestNoPromptOnStdout runs the symcli programs for real, fake ones, the way
// an action queued by the collector does, with no stdin: the action asks
// nothing and stdout carries the json result alone.
func TestNoPromptOnStdout(t *testing.T) {
	require.Nil(t, PromptReader, "no symcli command is confirmed on a prompt")

	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	showFile := filepath.Join(dir, "show.xml")
	require.NoError(t, os.WriteFile(showFile, []byte(devShowXML("0ABCD", 5461, "False", "")), 0o644))
	logFile := filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
echo "$(basename "$0") $*" >>` + logFile + `
case "$*" in
*" show "*) cat ` + showFile + ` ;;
*fail*) echo "the array said no" >&2; exit 3 ;;
*) echo "done" ;;
esac
`
	require.NoError(t, os.WriteFile(filepath.Join(bin, "symdev"), []byte(script), 0o755))

	config := fmt.Sprintf("[array#sym1]\ntype = symmetrix\nname = %s\nsymcli_path = %s\n", testSID, dir)
	n, err := object.NewNode(object.WithConfigData([]byte(config)), object.WithVolatile(true))
	require.NoError(t, err)
	a := New()
	a.SetName("array#sym1")
	a.SetConfig(n.MergedConfig())

	devNull, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer devNull.Close()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	stdin, stdout := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = devNull, w
	var buf bytes.Buffer
	runErr := array.RunActions(context.Background(), a.Actions(), []string{"rename", "disk", "--dev", "0ABCD", "--name", "newname"}, &buf)
	os.Stdin, os.Stdout = stdin, stdout
	require.NoError(t, w.Close())
	stray, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, runErr)
	assert.Empty(t, string(stray), "nothing but the result reaches stdout")
	var dev map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &dev), "the result is json")

	b, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Equal(t, `symdev -sid `+testSID+` -output xml_e show 0ABCD
symdev -sid `+testSID+` set dev 0ABCD -attribute device_name=newname
symdev -sid `+testSID+` -output xml_e show 0ABCD
`, string(b))

	t.Run("a non-zero exit is an error carrying what the array said", func(t *testing.T) {
		_, err := a.sym(context.Background(), "symdev", "-sid", testSID, "fail")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exit code 3: the array said no")
	})
}

// TestTestdataParses pins that the listings print N/A in counts without
// failing the decoding of the listing.
func TestTestdataParses(t *testing.T) {
	a := New()
	for name, parse := range map[string]func([]byte) error{
		"18-symaccess_list_type_initiator": func(b []byte) error {
			l, err := a.parseSymAccessListDevInitiator(b)
			if err == nil && len(l) != 2 {
				err = fmt.Errorf("%d initiator groups", len(l))
			}
			return err
		},
		"23-symaccess_list_type_storage": func(b []byte) error {
			l, err := a.parseSymAccessListDevStorage(b)
			if err == nil && len(l) != 2 {
				err = fmt.Errorf("%d storage groups", len(l))
			}
			return err
		},
		"15-symcfg_list_tdev_detail": func(b []byte) error {
			l, err := a.parseSymCfgListThinDevs(b)
			if err == nil && (len(l) == 0 || l[0].AllocTracks != "0") {
				err = fmt.Errorf("thin devs %v", l)
			}
			return err
		},
		"01-symsg_list": func(b []byte) error {
			l, err := a.parseSymSGList(b)
			if err == nil && (len(l) == 0 || l[0].Name != "CSG_XXX1") {
				err = fmt.Errorf("storage groups %v", l)
			}
			return err
		},
		"06-symdev_show_wwn": func(b []byte) error {
			l, err := a.parseSymDevShow(b)
			if err != nil {
				return err
			}
			if len(l) != 1 || l[0].Capacity.Cylinders != 177494 || l[0].Capacity.Kilobytes != 177494*cylinderKB {
				return fmt.Errorf("capacity %+v", l)
			}
			rdf := l[0].paired()
			if rdf == nil || rdf.RAGroupNum() != "1" || rdf.RemoteSID() != "000297600002" || rdf.Mode() != "Synchronous" {
				return fmt.Errorf("rdf %+v", rdf)
			}
			return nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", name))
			require.NoError(t, err)
			require.NoError(t, parse(b))
		})
	}
}

// TestAddDiskTakesTheStorageGroupNamed pins that --sg is the storage group
// the device is put in, checked before anything is created, rather than
// accepted and ignored.
func TestAddDiskTakesTheStorageGroupNamed(t *testing.T) {
	fake := newFakeSymcli(t).
		on("symsg -sid "+testSID+" -output xml_e show SG_X", fakeAnswer{err: "The specified Storage Group was not found", code: 1})
	_, err := runAction(t, newTestArray(t, fake), "add", "disk", "-a", testSID, "--name", "d1", "--size", "10g", "--sg", "SG_X")
	require.ErrorContains(t, err, "Storage Group was not found")
	assert.Equal(t, []string{"symsg -sid " + testSID + " -output xml_e show SG_X"}, fake.calls, "nothing created")
}

// TestAddDiskRefusesADeviceOfAnotherCylinderSize pins that a device created
// smaller than asked for, on an array of 960 KB cylinders, is an error
// naming it, rather than a success the collector records at the size asked.
func TestAddDiskRefusesADeviceOfAnotherCylinderSize(t *testing.T) {
	show := "symdev -sid " + testSID + " -output xml_e show 0ABCD"
	doc := strings.Replace(devShowXML("0ABCD", 5461, "False", ""), fmt.Sprintf("<kilobytes>%d</kilobytes>", 5461*cylinderKB), fmt.Sprintf("<kilobytes>%d</kilobytes>", 5461*960), 1)
	fake := newFakeSymcli(t).
		onMappingResolution().
		ok("symdev -sid "+testSID+" create -tdev -N 1 -cap 5461 -captype cyl -emulation FBA -device_name d1 -noprompt -v", createdXML("0ABCD")).
		ok(show, doc)
	_, err := runAction(t, newTestArray(t, fake), "add", "disk", "-a", testSID, "--name", "d1", "--size", "10g", "--mappings", "h1:t1", "--srp", "SRP_1")
	require.ErrorContains(t, err, "KB per cylinder")
	assert.Contains(t, err.Error(), "created dev 0ABCD on array "+testSID+", left in place")
	for _, call := range fake.calls {
		assert.NotContains(t, call, " add dev ", "not mapped")
	}
}

// TestDelDiskRefusesAnR2 pins that the R2 of a pair is not deleted: its
// pairing names the R1 as remote, which the collector would delete next.
func TestDelDiskRefusesAnR2(t *testing.T) {
	show := "symdev -sid " + testSID + " -output xml_e show 0ABCD"
	r2 := strings.Replace(rdfXML, "<type>R1</type>", "<type>R2</type>", 1)
	require.NotEqual(t, rdfXML, r2)
	fake := newFakeSymcli(t).ok(show, devShowXML("0ABCD", 5461, "False", r2))
	out, err := runAction(t, newTestArray(t, fake), "del", "disk", "-a", testSID, "--dev", "0ABCD")
	require.ErrorContains(t, err, "is the R2 of a SRDF pair")
	assert.Contains(t, err.Error(), "delete the R1")
	assert.Empty(t, out)
	assert.Equal(t, []string{show}, fake.calls, "nothing changed")
}

// TestDelDiskGoesOnPastAFailedWriteDisable pins v2's handling of a write
// disable refused, as on a device already write disabled: the unmap and
// the delete decide.
func TestDelDiskGoesOnPastAFailedWriteDisable(t *testing.T) {
	show := "symdev -sid " + testSID + " -output xml_e show 0ABCD"
	fake := newFakeSymcli(t).
		ok(show, devShowXML("0ABCD", 5461, "False", "")).
		on("symdev -sid "+testSID+" write_disable 0ABCD -noprompt", fakeAnswer{err: "already write disabled", code: 1}).
		ok("symaccess -sid "+testSID+" -output xml_e list -type storage -devs 0ABCD", devSGsXML).
		ok("symaccess -sid "+testSID+" -name SG_A -type storage remove dev 0ABCD -unmap", "").
		ok("symcli", "Symmetrix Command Line Interface (SYMCLI) Version V10.1.0.0 (Edit Level: 2752)\n").
		ok("symdev -sid "+testSID+" delete 0ABCD -noprompt", "")
	_, err := runAction(t, newTestArray(t, fake), "del", "disk", "-a", testSID, "--dev", "0ABCD")
	require.NoError(t, err)
	assert.Contains(t, fake.calls, "symdev -sid "+testSID+" delete 0ABCD -noprompt")
}

// TestResizeByNothingChangesNothing pins that "+0" grows nothing, where the
// one cylinder a size rounds up to would have grown the device.
func TestResizeByNothingChangesNothing(t *testing.T) {
	show := "symdev -sid " + testSID + " -output xml_e show 0ABCD"
	fake := newFakeSymcli(t).ok(show, devShowXML("0ABCD", 5461, "False", ""))
	out, err := runAction(t, newTestArray(t, fake), "resize", "disk", "-a", testSID, "--dev", "0ABCD", "--size", "+0")
	require.NoError(t, err)
	assert.Equal(t, []string{show}, fake.calls)
	assert.JSONEq(t, `{"driver_data": {"pair_deleted": false}}`, out)
}
