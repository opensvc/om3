package resvol

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/xconfig"
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
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextNodes,
		manifest.ContextObjectPath,
		manifest.ContextObjectParents,
		manifest.ContextTopology,
		manifest.KWToInstall,
		keywords.Keyword{
			Attr:     "Name",
			Default:  "{name}-vol-{rindex}",
			Option:   "name",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/name"),
		},
		keywords.Keyword{
			Attr:         "PoolType",
			Option:       "type",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/type"),
		},
		keywords.Keyword{
			Attr:         "Access",
			Candidates:   []string{"rwo", "roo", "rwx", "rox"},
			Default:      "rwo",
			Option:       "access",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/access"),
		},
		keywords.Keyword{
			Attr:         "Size",
			Converter:    converters.Size,
			Option:       "size",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/size"),
		},
		keywords.Keyword{
			Attr:         "Pool",
			Option:       "pool",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/pool"),
		},
		keywords.Keyword{
			Attr:         "VolNodes",
			Converter:    xconfig.NodesConverter,
			Default:      "{.nodes}",
			Option:       "nodes",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/nodes"),
		},
		keywords.Keyword{
			Attr:         "Format",
			Converter:    converters.Bool,
			Default:      "true",
			Option:       "format",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/format"),
		},
		keywords.Keyword{
			Attr:      "Configs",
			Converter: converters.Shlex,
			Example:   "conf/mycnf:/etc/mysql/my.cnf:ro conf/sysctl:/etc/sysctl.d/01-db.conf",
			Option:    "configs",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/configs"),
		},
		keywords.Keyword{
			Attr:      "Secrets",
			Converter: converters.Shlex,
			Default:   "",
			Example:   "cert/pem:server.pem cert/key:server.key",
			Option:    "secrets",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/secrets"),
			Types:     []string{"shm"},
		},
		keywords.Keyword{
			Attr:      "Directories",
			Converter: converters.List,
			Default:   "",
			Example:   "a/b/c d /e",
			Option:    "directories",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/directories"),
		},
		keywords.Keyword{
			Attr:     "User",
			Example:  "1001",
			Option:   "user",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/user"),
		},
		keywords.Keyword{
			Attr:     "Group",
			Example:  "1001",
			Option:   "group",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/group"),
		},
		keywords.Keyword{
			Attr:      "Perm",
			Converter: converters.FileMode,
			Example:   "660",
			Option:    "perm",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/perm"),
		},
		keywords.Keyword{
			Attr:      "DirPerm",
			Converter: converters.FileMode,
			// Default value is fmt.Sprintf("%o", defaultDirPerm)
			Default:  "700",
			Example:  "750",
			Option:   "dirperm",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/dirperm"),
		},
		keywords.Keyword{
			Attr:     "Signal",
			Example:  "hup:container#1",
			Option:   "signal",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/signal"),
		},
	)
	return m
}
