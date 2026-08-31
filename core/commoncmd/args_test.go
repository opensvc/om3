package commoncmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestTree returns a command tree shaped like the om one: a non runnable
// root, non runnable subsystem levels, and a runnable leaf.
func newTestTree() (*cobra.Command, *string) {
	var selector string
	root := &cobra.Command{Use: "om"}
	root.PersistentFlags().StringVar(new(string), "color", "auto", "")
	root.PersistentFlags().StringVarP(&selector, "selector", "s", "", "")
	root.PersistentFlags().Bool("debug", false, "")

	svc := &cobra.Command{Use: "svc"}
	instance := &cobra.Command{Use: "instance"}
	status := &cobra.Command{
		Use: "status",
		RunE: func(*cobra.Command, []string) error {
			return nil
		},
	}
	status.Flags().BoolP("refresh", "r", false, "")

	instance.AddCommand(status)
	svc.AddCommand(instance)
	root.AddCommand(svc)
	return root, &selector
}

func TestValidateArgs(t *testing.T) {
	for name, test := range map[string]struct {
		args    []string
		unknown string
	}{
		"a runnable leaf": {
			args: []string{"svc", "instance", "status"},
		},
		"a runnable leaf and its flags": {
			args: []string{"svc", "instance", "status", "-r"},
		},
		"a bare subsystem asks for the command list": {
			args: []string{"svc", "instance"},
		},
		"no argument at all": {
			args: nil,
		},
		"a valueless flag does not eat the next word": {
			args: []string{"svc", "instance", "--debug"},
		},
		"an unknown command under a subsystem": {
			args:    []string{"svc", "instance", "resource", "info", "push"},
			unknown: "resource",
		},
		"a flag value is not mistaken for a command": {
			args:    []string{"svc", "-s", "test/svc/s1", "instance", "resource", "info", "push"},
			unknown: "resource",
		},
		"a flag value passed with an equal sign": {
			args:    []string{"svc", "--color=no", "instance", "bogus"},
			unknown: "bogus",
		},
		"a valueless flag before an unknown command": {
			args:    []string{"svc", "instance", "--debug", "bogus"},
			unknown: "bogus",
		},
		"an unknown first word is left to cobra": {
			args: []string{"nosuchsubsystem", "instance"},
		},
		"shell completion never runs a command": {
			args: []string{"__complete", "svc", "instance", "bogus"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, _ := newTestTree()
			err := ValidateArgs(root, test.args)
			if test.unknown == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"`+test.unknown+`"`)
		})
	}
}

// ValidateArgs parses the args to tell the flags from the command names, and
// cobra parses them again right after: it must not have written anything.
func TestValidateArgsLeavesTheFlagValuesAlone(t *testing.T) {
	root, selector := newTestTree()
	args := []string{"svc", "-s", "test/svc/s1", "instance", "bogus"}

	require.Error(t, ValidateArgs(root, args))
	assert.Equal(t, "", *selector, "the selector flag value was written")
	assert.False(t, root.PersistentFlags().Lookup("selector").Changed)
}
