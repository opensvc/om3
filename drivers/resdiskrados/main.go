package resdiskrados

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/udevadm"
)

type (
	T struct {
		resdisk.T
		resource.SSH
		Name       string `json:"name"`
		ObjectFQDN string `json:"object_fqdn"`
		Size       string `json:"size"`
		Access     string `json:"access"`
		Keyring    string `json:"keyring"`

		featureDisabled  []string
		keyringArgsCache []string
	}

	RBDMap struct {
		ID        string `json:"id"`
		Pool      string `json:"pool"`
		Namespace string `json:"namespace"`
		Name      string `json:"name"`
		Snap      string `json:"snap"`
		Device    string `json:"device"`
	}

	RBDInfo struct {
		Name            string         `json:"name"`
		ID              string         `json:"id"`
		Size            int64          `json:"size"`
		Objects         int            `json:"objects"`
		Order           int            `json:"order"`
		ObjectSize      int64          `json:"object_size"`
		SnapshotCount   int            `json:"snapshot_count"`
		BlockNamePrefix string         `json:"block_name_prefix"`
		Format          int            `json:"format"`
		Features        []string       `json:"features"`
		OpFeatures      []any          `json:"op_features"`
		Flags           []any          `json:"flags"`
		CreateTimestamp string         `json:"create_timestamp"`
		AccessTimestamp string         `json:"access_timestamp"`
		ModifyTimestamp string         `json:"modify_timestamp"`
		Parent          *RBDParentInfo `json:"parent,omitempty"`
	}

	RBDParentInfo struct {
		ID            string `json:"id"`
		Image         string `json:"image"`
		Overlap       string `json:"overlap"`
		Pool          string `json:"pool"`
		PoolNamespace string `json:"pool_namespace"`
		Snapshot      string `json:"snapshot"`
		Trash         bool   `json:"trash"`
	}

	RBDLock struct {
		ID      string `json:"id"`
		Locker  string `json:"locker"`
		Address string `json:"address"`
	}
)

const (
	DefaultCommandTimeout = 10 * time.Second
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t RBDMap) ImageSpec() string {
	s := t.Pool
	if t.Namespace != "" {
		s += "/" + t.Namespace
	}
	return s + "/" + t.Name
}

func (t *T) Start(ctx context.Context) error {
	if err := t.mapDevice(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.unmapDevice(ctx)
	})
	return nil
}

func (t *T) mapDevice(ctx context.Context) error {
	if v, err := t.isMapped(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s is already mapped", t.Name)
		return nil
	}
	args, err := t.keyringArgs()
	if err != nil {
		return err
	}
	args = append(args, "map", t.Name)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithTimeout(DefaultCommandTimeout),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) unmapDevice(ctx context.Context) error {
	if v, err := t.isMapped(ctx); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s is already unmapped", t.Name)
		return nil
	}
	if err := t.removeHolders(ctx); err != nil {
		return err
	}
	udevadm.Settle()
	args, err := t.keyringArgs()
	if err != nil {
		return err
	}
	args = append(args, "unmap", t.Name)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithTimeout(DefaultCommandTimeout),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) createDevice(ctx context.Context) error {
	bytes, err := sizeconv.FromSize(t.Size)
	if err != nil {
		return err
	}
	args, err := t.keyringArgs()
	if err != nil {
		return err
	}
	args = append(args, "create", "--size", fmt.Sprintf("%dB", bytes), t.Name)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithTimeout(DefaultCommandTimeout),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	udevadm.Settle()
	return nil
}

func (t *T) removeDevice(ctx context.Context) error {
	args, err := t.keyringArgs()
	if err != nil {
		return err
	}
	args = append(args, "remove", t.Name)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithTimeout(DefaultCommandTimeout),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) lockDevice(ctx context.Context) error {
	if v, err := t.isLocked(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s is already locked", t.Name)
		return nil
	}
	args, err := t.keyringArgs()
	if err != nil {
		return err
	}
	switch t.Access {
	case "rwx", "rox":
		args = append(args, "lock", "--shared", t.sharedLockID())
	default:
		args = append(args, "lock", t.lockID())
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithTimeout(DefaultCommandTimeout),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) unlockDevice(ctx context.Context) error {
	if v, err := t.isLocked(ctx); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s is already unlocked", t.Name)
		return nil
	}
	args, err := t.keyringArgs()
	if err != nil {
		return err
	}
	switch t.Access {
	case "rwx", "rox":
		args = append(args, "unlock", "--shared", t.sharedLockID())
	default:
		args = append(args, "unlock", t.lockID())
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithTimeout(DefaultCommandTimeout),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "name", Value: t.Name},
	}
	return m, nil
}

func (t *T) deviceInfo(ctx context.Context) (*RBDInfo, error) {
	args, err := t.keyringArgs()
	if err != nil {
		return nil, err
	}
	args = append(args, "info", t.Name, "--format", "json")
	cmd := command.New(
		command.WithContext(ctx),
		command.WithTimeout(DefaultCommandTimeout),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
		command.WithIgnoredExitCodes(0, 2),
	)
	b, err := cmd.Output()
	if cmd.ExitCode() == 2 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var data RBDInfo
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (t *T) listLocks(ctx context.Context) ([]RBDLock, error) {
	args, err := t.keyringArgs()
	if err != nil {
		return nil, err
	}
	args = append(args, "lock", "list", t.Name, "--format", "json")
	cmd := command.New(
		command.WithContext(ctx),
		command.WithTimeout(DefaultCommandTimeout),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var data []RBDLock
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (t *T) listDevices(ctx context.Context) ([]RBDMap, error) {
	args, err := t.keyringArgs()
	if err != nil {
		return nil, err
	}
	args = append(args, "device", "list", "--format", "json")
	cmd := command.New(
		command.WithContext(ctx),
		command.WithTimeout(DefaultCommandTimeout),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var data []RBDMap
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (t *T) Stop(ctx context.Context) error {
	if err := t.unmapDevice(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) removeHolders(ctx context.Context) error {
	return t.exposedDevice().RemoveHolders(ctx)
}

func (t *T) Status(ctx context.Context) status.T {
	if v, err := t.isMapped(ctx); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if v {
		return status.Up
	}
	return status.Down
}

func (t *T) Label(_ context.Context) string {
	return t.Name
}

func (t *T) isMapped(ctx context.Context) (bool, error) {
	data, err := t.listDevices(ctx)
	if err != nil {
		return false, err
	}
	for _, dev := range data {
		if dev.ImageSpec() == t.Name {
			return true, nil
		}
	}
	return false, nil
}

func (t *T) sharedLockID() string {
	return t.ObjectFQDN
}

func (t *T) lockID() string {
	return t.ObjectFQDN + ":" + hostname.Hostname()
}

func (t *T) isLocked(ctx context.Context) (bool, error) {
	data, err := t.listLocks(ctx)
	if err != nil {
		return false, err
	}
	var lockID string
	switch t.Access {
	case "rwx", "rox":
		lockID = t.lockID()
	default:
		lockID = t.sharedLockID()
	}
	for _, lock := range data {
		if lock.ID == lockID {
			return true, nil
		}
	}
	if len(data) > 0 {
		return true, fmt.Errorf("device is locked by a tiers")
	}
	return false, nil
}

func (t *T) exists(ctx context.Context) (bool, error) {
	data, err := t.deviceInfo(ctx)
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, nil
	}
	return true, nil
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	exists, err := t.exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		t.Log().Infof("%s is already provisioned", t.Name)
		return nil
	}
	if err := t.createDevice(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.removeDevice(ctx)
	})
	return nil
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	exists, err := t.exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		t.Log().Infof("%s is already unprovisioned", t.Name)
		return nil
	}
	return t.removeDevice(ctx)
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	v, err := t.exists(ctx)
	return provisioned.FromBool(v), err
}

func (t *T) devpath() string {
	return "/dev/rbd/" + t.Name
}

func (t *T) exposedDevice() device.T {
	return device.New(t.devpath())
}

func (t *T) ClaimedDevices(ctx context.Context) device.L {
	return device.L{}
}

func (t *T) ExposedDevices(ctx context.Context) device.L {
	return device.L{t.exposedDevice()}
}

func (t *T) SubDevices(ctx context.Context) device.L {
	return device.L{}
}

func (t *T) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}

func (t *T) PreMove(ctx context.Context, to string) error {
	info, err := t.deviceInfo(ctx)
	if err != nil {
		return err
	}
	if !slices.Contains(info.Features, "exclusive-lock") {
		t.Log().Infof("feature exclusive-lock is already disabled")
		return nil
	}
	for _, feature := range []string{"journaling", "object-map", "exclusive-lock"} {
		if !slices.Contains(info.Features, feature) {
			t.Log().Infof("feature %s is already disabled")
			continue
		}
		if err := t.disableFeature(ctx, feature); err != nil {
			return err
		}
		t.featureDisabled = append(t.featureDisabled, feature)
	}
	sshKeyFile := t.GetSSHKeyFile()
	if sshKeyFile == "" {
		return fmt.Errorf("no opensvc ssh key file")
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("ssh"),
		command.WithVarArgs("-i", sshKeyFile, to, fmt.Sprintf("rbd map %s", t.Name)),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) disableFeature(ctx context.Context, feature string) error {
	args, err := t.keyringArgs()
	if err != nil {
		return err
	}
	args = append(args, "feature", "disable", t.Name, feature)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

func (t *T) enableFeature(ctx context.Context, feature string) error {
	args, err := t.keyringArgs()
	if err != nil {
		return err
	}
	args = append(args, "feature", "enable", t.Name, feature)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("rbd"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()

}

func (t *T) restoreFeatures(ctx context.Context) error {
	info, err := t.deviceInfo(ctx)
	if err != nil {
		return err
	}
	for _, feature := range t.featureDisabled {
		if !slices.Contains(info.Features, feature) {
			t.Log().Infof("feature %s is already re-enabled")
			continue
		}
		if err := t.enableFeature(ctx, feature); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) PreMoveRollback(ctx context.Context, to string) error {
	if err := t.restoreFeatures(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) PostMove(ctx context.Context, to string) error {
	defer func() {
		t.featureDisabled = nil
	}()
	if err := t.restoreFeatures(ctx); err != nil {
		return err
	}
	return t.unmapDevice(ctx)
}

func (t *T) keyringArgs() ([]string, error) {
	if t.Keyring == "" {
		return []string{}, nil
	}
	if t.keyringArgsCache != nil {
		return t.keyringArgsCache, nil
	}
	km, err := datarecv.ParseKeyMetaRelObj(t.Keyring, t.GetObject())
	if err != nil {
		t.keyringArgsCache = []string{}
		return t.keyringArgsCache, err
	}
	keyringFile, err := km.CacheFile()
	if err != nil {
		t.keyringArgsCache = []string{}
		return t.keyringArgsCache, err
	}
	t.keyringArgsCache = []string{"--keyring", keyringFile}
	return t.keyringArgsCache, nil
}
