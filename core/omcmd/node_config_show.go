package omcmd

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/rawconfig"
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
	nodenames, err := nodeselector.New(t.NodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	switch len(nodenames) {
	case 0:
		nodenames = []string{hostname.Hostname()}
	case 1:
	default:
		return fmt.Errorf("match more than one node: %s", nodenames)
	}

	var b []byte

	if nodenames[0] == hostname.Hostname() {
		b, err = os.ReadFile(rawconfig.NodeConfigFile())
	} else {
		b, err = fetchNodeConfig(nodenames[0], c)
	}
	if err != nil {
		return err
	}
	b = commoncmd.Sections(b, t.Sections)
	b = commoncmd.ColorizeINI(b)
	_, err = os.Stdout.Write(b)
	return err
}
