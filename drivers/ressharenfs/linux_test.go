//go:build linux

package ressharenfs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/capabilities"
)

// fakeExportfs answers "exportfs -v" from the exports file, a "<path>
// <client>(<opts>)" line per export, and adds or removes a line on "exportfs
// -i -o <opts> <client>:<path>" and "exportfs -u <client>:<path>", which
// fails on an export it does not hold as exportfs does. Every call is
// appended to the calls file.
const fakeExportfs = `#!/bin/sh
dir=$(dirname "$0")
echo "$*" >>"$dir/calls"
touch "$dir/exports"
case "$1" in
-v)
	cat "$dir/exports"
	;;
-u)
	client=${2%%:*}
	path=${2#*:}
	grep -v "^$path $client(" "$dir/exports" >"$dir/exports.new"
	if cmp -s "$dir/exports" "$dir/exports.new"; then
		echo "exportfs: Could not find '$2' to unexport." >&2
		exit 1
	fi
	mv "$dir/exports.new" "$dir/exports"
	;;
-i)
	client=${4%%:*}
	path=${4#*:}
	echo "$path $client($3)" >>"$dir/exports"
	;;
esac
`

// fakeShowmount lists the exports of the exports file as the kernel holds
// them, the path and its clients, but for the clients of the nokernel file.
const fakeShowmount = `#!/bin/sh
dir=$(dirname "$0")
touch "$dir/exports" "$dir/nokernel"
grep -v -F -f "$dir/nokernel" "$dir/exports" | sed 's/(.*//' | awk '{ c[$1] = c[$1] ? c[$1] "," $2 : $2 } END { for (p in c) print p " " c[p] }'
`

var fakeDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ressharenfs.")
	if err != nil {
		panic(err)
	}
	fakeDir = dir
	for name, script := range map[string]string{"exportfs": fakeExportfs, "showmount": fakeShowmount} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0755); err != nil {
			panic(err)
		}
	}
	caps, _ := json.Marshal([]string{
		capabilities.MakePath("exportfs", filepath.Join(dir, "exportfs")),
		capabilities.MakePath("showmount", filepath.Join(dir, "showmount")),
		drvID.Cap(),
	})
	if err := os.WriteFile(filepath.Join(dir, "capabilities.json"), caps, 0644); err != nil {
		panic(err)
	}
	capabilities.SetCacheFile(filepath.Join(dir, "capabilities.json"))
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// setExports sets the exports the fake commands hold, and the clients the
// kernel does not export to, and forgets the calls made.
func setExports(t *testing.T, exports []string, noKernel ...string) {
	t.Helper()
	write := func(name string, lines []string) {
		s := strings.Join(lines, "\n")
		if s != "" {
			s += "\n"
		}
		require.NoError(t, os.WriteFile(filepath.Join(fakeDir, name), []byte(s), 0644))
	}
	write("exports", exports)
	write("nokernel", noKernel)
	write("calls", nil)
}

func readFake(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fakeDir, name))
	require.NoError(t, err)
	return strings.FieldsFunc(string(b), func(r rune) bool { return r == '\n' })
}

func newTestShare() *T {
	o := New().(*T)
	o.SharePath = "/srv/share"
	o.ShareOpts = "h1(rw,sync) h2(ro)"
	return o
}

func TestStopUnexportsEveryClientExported(t *testing.T) {
	setExports(t, []string{
		"/srv/share h1(ro)",
		"/srv/share h2(ro,root_squash)",
		"/srv/share h3(rw)",
		"/srv/other h1(rw,sync)",
	}, "h2(")
	o := newTestShare()
	require.Equal(t, status.Warn, o.Status(context.Background()))

	require.NoError(t, o.Stop(context.Background()))
	assert.Equal(t, []string{
		"/srv/share h3(rw)",
		"/srv/other h1(rw,sync)",
	}, readFake(t, "exports"), "the client with missing options and the one the kernel does not export to are unexported, the others kept")
	assert.Equal(t, status.Down, o.Status(context.Background()), "an export to another client is not the share")
}

func TestStopUnexportsNothingNotExported(t *testing.T) {
	setExports(t, []string{"/srv/share h2(ro)"})
	o := newTestShare()
	require.NoError(t, o.Stop(context.Background()))
	assert.Equal(t, []string{"-v", "-u h2:/srv/share"}, readFake(t, "calls"), "h1 is not unexported, which exportfs would fail")
	assert.Empty(t, readFake(t, "exports"))

	setExports(t, nil)
	require.NoError(t, o.Stop(context.Background()))
	assert.Equal(t, []string{"-v"}, readFake(t, "calls"))
}

func TestStatus(t *testing.T) {
	cases := map[string]struct {
		exports  []string
		noKernel []string
		want     status.T
		warns    []string
	}{
		"up": {
			exports: []string{"/srv/share h1(rw,sync,no_subtree_check)", "/srv/share h2(ro)"},
			want:    status.Up,
		},
		"not exported": {
			exports: []string{"/srv/other h1(rw,sync)"},
			want:    status.Down,
		},
		"one client missing": {
			exports: []string{"/srv/share h1(rw,sync)"},
			want:    status.Warn,
			warns:   []string{"/srv/share not exported to client h2"},
		},
		"every client with an issue": {
			exports:  []string{"/srv/share h1(ro)", "/srv/share h2(ro)"},
			noKernel: []string{"h2("},
			want:     status.Warn,
			warns: []string{
				"/srv/share is exported to client h1 with missing options: current 'ro', minimum required 'rw,sync'",
				"/srv/share not exported to client h2 in kernel etab",
			},
		},
		"exported to other clients only": {
			exports: []string{"/srv/share h3(rw)"},
			want:    status.Down,
		},
		"not exported by the kernel": {
			exports:  []string{"/srv/share h1(rw,sync)", "/srv/share h2(ro)"},
			noKernel: []string{"h1(", "h2("},
			want:     status.Warn,
			warns: []string{
				"/srv/share not exported to client h1 in kernel etab",
				"/srv/share not exported to client h2 in kernel etab",
			},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			setExports(t, c.exports, c.noKernel...)
			o := newTestShare()
			assert.Equal(t, c.want, o.Status(context.Background()))
			warns := make([]string, 0)
			for _, e := range o.StatusLog().Entries() {
				if e.Level == "warn" {
					warns = append(warns, e.Message)
				}
			}
			assert.Equal(t, append([]string{}, c.warns...), warns)
		})
	}
}

func TestStartFixesTheOptions(t *testing.T) {
	setExports(t, []string{"/srv/share h1(ro)"})
	o := newTestShare()
	require.NoError(t, o.Start(context.Background()))
	assert.ElementsMatch(t, []string{"/srv/share h1(rw,sync)", "/srv/share h2(ro)"}, readFake(t, "exports"))
	assert.Equal(t, status.Up, o.Status(context.Background()))
}
