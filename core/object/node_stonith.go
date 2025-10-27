package object

import (
	"fmt"
	"slices"

	_ "github.com/opensvc/om3/drivers/chkfsidf"
	_ "github.com/opensvc/om3/drivers/chkfsudf"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/rs/zerolog"
)

func (t Node) Stonith(nodename string) error {
	if nodename == "" {
		return fmt.Errorf("node name is not set")
	}
	if nodename == hostname.Hostname() {
		return fmt.Errorf("fencing the local node is not allowed")
	}
	nodenames, err := t.Nodes()
	if err != nil {
		return err
	}
	if !slices.Contains(nodenames, nodename) {
		return fmt.Errorf("node %s is not a peer", nodename)
	}
	argv := t.mergedConfig.GetStrings(key.T{"stonith#" + nodename, "command"})
	if len(argv) == 0 {
		return fmt.Errorf("fencing command for node %s is not defined", nodename)
	}
	cmd := command.New(
		command.WithName(argv[0]),
		command.WithArgs(argv[1:]),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
