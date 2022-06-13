//go:build linux

package resdiskmd

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupDisk, "md")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddContext([]manifest.Context{
		{
			Key:  "path",
			Attr: "Path",
			Ref:  "object.path",
		},
		{
			Key:  "nodes",
			Attr: "Nodes",
			Ref:  "object.nodes",
		},
	}...)
	m.AddKeyword(manifest.ProvisioningKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "uuid",
			Attr:     "UUID",
			Scopable: true,
			Text:     "The md uuid to use with mdadm assemble commands",
			Example:  "dev1",
		},
		{
			Option:       "devs",
			Attr:         "Devs",
			Scopable:     true,
			Converter:    converters.List,
			Provisioning: true,
			Text:         "The md member devices to use with mdadm create command",
			Example:      "/dev/mapper/23 /dev/mapper/24",
		},
		{
			Option:       "level",
			Attr:         "Level",
			Scopable:     true,
			Provisioning: true,
			Text:         "The md raid level to use with mdadm create command (see mdadm man for values)",
			Example:      "raid1",
		},
		{
			Option:       "chunk",
			Attr:         "Chunk",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         "The md chunk size to use with mdadm create command. The value is adjusted to the first greater or equal multiple of 4.",
			Example:      "128k",
		},
		{
			Option:       "spares",
			Attr:         "Spares",
			Scopable:     true,
			Converter:    converters.Int,
			Provisioning: true,
			Text:         "The md number of spare devices to use with mdadm create command",
			Default:      "0",
			Example:      "1",
		},
	}...)
	return m
}
