package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/objectdevice"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdObjectInstanceDeviceList struct {
		OptsGlobal
		NodeSelector string
		Roles        string
	}

	devicer interface {
		PrintDevices(roles objectdevice.Role) objectdevice.L
	}
)

func (t *CmdObjectInstanceDeviceList) extract(selector string, c *client.T) (objectdevice.L, error) {
	return t.extractFromDaemon(selector, c)
}

func (t *CmdObjectInstanceDeviceList) extractFromDaemon(selector string, c *client.T) (objectdevice.L, error) {
	data := objectdevice.NewList()
	return data, fmt.Errorf("TODO")
}

func (t *CmdObjectInstanceDeviceList) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	data, err := t.extract(mergedSelector, c)
	if err != nil {
		return err
	}

	output.Renderer{
		DefaultOutput: "tab=OBJECT:path,RESOURCE:rid,DRIVER_GROUP:driver.group,DRIVER_NAME:driver.name,ROLE:role,DEVICE:device",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}
