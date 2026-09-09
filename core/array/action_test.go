package array

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recorder is a driver's action recording what the command line handed it.
type recorder struct {
	called bool
	in     Input
}

func (t *recorder) action(path ...string) Action {
	return Action{
		Path:  path,
		Short: strings.Join(path, " ") + " something",
		Flags: []Flag{FlagName, FlagSize, FlagLUN, FlagMapping, FlagForce},
		Run: func(_ context.Context, in Input) (any, error) {
			t.called = true
			t.in = in
			return map[string]string{"ok": "yes"}, nil
		},
	}
}

func TestTheTreeReachesAnActionAndHandsItItsOptions(t *testing.T) {
	r := &recorder{}
	var out bytes.Buffer
	err := RunActions(context.Background(), []Action{r.action("add", "disk")},
		[]string{"add", "disk", "--name", "d1", "--size", "1g", "--lun", "7"}, &out)
	require.NoError(t, err)
	require.True(t, r.called, "the action must be reached")

	assert.Equal(t, "d1", r.in.String("name"))
	assert.Equal(t, "1g", r.in.String("size"))
	assert.Equal(t, 7, r.in.Int("lun"))
	assert.False(t, r.in.Bool("force"))

	// What the action returned is rendered.
	var data map[string]string
	require.NoError(t, json.Unmarshal(out.Bytes(), &data))
	assert.Equal(t, "yes", data["ok"])
}

// TestAMappingIsNotSplitOnItsCommas is the trap this catalogue exists to
// avoid: a mapping holds commas of its own, and the option type that splits on
// them would turn one mapping into two halves of one.
func TestAMappingIsNotSplitOnItsCommas(t *testing.T) {
	r := &recorder{}
	err := RunActions(context.Background(), []Action{r.action("add", "disk")},
		[]string{"add", "disk", "--mappings", "iqn.a:tgt1,tgt2", "--mappings", "iqn.b:tgt3"}, &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, []string{"iqn.a:tgt1,tgt2", "iqn.b:tgt3"}, r.in.StringSlice("mappings"))
}

// TestAPathOfThreeWordsIsReached covers the shape the collector calls on a
// freenas array: "array add iscsi zvol".
func TestAPathOfThreeWordsIsReached(t *testing.T) {
	r := &recorder{}
	err := RunActions(context.Background(), []Action{r.action("add", "iscsi", "zvol")},
		[]string{"add", "iscsi", "zvol", "--name", "z1"}, &bytes.Buffer{})
	require.NoError(t, err)
	require.True(t, r.called)
	assert.Equal(t, "z1", r.in.String("name"))
}

// TestActionsShareTheWordsTheyHaveInCommon pins that two actions under one
// word are two leaves of one branch, not two branches.
func TestActionsShareTheWordsTheyHaveInCommon(t *testing.T) {
	disk := &recorder{}
	zvol := &recorder{}
	actions := []Action{disk.action("add", "disk"), zvol.action("add", "zvol")}

	root, err := NewCommand(actions, &bytes.Buffer{})
	require.NoError(t, err)

	var addCmds int
	for _, cmd := range root.Commands() {
		if cmd.Name() == "add" {
			addCmds++
			assert.Len(t, cmd.Commands(), 2, "both actions hang under the one add")
		}
	}
	assert.Equal(t, 1, addCmds, "one add command, not one per action")

	require.NoError(t, RunActions(context.Background(), actions, []string{"add", "zvol"}, &bytes.Buffer{}))
	assert.False(t, disk.called)
	assert.True(t, zvol.called)
}

// TestAWordNobodyDeclaredIsRefused pins that the tree is complete before
// anything is parsed, so a typo is refused where it is typed.
func TestAWordNobodyDeclaredIsRefused(t *testing.T) {
	r := &recorder{}
	actions := []Action{r.action("add", "disk")}

	assert.Error(t, RunActions(context.Background(), actions, []string{"add", "dsik"}, &bytes.Buffer{}))
	assert.False(t, r.called)

	err := RunActions(context.Background(), actions, []string{"add", "disk", "--nosuch", "x"}, &bytes.Buffer{})
	assert.Error(t, err, "an option the action does not declare is refused")
	assert.False(t, r.called)
}

// TestTheArrayIsNamedWhereItWasTyped pins that the tree accepts the option
// that chose the driver, wherever on the line it was written.
func TestTheArrayIsNamedWhereItWasTyped(t *testing.T) {
	for _, args := range [][]string{
		{"-a", "arr1", "add", "disk", "--name", "d1"},
		{"--array", "arr1", "add", "disk", "--name", "d1"},
		{"add", "disk", "-a", "arr1", "--name", "d1"},
		{"add", "disk", "--array=arr1", "--name", "d1"},
	} {
		r := &recorder{}
		err := RunActions(context.Background(), []Action{r.action("add", "disk")}, args, &bytes.Buffer{})
		require.NoErrorf(t, err, "args %v", args)
		assert.Truef(t, r.called, "args %v", args)
		assert.Equalf(t, "d1", r.in.String("name"), "args %v", args)
	}
}

// TestNameFromArgsReadsEverySpelling covers the extraction that picks the
// driver, before any tree exists to parse with.
func TestNameFromArgsReadsEverySpelling(t *testing.T) {
	cases := []struct {
		args     []string
		expected string
	}{
		{[]string{"-a", "arr1", "add", "disk"}, "arr1"},
		{[]string{"-a=arr1", "add", "disk"}, "arr1"},
		{[]string{"--array", "arr1", "add", "disk"}, "arr1"},
		{[]string{"--array=arr1", "add", "disk"}, "arr1"},
		{[]string{"add", "disk", "-a", "arr1"}, "arr1"},
		{[]string{"add", "disk", "--name", "d1", "-a", "arr1"}, "arr1"},
		{[]string{"add", "disk", "--name", "d1"}, ""},
		{[]string{}, ""},
	}
	for _, tc := range cases {
		name, err := NameFromArgs(tc.args)
		require.NoErrorf(t, err, "args %v", tc.args)
		assert.Equalf(t, tc.expected, name, "args %v", tc.args)
	}
}

// TestAnActionRunsWithNoCommandLine is the other half of what the declaration
// buys: an action is callable by something that is not a command line, an api
// handler among them.
func TestAnActionRunsWithNoCommandLine(t *testing.T) {
	r := &recorder{}
	action := r.action("add", "disk")

	in, err := NewInput(action, map[string]any{"name": "d1", "lun": 3})
	require.NoError(t, err)

	data, err := action.Run(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ok": "yes"}, data)
	assert.Equal(t, "d1", r.in.String("name"))
	assert.Equal(t, 3, r.in.Int("lun"))

	// An option the action does not declare is refused rather than ignored.
	_, err = NewInput(action, map[string]any{"nosuch": "x"})
	assert.Error(t, err)
}

// TestAnOptionLeftAloneKeepsItsDefault covers the sentinel defaults a driver
// relies on to tell "not set" from "set to zero".
func TestAnOptionLeftAloneKeepsItsDefault(t *testing.T) {
	r := &recorder{}
	require.NoError(t, RunActions(context.Background(), []Action{r.action("add", "disk")},
		[]string{"add", "disk"}, &bytes.Buffer{}))
	assert.Equal(t, -1, r.in.Int("lun"), "the lun default says nobody chose one")
	assert.False(t, r.in.Changed("lun"))

	require.NoError(t, RunActions(context.Background(), []Action{r.action("add", "disk")},
		[]string{"add", "disk", "--lun", "0"}, &bytes.Buffer{}))
	assert.Equal(t, 0, r.in.Int("lun"))
	assert.True(t, r.in.Changed("lun"), "a lun of zero was chosen")
}

// TestAskingForHelpStillNamesTheArray pins that a line asking for help is
// still read for the array it asks about: the option parser used to pick the
// driver stops at --help, and stopping there would answer with the help of the
// command that only picks the array.
func TestAskingForHelpStillNamesTheArray(t *testing.T) {
	for _, args := range [][]string{
		{"-a", "arr1", "add", "disk", "--help"},
		{"--array", "arr1", "--help"},
		{"add", "disk", "-h", "--array=arr1"},
	} {
		name, err := NameFromArgs(args)
		require.NoErrorf(t, err, "args %v", args)
		assert.Equalf(t, "arr1", name, "args %v", args)
	}

	// With no array named there is nothing to ask the help of.
	name, err := NameFromArgs([]string{"add", "disk", "--help"})
	require.NoError(t, err)
	assert.Equal(t, "", name)
}
