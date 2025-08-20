//go:build linux

package lvm2

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/fcache"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/sizeconv"
	"github.com/rs/zerolog"
)

type (
	LVInfo struct {
		LVName          string `json:"lv_name"`
		VGName          string `json:"vg_name"`
		LVAttr          string `json:"lv_attr"`
		LVSize          string `json:"lv_size"`
		Origin          string `json:"origin"`
		DataPercent     string `json:"data_percent"`
		CopyPercent     string `json:"copy_percent"`
		MetadataPercent string `json:"metadata_percent"`
		MetadataDevices string `json:"metadata_devices"`
		MovePV          string `json:"move_pv"`
		ConvertPV       string `json:"convert_pv"`
		MirrorLog       string `json:"mirror_log"`
		Devices         string `json:"devices"`
	}
	LV struct {
		driver
		LVName string
		VGName string
	}
	LVAttrIndex uint8
	LVAttrs     string
	LVAttr      rune
)

const (
	LVAttrIndexType        LVAttrIndex = 0
	LVAttrIndexPermissions LVAttrIndex = iota
	LVAttrIndexAllocationPolicy
	LVAttrIndexAllocationFixedMinor
	LVAttrIndexState
	LVAttrIndexDeviceOpen
	LVAttrIndexTargetType
	LVAttrIndexZeroDataBlocks
	LVAttrIndexVolumeHealth
	LVAttrIndexSkipActivation
)

const (
	// State attrs field (index 4)

	LVAttrStateActive                               LVAttr = 'a'
	LVAttrStateHistorical                           LVAttr = 'h'
	LVAttrStateSuspended                            LVAttr = 's'
	LVAttrStateInvalidSnapshot                      LVAttr = 'I'
	LVAttrStateSuspendedSnapshot                    LVAttr = 'S'
	LVAttrStateSnapshotMergeFailed                  LVAttr = 'm'
	LVAttrStateSuspendedSnapshotMergeFailed         LVAttr = 'M'
	LVAttrStateMappedDevicePresentWithoutTable      LVAttr = 'd'
	LVAttrStateMappedDevicePresentWithInactiveTable LVAttr = 'i'
	LVAttrStateThinPoolCheckNeeded                  LVAttr = 'c'
	LVAttrStateSuspendedThinPoolCheckNeeded         LVAttr = 'C'
	LVAttrStateUnknown                              LVAttr = 'X'
)

func DMDevPath(vg, lv string) string {
	return "/dev/mapper/" + DMName(vg, lv)
}

func DMName(vg, lv string) string {
	lv = strings.ReplaceAll(lv, "-", "--")
	vg = strings.ReplaceAll(vg, "-", "--")
	return vg + "-" + lv
}

func NewLV(vg string, lv string, opts ...funcopt.O) *LV {
	t := LV{
		VGName: vg,
		LVName: lv,
	}
	_ = funcopt.Apply(&t, opts...)
	return &t
}

func (t LV) FQN() string {
	return fmt.Sprintf("%s/%s", t.VGName, t.LVName)
}

func (t LV) DevPath() string {
	return fmt.Sprintf("/dev/%s/%s", t.VGName, t.LVName)
}

func (t *LV) Activate() error {
	return t.change([]string{"-ay"})
}

func (t *LV) Deactivate() error {
	return t.change([]string{"-an"})
}

func (t *LV) change(args []string) error {
	fqn := t.FQN()
	cmd := command.New(
		command.WithName("lvchange"),
		command.WithArgs(append(args, fqn)),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()

	// deactivating the last lv of a vg changes it's activation state
	fcache.Clear("vgs")
	fcache.Clear("vgs-devices")

	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *LV) Show() (*LVInfo, error) {
	data := ShowData{}
	fqn := t.FQN()
	cmd := command.New(
		command.WithName("lvs"),
		command.WithVarArgs("--reportformat", "json", fqn),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	if err := cmd.Run(); err != nil {
		if cmd.ExitCode() == 5 {
			return nil, fmt.Errorf("%w: %s", ErrExist, fqn)
		}
		return nil, err
	}
	if err := json.Unmarshal(cmd.Stdout(), &data); err != nil {
		return nil, err
	}
	if len(data.Report) == 1 && len(data.Report[0].LV) == 1 {
		return &data.Report[0].LV[0], nil
	}
	return nil, fmt.Errorf("%w: %s", ErrExist, fqn)
}

func (t *LV) Attrs() (LVAttrs, error) {
	lvInfo, err := t.Show()
	switch {
	case errors.Is(err, ErrExist):
		return "", nil
	case err != nil:
		return "", err
	default:
		return LVAttrs(lvInfo.LVAttr), nil
	}
}

func (t LVAttrs) Attr(index LVAttrIndex) LVAttr {
	if len(t) < int(index)+1 {
		return ' '
	}
	return LVAttr(t[index])
}

func (t *LV) Exists() (bool, error) {
	_, err := t.Show()
	switch {
	case errors.Is(err, ErrExist):
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}

func (t *LV) IsActive() (bool, error) {
	if attrs, err := t.Attrs(); err != nil {
		return false, err
	} else {
		return attrs.Attr(LVAttrIndexState) == LVAttrStateActive, nil
	}
}

func (t *LV) Devices() (device.L, error) {
	l := make(device.L, 0)
	data := ShowData{}
	fqn := t.FQN()
	cmd := command.New(
		command.WithName("lvs"),
		command.WithVarArgs("-o", "devices,metadata_devices", "--reportformat", "json", fqn),
		command.WithLogger(t.Log()),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cmd.Stdout(), &data); err != nil {
		return nil, err
	}
	if len(data.Report) == 0 {
		return nil, fmt.Errorf("%s: no report", cmd)
	}
	switch len(data.Report[0].LV) {
	case 0:
		return nil, fmt.Errorf("lv %s not found", fqn)
	case 1:
		// expected
	default:
		return nil, fmt.Errorf("lv %s has multiple matches", fqn)
	}
	devices := strings.Split(data.Report[0].LV[0].Devices, ",")
	devices = append(devices, strings.Split(data.Report[0].LV[0].MetadataDevices, ",")...)
	for _, s := range devices {
		if s == "" {
			continue
		}
		path := strings.Split(s, "(")[0]
		if !strings.HasPrefix(path, "/") {
			// implicitly a lv name in the same vg
			// convert to a DeviceMapper devpath
			// (rmeta are not exposed as /dev/<vg>/<lv>)
			path = DMDevPath(t.VGName, path)
		}
		dev := device.New(path, device.WithLogger(t.Log()))
		l = append(l, dev)
	}
	return l, nil
}

func (t *LV) Create(size string, args []string) error {
	if strings.Contains(size, "%") {
		args = append(args, "-l", size)
	} else if i, err := sizeconv.FromSize(size); err == nil {
		// default unit is not "B", explicitly tell
		size = fmt.Sprintf("%dB", i)
		args = append(args, "-L", size)
	} else {
		args = append(args, "-L", size)
	}
	cmd := command.New(
		command.WithName("lvcreate"),
		command.WithArgs(append(args, "--yes", "-n", t.LVName, t.VGName)),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *LV) Wipe() error {
	path := t.DevPath()
	if !file.Exists(path) {
		t.Log().Infof("skip wipe: %s does not exist", path)
		return nil
	}
	dev := device.New(path, device.WithLogger(t.Log()))
	return dev.Wipe()
}

func (t *LV) Remove(args []string) error {
	bdev := t.DevPath()
	cmd := command.New(
		command.WithName("lvremove"),
		command.WithArgs(append(args, bdev)),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}
