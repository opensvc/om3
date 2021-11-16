package resfshost

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resfsdir"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/filesystems"
	"opensvc.com/opensvc/util/findmnt"
)

const (
	driverGroup = drivergroup.FS
	driverName  = "host"
)

type (
	T struct {
		resource.T
		MountPoint      string         `json:"mnt"`
		Device          string         `json:"dev"`
		Type            string         `json:"type"`
		MountOptions    string         `json:"mnt_opt"`
		StatTimeout     *time.Duration `json:"stat_timeout"`
		Size            *int64         `json:"size"`
		SnapSize        *int64         `json:"snap_size"`
		Zone            string         `json:"zone"`
		VG              string         `json:"vg"`
		PRKey           string         `json:"prkey"`
		MKFSOptions     []string       `json:"mkfs_opt"`
		CreateOptions   []string       `json:"create_options"`
		User            *user.User     `json:"user"`
		Group           *user.Group    `json:"group"`
		Perm            *os.FileMode   `json:"perm"`
		SCSIReservation bool           `json:"scsireserv"`
		NoPreemptAbort  bool           `json:"no_preempt_abort"`
		PromoteRW       bool           `json:"promote_rw"`
	}

	IsFormateder interface {
		IsFormated(string) (bool, error)
	}
	MKFSer interface {
		MKFS(string, []string) error
	}
)

var (
	KeywordPRKey = keywords.Keyword{
		Option:   "prkey",
		Attr:     "PRKey",
		Scopable: true,
		Text:     "Defines a specific persistent reservation key for the resource. Takes priority over the service-level defined prkey and the node.conf specified prkey.",
	}
	KeywordCreateOptions = keywords.Keyword{
		Option:       "create_options",
		Attr:         "CreateOptions",
		Converter:    converters.Shlex,
		Scopable:     true,
		Provisioning: true,
		Text:         "Additional options to pass to the logical volume create command. Size and name are alread set.",
		Example:      "--contiguous y",
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
	KeywordVG = keywords.Keyword{
		Option:       "vg",
		Attr:         "VG",
		Required:     false,
		Scopable:     true,
		Text:         "The name of the disk group the filesystem device should be provisioned from.",
		Provisioning: true,
	}
	KeywordSize = keywords.Keyword{
		Option:       "size",
		Attr:         "Size",
		Required:     false,
		Converter:    converters.Size,
		Scopable:     true,
		Text:         "The size of the logical volume to provision for this filesystem. On linux, can also be expressed as <n>%{FREE|PVS|VG}.",
		Provisioning: true,
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
	KeywordSnapSize = keywords.Keyword{
		Option:    "snap_size",
		Attr:      "SnapSize",
		Converter: converters.Size,
		Scopable:  true,
		Text:      "If this filesystem is build on a snapable logical volume or is natively snapable (jfs, vxfs, ...) this setting overrides the default 10% of the filesystem size to compute the snapshot size. The snapshot is created by snap-enabled rsync-type sync resources. The unit is Megabytes.",
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
		KeywordSize,
		KeywordDevice,
		KeywordStatTimeout,
		KeywordZone,
	}

	KeywordsBase = []keywords.Keyword{
		KeywordMountPoint,
		KeywordDevice,
		KeywordMountOptions,
		KeywordSize,
		KeywordStatTimeout,
		KeywordSnapSize,
		KeywordPRKey,
		KeywordSCSIReservation,
		KeywordNoPreemptAbort,
		KeywordPromoteRW,
		KeywordMKFSOptions,
		KeywordCreateOptions,
		KeywordVG,
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
		KeywordSnapSize,
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
		resource.Register(driverGroup, t, NewF(t))
	}
}

func NewF(s string) func() resource.Driver {
	n := func() resource.Driver {
		t := &T{Type: s}
		return t
	}
	return n
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, t.Type, t)
	m.AddKeyword(KeywordsBase...)
	return m
}

func (t T) Start(ctx context.Context) error {
	if err := t.mount(ctx); err != nil {
		return err
	}
	if err := t.fsDir().Start(ctx); err != nil {
		return err
	}
	return nil
}

func (t T) Stop(ctx context.Context) error {
	if v, err := t.isMounted(); err != nil {
		return err
	} else if !v {
		t.Log().Info().Msg("already umounted")
		return nil
	}
	if err := t.fs().Umount(t.mountPoint()); err != nil {
		return err
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	if t.Device == "" {
		t.StatusLog().Info("dev is not defined")
		return status.NotApplicable
	}
	if t.MountPoint == "" {
		t.StatusLog().Info("mnt is not defined")
		return status.NotApplicable
	}
	if v, err := t.isMounted(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if !v {
		return status.Down
	}
	return status.Up
}

func (t T) Label() string {
	s := t.devpath()
	m := t.mountPoint()
	if m != "" {
		s += "@" + m
	}
	return s
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) Info() map[string]string {
	m := make(map[string]string)
	m["dev"] = t.devpath()
	m["mnt"] = t.mountPoint()
	m["mnt_opt"] = t.MountOptions
	return m
}

func (t T) fsDir() *resfsdir.T {
	r := resfsdir.New().(*resfsdir.T)
	r.SetRID(t.RID())
	r.SetObject(t.GetObject())
	r.Path = t.MountPoint
	r.User = t.User
	r.Group = t.Group
	r.Perm = t.Perm
	return r
}

func (t T) testFile() string {
	return filepath.Join(t.mountPoint(), ".opensvc")
}

func (t T) mountOptions() string {
	// in can we need to mangle options
	return t.MountOptions
}

func (t T) mountPoint() string {
	// add zonepath translation, and cache ?
	return filepath.Clean(t.MountPoint)
}

func (t T) device() *device.T {
	return device.New(t.devpath(), device.WithLogger(t.Log()))
}

func (t T) devpath() string {
	// lazy ref
	switch {
	case strings.HasPrefix(t.Device, "/"):
		return t.Device
	default:
		return t.deviceFromVolume(t.Device)
	}
}

func (t T) deviceFromVolume(p string) string {
	l := filepath.SplitList(p)
	if len(l) < 2 {
		return p
	}
	/*
		vol = resvol.New()
		vol.Name = l[0]
		l[0] = vol.mountPoint()
	*/
	return filepath.Join(l...)
}

func (t *T) mount(ctx context.Context) error {
	if err := t.validateDevice(); err != nil {
		return err
	}
	if err := t.promoteDevicesReadWrite(ctx); err != nil {
		return err
	}
	if v, err := t.isMounted(); err != nil {
		return err
	} else if v {
		t.Log().Info().Msg("already mounted")
		return nil
	}
	if err := t.createMountPoint(ctx); err != nil {
		return err
	}
	if err := t.fsck(); err != nil {
		return err
	}
	if err := t.fs().Mount(t.devpath(), t.mountPoint(), t.mountOptions()); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.fs().Umount(t.mountPoint())
	})
	return nil
}

func (t *T) createMountPoint(ctx context.Context) error {
	if file.ExistsAndDir(t.MountPoint) {
		return nil
	}
	if file.Exists(t.MountPoint) {
		return fmt.Errorf("mountpoint %s already exists but is not a directory", t.MountPoint)
	}
	t.Log().Info().Msgf("create missing mountpoint %s", t.MountPoint)
	if err := os.MkdirAll(t.MountPoint, 0755); err != nil {
		return fmt.Errorf("error creating mountpoint %s: %s", t.MountPoint, err)
	}
	return nil
}

func (t *T) validateDevice() error {
	fs := t.fs()
	if fs.IsZero() {
		return nil
	}
	if fs.IsMultiDevice() {
		return nil
	}
	if fs.IsVirtual() {
		return nil
	}
	if t.Device == "" {
		return fmt.Errorf("device keyword not set or evaluates to None")
	}
	if t.isByLabel() {
		return nil
	}
	if t.isByUUID() {
		return nil
	}
	if !file.Exists(t.Device) {
		return fmt.Errorf("device does not exist: %s", t.Device)
	}
	return nil
}

func (t T) isByUUID() bool {
	return strings.HasPrefix(t.Device, "UUID=")
}

func (t T) isByLabel() bool {
	return strings.HasPrefix(t.Device, "LABEL=")
}

func (t *T) SubDevices() ([]*device.T, error) {
	l := make([]*device.T, 0)
	fs := t.fs()
	if !fs.IsMultiDevice() {
		l = append(l, t.device())
		return l, nil
	}
	return l, fmt.Errorf("TODO: multi dev SubDevices()")
}

func (t *T) promoteDevicesReadWrite(ctx context.Context) error {
	if !t.PromoteRW {
		return nil
	}
	devices, err := t.SubDevices()
	if err != nil {
		return err
	}
	for _, dev := range devices {
		currentRO, err := dev.IsReadOnly()
		if err != nil {
			return err
		}
		if !currentRO {
			t.Log().Debug().Stringer("dev", dev).Msgf("already read-write")
			continue
		}
		t.Log().Info().Stringer("dev", dev).Msgf("promote read-write")
		if err := dev.SetReadWrite(); err != nil {
			return err
		}
		actionrollback.Register(ctx, func() error {
			return dev.SetReadOnly()
		})
	}
	return nil
}

func (t T) fs() filesystems.I {
	fs := filesystems.FromType(t.Type)
	fs.SetLog(t.Log())
	return fs
}

func (t *T) fsck() error {
	fs := t.fs()
	if !filesystems.HasFSCK(fs) {
		t.Log().Debug().Msgf("skip fsck, not implemented for type %s", fs)
		return nil
	}
	if err := filesystems.CanFSCK(fs); err != nil {
		t.Log().Warn().Msgf("skip fsck: %s", err)
		return nil
	}
	return filesystems.DevicesFSCK(fs, t)
}

func (t *T) isMounted() (bool, error) {
	v, err := findmnt.Has(t.devpath(), t.mountPoint())
	return v, err
}

func (t *T) ProvisionLeader(ctx context.Context) error {
	fs := t.fs()
	i1, ok := fs.(IsFormateder)
	if !ok {
		t.Log().Info().Msgf("skip mkfs, formatted detection is not implemented for type %s", fs)
		return nil
	}
	if v, err := i1.IsFormated(t.Device); err != nil {
		t.Log().Warn().Msgf("skip mkfs: %s", err)
	} else if v {
		t.Log().Info().Msgf("%s is already formated", fs)
		return nil
	}
	i2, ok := fs.(filesystems.MKFSer)
	if ok {
		return i2.MKFS(t.Device, t.MKFSOptions)
	}
	t.Log().Info().Msgf("skip mkfs, not implemented for type %s", fs)
	return nil
}

func (t T) Head() string {
	return t.MountPoint
}
