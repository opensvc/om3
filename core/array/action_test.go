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

// TestNameFromFirstArgReadsTheWordAndNotTheOption covers the extraction of the
// array named where a reader expects it, before the words saying what to do
// with it.
func TestNameFromFirstArgReadsTheWordAndNotTheOption(t *testing.T) {
	cases := []struct {
		args     []string
		name     string
		expected []string
	}{
		{[]string{"freenas", "add", "disk"}, "freenas", []string{"add", "disk"}},
		{[]string{"array#freenas", "add", "disk"}, "array#freenas", []string{"add", "disk"}},

		// The array alone, which is how the actions of its driver are listed.
		{[]string{"freenas"}, "freenas", []string{}},

		// An option names no array, whether it is the one that used to name it
		// or the help.
		{[]string{"-a", "freenas", "add", "disk"}, "", []string{"-a", "freenas", "add", "disk"}},
		{[]string{"--array=freenas"}, "", []string{"--array=freenas"}},
		{[]string{"--help"}, "", []string{"--help"}},

		// Nothing at all.
		{[]string{}, "", []string{}},
	}
	for _, tc := range cases {
		name, rest := NameFromFirstArg(tc.args)
		require.Equalf(t, tc.name, name, "args %v", tc.args)
		require.Equalf(t, tc.expected, rest, "args %v", tc.args)
	}
}

// A help text of a driver has to print a command the reader can type back.
// The tree is reached as "om array <name>", which is no command of its own, so
// the words that reached it are handed in and cobra has to use them for this
// command and for every command under it.
func TestTheHelpNamesTheWordsThatReachTheDriver(t *testing.T) {
	r := &recorder{}
	root, err := NewCommandAs("om array freenas", []Action{r.action("add", "disk")}, &bytes.Buffer{})
	require.NoError(t, err)

	assert.Equal(t, "om array freenas", root.CommandPath())

	// The words must appear once. Cobra substitutes the display name for the
	// command name inside Use, so holding them in both repeats them: the
	// usage line read "om array freenas array freenas".
	assert.Equal(t, "om array freenas", strings.TrimSuffix(root.UseLine(), " [flags]"))

	add := root.Commands()[0]
	require.Equal(t, "add", add.Name())
	disk := add.Commands()[0]
	require.Equal(t, "disk", disk.Name())
	assert.Equal(t, "om array freenas add disk", disk.CommandPath())
	assert.Equal(t, "om array freenas add disk [flags]", disk.UseLine())
}

// A driver reached by something that is not a command line says the one word
// it is, and nothing about a program or an array.
func TestTheHelpOfABareTreeNamesTheArrayWord(t *testing.T) {
	r := &recorder{}
	root, err := NewCommand([]Action{r.action("add", "disk")}, &bytes.Buffer{})
	require.NoError(t, err)
	assert.Equal(t, "array", root.CommandPath())
	assert.Equal(t, "array add disk", root.Commands()[0].Commands()[0].CommandPath())
}

// The tree is a branch of om and not a program, so it offers neither the
// completion command, which would write the completion of a program named
// after an array, nor the help command, which says what --help says.
//
// Cobra adds them both when the tree runs, not when it is built, so the tree
// has to run for this to mean anything.
func TestTheTreeOffersNeitherHelpNorCompletion(t *testing.T) {
	r := &recorder{}
	root, err := NewCommand([]Action{r.action("add", "disk")}, &bytes.Buffer{})
	require.NoError(t, err)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{})
	require.NoError(t, root.Execute())

	for _, cmd := range root.Commands() {
		if !cmd.IsAvailableCommand() {
			continue
		}
		assert.NotEqual(t, "completion", cmd.Name())
		assert.NotEqual(t, "help", cmd.Name())
	}

	// What it printed is the list an operator reads, and the two words must
	// not be in it either: the usage template lists a command named "help"
	// whether it is hidden or not.
	commands := out.String()
	if i := strings.Index(commands, "Flags:"); i > 0 {
		commands = commands[:i]
	}
	assert.NotContains(t, commands, "completion")
	assert.NotContains(t, commands, "help")
	assert.Contains(t, commands, "add", "the actions of the driver are still listed")
}

// The option that used to name the array is registered on the tree so a
// command line written before the argument existed still parses, and hidden
// so it is not offered: whoever reads this help has already named the array,
// and naming it a second time is refused.
func TestTheOptionThatNamedTheArrayIsNotOffered(t *testing.T) {
	r := &recorder{}
	root, err := NewCommand([]Action{r.action("add", "disk")}, &bytes.Buffer{})
	require.NoError(t, err)

	f := root.PersistentFlags().Lookup(FlagArray.Name)
	require.NotNil(t, f, "the option still has to parse")
	assert.True(t, f.Hidden, "the option must not be offered")

	// Hidden is not disabled: the tree still takes it where it is typed.
	require.NoError(t, RunActions(context.Background(),
		[]Action{r.action("add", "disk")},
		[]string{"--" + FlagArray.Name, "arr1", "add", "disk"}, &bytes.Buffer{}))
	assert.True(t, r.called)
}
