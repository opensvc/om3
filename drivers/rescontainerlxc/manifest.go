package rescontainerlxc

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/rescontainer"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupContainer, "lxc")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextObjectID,
		manifest.ContextNodes,
		manifest.ContextDNS,
		keywords.Keyword{
			Option:   "data_dir",
			Aliases:  []string{"container_data_dir"},
			Attr:     "DataDir",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/data_dir"),
			Example:  "/srv/svc1/data/containers",
		},
		keywords.Keyword{
			Option:       "rootfs",
			Attr:         "RootDir",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/rootfs"),
			Example:      "/srv/svc1/data/containers",
		},
		keywords.Keyword{
			Option:       "cf",
			Attr:         "ConfigFile",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/cf"),
			Example:      "/srv/svc1/config",
		},
		keywords.Keyword{
			Option:       "template",
			Attr:         "Template",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/template"),
			Example:      "ubuntu",
		},
		keywords.Keyword{
			Option:       "template_options",
			Attr:         "TemplateOptions",
			Provisioning: true,
			Converter:    converters.Shlex,
			Text:         keywords.NewText(fs, "text/kw/template_options"),
			Example:      "--release focal",
		},
		keywords.Keyword{
			Option:       "create_secrets_environment",
			Attr:         "CreateSecretsEnvironment",
			Provisioning: true,
			Scopable:     true,
			Converter:    converters.Shlex,
			Text:         keywords.NewText(fs, "text/kw/create_secrets_environment"),
			Example:      "CRT=cert1/server.crt PEM=cert1/server.pem",
		},
		keywords.Keyword{
			Option:       "create_configs_environment",
			Attr:         "CreateConfigsEnvironment",
			Provisioning: true,
			Scopable:     true,
			Converter:    converters.Shlex,
			Text:         keywords.NewText(fs, "text/kw/create_configs_environment"),
			Example:      "CRT=cert1/server.crt PEM=cert1/server.pem",
		},
		keywords.Keyword{
			Option:       "create_environment",
			Attr:         "CreateEnvironment",
			Provisioning: true,
			Scopable:     true,
			Converter:    converters.Shlex,
			Text:         keywords.NewText(fs, "text/kw/create_environment"),
			Example:      "FOO=bar BAR=baz",
		},
		rescontainer.KWRCmd,
		rescontainer.KWName,
		rescontainer.KWHostname,
		rescontainer.KWStartTimeout,
		rescontainer.KWStopTimeout,
		rescontainer.KWPromoteRW,
		rescontainer.KWOsvcRootPath,
		rescontainer.KWGuestOS,
	)
	return m
}
