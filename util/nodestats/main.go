// Package nodestats collects the performance statistics of the node the
// collector keeps: cpu, memory, swap, load, block and network i/o, as the
// pushstats action of the opensvc v2 agent read them from sysstat, and the file
// system usage.
//
// The statistics are given by group, each group a list of columns and of rows,
// in the shape and with the column names the collector stores them by: the
// "date" column dates a row, the cpu, dev or mntpt column, for the groups
// having one, names the unit the row is about, and the other columns are the
// metrics.
package nodestats

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/df"
)

type (
	// Group is the statistics of a group: the column names, then the rows,
	// each with a value per column.
	Group struct {
		Columns []string   `json:"columns"`
		Rows    [][]string `json:"rows"`
	}

	// Stats are the statistics by group name.
	Stats map[string]Group

	// Options selects what Collect reads.
	Options struct {
		// Begin and End bound the samples read from sysstat.
		Begin time.Time
		End   time.Time

		// Dir is the sysstat data directory; /var/log/sysstat, else
		// /var/log/sa, when empty.
		Dir string

		// Disable lists the groups not to collect.
		Disable []string
	}
)

// Groups are the groups Collect knows, in the order they are collected.
var Groups = []string{"cpu", "mem_u", "swap", "proc", "block", "blockdev", "netdev", "netdev_err", "fs_u"}

// ErrNotSupported is returned on an operating system whose statistics are not
// collected.
var ErrNotSupported = errors.New("node statistics are collected on linux only")

// sadfArgs are the sar reports asked to sadf: cpu per processor, memory, swap,
// queue and load, block i/o, block devices by their pretty name, network
// interfaces and their errors.
var sadfArgs = []string{"--", "-u", "ALL", "-P", "ALL", "-r", "-S", "-q", "-b", "-d", "-p", "-n", "DEV,EDEV"}

// Collect returns the statistics of the node between o.Begin and o.End, but for
// the disabled groups. The file system usage is the one at the time of the
// call, sysstat not keeping it.
func Collect(ctx context.Context, o Options) (Stats, error) {
	if runtime.GOOS != "linux" {
		return nil, ErrNotSupported
	}
	if !o.End.After(o.Begin) {
		return nil, fmt.Errorf("the end %s is not after the begin %s", o.End, o.Begin)
	}
	stats := Stats{}
	enabled := func(group string) bool { return !slices.Contains(o.Disable, group) }
	needSadf := slices.ContainsFunc(Groups, func(g string) bool { return g != "fs_u" && enabled(g) })
	if needSadf {
		if _, err := exec.LookPath("sadf"); err != nil {
			return nil, fmt.Errorf("sadf not found, install sysstat: %w", err)
		}
		dir, err := sysstatDir(o.Dir)
		if err != nil {
			return nil, err
		}
		for _, file := range sysstatFiles(dir, o.Begin, o.End) {
			b, err := exec.CommandContext(ctx, "sadf", append([]string{"-j", file}, sadfArgs...)...).Output()
			if err != nil {
				return nil, fmt.Errorf("sadf -j %s: %w", file, err)
			}
			if err := stats.addSadf(b, o.Begin, o.End); err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
		}
	}
	if enabled("fs_u") {
		entries, err := df.Usage(ctx)
		if err != nil {
			return nil, fmt.Errorf("df: %w", err)
		}
		stats["fs_u"] = fsUsage(entries, time.Now())
	}
	for _, group := range o.Disable {
		delete(stats, group)
	}
	return stats, nil
}

func sysstatDir(dir string) (string, error) {
	if dir != "" {
		return dir, nil
	}
	for _, dir := range []string{"/var/log/sysstat", "/var/log/sa"} {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no sysstat data directory: is the sysstat collection enabled?")
}

// sysstatFiles returns the daily data files covering begin to end: saYYYYMMDD
// when sysstat names them so, saDD otherwise. A saDD file of a past month is
// read too: its samples fall out of the range.
func sysstatFiles(dir string, begin, end time.Time) []string {
	var files []string
	// The files are named after the local day, a day being one file.
	day := time.Date(begin.Year(), begin.Month(), begin.Day(), 0, 0, 0, 0, time.Local)
	for !day.After(end) {
		for _, name := range []string{"sa" + day.Format("20060102"), "sa" + day.Format("02")} {
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); err == nil {
				files = append(files, p)
				break
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return files
}

// mntptBlacklist are the mount points whose usage is not pushed, as the v2
// agent left them out.
var mntptBlacklist = []*regexp.Regexp{
	regexp.MustCompile(`^/proc$`),
	regexp.MustCompile(`^/sys/fs/cgroup`),
	regexp.MustCompile(`^(/var)?/run/user/[0-9]+`),
	regexp.MustCompile(`^.*/docker/.*/[0-9a-f]{64}`),
}

// fsUsage is the usage of the local file systems at a time: the size in KiB and
// the percentage used, as the v2 agent pushed df -lP.
func fsUsage(entries []df.Entry, now time.Time) Group {
	g := Group{Columns: []string{"date", "mntpt", "size", "used"}}
	date := now.UTC().Format(time.RFC3339)
	for _, e := range entries {
		if slices.ContainsFunc(mntptBlacklist, func(re *regexp.Regexp) bool { return re.MatchString(e.MountPoint) }) {
			continue
		}
		g.Rows = append(g.Rows, []string{date, e.MountPoint, strconv.FormatInt(e.Total/1024, 10), strconv.FormatInt(e.UsedPercent, 10)})
	}
	return g
}

type (
	sadfDoc struct {
		Sysstat struct {
			Hosts []struct {
				Statistics []sadfSample `json:"statistics"`
			} `json:"hosts"`
		} `json:"sysstat"`
	}

	sadfSample struct {
		Timestamp struct {
			Date string `json:"date"`
			Time string `json:"time"`
			UTC  int    `json:"utc"`
		} `json:"timestamp"`
		CPULoad []map[string]any `json:"cpu-load"`
		Memory  map[string]any   `json:"memory"`
		Queue   map[string]any   `json:"queue"`
		IO      map[string]any   `json:"io"`
		Disk    []map[string]any `json:"disk"`
		Network struct {
			NetDev  []map[string]any `json:"net-dev"`
			NetEdev []map[string]any `json:"net-edev"`
		} `json:"network"`
	}

	// field maps a sadf key, dotted for a nested object, to a collector column.
	field struct {
		key    string
		column string
	}
)

// The sadf keys of each group and the column the collector names them by, the
// sar column names of the v2 agent. The first field of a group by unit names
// the unit.
var (
	cpuFields = []field{
		{"cpu", "cpu"}, {"usr", "usr"}, {"nice", "nice"}, {"sys", "sys"}, {"iowait", "iowait"},
		{"steal", "steal"}, {"irq", "irq"}, {"soft", "soft"}, {"guest", "guest"}, {"gnice", "gnice"},
		{"idle", "idle"},
	}
	memFields = []field{
		{"memfree", "kbmemfree"}, {"avail", "kbavail"}, {"memused", "kbmemused"},
		{"memused-percent", "pct_memused"}, {"buffers", "kbbuffers"}, {"cached", "kbcached"},
		{"commit", "kbcommit"}, {"commit-percent", "pct_commit"}, {"active", "kbactive"},
		{"inactive", "kbinact"}, {"dirty", "kbdirty"},
	}
	swapFields = []field{
		{"swpfree", "kbswpfree"}, {"swpused", "kbswpused"}, {"swpused-percent", "pct_swpused"},
		{"swpcad", "kbswpcad"}, {"swpcad-percent", "pct_swpcad"},
	}
	procFields = []field{
		{"runq-sz", "runq_sz"}, {"plist-sz", "plist_sz"}, {"ldavg-1", "ldavg_1"},
		{"ldavg-5", "ldavg_5"}, {"ldavg-15", "ldavg_15"},
	}
	blockFields = []field{
		{"tps", "tps"}, {"io-reads.rtps", "rtps"}, {"io-writes.wtps", "wtps"},
		{"io-reads.bread", "rbps"}, {"io-writes.bwrtn", "wbps"},
	}
	blockdevFields = []field{
		{"disk-device", "dev"}, {"tps", "tps"}, {"rd_sec", "rsecps"}, {"wr_sec", "wsecps"},
		{"avgrq-sz", "avgrq_sz"}, {"avgqu-sz", "avgqu_sz"}, {"await", "await"}, {"svctm", "svctm"},
		{"util-percent", "pct_util"},
	}
	netdevFields = []field{
		{"iface", "dev"}, {"rxpck", "rxpckps"}, {"txpck", "txpckps"}, {"rxkB", "rxkBps"}, {"txkB", "txkBps"},
	}
	netdevErrFields = []field{
		{"iface", "dev"}, {"rxerr", "rxerrps"}, {"txerr", "txerrps"}, {"coll", "collps"},
		{"rxdrop", "rxdropps"}, {"txdrop", "txdropps"},
	}
)

// ifaceSkip are the interfaces the v2 agent left out: the loopback and the
// virtual ones, by name prefix.
var ifaceSkip = regexp.MustCompile(`^(lo$|dummy|vnet|veth|pan|sit)`)

// addSadf adds the samples of a sadf -j document dated between begin and end.
func (stats Stats) addSadf(b []byte, begin, end time.Time) error {
	var doc sadfDoc
	if err := json.NewDecoder(bytes.NewReader(b)).Decode(&doc); err != nil {
		return fmt.Errorf("decode sadf output: %w", err)
	}
	for _, host := range doc.Sysstat.Hosts {
		for _, sample := range host.Statistics {
			loc := time.Local
			if sample.Timestamp.UTC == 1 {
				loc = time.UTC
			}
			t, err := time.ParseInLocation("2006-01-02 15:04:05", sample.Timestamp.Date+" "+sample.Timestamp.Time, loc)
			if err != nil || t.Before(begin) || t.After(end) {
				continue
			}
			date := t.UTC().Format(time.RFC3339)
			for _, cpu := range sample.CPULoad {
				stats.add("cpu", cpuFields, date, cpu)
			}
			if sample.Memory != nil {
				stats.add("mem_u", memFields, date, sample.Memory)
				stats.add("swap", swapFields, date, sample.Memory)
			}
			if sample.Queue != nil {
				stats.add("proc", procFields, date, sample.Queue)
			}
			if sample.IO != nil {
				stats.add("block", blockFields, date, sample.IO)
			}
			for _, disk := range sample.Disk {
				stats.add("blockdev", blockdevFields, date, disk)
			}
			for _, iface := range sample.Network.NetDev {
				if name, _ := iface["iface"].(string); !ifaceSkip.MatchString(name) {
					stats.add("netdev", netdevFields, date, iface)
				}
			}
			for _, iface := range sample.Network.NetEdev {
				if name, _ := iface["iface"].(string); !ifaceSkip.MatchString(name) {
					stats.add("netdev_err", netdevErrFields, date, iface)
				}
			}
		}
	}
	return nil
}

// add appends a row to a group, the fields the sample lacks left out of the
// columns: a sysstat version reports some, another not. The columns of a group
// are those of its first row; a later row lacking one has it empty.
func (stats Stats) add(group string, fields []field, date string, values map[string]any) {
	g, ok := stats[group]
	if !ok {
		g.Columns = []string{"date"}
		for _, f := range fields {
			if _, ok := lookup(values, f.key); ok {
				g.Columns = append(g.Columns, f.column)
			}
		}
	}
	byColumn := map[string]string{}
	for _, f := range fields {
		if v, ok := lookup(values, f.key); ok {
			byColumn[f.column] = v
		}
	}
	row := make([]string, len(g.Columns))
	row[0] = date
	for i, col := range g.Columns[1:] {
		row[i+1] = byColumn[col]
	}
	g.Rows = append(g.Rows, row)
	stats[group] = g
}

// lookup reads a value of a sadf object, a dotted key reading a nested object,
// as the text the collector expects.
func lookup(values map[string]any, key string) (string, bool) {
	var v any = values
	for _, part := range strings.Split(key, ".") {
		m, ok := v.(map[string]any)
		if !ok {
			return "", false
		}
		if v, ok = m[part]; !ok {
			return "", false
		}
	}
	switch x := v.(type) {
	case string:
		return x, true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case json.Number:
		return x.String(), true
	default:
		return "", false
	}
}
