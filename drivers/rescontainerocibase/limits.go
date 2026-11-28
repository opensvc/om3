package rescontainerocibase

import (
	"slices"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/drivers/resapp"
	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/ulimit"
)

// LimitKeywords are the limit_* keywords of the app resources, for the
// containers and the tasks of the oci drivers: the same names and values,
// with a text and a group of their own, as the engine sets them on the
// container where om exec sets them on a command of the node.
var LimitKeywords = limitKeywords(
	resapp.UnixKeywordLimitCPU,
	resapp.UnixKeywordLimitCore,
	resapp.UnixKeywordLimitData,
	resapp.UnixKeywordLimitFSize,
	resapp.UnixKeywordLimitMemLock,
	resapp.UnixKeywordLimitNoFile,
	resapp.UnixKeywordLimitNProc,
	resapp.UnixKeywordLimitRSS,
	resapp.UnixKeywordLimitStack,
	resapp.UnixKeywordLimitVmem,
	resapp.UnixKeywordLimitAS,
)

func limitKeywords(l ...keywords.Keyword) []*keywords.Keyword {
	kws := make([]*keywords.Keyword, 0, len(l))
	for _, kw := range l {
		kw.Group = GroupLimits.ID
		kw.Text = keywords.NewText(fs, "text/kw/"+kw.Option)
		kws = append(kws, &kw)
	}
	return kws
}

// engineUlimit is a limit as the engines take it: --ulimit <name>=<value>.
type engineUlimit struct {
	name  string
	value int64
}

// engineUlimits returns the limits of the limit_* keywords set, by the names
// the engines know them, in a stable order.
//
// limit_vmem and limit_as are the same limit of the kernel, the address
// space: the larger is kept when both are set, as their texts say. limit_rss
// is passed as the others, though a Linux kernel enforces none.
func engineUlimits(c ulimit.Config) []engineUlimit {
	l := make([]engineUlimit, 0)
	add := func(name string, v *int64) {
		if v != nil {
			l = append(l, engineUlimit{name, *v})
		}
	}
	if c.CPU != nil {
		l = append(l, engineUlimit{"cpu", int64(c.CPU.Seconds())})
	}
	add("fsize", c.FSize)
	add("data", c.Data)
	add("stack", c.Stack)
	add("core", c.Core)
	add("nofile", c.NoFile)
	switch {
	case c.AS != nil && c.VMem != nil:
		v := max(*c.AS, *c.VMem)
		add("as", &v)
	case c.AS != nil:
		add("as", c.AS)
	default:
		add("as", c.VMem)
	}
	add("memlock", c.MemLock)
	add("nproc", c.NProc)
	add("rss", c.RSS)
	return l
}

// runArgsUlimit returns the --ulimit options of the limit_* keywords set,
// after dropping the same limits from the run_args: a keyword and an option
// setting one limit would be two settings of it, and the keyword is the one
// documented to decide, as hostname over --hostname. The limits no keyword
// sets are left to the run_args, and to the defaults of the engine.
//
// The soft and the hard limits are set to the same value, as om exec sets
// the limit_* of an app: the soft one is the one enforced.
func (ea *ExecutorArg) runArgsUlimit(runArgs *args.T) []string {
	limits := engineUlimits(ea.BT.Limit)
	a := make([]string, 0, 2*len(limits))
	for _, limit := range limits {
		runArgs.DropOptionAndMatchingValue("--ulimit", "^"+limit.name+"=")
		joined := "--ulimit=" + limit.name + "="
		runArgs.Set(slices.DeleteFunc(runArgs.Get(), func(s string) bool {
			return strings.HasPrefix(s, joined)
		}))
		v := strconv.FormatInt(limit.value, 10)
		a = append(a, "--ulimit", limit.name+"="+v+":"+v)
	}
	return a
}
