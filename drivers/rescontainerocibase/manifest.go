package rescontainerocibase

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/rescontainer"
	"github.com/opensvc/om3/v3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
		{
			Option:      "name",
			Attr:        "Name",
			Scopable:    true,
			DefaultText: keywords.NewText(fs, "text/kw/name.default"),
			Text:        keywords.NewText(fs, "text/kw/name"),
			Example:     "osvcprd..rundeck.container.db",
		},
		{
			Option:   "hostname",
			Attr:     "Hostname",
			Scopable: true,
			Example:  "nginx1",
			Text:     keywords.NewText(fs, "text/kw/hostname"),
		},
		{
			Option:    "dns",
			Attr:      "DNSExtra",
			Converter: converters.List,
			Scopable:  true,
			Example:   "1.1.1.1 8.8.8.8",
			Text:      keywords.NewText(fs, "text/kw/dns"),
		},
		{
			Option:    "dns_search",
			Attr:      "DNSSearch",
			Converter: converters.List,
			Aliases:   []string{},
			Scopable:  true,
			Required:  false,
			Example:   "opensvc.com",
			Text:      keywords.NewText(fs, "text/kw/dns_search"),
		},
		{
			Option:   "image",
			Attr:     "Image",
			Aliases:  []string{"run_image"},
			Scopable: true,
			Default:  "ghcr.io/opensvc/pause",
			Text:     keywords.NewText(fs, "text/kw/image"),
		},
		{
			Option:     "image_pull_policy",
			Attr:       "ImagePullPolicy",
			Scopable:   true,
			Candidates: []string{imagePullPolicyOnce, imagePullPolicyAlways},
			Example:    imagePullPolicyOnce,
			Text:       keywords.NewText(fs, "text/kw/image_pull_policy"),
		},
		{
			Option:   "cwd",
			Attr:     "CWD",
			Scopable: true,
			Example:  "/opt/foo",
			Text:     keywords.NewText(fs, "text/kw/cwd"),
		},
		{
			Option:    "command",
			Attr:      "Command",
			Aliases:   []string{"run_command"},
			Scopable:  true,
			Converter: converters.Shlex,
			Example:   "/opt/tomcat/bin/catalina.sh",
			Text:      keywords.NewText(fs, "text/kw/command"),
		},
		{
			Option:    "run_args",
			Attr:      "RunArgs",
			Scopable:  true,
			Converter: converters.Shlex,
			Example:   "-v /opt/docker.opensvc.com/vol1:/vol1:rw -p 37.59.71.25:8080:8080",
			Text:      keywords.NewText(fs, "text/kw/run_args"),
		},
		{
			Option:    "entrypoint",
			Attr:      "Entrypoint",
			Scopable:  true,
			Converter: converters.Shlex,
			Example:   "/bin/sh",
			Text:      keywords.NewText(fs, "text/kw/entrypoint"),
		},
		{
			Option:    "detach",
			Attr:      "Detach",
			Scopable:  true,
			Converter: converters.Bool,
			Default:   "true",
			Text:      keywords.NewText(fs, "text/kw/detach"),
		},
		{
			Option:    "rm",
			Attr:      "Remove",
			Scopable:  true,
			Converter: converters.Bool,
			Example:   "false",
			Text:      keywords.NewText(fs, "text/kw/rm"),
		},
		{
			Option:    "read_only",
			Attr:      "ReadOnly",
			Scopable:  true,
			Converter: converters.Tristate,
			Text:      keywords.NewText(fs, "text/kw/read_only"),
		},
		{
			Option:    "privileged",
			Attr:      "Privileged",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/privileged"),
		},
		{
			Option:    "init",
			Attr:      "Init",
			Scopable:  true,
			Default:   "true",
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/init"),
		},
		{
			Option:    "interactive",
			Attr:      "Interactive",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/interactive"),
		},
		{
			Option:    "tty",
			Attr:      "TTY",
			Scopable:  true,
			Converter: converters.Bool,
			Text:      keywords.NewText(fs, "text/kw/tty"),
		},
		{
			Option:    "volume_mounts",
			Attr:      "VolumeMounts",
			Scopable:  true,
			Converter: converters.Shlex,
			Example:   "myvol1:/vol1 myvol2:/vol2:rw /localdir:/data:ro",
			Text:      keywords.NewText(fs, "text/kw/volume_mounts"),
		},
		{
			Option:    "sysctl",
			Attr:      "Sysctl",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/sysctl"),
			Example:   "kernel.shm_rmid_forced=1 net.ipv4.tcp_syncookies=1",
		},
		{
			Option:    "environment",
			Attr:      "Env",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/environment"),
			Example:   "KEY=cert1/server.key PASSWORD=db/password",
		},
		{
			Option:    "configs_environment",
			Attr:      "ConfigsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/configs_environment"),
			Example:   "CRT=cert1/server.crt PEM=cert1/server.pem",
		},
		{
			Option:    "devices",
			Attr:      "Devices",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/devices"),
			Example:   "myvol1:/dev/xvda myvol2:/dev/xvdb",
		},
		{
			Option:   "netns",
			Attr:     "NetNS",
			Aliases:  []string{"net"},
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/netns"),
		},
		{
			Option:   "user",
			Attr:     "User",
			Scopable: true,
			Example:  "guest",
			Text:     keywords.NewText(fs, "text/kw/user"),
		},
		{
			Option:   "pidns",
			Attr:     "PIDNS",
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/pidns"),
		},
		{
			Option:   "ipcns",
			Attr:     "IPCNS",
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/ipcns"),
		},
		{
			Option:     "utsns",
			Attr:       "UTSNS",
			Scopable:   true,
			Candidates: []string{"", "host"},
			Example:    "container#0",
			Text:       keywords.NewText(fs, "text/kw/utsns"),
		},
		{
			Option:   "registry_creds",
			Attr:     "RegistryCreds",
			Scopable: true,
			Example:  "creds-registry-opensvc-com",
			Text:     keywords.NewText(fs, "text/kw/registry_creds"),
		},
		{
			Option:    "pull_timeout",
			Attr:      "PullTimeout",
			Scopable:  true,
			Converter: converters.Duration,
			Text:      keywords.NewText(fs, "text/kw/pull_timeout"),
			Example:   "2m",
			Default:   "2m",
		},
		{
			Option:    "start_timeout",
			Attr:      "StartTimeout",
			Scopable:  true,
			Converter: converters.Duration,
			Text:      keywords.NewText(fs, "text/kw/start_timeout"),
			Example:   "1m5s",
			Default:   "5s",
		},
		{
			Option:    "stop_timeout",
			Attr:      "StopTimeout",
			Scopable:  true,
			Converter: converters.Duration,
			Text:      keywords.NewText(fs, "text/kw/stop_timeout"),
			Example:   "2m",
			Default:   "10s",
		},
		{
			Option:    "secrets_environment",
			Attr:      "SecretsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/secrets_environment"),
			Example:   "CRT=cert1/server.pem sec1/*",
		},
		{
			Option:    "configs_environment",
			Attr:      "ConfigsEnv",
			Scopable:  true,
			Converter: converters.Shlex,
			Text:      keywords.NewText(fs, "text/kw/configs_environment"),
			Example:   "PORT=http/port webapp/app1* {name}/* {name}-debug/settings",
		},
		&rescontainer.KWOsvcRootPath,
		&rescontainer.KWGuestOS,
		{
			Option:    "log_outputs",
			Attr:      "LogOutputs",
			Scopable:  true,
			Converter: converters.Bool,
			Default:   "false",
			Text:      keywords.NewText(fs, "text/kw/log_outputs"),
		},
	}
)

// ManifestWithID exposes to the core the input expected by the driver.
func (t *BT) ManifestWithID(drvID driver.ID) *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextObjectID,
		manifest.ContextObjectDomain,
		manifest.ContextDNS,
	)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	m.AddKeywords(kws...)
	return m
}
