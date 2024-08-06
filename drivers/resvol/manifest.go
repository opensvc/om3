package resvol

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupVolume, "")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextNodes,
		manifest.ContextObjectPath,
		manifest.ContextTopology,
		keywords.Keyword{
			Option:   "name",
			Attr:     "Name",
			Scopable: true,
			Default:  "{name}-vol-{rindex}",
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Option:       "type",
			Attr:         "PoolType",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/type"),
		},
		keywords.Keyword{
			Option:       "access",
			Attr:         "Access",
			Default:      "rwo",
			Candidates:   []string{"rwo", "roo", "rwx", "rox"},
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/access"),
		},
		keywords.Keyword{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		keywords.Keyword{
			Option:       "pool",
			Attr:         "Pool",
			Scopable:     true,
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/pool"),
		},
		keywords.Keyword{
			Option:       "format",
			Attr:         "Format",
			Scopable:     true,
			Provisioning: true,
			Default:      "true",
			Converter:    converters.Bool,
			Text:         keywords.NewText(fs, "text/kw/format"),
		},
		keywords.Keyword{
			Option:    "configs",
			Attr:      "Configs",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/configs"),
			Example:   "conf/mycnf:/etc/mysql/my.cnf:ro conf/sysctl:/etc/sysctl.d/01-db.conf",
		},
		keywords.Keyword{
			Option:    "secrets",
			Attr:      "Secrets",
			Scopable:  true,
			Types:     []string{"shm"},
			Converter: converters.Shlex,
			Default:   "",
			Text:      keywords.NewText(fs, "text/kw/secrets"),
			Example:   "cert/pem:server.pem cert/key:server.key",
		},
		keywords.Keyword{
			Option:    "directories",
			Attr:      "Directories",
			Scopable:  true,
			Converter: converters.List,
			Default:   "",
			Text:      keywords.NewText(fs, "text/kw/directories"),
			Example:   "a/b/c d /e",
		},
		keywords.Keyword{
			Option:   "user",
			Attr:     "User",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/user"),
			Example:  "1001",
		},
		keywords.Keyword{
			Option:   "group",
			Attr:     "Group",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/group"),
			Example:  "1001",
		},
		keywords.Keyword{
			Option:    "perm",
			Attr:      "Perm",
			Scopable:  true,
			Converter: converters.FileMode,
			Text:      keywords.NewText(fs, "text/kw/perm"),
			Example:   "660",
		},
		keywords.Keyword{
			Option:    "dirperm",
			Attr:      "DirPerm",
			Scopable:  true,
			Converter: converters.FileMode,
			Text:      keywords.NewText(fs, "text/kw/dirperm"),
			Default:   "700",
			Example:   "750",
		},
		keywords.Keyword{
			Option:   "signal",
			Attr:     "Signal",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/signal"),
			Example:  "hup:container#1",
		},
	)
	return m
}
