//go:build linux

package pg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// v1Reset is a write lifting a capping of a group of the v1 hierarchy.
type v1Reset struct {
	// controller is the hierarchy the file is in, as it is mounted under
	// the cgroup mount: cpu is the link systemd makes to cpu,cpuacct.
	controller string
	file       string

	// value is what the file is written with, unless fromParent is set.
	value string

	// fromParent says the value is the one the parent group holds in the
	// same file: a cpuset is the cpus its parent offers, not a constant.
	fromParent bool

	// optional says a group may have no such file, and is not failed on:
	// memory.memsw.* is there only with the swap accounting enabled, and
	// blkio.weight only with the cfq scheduler.
	optional bool

	// keywords are the pg_* keywords whose DefaultValue the write is part
	// of. A write no keyword names is done by a full reset only.
	keywords []string
}

// v1Resets is what lifting a capping writes in a group of the v1
// hierarchy, in the order it writes it: all of it for "pg reset", the writes
// of a keyword for that keyword set to DefaultValue, on a node holding the
// v1 or the hybrid hierarchy.
//
// The values are the ones a fresh group holds, read on a v1 node: a RHEL 7.9
// lab node, kernel 3.10.0-1160, on 2026-11-13, in groups made under
// opensvc.slice. -1 reads back as 9223372036854771712 in the memory limits,
// the swappiness and the cpuset are the parent's (a fresh cpuset is empty
// until it is given its parent's), and cpu.cfs_period_us needs no reset: om
// always writes 100000, the default. TestOnV1AFullResetLiftsTheCappingToWhatAFreshSiblingHolds
// and TestOnV1ADefaultKeywordLiftsItsCappingToWhatAFreshSiblingHolds compare
// them again, as root on a v1 node.
//
// The memory+swap limit is lifted before the memory limit: the kernel
// refuses a memory limit above the memory+swap one, so lifting the memory
// limit first fails on a group capping both. For the same reason a
// pg_mem_limit set to DefaultValue lifts the memory+swap limit too: a group
// with no memory limit cannot keep a memory+swap one below it.
var v1Resets = []v1Reset{
	{controller: "memory", file: "memory.memsw.limit_in_bytes", value: "-1", optional: true, keywords: []string{"pg_mem_limit", "pg_vmem_limit"}},
	{controller: "memory", file: "memory.limit_in_bytes", value: "-1", keywords: []string{"pg_mem_limit"}},
	{controller: "memory", file: "memory.swappiness", fromParent: true},
	{controller: "memory", file: "memory.oom_control", value: "0"},
	{controller: "cpu", file: "cpu.cfs_quota_us", value: "-1", keywords: []string{"pg_cpu_quota"}},
	{controller: "cpu", file: "cpu.shares", value: "1024", keywords: []string{"pg_cpu_shares"}},
	{controller: "cpuset", file: "cpuset.cpus", fromParent: true, keywords: []string{"pg_cpus"}},
	{controller: "cpuset", file: "cpuset.mems", fromParent: true, keywords: []string{"pg_mems"}},
	{controller: "blkio", file: "blkio.weight", value: "500", optional: true, keywords: []string{"pg_blkio_weight"}},
	{controller: "pids", file: "pids.max", value: "max", keywords: []string{"pg_pids_max"}},
}

// v1ResetsFor returns the writes of v1Resets lifting the cappings of the
// keywords kws, in the order of the table, or all of them for a full reset.
func v1ResetsFor(full bool, kws []string) []v1Reset {
	if full {
		return v1Resets
	}
	l := make([]v1Reset, 0)
	for _, reset := range v1Resets {
		for _, kw := range kws {
			if slices.Contains(reset.keywords, kw) {
				l = append(l, reset)
				break
			}
		}
	}
	return l
}

// hasV1Reset reports whether the v1 hierarchy has a reset for the keyword.
func hasV1Reset(kw string) bool {
	return len(v1ResetsFor(false, []string{kw})) > 0
}

// applyV1Resets writes resets in the group id of the v1 hierarchies mounted
// under root.
//
// A controller the node does not mount caps nothing, and is skipped. Every
// write is tried, and the failures are reported together.
func applyV1Resets(root, id string, resets []v1Reset) error {
	var errs error
	for _, reset := range resets {
		hierarchy := filepath.Join(root, reset.controller)
		if _, err := os.Stat(hierarchy); errors.Is(err, os.ErrNotExist) {
			continue
		}
		path := filepath.Join(hierarchy, id, reset.file)
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) && reset.optional {
			continue
		}
		value := reset.value
		if reset.fromParent {
			b, err := os.ReadFile(filepath.Join(hierarchy, filepath.Dir(id), reset.file))
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("reset %s: read the parent group: %w", reset.file, err))
				continue
			}
			value = strings.TrimSpace(string(b))
			if value == "" {
				errs = errors.Join(errs, fmt.Errorf("reset %s: the parent group holds none", reset.file))
				continue
			}
		}
		if err := writeExisting(path, value); err != nil {
			errs = errors.Join(errs, fmt.Errorf("reset %s: %w", reset.file, err))
		}
	}
	return errs
}

// writeExisting writes value in the file at path, which it does not create:
// a file a group lacks is a controller the group was not made with.
func writeExisting(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString(value)
	return errors.Join(err, f.Close())
}
