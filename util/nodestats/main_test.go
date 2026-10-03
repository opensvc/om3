package nodestats

import (
	"os"
	"testing"
	"time"

	"github.com/opensvc/om3/v3/util/df"
)

func column(g Group, name string) int {
	for i, c := range g.Columns {
		if c == name {
			return i
		}
	}
	return -1
}

func TestAddSadf(t *testing.T) {
	b, err := os.ReadFile("testdata/sadf.json")
	if err != nil {
		t.Fatal(err)
	}
	begin := time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 30, 22, 15, 0, 0, time.UTC)
	stats := Stats{}
	if err := stats.addSadf(b, begin, end); err != nil {
		t.Fatal(err)
	}

	// One sample of the two is in the range: one row per cpu, "all" and the
	// two processors.
	cpu := stats["cpu"]
	if len(cpu.Rows) != 3 {
		t.Fatalf("cpu rows: %v", cpu.Rows)
	}
	if cpu.Columns[0] != "date" || column(cpu, "cpu") < 0 || column(cpu, "usr") < 0 || column(cpu, "idle") < 0 {
		t.Errorf("cpu columns: %v", cpu.Columns)
	}
	if got := cpu.Rows[0][0]; got != "2026-09-30T22:10:03Z" {
		t.Errorf("date: %s", got)
	}
	if got := cpu.Rows[0][column(cpu, "cpu")]; got != "all" {
		t.Errorf("first cpu: %s", got)
	}

	for group, cols := range map[string][]string{
		"mem_u":      {"kbmemfree", "kbmemused", "pct_memused", "pct_commit"},
		"swap":       {"kbswpfree", "pct_swpused", "pct_swpcad"},
		"proc":       {"runq_sz", "ldavg_1", "ldavg_15"},
		"block":      {"tps", "rtps", "wtps", "rbps", "wbps"},
		"blockdev":   {"dev", "tps", "await", "pct_util"},
		"netdev":     {"dev", "rxkBps", "txkBps", "rxpckps", "txpckps"},
		"netdev_err": {"dev", "rxerrps", "txerrps", "collps", "rxdropps", "txdropps"},
	} {
		g := stats[group]
		if len(g.Rows) == 0 {
			t.Errorf("%s: no row", group)
			continue
		}
		for _, col := range cols {
			if column(g, col) < 0 {
				t.Errorf("%s: no %s column in %v", group, col, g.Columns)
			}
		}
		for _, row := range g.Rows {
			if len(row) != len(g.Columns) {
				t.Errorf("%s: a row of %d values for %d columns", group, len(row), len(g.Columns))
			}
		}
	}
	// The loopback and the virtual interfaces are left out.
	for _, group := range []string{"netdev", "netdev_err"} {
		g := stats[group]
		for _, row := range g.Rows {
			if dev := row[column(g, "dev")]; dev != "enp1s0" {
				t.Errorf("%s: interface %s pushed", group, dev)
			}
		}
	}
	if got := len(stats["blockdev"].Rows); got != 2 {
		t.Errorf("blockdev rows: %d", got)
	}
}

func TestFsUsage(t *testing.T) {
	now := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	g := fsUsage([]df.Entry{
		{MountPoint: "/", Total: 10 * 1024 * 1024, UsedPercent: 42},
		{MountPoint: "/proc", Total: 0},
		{MountPoint: "/run/user/1000", Total: 1024},
		{MountPoint: "/var/lib/docker/overlay2/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef/merged", Total: 1024},
	}, now)
	if len(g.Rows) != 1 {
		t.Fatalf("rows: %v", g.Rows)
	}
	want := []string{"2026-10-01T10:00:00Z", "/", "10240", "42"}
	for i, v := range want {
		if g.Rows[0][i] != v {
			t.Errorf("row: %v, want %v", g.Rows[0], want)
			break
		}
	}
}

func TestSysstatFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"sa30", "sa20261001"} {
		if err := os.WriteFile(dir+"/"+name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	begin := time.Date(2026, 9, 30, 12, 0, 0, 0, time.Local)
	end := time.Date(2026, 10, 1, 1, 0, 0, 0, time.Local)
	files := sysstatFiles(dir, begin, end)
	if len(files) != 2 || files[0] != dir+"/sa30" || files[1] != dir+"/sa20261001" {
		t.Errorf("files: %v", files)
	}
}
