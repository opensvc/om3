package ox

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/commoncmd"
)

// cmdArray holds the array commands ox can serve.
//
// ox drives a cluster through the api of its daemon, from a host that is not
// necessarily one of its nodes. Acting on an array is neither: the driver runs
// where it is invoked, reaching the array over its own management network, and
// it reads the credentials from the configuration of a node. So there is no
// "ox array <name> <command>": the words after the array would name actions of
// a driver ox cannot run, of an array declared in a configuration ox must not
// read. "om array <name> <command>" is where those live.
//
// What is left is what the daemon can answer, which is the listing.
var (
	cmdArray = &cobra.Command{
		GroupID: commoncmd.GroupIDSubsystems,
		Use:     "array",
		Short:   "manage storage arrays",
		Long:    `An array is a backend storage provider for pools.`,
	}
)

func init() {
	root.AddCommand(
		cmdArray,
	)
	cmdArray.AddGroup(
		commoncmd.NewGroupQuery(),
	)
	cmdArray.AddCommand(
		newCmdArrayList(),
	)
}
