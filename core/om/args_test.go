package om

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/scheduler"
	"github.com/opensvc/om3/v3/testhelper"
	"github.com/opensvc/om3/v3/util/hostname"
)

// requireResolves fails when args name no runnable command of the om tree.
//
// cobra prints a help text and exits 0 in that case, so an argv the daemon
// execs can rot unnoticed: this is the only thing keeping the daemon argv and
// the command tree in sync.
func requireResolves(t *testing.T, args []string) {
	t.Helper()
	final := setExecuteArgs(args)
	require.NoErrorf(t, commoncmd.ValidateArgs(root, final),
		"om %s", strings.Join(args, " "))
	cmd, _, err := root.Find(final)
	require.NoErrorf(t, err, "om %s", strings.Join(args, " "))
	assert.Truef(t, cmd.Runnable(), "om %s: %q is not runnable",
		strings.Join(args, " "), cmd.CommandPath())
}

// The daemon scheduler execs om with an argv it builds from the entry action.
func TestSchedulerCmdArgsResolve(t *testing.T) {
	objectPath, err := naming.ParsePath("test/svc/s1")
	require.NoError(t, err)

	for _, test := range []struct {
		actions []string
		path    naming.Path
	}{
		{actions: scheduler.ObjectActions, path: objectPath},
		{actions: scheduler.NodeActions},
	} {
		for _, action := range test.actions {
			t.Run(action, func(t *testing.T) {
				e := schedule.Entry{
					Path: test.path,
					Config: schedule.Config{
						Action: action,
						Key:    "task#1.schedule",
					},
				}
				args, err := scheduler.CmdArgs(e)
				require.NoError(t, err)
				requireResolves(t, args)
			})
		}
	}
}

// An action scheduler.CmdArgs does not know must be an error, not an argv the
// scheduler happily execs.
func TestSchedulerCmdArgsRejectsUnknownAction(t *testing.T) {
	_, err := scheduler.CmdArgs(schedule.Entry{Config: schedule.Config{Action: "no_such_action"}})
	assert.Error(t, err)
}

// The api handlers exec om with an argv they build inline. Read them back from
// the source: there is no other enumeration of them.
func TestDaemonAPIExecArgsResolve(t *testing.T) {
	args := daemonAPIExecArgs(t)
	require.NotEmpty(t, args, "no api exec argv found: has the source moved?")
	for _, a := range args {
		t.Run(strings.Join(a, " "), func(t *testing.T) {
			requireResolves(t, a)
		})
	}
}

// daemonAPIExecArgs returns the `args := []string{...}` literals of the
// daemonapi package, with the object path placeholder substituted. Elements
// that are not string literals, and the args appended conditionally, are left
// out: they are flags, never command names.
func daemonAPIExecArgs(t *testing.T) [][]string {
	t.Helper()
	dir := filepath.Join("..", "..", "daemon", "daemonapi")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	fset := token.NewFileSet()
	var out [][]string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(n ast.Node) bool {
			assign, ok := n.(*ast.AssignStmt)
			if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
				return true
			}
			if ident, ok := assign.Lhs[0].(*ast.Ident); !ok || ident.Name != "args" {
				return true
			}
			lit, ok := assign.Rhs[0].(*ast.CompositeLit)
			if !ok {
				return true
			}
			if arr, ok := lit.Type.(*ast.ArrayType); !ok {
				return true
			} else if ident, ok := arr.Elt.(*ast.Ident); !ok || ident.Name != "string" {
				return true
			}
			words := make([]string, 0, len(lit.Elts))
			for _, elt := range lit.Elts {
				switch v := elt.(type) {
				case *ast.BasicLit:
					if v.Kind != token.STRING {
						return true
					}
					s, err := strconv.Unquote(v.Value)
					if err != nil {
						return true
					}
					words = append(words, s)
				case *ast.CallExpr:
					// p.String(), the object path
					words = append(words, "test/svc/s1")
				default:
					// something we can not evaluate: skip the whole argv
					// rather than test a truncated one
					return true
				}
			}
			if len(words) > 0 {
				out = append(out, words)
			}
			return true
		})
	}
	return out
}

// The node configuration generates schedule entries of its own, one per array,
// switch and backup section. Their actions are in no list this file can
// iterate, which is how "push"+type entries went unrunnable unnoticed: the
// scheduler had no case for them, so every due run failed with "unknown
// scheduler action".
//
// So the entries are generated here, from a configuration holding one section
// of each kind, and every action they carry has to be one the scheduler knows:
// either an argv resolving in the om tree, or a declared placeholder.
func TestNodeConfigSchedulesRunnableActions(t *testing.T) {
	env := testhelper.Setup(t)

	// The sections go in cluster.conf, which is where an array is declared:
	// it is a resource of the cluster, reachable from every node, not of one
	// node. Only the merged configuration holds both files.
	require.NoError(t, os.WriteFile(
		filepath.Join(env.Root, "etc", "cluster.conf"),
		[]byte(`[cluster]
name = clu1
nodes = `+hostname.Hostname()+`

[array#baie1]
type = pure
schedule = @1440

[switch#sw1]
type = brocade
schedule = @1440

[backup#bck1]
type = nsr
schedule = @1440
`), 0644))

	n, err := object.NewNode()
	require.NoError(t, err)

	seen := make(map[string]bool)
	for _, e := range n.Schedules() {
		seen[e.Action] = true
		args, err := scheduler.CmdArgs(e)
		if slices.Contains(scheduler.PlaceholderActions, e.Action) {
			assert.Errorf(t, err, "%s is a placeholder: it must not resolve to an argv", e.Action)
			continue
		}
		require.NoErrorf(t, err, "%s scheduled by %s", e.Action, e.Key)
		requireResolves(t, args)
	}

	// The sections above are there to be scheduled: a change silently dropping
	// one would leave this test asserting nothing.
	for _, action := range []string{"pusharray", "pushswitch", "pushbackup"} {
		assert.Truef(t, seen[action], "no schedule entry for %s", action)
	}
}

// The array a "pusharray" entry pushes is the section its schedule was read
// from, and the argv has to name it: without it every array section would push
// every array, once per section.
func TestPushArrayArgvNamesTheArray(t *testing.T) {
	args, err := scheduler.CmdArgs(schedule.Entry{
		Config: schedule.Config{Action: "pusharray", Key: "array#baie1.schedule"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"node", "push", "array", "array#baie1"}, args)
}

// The array is named as an argument, which is the documented form, and with
// --array, which is the form the command was born with. Naming it twice is a
// mistake rather than a precedence question.
func TestNodePushArrayNamesTheArrayOnce(t *testing.T) {
	cmd := newCmdNodePushArray()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"freenas", "--array", "freenas"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "named twice")
}

// One array or none: a second argument is a typo, not a second array, since
// the command pushes every array when it is handed no name at all.
func TestNodePushArrayTakesAtMostOneName(t *testing.T) {
	cmd := newCmdNodePushArray()
	assert.NoError(t, cmd.Args(cmd, []string{}))
	assert.NoError(t, cmd.Args(cmd, []string{"freenas"}))
	assert.Error(t, cmd.Args(cmd, []string{"freenas", "baie2"}))
}
