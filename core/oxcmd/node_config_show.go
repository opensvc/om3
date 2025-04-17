package oxcmd

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdNodeConfigShow struct {
		NodeSelector string
		Sections     []string
	}
)

func (t *CmdNodeConfigShow) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}

	var nodenames []string
	if t.NodeSelector == "" {
		if !clientcontext.IsSet() {
			nodenames = []string{hostname.Hostname()}
		} else {
			return fmt.Errorf("--node must be specified")
		}
	} else {
		l, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
		if err != nil {
			return err
		}
		nodenames = l
	}
	switch len(nodenames) {
	case 0:
		return fmt.Errorf("no match")
	case 1:
	default:
		return fmt.Errorf("match more than one node: %s", nodenames)
	}

	b, err := fetchNodeConfig(nodenames[0], c)
	if err != nil {
		return err
	}
	b = commoncmd.Sections(b, t.Sections)
	b = commoncmd.ColorizeINI(b)
	_, err = os.Stdout.Write(b)
	return err
}
