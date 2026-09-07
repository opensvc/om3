package resdiskhp3par

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/converters"
)

//go:embed text
var fs embed.FS

var drvID = driver.NewID(driver.GroupDisk, "hp3par")

var kws = []*keywords.Keyword{
	{
		Attr:     "Array",
		Example:  "myarray",
		Option:   "array",
		Required: true,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/array"),
	},
	{
		Aliases:  []string{"rcg"},
		Attr:     "Group",
		Example:  "u",
		Option:   "group",
		Required: true,
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/rcg"),
	},
	{
		Attr:     "Mode",
		Example:  "sync",
		Option:   "mode",
		Default:  "sync",
		Scopable: true,
		Text:     keywords.NewText(fs, "text/kw/mode"),
	},
	{
		Attr:      "AutoTakeover",
		Converter: converters.Bool,
		Option:    "auto_takeover",
		Default:   "false",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/auto_takeover"),
	},
	{
		Attr:      "ForceSync",
		Converter: converters.Bool,
		Option:    "force_sync",
		Default:   "false",
		Scopable:  false,
		Text:      keywords.NewText(fs, "text/kw/force_sync"),
	},
	{
		Attr:      "SwapRoles",
		Converter: converters.Bool,
		Option:    "swap_roles",
		Default:   "false",
		Scopable:  true,
		Text:      keywords.NewText(fs, "text/kw/swap_roles"),
	},
	{
		Attr:         "Timeout",
		Converter:    converters.Duration,
		Example:      "10s",
		Option:       "timeout",
		Default:      "10s",
		Provisioning: true,
		Scopable:     true,
		Text:         keywords.NewText(fs, "text/kw/timeout"),
	},
	{
		Attr:         "StartTimeout",
		Converter:    converters.Duration,
		Example:      "5m",
		Option:       "start_timeout",
		Default:      "5m",
		Provisioning: true,
		Scopable:     true,
		Text:         keywords.NewText(fs, "text/kw/start_timeout"),
	},
	{
		Aliases:       []string{"sync_max_delay"},
		Attr:          "MaxDelay",
		Converter:     converters.Duration,
		DefaultOption: "sync_max_delay",
		DefaultText:   "Two times the rcg sync period.",
		Option:        "max_delay",
		Text:          keywords.NewText(fs, "text/kw/max_delay"),
	},
}

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(manifest.ContextObjectPath)
	m.AddKeywords(resdisk.BaseKeywords...)
	m.AddKeywords(kws...)
	return m
}
