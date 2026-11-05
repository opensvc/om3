package arraysymmetrix

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/array"
)

// reportCommands are the symcli commands of the sections the collector
// parses, in the order they are reported.
var reportCommands = []struct{ key, cmd string }{
	{"sym_info", "symcfg -sid " + testSID + " -output xml_e list"},
	{"sym_dir_info", "symcfg -sid " + testSID + " -output xml_e -dir all -v list"},
	{"sym_dev_info", "symdev -sid " + testSID + " -output xml_e list"},
	{"sym_dev_wwn_info", "symdev -sid " + testSID + " -output xml_e list -wwn"},
	{"sym_dev_name_info", "symdev -sid " + testSID + " -output xml_e list -identifier device_name"},
	{"sym_ficondev_info", "symdev -sid " + testSID + " -output xml_e list -ficon"},
	{"sym_meta_info", "symdev -sid " + testSID + " -output xml_e list -meta -v"},
	{"sym_disk_info", "symdisk -sid " + testSID + " -output xml_e list -v"},
	{"sym_diskgroup_info", "symdisk -sid " + testSID + " -output xml_e list -dskgrp_summary"},
	{"sym_pool_info", "symcfg -sid " + testSID + " -output xml_e -pool list -v"},
	{"sym_tdev_info", "symcfg -sid " + testSID + " -output xml_e list -tdev -detail"},
	{"sym_srp_info", "symcfg -sid " + testSID + " -output xml_e list -srp -detail -v"},
	{"sym_sg_info", "symsg -sid " + testSID + " -output xml_e list -v"},
}

// The sections are the xml outputs of the commands, an optional one failing
// reported empty: a powermax has no thin pool.
func TestReportsAreTheOutputsTheCollectorParses(t *testing.T) {
	fake := newFakeSymcli(t)
	for _, c := range reportCommands {
		if c.key == "sym_pool_info" {
			fake.on(c.cmd, fakeAnswer{err: "No Thin Pools were found", code: 1})
			continue
		}
		fake.ok(c.cmd, xmlDoc("<"+c.key+"/>"))
	}
	data, err := array.Collect(context.Background(), newTestArray(t, fake))
	require.NoError(t, err)
	require.Len(t, data.Keys, len(reportCommands))
	for i, c := range reportCommands {
		assert.Equal(t, c.key, data.Keys[i])
		if c.key == "sym_pool_info" {
			assert.Equal(t, "", data.Values[i])
			continue
		}
		assert.Equal(t, xmlDoc("<"+c.key+"/>"), data.Values[i])
	}
}

// A required section failing fails the report: the collector makes no row
// of an array without its devices.
func TestReportsFailWithoutTheDevices(t *testing.T) {
	fake := newFakeSymcli(t)
	fake.ok(reportCommands[0].cmd, xmlDoc(""))
	fake.ok(reportCommands[1].cmd, xmlDoc(""))
	fake.on(reportCommands[2].cmd, fakeAnswer{err: "The gatekeeper is busy", code: 1})
	_, err := array.Collect(context.Background(), newTestArray(t, fake))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sym_dev_info")
}
