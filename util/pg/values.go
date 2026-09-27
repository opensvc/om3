package pg

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

// The Valid* functions say whether a value is one the pg_* keyword they are
// named after takes, so the validation of a configuration refuses what the
// apply of the capping would, rather than the apply failing on every node,
// when the instance next starts or is capped.
//
// They accept DefaultValue where the apply puts the capping back, which is
// every keyword but the two of the v1 hierarchy alone.

// ValidCPUs says whether s is a pg_cpus or pg_mems value: a list of numbers
// and ranges of numbers, as 0-2,4.
func ValidCPUs(s string) error {
	if s == DefaultValue {
		return nil
	}
	for _, e := range strings.Split(s, ",") {
		first, last, isRange := strings.Cut(e, "-")
		lo, err := strconv.ParseUint(first, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid list %q: %q is not a number or a range of numbers (accepted expressions: 0,1,2 or 0-2)", s, e)
		}
		if !isRange {
			continue
		}
		hi, err := strconv.ParseUint(last, 10, 32)
		if err != nil || hi < lo {
			return fmt.Errorf("invalid list %q: %q is not a number or a range of numbers (accepted expressions: 0,1,2 or 0-2)", s, e)
		}
	}
	return nil
}

// ValidCPUQuota says whether s is a pg_cpu_quota or pg_cpu_burst value: a
// percentage of cpus, as 50%, 50%@all or 10%@2.
func ValidCPUQuota(s string) error {
	if s == DefaultValue {
		return nil
	}
	_, _, err := CPUQuota(s).parse()
	return err
}

// ValidSize says whether s is a size, as the memory keywords and
// pg_cpu_shares take.
func ValidSize(s string) error {
	if s == DefaultValue {
		return nil
	}
	n, err := sizeconv.FromSize(s)
	if err != nil {
		return err
	}
	if n < 0 {
		return fmt.Errorf("invalid size %q: negative", s)
	}
	return nil
}

// ValidPidsMax says whether s is a pg_pids_max value: a count of processes.
func ValidPidsMax(s string) error {
	if s == DefaultValue {
		return nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err != nil || n < 1 {
		return fmt.Errorf("%q is not a count of processes", s)
	}
	return nil
}

// ValidBlkioWeight says whether s is a pg_blkio_weight value: a weight the
// kernel takes, between 1 and 10000 on the unified hierarchy, where the v1
// one takes 10 to 1000.
func ValidBlkioWeight(s string) error {
	if s == DefaultValue {
		return nil
	}
	if n, err := strconv.ParseUint(s, 10, 16); err != nil || n < 1 || n > 10000 {
		return fmt.Errorf("%q is not a block io weight, between 1 and 10000", s)
	}
	return nil
}

// ValidMemSwappiness says whether s is a pg_mem_swappiness value: a
// percentage.
//
// It has no DefaultValue: the capping is of the v1 hierarchy, which the reset
// is not written for. The keyword removed leaves the kernel value alone.
func ValidMemSwappiness(s string) error {
	if n, err := strconv.ParseUint(s, 10, 64); err != nil || n > 100 {
		return fmt.Errorf("%q is not a swappiness, between 0 and 100", s)
	}
	return nil
}

// ValidMemOOMControl says whether s is a pg_mem_oom_control value: a flag.
//
// It has no DefaultValue, for the reason ValidMemSwappiness has none.
func ValidMemOOMControl(s string) error {
	if _, err := converters.Bool.Convert(s); err != nil {
		return fmt.Errorf("%q is not a flag, 0 or 1", s)
	}
	return nil
}
