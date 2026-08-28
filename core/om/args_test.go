package om

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/schedule"
	"github.com/opensvc/om3/v3/daemon/scheduler"
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
