package filesystems

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
)

type (
	BTRFS struct{ T }

	// MountOptionsProvisioner is implemented by a filesystem whose mount
	// options name a part of it to create when it is provisioned, as the
	// subvolume of a btrfs.
	MountOptionsProvisioner interface {
		ProvisionMountOptions(ctx context.Context, dev, mountOptions string) error
	}

	// DeviceLister is implemented by a filesystem that can span several
	// devices, to tell the devices of the one on dev.
	DeviceLister interface {
		Devices(ctx context.Context, dev string) ([]string, error)
	}
)

func init() {
	registerFS(NewBTRFS())
}

func NewBTRFS() *BTRFS {
	return &BTRFS{T{fsType: "btrfs", isMultiDevice: true}}
}

func (t BTRFS) IsCapable() bool {
	if _, err := exec.LookPath("mkfs.btrfs"); err != nil {
		return false
	}
	return true
}

// IsFormated reports whether the device holds a btrfs, which "btrfs
// filesystem show" lists the devices of.
func (t BTRFS) IsFormated(ctx context.Context, dev string) (bool, error) {
	if _, err := exec.LookPath("btrfs"); err != nil {
		return false, errors.New("btrfs not found")
	}
	cmd := exec.CommandContext(ctx, "btrfs", "filesystem", "show", dev)
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// MKFS formats the device. The label the syncs find the filesystem by on
// every node is given in the mkfs options, as -L <label>.
func (t BTRFS) MKFS(ctx context.Context, dev string, args []string) error {
	if _, err := exec.LookPath("mkfs.btrfs"); err != nil {
		return fmt.Errorf("mkfs.btrfs not found")
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("mkfs.btrfs"),
		command.WithArgs(append(append([]string{"-f", "-q"}, args...), dev)),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

// subvolOf is the subvolume a mount option string names, "" when none.
func subvolOf(mountOptions string) string {
	for _, opt := range strings.Split(mountOptions, ",") {
		if v, ok := strings.CutPrefix(opt, "subvol="); ok {
			return strings.Trim(filepath.Clean("/"+v), "/")
		}
	}
	return ""
}

// ProvisionMountOptions creates the subvolume the mount options name, as
// subvol=data, which the mount of the filesystem needs to exist.
func (t BTRFS) ProvisionMountOptions(ctx context.Context, dev, mountOptions string) error {
	subvol := subvolOf(mountOptions)
	if subvol == "" || subvol == "." {
		return nil
	}
	mnt, err := os.MkdirTemp("", "btrfs-root.")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(mnt) }()
	if err := t.run(ctx, "mount", "-t", "btrfs", "-o", "subvolid=5", dev, mnt); err != nil {
		return err
	}
	defer func() {
		if err := t.run(context.Background(), "umount", mnt); err != nil {
			t.log.Warnf("%s", err)
		}
	}()
	p := filepath.Join(mnt, subvol)
	if _, err := os.Stat(p); err == nil {
		t.log.Infof("subvolume %s already exists", subvol)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return t.run(ctx, "btrfs", "subvolume", "create", p)
}

// Devices returns the devices of the btrfs on dev.
func (t BTRFS) Devices(ctx context.Context, dev string) ([]string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "btrfs", "filesystem", "show", dev)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %w", cmd, err)
	}
	l := make([]string, 0)
	for _, d := range parseShowDevices(stdout.Bytes()) {
		l = append(l, d.path)
	}
	return l, nil
}

type btrfsDevice struct {
	id   string
	path string
}

// parseShowDevices reads the devices "btrfs filesystem show" lists:
//
//	devid    1 size 5.00GiB used 1.51GiB path /dev/vdb
func parseShowDevices(b []byte) []btrfsDevice {
	l := make([]btrfsDevice, 0)
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "devid" {
			continue
		}
		_, path, ok := strings.Cut(line, " path ")
		if !ok {
			continue
		}
		l = append(l, btrfsDevice{id: fields[1], path: strings.TrimSpace(path)})
	}
	return l
}

// Grow takes the device dev of the filesystem mounted at mountPoint up to
// the size of the device: a btrfs grows a device at a time, and only while
// it is mounted.
func (t BTRFS) Grow(ctx context.Context, dev, mountPoint string) error {
	if mountPoint == "" {
		return fmt.Errorf("btrfs grows only while it is mounted")
	}
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "btrfs", "filesystem", "show", mountPoint)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", cmd, err)
	}
	devs := parseShowDevices(stdout.Bytes())
	devid := ""
	real, _ := filepath.EvalSymlinks(dev)
	for _, d := range devs {
		if d.path == dev || (real != "" && d.path == real) {
			devid = d.id
		}
	}
	switch {
	case devid != "":
	case len(devs) == 1:
		devid = devs[0].id
	default:
		return fmt.Errorf("device %s is not a device of the btrfs mounted on %s", dev, mountPoint)
	}
	return t.run(ctx, "btrfs", "filesystem", "resize", devid+":max", mountPoint)
}

func (t BTRFS) run(ctx context.Context, name string, args ...string) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(name),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
