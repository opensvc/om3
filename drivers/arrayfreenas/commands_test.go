package arrayfreenas

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addArgs is the command line the collector queues to allocate a disk, the
// two nodes of a cluster reaching it through the same two targets.
func addArgs(extra ...string) []string {
	args := []string{"add", "iscsi", "zvol", "-a", "A", "--volume", "DG", "--name", "svc_0", "--size", "1g",
		"--mappings", testHBA1 + ":" + testTgt1 + "," + testTgt2,
		"--mappings", testHBA2 + ":" + testTgt1 + "," + testTgt2,
	}
	return append(args, extra...)
}

// TestAddExportsANewZvolThroughEachTargetOnce is the collector's add: real
// initiator names, two initiators sharing their targets, and a report in
// the keys v2 reports.
func TestAddExportsANewZvolThroughEachTargetOnce(t *testing.T) {
	f := newFakeNAS()
	a := newFakeArray(t, f)

	out, err := run(t, a, addArgs()...)
	require.NoError(t, err)

	zvol := f.dataset("DG/svc_0")
	require.NotNil(t, zvol)
	assert.Equal(t, "1073741824", zvol.Volsize.Rawvalue, "1g is a power of 1024")
	require.Len(t, f.extents, 1)
	extent := f.extents[0]
	assert.Equal(t, "svc_0", extent.Name, "the extent is named as v2 names it, the name the collector knows")
	assert.Equal(t, "zvol/DG/svc_0", extent.Disk)
	require.Len(t, f.targetExtents, 2, "each target once, whatever the number of initiators reaching it")
	for _, te := range f.targetExtents {
		assert.Equal(t, extent.Id, te.ExtentId)
	}

	for _, s := range f.requests {
		if strings.HasPrefix(s, "GET ") {
			assert.Containsf(t, s, "limit=0", "%s: a listing not told limit=0 is paged", s)
		}
		if strings.HasPrefix(s, "POST /pool/dataset") {
			assert.NotContains(t, s, "volblocksize")
		}
	}

	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &report), out)
	for _, key := range []string{"driver_data", "disk_id", "disk_devid", "mappings"} {
		assert.Containsf(t, report, key, "v2 reports %s", key)
	}
	assert.NotContains(t, report, "warnings", "no warning to report")
	assert.Equal(t, strings.TrimPrefix(extent.NAA, "0x"), report["disk_id"])
	assert.EqualValues(t, extent.Id, report["disk_devid"])
	assert.EqualValues(t, extent.Id, report["driver_data"].(map[string]any)["id"])

	mappings := report["mappings"].([]any)
	require.Len(t, mappings, 4, "two initiators through two targets")
	var keys []string
	for _, m := range mappings {
		m := m.(map[string]any)
		for _, key := range []string{"targetgroup", "extent", "disk_id", "tgt_id", "hba_id"} {
			assert.Contains(t, m, key)
		}
		keys = append(keys, m["hba_id"].(string)+" "+m["tgt_id"].(string))
	}
	assert.Equal(t, []string{
		testHBA2 + " " + testTgt1,
		testHBA2 + " " + testTgt2,
		testHBA1 + " " + testTgt1,
		testHBA1 + " " + testTgt2,
	}, keys, "sorted by initiator then target, as v2 sorts them")
}

// TestAddRefusesWhatExists keeps an add from adopting a zvol or an extent
// it did not create: they hold another client's data, and the add exports
// them.
func TestAddRefusesWhatExists(t *testing.T) {
	t.Run("dataset", func(t *testing.T) {
		f := newFakeNAS()
		f.addZvol("DG/svc_0", 5<<30)
		a := newFakeArray(t, f)
		_, err := run(t, a, addArgs()...)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DG/svc_0 already exists")
		assert.Empty(t, f.writes())
	})
	t.Run("extent name", func(t *testing.T) {
		f := newFakeNAS()
		f.addZvol("DG2/other", 5<<30)
		f.addExtent("svc_0", "DG2/other")
		a := newFakeArray(t, f)
		_, err := run(t, a, addArgs()...)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "extent svc_0 already exists")
		assert.Empty(t, f.writes())
	})
}

// TestAddRefusesBeforeWriting covers what an add can be refused for without
// writing anything: a target the array does not have, a mapping that can
// not be read, a size to grow by.
func TestAddRefusesBeforeWriting(t *testing.T) {
	for name, args := range map[string][]string{
		"unknown target": {"add", "iscsi", "zvol", "--volume", "DG", "--name", "svc_0", "--size", "1g",
			"--mappings", testHBA1 + ":" + testTgt1 + ",iqn.2005-10.org.freenas.ctl:nosuch"},
		"bad mapping": {"add", "iscsi", "zvol", "--volume", "DG", "--name", "svc_0", "--size", "1g",
			"--mappings", testHBA1},
		"relative size": {"add", "iscsi", "zvol", "--volume", "DG", "--name", "svc_0", "--size", "+1g",
			"--mappings", testHBA1 + ":" + testTgt1},
		"negative size": {"add", "iscsi", "zvol", "--volume", "DG", "--name", "svc_0", "--size", "-1g",
			"--mappings", testHBA1 + ":" + testTgt1},
		"no volume": {"add", "iscsi", "zvol", "--name", "svc_0", "--size", "1g",
			"--mappings", testHBA1 + ":" + testTgt1},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeNAS()
			a := newFakeArray(t, f)
			_, err := run(t, a, args...)
			require.Error(t, err)
			assert.Empty(t, f.writes(), "nothing is made before a refusal")
			assert.Len(t, f.datasets, 1)
		})
	}
}

// TestAddFailingMidwayNamesWhatItMade is a targetextent refused after the
// zvol, the extent and a first targetextent are made: the error names them,
// and they are left for the operator to judge.
func TestAddFailingMidwayNamesWhatItMade(t *testing.T) {
	f := newFakeNAS()
	f.fail["POST /iscsi/targetextent"] = 1
	a := newFakeArray(t, f)

	out, err := run(t, a, addArgs()...)
	require.Error(t, err)
	assert.Empty(t, out)
	require.Len(t, f.extents, 1)
	require.Len(t, f.targetExtents, 1)
	msg := err.Error()
	assert.Contains(t, msg, "zvol DG/svc_0")
	assert.Contains(t, msg, fmt.Sprintf("extent %d (svc_0)", f.extents[0].Id))
	assert.Contains(t, msg, fmt.Sprintf("targetextent %d", f.targetExtents[0].Id))
	assert.NotNil(t, f.dataset("DG/svc_0"), "nothing is rolled back")
	for _, s := range f.writes() {
		assert.Falsef(t, strings.HasPrefix(s, "DELETE"), "%s: an add deletes nothing", s)
	}
}

// TestAddFailingAfterTheArrayActedSaysSo is a write the array made but
// answered an error to, as a timeout does: the error names it made.
func TestAddFailingAfterTheArrayActedSaysSo(t *testing.T) {
	f := newFakeNAS()
	f.failAfterWrite["POST /iscsi/extent"] = true
	a := newFakeArray(t, f)

	_, err := run(t, a, addArgs()...)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zvol DG/svc_0")
	assert.Contains(t, err.Error(), "extent svc_0")
	assert.Empty(t, f.targetExtents)
}

// TestAddMapsTheExtentItCreated pins that the targetextents are made with
// the id the extent creation answered, not one found by a lookup that finds
// another extent of the same disk path.
func TestAddMapsTheExtentItCreated(t *testing.T) {
	f := newFakeNAS()
	a := newFakeArray(t, f)
	_, err := run(t, a, addArgs()...)
	require.NoError(t, err)
	for _, te := range f.targetExtents {
		assert.Equal(t, f.extents[0].Id, te.ExtentId)
	}
}

// exportedZvol makes the disk the collector allocated: zvol DG/svc_0 of
// 1 GiB, exported by extent svc_0 through both targets.
func exportedZvol(f *fakeNAS) ISCSIExtent {
	f.addZvol("DG/svc_0", 1<<30)
	e := f.addExtent("svc_0", "DG/svc_0")
	f.addTargetExtent(1, e.Id, 0)
	f.addTargetExtent(2, e.Id, 0)
	return e
}

// TestDelResolvesTheNameTheCollectorKnows is the collector's delete, by the
// extent name: the targetextents, the extent and the zvol are deleted, in
// that order.
func TestDelResolvesTheNameTheCollectorKnows(t *testing.T) {
	f := newFakeNAS()
	e := exportedZvol(f)
	tes := append(ISCSITargetExtents{}, f.targetExtents...)
	a := newFakeArray(t, f)

	_, err := run(t, a, "del", "iscsi", "zvol", "-a", "A", "--name", "svc_0")
	require.NoError(t, err)
	assert.Equal(t, []string{
		fmt.Sprintf("DELETE /iscsi/targetextent/id/%d", tes[0].Id),
		fmt.Sprintf("DELETE /iscsi/targetextent/id/%d", tes[1].Id),
		fmt.Sprintf("DELETE /iscsi/extent/id/%d", e.Id),
		"DELETE /pool/dataset/id/DG%2Fsvc_0",
	}, f.writes())
	assert.Nil(t, f.dataset("DG/svc_0"))
	assert.Empty(t, f.extents)
	assert.Empty(t, f.targetExtents)
}

// TestDelResolvesOtherNames covers the other names v2 reads: the naa of the
// extent, with or without its prefix, and the zvol name itself.
func TestDelResolvesOtherNames(t *testing.T) {
	for name, nameOf := range map[string]func(ISCSIExtent) string{
		"naa":            func(e ISCSIExtent) string { return e.NAA },
		"naa unprefixed": func(e ISCSIExtent) string { return strings.TrimPrefix(e.NAA, "0x") },
		"zvol name":      func(ISCSIExtent) string { return "DG/svc_0" },
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeNAS()
			e := exportedZvol(f)
			a := newFakeArray(t, f)
			_, err := run(t, a, "del", "iscsi", "zvol", "--name", nameOf(e))
			require.NoError(t, err)
			assert.Nil(t, f.dataset("DG/svc_0"))
			assert.Empty(t, f.extents)
			assert.Empty(t, f.targetExtents)
		})
	}
}

// TestDelRefuses covers the deletes refused before deleting anything.
func TestDelRefuses(t *testing.T) {
	for name, tc := range map[string]struct {
		setup func(*fakeNAS)
		name  string
		want  string
	}{
		"not found": {
			setup: func(f *fakeNAS) { exportedZvol(f) },
			name:  "svc_9",
			want:  "no extent named svc_9",
		},
		"filesystem": {
			setup: func(f *fakeNAS) {
				f.datasets = append(f.datasets, fakeDataset{Id: "DG/share", Name: "DG/share", Pool: "DG", Type: DatasetTypeFilesystem})
			},
			name: "DG/share",
			want: "is a FILESYSTEM, not a zvol",
		},
		"two extents": {
			setup: func(f *fakeNAS) {
				exportedZvol(f)
				f.addExtent("svc_0_b", "DG/svc_0")
			},
			name: "svc_0",
			want: "exported by 2 iscsi extents",
		},
		"file extent": {
			setup: func(f *fakeNAS) {
				f.extents = append(f.extents, ISCSIExtent{Id: 7, Name: "svc_0", Type: "FILE", Path: "/mnt/DG/svc_0"})
			},
			name: "svc_0",
			want: "not a zvol",
		},
		"zvol gone": {
			setup: func(f *fakeNAS) { f.addExtent("svc_0", "DG/svc_0") },
			name:  "svc_0",
			want:  "which does not exist",
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeNAS()
			tc.setup(f)
			a := newFakeArray(t, f)
			out, err := run(t, a, "del", "iscsi", "zvol", "--name", tc.name)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Empty(t, out)
			assert.Empty(t, f.writes(), "nothing is deleted")
		})
	}
}

// TestDelFailingMidwayNamesWhatItDeleted is an extent delete refused after
// its targetextents are deleted.
func TestDelFailingMidwayNamesWhatItDeleted(t *testing.T) {
	f := newFakeNAS()
	exportedZvol(f)
	tes := append(ISCSITargetExtents{}, f.targetExtents...)
	f.fail["DELETE /iscsi/extent/id/"] = 0
	a := newFakeArray(t, f)

	_, err := run(t, a, "del", "iscsi", "zvol", "--name", "svc_0")
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("targetextent %d", tes[0].Id))
	assert.Contains(t, err.Error(), fmt.Sprintf("targetextent %d", tes[1].Id))
	assert.NotNil(t, f.dataset("DG/svc_0"), "the zvol of an extent not deleted is kept")
}

// TestDelZvolRefusesAnExportedZvol keeps "del zvol" from pulling the storage
// from under an extent.
func TestDelZvolRefusesAnExportedZvol(t *testing.T) {
	f := newFakeNAS()
	exportedZvol(f)
	a := newFakeArray(t, f)
	_, err := run(t, a, "del", "zvol", "--name", "DG/svc_0")
	require.Error(t, err)
	assert.Empty(t, f.writes())
}

// TestResize covers the collector's resize, by the extent name, and the
// shrink guard.
func TestResize(t *testing.T) {
	for name, tc := range map[string]struct {
		args []string
		want string
		err  string
	}{
		"grow":            {args: []string{"--size", "10g"}, want: "10737418240"},
		"grow relative":   {args: []string{"--size", "+1g"}, want: "2147483648"},
		"same size":       {args: []string{"--size", "1g"}, want: "1073741824"},
		"shrink refused":  {args: []string{"--size", "512m"}, err: "--truncate allows it"},
		"shrink truncate": {args: []string{"--size", "512m", "--truncate"}, want: "536870912"},
		"negative":        {args: []string{"--size", "-1g"}, err: "can not be negative"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeNAS()
			exportedZvol(f)
			a := newFakeArray(t, f)
			args := append([]string{"resize", "zvol", "-a", "A", "--name", "svc_0"}, tc.args...)
			out, err := run(t, a, args...)
			if tc.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.err)
				assert.Empty(t, f.writes())
				assert.Equal(t, "1073741824", f.dataset("DG/svc_0").Volsize.Rawvalue)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{"PUT /pool/dataset/id/DG%2Fsvc_0"}, f.writes(), "the id is a path element, escaped")
			assert.Equal(t, tc.want, f.dataset("DG/svc_0").Volsize.Rawvalue)
			assert.Contains(t, out, `"DG/svc_0"`)
		})
	}
}

// TestResizeRefuses covers the resizes refused before writing.
func TestResizeRefuses(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		f := newFakeNAS()
		a := newFakeArray(t, f)
		_, err := run(t, a, "resize", "zvol", "--name", "svc_0", "--size", "+1g")
		require.Error(t, err)
		assert.Empty(t, f.writes())
	})
	t.Run("filesystem", func(t *testing.T) {
		f := newFakeNAS()
		a := newFakeArray(t, f)
		_, err := run(t, a, "resize", "zvol", "--name", "DG", "--size", "10g")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a zvol")
		assert.Empty(t, f.writes())
	})
	t.Run("no volsize", func(t *testing.T) {
		f := newFakeNAS()
		exportedZvol(f)
		f.dataset("DG/svc_0").Volsize = nil
		a := newFakeArray(t, f)
		_, err := run(t, a, "resize", "zvol", "--name", "svc_0", "--size", "+1g")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no volsize")
		assert.Empty(t, f.writes())
	})
}

// TestMapDiskAttachesEachTargetOnce is "map disk" given two initiators
// sharing a target the extent is already attached to.
func TestMapDiskAttachesEachTargetOnce(t *testing.T) {
	f := newFakeNAS()
	f.addZvol("DG/svc_0", 1<<30)
	e := f.addExtent("svc_0", "DG/svc_0")
	f.addTargetExtent(1, e.Id, 0)
	a := newFakeArray(t, f)
	_, err := run(t, a, "map", "disk", "--name", "svc_0",
		"--mappings", testHBA1+":"+testTgt1+","+testTgt2,
		"--mappings", testHBA2+":"+testTgt1+","+testTgt2)
	require.NoError(t, err)
	assert.Len(t, f.targetExtents, 2)
	assert.Len(t, f.writes(), 1, "only the target the extent was not attached to")
}

// TestARequestWithABodyCarriesTheContext pins that a write is abandoned
// with its context, as a read is.
func TestARequestWithABodyCarriesTheContext(t *testing.T) {
	f := newFakeNAS()
	a := newFakeArray(t, f)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	params, err := AddZvolOptions{Name: "DG/svc_0", Size: "1g"}.Params()
	require.NoError(t, err)
	_, err = a.CreateDataset(ctx, params)
	require.Error(t, err)
	assert.Empty(t, f.requests)
}

// TestUnmapDetachesOnlyForAnAdmittedInitiator pins that a targetextent is
// deleted only for an initiator its target admits: deleting it cuts the disk
// from every host the target admits, not from the initiator named alone.
func TestUnmapDetachesOnlyForAnAdmittedInitiator(t *testing.T) {
	const stranger = "iqn.1993-08.org.debian:01:stranger"
	f := newFakeNAS()
	f.addZvol("DG/svc_0", 1<<30)
	e := f.addExtent("svc_0", "DG/svc_0")
	f.addTargetExtent(1, e.Id, 0)
	a := newFakeArray(t, f)

	_, err := run(t, a, "unmap", "iscsi", "zvol", "--name", "svc_0", "--mappings", stranger+":"+testTgt1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not admit initiator")
	assert.Len(t, f.targetExtents, 1, "nothing detached")

	_, err = run(t, a, "unmap", "iscsi", "zvol", "--name", "svc_0", "--mappings", testHBA1+":iqn.2005-10.org.freenas.ctl:nosuch")
	require.Error(t, err, "an unknown target is not skipped as done")
	assert.Contains(t, err.Error(), "not found")
	assert.Len(t, f.targetExtents, 1, "nothing detached")

	_, err = run(t, a, "unmap", "iscsi", "zvol", "--name", "svc_0", "--mappings", testHBA1+":"+testTgt1)
	require.NoError(t, err)
	assert.Empty(t, f.targetExtents, "detached for an admitted initiator")
}

// TestMappingsOfATargetAdmittingAnyInitiator pins that a target group naming
// no initiator group, which admits any initiator, is reported as such
// rather than as no mapping.
func TestMappingsOfATargetAdmittingAnyInitiator(t *testing.T) {
	f := newFakeNAS()
	for i := range f.targets {
		f.targets[i].Groups = ISCSITargetGroups{{PortalId: 1, AuthMethod: "NONE"}}
	}
	f.addZvol("DG/svc_0", 1<<30)
	e := f.addExtent("svc_0", "DG/svc_0")
	f.addTargetExtent(1, e.Id, 0)
	a := newFakeArray(t, f)
	l, err := a.diskMappings(context.Background(), e)
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, anyInitiator, l[0].HBAID)
	assert.Equal(t, testTgt1, l[0].TgtID)
}
