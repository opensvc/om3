package commoncmd

import (
	"github.com/spf13/cobra"
)

// NewCmdAll creates the "all" command
func NewCmdAll() *cobra.Command {
	return &cobra.Command{
		Use:   "all",
		Short: "manage a mix of objects, tentatively exposing all commands",
	}
}

// NewCmdCcfg creates the "ccfg" command
func NewCmdCcfg() *cobra.Command {
	return &cobra.Command{
		Use:   "ccfg",
		Short: "manage the cluster shared configuration",
		Long: `The cluster nodes merge their private configuration
over the cluster shared configuration.

The shared configuration is hosted in a ccfg-kind object, and is
replicated using the same rules as other kinds of object (last write is
eventually replicated).`,
	}
}

// NewCmdCfg creates the "cfg" command
func NewCmdCfg() *cobra.Command {
	return &cobra.Command{
		Use:   "cfg",
		Short: "manage configmaps",
		Long: `A configmap is an unencrypted key-value store.

Values can be binary or text.

A key can be installed as a file in a Vol, then exposed to apps
and containers.

A key can be exposed as a environment variable for apps and
containers.

A signal can be sent to consumer processes upon exposed key value
changes.

The key names can include the '/' character, interpreted as a path separator
when installing the key in a volume.`,
	}
}

// NewCmdSec creates the "sec" command
func NewCmdSec() *cobra.Command {
	return &cobra.Command{
		Use:   "sec",
		Short: "manage secrets",
		Long: `A secret is an encrypted key-value store.

Values can be binary or text.

A key can be installed as a file in a Vol, then exposed to apps
and containers.

A key can be exposed as a environment variable for apps and
containers.

A signal can be sent to consumer processes upon exposed key value
changes.

The key names can include the '/' character, interpreted as a path separator
when installing the key in a volume.`,
	}
}

// NewCmdSVC creates the "svc" command
func NewCmdSVC() *cobra.Command {
	return &cobra.Command{
		Use:   "svc",
		Short: "manage services",
		Long: `Service objects subsystem.

A service is typically made of ip, app, container and task resources.

They can use support objects like volumes, secrets and configmaps to
isolate lifecycles or to abstract cluster-specific knowledge.`,
	}
}

// NewCmdUsr creates the "usr" command
func NewCmdUsr() *cobra.Command {
	return &cobra.Command{
		Use:   "usr",
		Short: "manage users",
		Long: `A user stores the grants and credentials of user of the agent API.

User objects are not necessary with OpenID authentication, as the
grants are embedded in the trusted bearer tokens.`,
	}
}

// NewCmdVol creates the "vol" command
func NewCmdVol() *cobra.Command {
	return &cobra.Command{
		Use:   "vol",
		Short: "manage volumes",
		Long: `A volume is a persistent data provider.

A volume is made of disk, fs and sync resources. It is created by a pool,
to satisfy a demand from a volume resource in a service.

Volumes and their subdirectories can be mounted inside containers.`,
	}
}

// NewCmdNscfg creates the "nscfg" command
func NewCmdNscfg() *cobra.Command {
	return &cobra.Command{
		Use:   "nscfg",
		Short: "manage namespace configurations",
	}
}
