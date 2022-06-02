package resvol

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupVolume, "")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.AddKeyword(manifest.ProvisioningKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "name",
			Attr:     "Name",
			Scopable: true,
			Default:  "{name}-vol-{rindex}",
			Text:     "The volume service name. A service can only reference volumes in the same namespace.",
		},
		{
			Option:       "type",
			Attr:         "PoolType",
			Provisioning: true,
			Scopable:     true,
			Text:         "The type of the pool to allocate from. The selected pool will be the one matching type and capabilities and with the maximum available space.",
		},
		{
			Option:       "access",
			Attr:         "Access",
			Default:      "rwo",
			Candidates:   []string{"rwo", "roo", "rwx", "rox"},
			Provisioning: true,
			Scopable:     true,
			Text:         "The access mode of the volume.\n``rwo`` is Read Write Once,\n``roo`` is Read Only Once,\n``rwx`` is Read Write Many,\n``rox`` is Read Only Many.\n``rox`` and ``rwx`` modes are served by flex volume services.",
		},
		{
			Option:       "size",
			Attr:         "Size",
			Scopable:     true,
			Converter:    converters.Size,
			Provisioning: true,
			Text:         "The size to allocate in the pool.",
		},
		{
			Option:       "pool",
			Attr:         "Pool",
			Scopable:     true,
			Provisioning: true,
			Text:         "The name of the pool to allocate from.",
		},
		{
			Option:       "format",
			Attr:         "Format",
			Scopable:     true,
			Provisioning: true,
			Default:      "true",
			Converter:    converters.Bool,
			Text:         "If true the volume translator will also produce a fs resource layered over the disk allocated in the pool.",
		},
		{
			Option:    "configs",
			Attr:      "Configs",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      "The whitespace separated list of ``<config name>/<key>:<volume relative path>:<options>``.",
			Example:   "conf/mycnf:/etc/mysql/my.cnf:ro conf/sysctl:/etc/sysctl.d/01-db.conf",
		},
		{
			Option:    "secrets",
			Attr:      "Secrets",
			Scopable:  true,
			Types:     []string{"shm"},
			Converter: converters.Shlex,
			Default:   "",
			Text:      "The whitespace separated list of ``<secret name>/<key>:<volume relative path>:<options>``.",
			Example:   "cert/pem:server.pem cert/key:server.key",
		},
		{
			Option:    "directories",
			Attr:      "Directories",
			Scopable:  true,
			Converter: converters.List,
			Default:   "",
			Text:      "The whitespace separated list of directories to create in the volume.",
			Example:   "a/b/c d /e",
		},
		{
			Option:    "user",
			Attr:      "User",
			Scopable:  true,
			Converter: converters.User,
			Text:      "The user name or id that will own the volume root and installed files and directories.",
			Example:   "1001",
		},
		{
			Option:    "group",
			Attr:      "Group",
			Scopable:  true,
			Converter: converters.Group,
			Text:      "The group name or id that will own the volume root and installed files and directories.",
			Example:   "1001",
		},
		{
			Option:    "perm",
			Attr:      "Perm",
			Scopable:  true,
			Converter: converters.FileMode,
			Text:      "The permissions, in octal notation, to apply to the installed files.",
			Example:   "660",
		},
		{
			Option:    "dirperm",
			Attr:      "DirPerm",
			Scopable:  true,
			Converter: converters.FileMode,
			Text:      "The permissions, in octal notation, to apply to the volume root and installed directories.",
			Default:   "700",
			Example:   "750",
		},
		{
			Option:   "signal",
			Attr:     "Signal",
			Scopable: true,
			Text:     "A <signal>:<target> whitespace separated list, where signal is a signal name or number (ex. 1, hup or sighup), and target is the comma separated list of resource ids to send the signal to (ex: container#1,container#2). If only the signal is specified, all candidate resources will be signaled. This keyword is usually used to reload daemons on certicate or configuration files changes.",
			Example:  "hup:container#1",
		},
	}...)
	m.AddContext([]manifest.Context{
		{
			Key:  "nodes",
			Attr: "Nodes",
			Ref:  "object.nodes",
		},
		{
			Key:  "path",
			Attr: "Path",
			Ref:  "object.path",
		},
		{
			Key:  "topology",
			Attr: "Topology",
			Ref:  "object.topology",
		},
	}...)
	return m
}
