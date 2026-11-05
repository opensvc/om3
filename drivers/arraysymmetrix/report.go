package arraysymmetrix

import (
	"context"

	"github.com/opensvc/om3/v3/core/array"
)

// Reports returns the sections of its configuration this array pushes to the
// collector: the xml outputs of the symcli commands the collector parses,
// under the names v2 pushed them.
//
// v2 pushed seven more, the masking views and groups, the rdf groups, the
// rdf devices and the service levels, which the collector does not read.
//
// The array, its directors, its devices and their wwn are required: the
// collector makes no row without them. The other outputs are reported
// empty when their command fails, as the array has none of what they list:
// a powermax has no thin pool and no back-end disk group, a vmax2 no storage
// resource pool, an array without ficon no ficon device.
func (t *Array) Reports() []array.Report {
	section := func(key string, required bool, bin string, args ...string) array.Report {
		return array.Report{
			Key: key,
			Get: func(ctx context.Context) (any, error) {
				b, err := t.symXML(ctx, bin, t.kwSID(), args...)
				if err != nil {
					if required {
						return nil, err
					}
					t.Log().Debugf("report %s empty: %s", key, err)
					return "", nil
				}
				return string(b), nil
			},
		}
	}
	return []array.Report{
		section("sym_info", true, "symcfg", "list"),
		section("sym_dir_info", true, "symcfg", "-dir", "all", "-v", "list"),
		section("sym_dev_info", true, "symdev", "list"),
		section("sym_dev_wwn_info", true, "symdev", "list", "-wwn"),
		section("sym_dev_name_info", false, "symdev", "list", "-identifier", "device_name"),
		section("sym_ficondev_info", false, "symdev", "list", "-ficon"),
		section("sym_meta_info", false, "symdev", "list", "-meta", "-v"),
		section("sym_disk_info", false, "symdisk", "list", "-v"),
		section("sym_diskgroup_info", false, "symdisk", "list", "-dskgrp_summary"),
		section("sym_pool_info", false, "symcfg", "-pool", "list", "-v"),
		section("sym_tdev_info", false, "symcfg", "list", "-tdev", "-detail"),
		section("sym_srp_info", false, "symcfg", "list", "-srp", "-detail", "-v"),
		section("sym_sg_info", false, "symsg", "list", "-v"),
	}
}
