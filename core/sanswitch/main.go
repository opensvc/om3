// Package sanswitch declares the drivers of the SAN switches a node
// inventories for the collector, as it does the storage arrays.
//
// A switch is a "switch#<name>" section of the node or cluster
// configuration, whose type chooses the driver.
package sanswitch

import (
	"context"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// Driver reads the configuration of a switch.
	Driver interface {
		// Name is the name of the section, "switch#<name>".
		Name() string
		SetName(string)
		SetConfig(*xconfig.T)
		Config() *xconfig.T

		// Report returns the raw output of each command the configuration
		// of the switch is read from, by command name, for the collector
		// to parse.
		Report(ctx context.Context) (map[string]string, error)

		// ReportName is the name the collector knows the switch by.
		ReportName() string
	}

	// Switch holds what every driver needs from its section.
	Switch struct {
		name   string
		config *xconfig.T
	}
)

// GetDriver returns a new driver of the switch type t, or nil when no driver
// serves it.
func GetDriver(t string) Driver {
	i, ok := driver.Get(driver.NewID(driver.GroupSwitch, t))
	if !ok {
		return nil
	}
	if a, ok := i.Allocator.(func() Driver); ok {
		return a()
	}
	return nil
}

func (t *Switch) Name() string {
	return t.name
}

// ShortName is the name of the section without its "switch#" prefix.
func (t *Switch) ShortName() string {
	return strings.TrimPrefix(t.name, "switch#")
}

func (t *Switch) SetName(name string) {
	t.name = name
}

func (t *Switch) Config() *xconfig.T {
	return t.config
}

func (t *Switch) SetConfig(c *xconfig.T) {
	t.config = c
}

// Key returns the key of the option in the section of the switch.
func (t *Switch) Key(option string) key.T {
	return key.New(t.name, option)
}
