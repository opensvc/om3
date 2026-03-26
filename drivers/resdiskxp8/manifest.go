package resdiskxp8

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
)

//go:embed text
var fs embed.FS

var drvID = driver.NewID(driver.GroupDisk, "xp8")

// Keywords exposes the driver keyword definitions to the om3 manifest system.
var Keywords = []*keywords.Keyword{
	{
		Attr:      "Instance",
		Converter: "int",
		Example:   "0",
		Option:    "instance",
		Required:  true,
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/instance"),
	},
	{
		Attr:     "Group",
		Example:  "orasvc1",
		Option:   "group",
		Required: true,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/group"),
	},
	{
		Attr:      "SplitStart",
		Option:    "split_start",
		Converter: "bool",
		Default:   "false",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/split_start"),
	},
	{
		Attr:      "Timeout",
		Converter: "duration",
		Default:   "10s",
		Option:    "timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/timeout"),
	},
	{
		Attr:      "StartTimeout",
		Converter: "duration",
		Default:   "5m",
		Option:    "start_timeout",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/start_timeout"),
	},
}

// Manifest exposes to the om3 core the driver identity and keyword schema.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
	)
	m.AddKeywords(Keywords...)
	return m
}

func (t *T) DriverID() driver.ID {
	return drvID
}

func (t *T) BeforeGroup() driver.Group {
	return driver.GroupDisk
}

func init() {
	driver.Register(drvID, New)
}
