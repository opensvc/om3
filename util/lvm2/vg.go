//go:build linux

package lvm2

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/fcache"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/sizeconv"
)

type (
	VG struct {
		driver
		VGName string
		log    *zerolog.Logger
	}
	VGAttrIndex uint8
	VGAttrs     string
	VGAttr      rune
)

const (
	VGAttrIndexPermissions VGAttrIndex = 0
	VGAttrIndexResizeable  VGAttrIndex = iota
	VGAttrIndexExported
	VGAttrIndexPartial
	VGAttrIndexAllocationPolicy
	VGAttrIndexClusteredOrShared
)

const (
	// State attrs field

	VGAttrStateWriteable       VGAttr = 'w'
	VGAttrStateReadOnly        VGAttr = 'r'
	VGAttrStateResizeable      VGAttr = 'z'
	VGAttrStateExported        VGAttr = 'x'
	VGAttrStatePartial         VGAttr = 'p'
	VGAttrStateAllocContiguous VGAttr = 'c'
	VGAttrStateAllocCling      VGAttr = 'l'
	VGAttrStateAllocNormal     VGAttr = 'n'
	VGAttrStateAllocAnywhere   VGAttr = 'a'
	VGAttrStateClustered       VGAttr = 'c'
	VGAttrStateShared          VGAttr = 's'
)

func NewVG(vg string, opts ...funcopt.O) *VG {
	t := VG{
		VGName: vg,
	}
	_ = funcopt.Apply(&t, opts...)
	return &t
}

func (t VG) FQN() string {
	return t.VGName
}

func (t *VG) Activate() error {
	return t.change([]string{"-ay"})
}

func (t *VG) Deactivate() error {
	return t.change([]string{"-an"})
}

func (t *VG) ImportDevices() error {
	if v, err := file.ExistsAndRegular("/etc/lvm/devices/system.devices"); err != nil {
		return err
	} else if !v {
		return nil
	}
	cmd := command.New(
		command.WithName("vgimportdevices"),
		command.WithVarArgs(t.VGName),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	err := cmd.Run()
	if pathErr, ok := err.(*os.PathError); ok && pathErr.Err == syscall.ENOENT {
		return nil
	}
	fcache.Clear("vgs")
	fcache.Clear("vgs-devices")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *VG) change(args []string) error {
	cmd := command.New(
		command.WithName("vgchange"),
		command.WithArgs(append(args, t.VGName)),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	fcache.Clear("vgs")
	fcache.Clear("vgs-devices")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *VG) AddNodeTag() error {
	return t.AddTag("@" + hostname.Hostname())
}

func (t *VG) DelNodeTag() error {
	return t.DelTag("@" + hostname.Hostname())
}

func (t *VG) DelTag(s string) error {
	cmd := command.New(
		command.WithName("vgchange"),
		command.WithVarArgs("--deltag", s, t.VGName),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	fcache.Clear("vgs")
	fcache.Clear("vgs-devices")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *VG) AddTag(s string) error {
	cmd := command.New(
		command.WithName("vgchange"),
		command.WithVarArgs("--addtag", s, t.VGName),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	fcache.Clear("vgs")
	fcache.Clear("vgs-devices")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *VG) CachedDevicesShow() (*VGInfo, error) {
	var (
		err error
		out []byte
	)
	data := ShowData{}
	cmd := command.New(
		command.WithName("vgs"),
		command.WithVarArgs("--reportformat", "json", "-o", "devices"),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	if out, err = fcache.Output(cmd, "vgs-devices"); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(out, &data); err != nil {
		return nil, err
	}
	if len(data.Report) != 1 {
		return nil, fmt.Errorf("vgs: no report")
	}
	for _, d := range data.Report[0].VG {
		if d.VGName == t.VGName {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrExist, t.VGName)
}

// CachedNormalShow retrieves cached volume group information, unmarshals the output data,
// and filters by the specified VGName.
//
// It uses the command: vgs --reportformat json -o "+tags,pv_name".
// => The following cmd output will return 2 entries when t.VGName == "data"
//
//	{
//	  "report": [
//	    {"vg": [
//	       {"vg_name": "data", "pv_count": "1", "lv_count": "0", "snap_count": "0", "vg_attr": "wz--n-","vg_size": "<5.00g", "vg_free": "<5.00g", "vg_tags": "local", "pv_name": "/dev/vdb"},
//	       {"vg_name": "data", "pv_count": "1", "lv_count": "0", "snap_count": "0", "vg_attr": "wz--n-","vg_size": "<5.00g", "vg_free": "<5.00g", "vg_tags": "local", "pv_name": "/dev/vdc"},
//	       {"vg_name": "root", "pv_count": "1", "lv_count": "2", "snap_count": "0", "vg_attr": "wz--n-","vg_size": "<38.75g", "vg_free": "0 ", "vg_tags": "local", "pv_name": "/dev/vda3"}
//	   ]}
//	  ]
//	}
func (t *VG) CachedNormalShow() (l []VGInfo, err error) {
	var out []byte
	data := ShowData{}
	cmd := command.New(
		command.WithName("vgs"),
		command.WithVarArgs("--reportformat", "json", "-o", "+tags,pv_name"),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	if out, err = fcache.Output(cmd, "vgs"); err != nil {
		return
	}
	if err = json.Unmarshal(out, &data); err != nil {
		return
	}
	if len(data.Report) != 1 {
		err = fmt.Errorf("vgs: no report")
		return
	}
	for _, d := range data.Report[0].VG {
		if d.VGName == t.VGName {
			l = append(l, d)
		}
	}
	if len(l) == 0 {
		err = fmt.Errorf("%w: %s", ErrExist, t.VGName)
	}
	return
}

func (t *VG) Show(fields string) (*VGInfo, error) {
	data := ShowData{}
	cmd := command.New(
		command.WithName("vgs"),
		command.WithVarArgs("--reportformat", "json", "-o", fields, t.VGName),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	if err := cmd.Run(); err != nil {
		if cmd.ExitCode() == 5 {
			return nil, fmt.Errorf("%w: %s", ErrExist, t.VGName)
		}
		return nil, err
	}
	if err := json.Unmarshal(cmd.Stdout(), &data); err != nil {
		return nil, err
	}
	if len(data.Report) == 1 && len(data.Report[0].VG) == 1 {
		return &data.Report[0].VG[0], nil
	}
	return nil, fmt.Errorf("%w: %s", ErrExist, t.VGName)
}

func (t *VG) Attrs() (VGAttrs, error) {
	vgL, err := t.CachedNormalShow()
	switch {
	case errors.Is(err, ErrExist):
		return "", nil
	case err != nil:
		return "", err
	default:
		if len(vgL) == 0 {
			return "", ErrExist
		}
		return VGAttrs(vgL[0].VGAttr), nil
	}
}

func (t *VG) Tags() ([]string, error) {
	vgL, err := t.CachedNormalShow()
	switch {
	case errors.Is(err, ErrExist):
		return []string{}, nil
	case err != nil:
		return []string{}, err
	default:
		if len(vgL) == 0 {
			return []string{}, nil
		}
		return strings.Split(vgL[0].VGTags, ","), nil
	}
}

func (t *VG) HasTag(s string) (bool, error) {
	tags, err := t.Tags()
	if err != nil {
		return false, err
	}
	for _, tag := range tags {
		if tag == s {
			return true, nil
		}
	}
	return false, nil
}

func (t *VG) HasNodeTag() (bool, error) {
	return t.HasTag(hostname.Hostname())
}

func (t VGAttrs) Attr(index VGAttrIndex) VGAttr {
	if len(t) < int(index)+1 {
		return ' '
	}
	return VGAttr(t[index])
}

func (t *VG) Exists() (bool, error) {
	_, err := t.CachedNormalShow()
	switch {
	case errors.Is(err, ErrExist):
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}

func (t *VG) IsActive() (bool, error) {
	/*
		if attrs, err := t.Attrs(); err != nil {
			return false, err
		} else {
			return attrs.Attr(VGAttrIndexState) == VGAttrStateActive, nil
		}
	*/
	return false, nil
}

func (t *VG) Devices() (device.L, error) {
	l := make(device.L, 0)
	data, err := t.CachedDevicesShow()
	if err != nil {
		return nil, err
	}
	for _, s := range strings.Fields(data.Devices) {
		path := strings.Split(s, "(")[0]
		dev := device.New(path, device.WithLogger(t.Log()))
		l = append(l, dev)
	}
	return l, nil
}

func (t *VG) Create(size string, pvs []string, options []string) error {
	if i, err := sizeconv.FromSize(size); err == nil {
		// default unit is not "B", explicitly tell
		size = fmt.Sprintf("%dB", i)
	}
	args := make([]string, 0)
	args = append(args, t.VGName)
	args = append(args, pvs...)
	args = append(args, options...)
	args = append(args, "--yes")
	cmd := command.New(
		command.WithName("vgcreate"),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	fcache.Clear("vgs")
	fcache.Clear("vgs-devices")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *VG) Wipe() error {
	return nil
}

func (t *VG) Remove(args []string) error {
	cmd := command.New(
		command.WithName("vgremove"),
		command.WithArgs(append(args, t.VGName)),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	fcache.Clear("vgs")
	fcache.Clear("vgs-devices")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *VG) PVs() (device.L, error) {
	l := make(device.L, 0)
	vgL, err := t.CachedNormalShow()
	switch {
	case errors.Is(err, ErrExist):
		return l, nil
	case err != nil:
		return l, err
	}
	for _, vg := range vgL {
		for _, s := range strings.Split(vg.PVName, ",") {
			l = append(l, device.New(s, device.WithLogger(t.Log())))
		}
	}
	return l, nil
}

func (t *VG) ActiveLVs() (device.L, error) {
	l := make(device.L, 0)
	pattern := fmt.Sprintf("/dev/mapper/%s-*", t.VGName)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return l, err
	}
	for _, p := range matches {
		switch {
		case strings.Contains(p, "_rimage_"), strings.Contains(p, "_rmeta_"):
			continue
		case strings.Contains(p, "_mimage_"), strings.Contains(p, "_mlog_"), strings.HasSuffix(p, "_mlog"):
			continue
		}
		l = append(l, device.New(p, device.WithLogger(t.Log())))
	}
	return l, nil
}
