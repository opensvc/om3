package oxcmd

import (
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/util/editor"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdNodeConfigEdit struct {
		OptsGlobal
		NodeSelector string
		Discard      bool
		Recover      bool
	}
)

func (t *CmdNodeConfigEdit) DoRemote(nodename string, c *client.T) error {
	var (
		err      error
		refSum   []byte
		filename string
	)
	if filename, err = createTempRemoteNodeConfig(nodename, c); err != nil {
		return err
	}
	defer os.Remove(filename)
	if refSum, err = file.MD5(filename); err != nil {
		return err
	}
	if err = editor.Edit(filename); err != nil {
		return err
	}
	if file.HaveSameMD5(refSum, filename) {
		fmt.Println("unchanged")
		return nil
	}
	if err = putNodeConfig(nodename, filename, c); err != nil {
		return err
	}
	return nil
}

func (t *CmdNodeConfigEdit) Run() error {
	nodename := t.NodeSelector
	if !clientcontext.IsSet() && nodename == "" {
		nodename = hostname.Hostname()
	}
	if nodename == "" {
		return fmt.Errorf("--node must be specified")
	}
	var (
		c   *client.T
		err error
	)
	if c, err = client.New(); err != nil {
		return err
	}
	return t.DoRemote(nodename, c)
}
