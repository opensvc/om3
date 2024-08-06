package rescontainerdocker

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

	drvID    = driver.NewID(driver.GroupContainer, "docker")
	altDrvID = driver.NewID(driver.GroupContainer, "oci")
)

func init() {
	driver.Register(drvID, New)
	driver.Register(altDrvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextObjectID,
		manifest.ContextDNS,
		keywords.Keyword{
			Option:      "name",
			Attr:        "Name",
			Scopable:    true,
			DefaultText: keywords.NewText(fs, "text/kw/name.default"),
			Text:        keywords.NewText(fs, "text/kw/name"),
			Example:     "osvcprd..rundeck.container.db",
		},
		keywords.Keyword{
			Option:   "hostname",
			Attr:     "Hostname",
			Scopable: true,
			Example:  "nginx1",
			Text:     keywords.NewText(fs, "text/kw/hostname"),
		},
		keywords.Keyword{
			Option:    "dns_search",
			Attr:      "DNSSearch",
			Converter: converters.List,
			Aliases:   []string{},
			Scopable:  true,
			Required:  false,
			Example:   "opensvc.com",
			Text:      keywords.NewText(fs, "text/kw/dns_search"),
		},
		keywords.Keyword{
			Option:   "image",
			Attr:     "Image",
			Aliases:  []string{"run_image"},
			Scopable: true,
			Required: true,
			Example:  "google/pause",
			Text:     keywords.NewText(fs, "text/kw/image"),
		},
		keywords.Keyword{
			Option:     "image_pull_policy",
			Attr:       "ImagePullPolicy",
			Scopable:   true,
			Candidates: []string{"once", "always"},
			Example:    "once",
			Text:       keywords.NewText(fs, "text/kw/image_pull_policy"),
		},
		keywords.Keyword{
			Option:   "cwd",
			Attr:     "CWD",
			Scopable: true,
			Example:  "/opt/foo",
			Text:     keywords.NewText(fs, "text/kw/cwd"),
		},
		keywords.Keyword{
			Option:    "command",
			Attr:      "Command",
			Aliases:   []string{"run_command"},
			Scopable:  true,
			Converter: converters.Shlex,
			Example:   "/opt/tomcat/bin/catalina.sh",
			Text:      keywords.NewText(fs, "text/kw/command"),
		},
		keywords.Keyword{
			Option:    "run_args",
			Attr:      "RunArgs",
			Scopable:  true,
			Converter: converters.Shlex,
			Example:   "-v /opt/docker.opensvc.com/vol1:/vol1:rw -p 37.59.71.25:8080:8080",
			Text:      keywords.NewText(fs, "text/kw/run_args"),
		},
		keywords.Keyword{
			Option:    "entrypoint",
			Attr:      "Entrypoint",
			Scopable:  true,
			Converter: converters.Shlex,
			Example:   "/bin/sh",
			Text:      keywords.NewText(fs, "text/kw/entrypoint"),
		},
		keywords.Keyword{
			Option:    "detach",
			Attr:      "Detach",
			Scopable:  true,
			Converter: converters.Bool,
			Default:   "true",
			Text:      keywords.NewText(fs, "text/kw/detach"),
		},
		keywords.Keyword{
			Option:    "rm",
			Attr:      "Remove",
			Scopable:  true,
			Converter: converters.Bool,
			Example:   "false",
			Text:      keywords.NewText(fs, "text/kw/rm"),
		},
		keywords.Keyword{
			Option:    "privileged",
			Attr:      "Privileged",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/privileged"),
		},
		keywords.Keyword{
			Option:    "init",
			Attr:      "Init",
			Scopable:  true,
			Default:   "true",
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/init"),
		},
		keywords.Keyword{
			Option:    "interactive",
			Attr:      "Interactive",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/interactive"),
		},
		keywords.Keyword{
			Option:    "tty",
			Attr:      "TTY",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/tty"),
		},
		keywords.Keyword{
			Option:    "volume_mounts",
			Attr:      "VolumeMounts",
			Scopable:  true,
			Converter: converters.Shlex,
			Example:   "myvol1:/vol1 myvol2:/vol2:rw /localdir:/data:ro",
			Text:      keywords.NewText(fs, "text/kw/volume_mounts"),
		},
		keywords.Keyword{
			Option:    "environment",
			Attr:      "Env",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/environment"),
			Example:   "KEY=cert1/server.key PASSWORD=db/password",
		},
		keywords.Keyword{
			Option:    "configs_environment",
			Attr:      "ConfigsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/configs_environment"),
			Example:   "CRT=cert1/server.crt PEM=cert1/server.pem",
		},
		keywords.Keyword{
			Option:    "devices",
			Attr:      "Devices",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/devices"),
			Example:   "myvol1:/dev/xvda myvol2:/dev/xvdb",
		},
		keywords.Keyword{
			Option:   "netns",
			Attr:     "NetNS",
			Aliases:  []string{"net"},
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/netns"),
		},
		keywords.Keyword{
			Option:   "user",
			Attr:     "User",
			Scopable: true,
			Example:  "guest",
			Text:     keywords.NewText(fs, "text/kw/user"),
		},
		keywords.Keyword{
			Option:   "userns",
			Attr:     "UserNS",
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/userns"),
		},
		keywords.Keyword{
			Option:   "pidns",
			Attr:     "PIDNS",
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/pidns"),
		},
		keywords.Keyword{
			Option:   "ipcns",
			Attr:     "IPCNS",
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/ipcns"),
		},
		keywords.Keyword{
			Option:     "utsns",
			Attr:       "UTSNS",
			Scopable:   true,
			Candidates: []string{"", "host"},
			Example:    "container#0",
			Text:       keywords.NewText(fs, "text/kw/utsns"),
		},
		keywords.Keyword{
			Option:   "registry_creds",
			Attr:     "RegistryCreds",
			Scopable: true,
			Example:  "creds-registry-opensvc-com",
			Text:     keywords.NewText(fs, "text/kw/registry_creds"),
		},
		keywords.Keyword{
			Option:    "pull_timeout",
			Attr:      "PullTimeout",
			Scopable:  true,
			Converter: converters.Duration,
			Text:      keywords.NewText(fs, "text/kw/pull_timeout"),
			Example:   "2m",
			Default:   "2m",
		},
		keywords.Keyword{
			Option:    "start_timeout",
			Attr:      "StartTimeout",
			Scopable:  true,
			Converter: converters.Duration,
			Text:      keywords.NewText(fs, "text/kw/start_timeout"),
			Example:   "1m5s",
			Default:   "5s",
		},
		keywords.Keyword{
			Option:    "stop_timeout",
			Attr:      "StopTimeout",
			Scopable:  true,
			Converter: converters.Duration,
			Text:      keywords.NewText(fs, "text/kw/stop_timeout"),
			Example:   "2m",
			Default:   "2m30s",
		},
		keywords.Keyword{
			Option:    "secrets_environment",
			Attr:      "SecretsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/secrets_environment"),
			Example:   "CRT=cert1/server.pem sec1/*",
		},
		keywords.Keyword{
			Option:    "configs_environment",
			Attr:      "ConfigsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/configs_environment"),
			Example:   "PORT=http/port webapp/app1* {name}/* {name}-debug/settings",
		},
		rescontainer.KWOsvcRootPath,
		rescontainer.KWGuestOS,
	)
	return m
}
