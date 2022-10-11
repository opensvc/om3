package resfshost

import (
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/filesystems"
)

var (
	KeywordPRKey = keywords.Keyword{
		Option:   "prkey",
		Attr:     "PRKey",
		Scopable: true,
		Text:     "Defines a specific persistent reservation key for the resource. Takes priority over the service-level defined prkey and the node.conf specified prkey.",
	}
	KeywordSCSIReservation = keywords.Keyword{
		Option:    "scsireserv",
		Attr:      "SCSIReservation",
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will try to acquire a type-5 (write exclusive, registrant only) scsi3 persistent reservation on every path to every disks held by this resource. Existing reservations are preempted to not block service start-up. If the start-up was not legitimate the data are still protected from being written over from both nodes. If set to ``false`` or not set, :kw:`scsireserv` can be activated on a per-resource basis.",
	}
	KeywordNoPreemptAbort = keywords.Keyword{
		Option:    "no_preempt_abort",
		Attr:      "NoPreemptAbort",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will preempt scsi reservation with a preempt command instead of a preempt and and abort. Some scsi target implementations do not support this last mode (esx). If set to ``false`` or not set, :kw:`no_preempt_abort` can be activated on a per-resource basis.",
	}
	KeywordDevice = keywords.Keyword{
		Option:   "dev",
		Attr:     "Device",
		Scopable: true,
		Required: true,
		Text:     "The block device file or filesystem image file hosting the filesystem to mount. Different device can be set up on different nodes using the ``dev@nodename`` syntax",
	}
	KeywordMKFSOptions = keywords.Keyword{
		Option:       "mkfs_opt",
		Attr:         "MKFSOptions",
		Converter:    converters.Shlex,
		Default:      "",
		Provisioning: true,
		Scopable:     true,
		Text:         "Eventual mkfs additional options.",
	}
	KeywordStatTimeout = keywords.Keyword{
		Option:    "stat_timeout",
		Attr:      "StatTimeout",
		Converter: converters.Duration,
		Default:   "5s",
		Scopable:  true,
		Text:      "The maximum wait time for a stat call to respond. When expired, the resource status is degraded is to warn, which might cause a TOC if the resource is monitored.",
	}
	KeywordMountPoint = keywords.Keyword{
		Option:   "mnt",
		Attr:     "MountPoint",
		Scopable: true,
		Required: true,
		Text:     "The mount point where to mount the filesystem.",
	}
	KeywordMountOptions = keywords.Keyword{
		Option:   "mnt_opt",
		Attr:     "MountOptions",
		Scopable: true,
		Text:     "The mount options, as they would be defined in the fstab.",
	}
	KeywordPromoteRW = keywords.Keyword{
		Option:    "promote_rw",
		Attr:      "PromoteRW",
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will try to promote the base devices to read-write on start.",
	}
	KeywordZone = keywords.Keyword{
		Option:   "zone",
		Attr:     "Zone",
		Scopable: true,
		Text:     "The zone name the fs refers to. If set, the fs mount point is reparented into the zonepath rootfs.",
	}
	KeywordUser = keywords.Keyword{
		Option:    "user",
		Attr:      "User",
		Converter: converters.User,
		Scopable:  true,
		Example:   "root",
		Text:      "The user that should be owner of the mnt directory. Either in numeric or symbolic form.",
	}
	KeywordGroup = keywords.Keyword{
		Option:    "group",
		Attr:      "Group",
		Converter: converters.Group,
		Scopable:  true,
		Example:   "sys",
		Text:      "The group that should be owner of the mnt directory. Either in numeric or symbolic form.",
	}
	KeywordPerm = keywords.Keyword{
		Option:    "perm",
		Attr:      "Perm",
		Converter: converters.FileMode,
		Scopable:  true,
		Example:   "1777",
		Text:      "The permissions the mnt directory should have. A string representing the octal permissions.",
	}

	KeywordsVirtual = []keywords.Keyword{
		KeywordMountPoint,
		KeywordMountOptions,
		KeywordDevice,
		KeywordStatTimeout,
		KeywordZone,
	}

	KeywordsBase = []keywords.Keyword{
		KeywordMountPoint,
		KeywordDevice,
		KeywordMountOptions,
		KeywordStatTimeout,
		KeywordPRKey,
		KeywordSCSIReservation,
		KeywordNoPreemptAbort,
		KeywordPromoteRW,
		KeywordMKFSOptions,
		KeywordZone,
		KeywordUser,
		KeywordGroup,
		KeywordPerm,
	}

	KeywordsPooling = []keywords.Keyword{
		KeywordMountPoint,
		KeywordDevice,
		KeywordMountOptions,
		KeywordStatTimeout,
		KeywordPRKey,
		KeywordSCSIReservation,
		KeywordNoPreemptAbort,
		KeywordMKFSOptions,
		KeywordZone,
		KeywordUser,
		KeywordGroup,
		KeywordPerm,
	}
)

func init() {
	for _, t := range filesystems.Types() {
		driver.Register(driver.NewID(driver.GroupFS, t), NewF(t))
	}
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driver.NewID(driver.GroupFS, t.Type), t)
	m.AddContext([]manifest.Context{
		{
			Key:  "path",
			Attr: "Path",
			Ref:  "object.path",
		},
	}...)
	m.AddKeyword(manifest.ProvisioningKeywords...)
	m.AddKeyword(KeywordsBase...)
	m.AddKeyword(resource.SCSIPersistentReservationKeywords...)
	return m
}
