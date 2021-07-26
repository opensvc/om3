// +build linux

package lvm2

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/sizeconv"
)

type (
	VGInfo struct {
		VGName    string `json:"vg_name"`
		VGAttr    string `json:"vg_attr"`
		VGSize    string `json:"vg_size"`
		VGFree    string `json:"vg_free"`
		VGTags    string `json:"vg_tags"`
		SnapCount string `json:"snap_count"`
		PVCount   string `json:"pv_count"`
		LVCount   string `json:"lv_count"`
		PVName    string `json:"pv_name"`
		LVName    string `json:"lv_name"`
		Devices   string `json:"devices"`
	}
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
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
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
			return nil, errors.Wrap(ErrExist, t.VGName)
		}
		return nil, err
	}
	if err := json.Unmarshal(cmd.Stdout(), &data); err != nil {
		return nil, err
	}
	if len(data.Report) == 1 && len(data.Report[0].VG) == 1 {
		return &data.Report[0].VG[0], nil
	}
	return nil, errors.Wrap(ErrExist, t.VGName)
}

func (t *VG) Attrs() (VGAttrs, error) {
	vgInfo, err := t.Show("vg_attr")
	switch {
	case errors.Is(err, ErrExist):
		return "", nil
	case err != nil:
		return "", err
	default:
		return VGAttrs(vgInfo.VGAttr), nil
	}
}

func (t *VG) Tags() ([]string, error) {
	vgInfo, err := t.Show("vg_tags")
	switch {
	case errors.Is(err, ErrExist):
		return []string{}, nil
	case err != nil:
		return []string{}, err
	default:
		return strings.Split(vgInfo.VGTags, ","), nil
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
	_, err := t.Show("vg_name")
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

func (t *VG) Devices() ([]*device.T, error) {
	l := make([]*device.T, 0)
	data := ShowData{}
	cmd := command.New(
		command.WithName("vgs"),
		command.WithVarArgs("-o", "devices", "--reportformat", "json", t.VGName),
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
	switch len(data.Report[0].VG) {
	case 0:
		return nil, fmt.Errorf("lv %s not found", t.VGName)
	case 1:
		// expected
	default:
		return nil, fmt.Errorf("lv %s has multiple matches", t.VGName)
	}
	for _, s := range strings.Fields(data.Report[0].VG[0].Devices) {
		path := strings.Split(s, "(")[0]
		dev := device.New(path, device.WithLogger(t.Log()))
		l = append(l, dev)
	}
	return l, nil
}

func (t *VG) Create(size string, pvs []string, options []string) error {
	if i, err := sizeconv.FromSize(size); err == nil {
		// default unit is not "B", explicitely tell
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
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *VG) PVs() ([]*device.T, error) {
	l := make([]*device.T, 0)
	vgInfo, err := t.Show("pv_name")
	switch {
	case errors.Is(err, ErrExist):
		return l, nil
	case err != nil:
		return l, err
	}
	for _, s := range strings.Split(vgInfo.PVName, ",") {
		l = append(l, device.New(s, device.WithLogger(t.Log())))
	}
	return l, nil
}
