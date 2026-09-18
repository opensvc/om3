//go:build linux

package resdiskcrypt

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resdisk"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/sizeconv"
	"github.com/opensvc/om3/v3/util/udevadm"

	"github.com/rs/zerolog"
)

const (
	cryptsetup     = "cryptsetup"
	lowerCharSet   = "abcdedfghijklmnopqrst"
	upperCharSet   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	specialCharSet = "!\"#$%&()*+,-./:;<=>?@[]^_`{|}~"
	numberSet      = "0123456789"
	allCharSet     = lowerCharSet + upperCharSet + specialCharSet + numberSet
)

type (
	T struct {
		resdisk.T
		Name             string      `json:"name"`
		Dev              string      `json:"dev"`
		ManagePassphrase bool        `json:"manage_passphrase"`
		Secret           string      `json:"secret"`
		FormatLabel      string      `json:"label"`
		Path             naming.Path `json:"path"`
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func genPassphrase() []byte {
	var s strings.Builder
	passLen := 256
	maxValue := len(allCharSet)

	for i := 0; i < passLen; i++ {
		n := rand.Intn(maxValue)
		s.WriteString(string(allCharSet[n]))
	}
	inRune := []rune(s.String())
	rand.Shuffle(len(inRune), func(i, j int) {
		inRune[i], inRune[j] = inRune[j], inRune[i]
	})
	return []byte(string(inRune))
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "name", Value: t.getName()},
		{Key: "dev", Value: t.getDev()},
		{Key: "secret", Value: t.Secret},
		{Key: "label", Value: t.FormatLabel},
		{Key: "manage_passphrase", Value: fmt.Sprint(t.ManagePassphrase)},
	}
	return m, nil
}

func (t *T) secPath() (naming.Path, error) {
	return naming.NewPath(t.Path.Namespace, naming.KindSec, t.Secret)
}

func (t *T) passphraseKeyname() string {
	s := t.RID()
	s = strings.ReplaceAll(s, "#", "_")
	s = s + "_crypt_passphrase"
	return s
}

func (t *T) forgetPassphrase() error {
	sec, err := t.sec()
	if err != nil {
		return err
	}
	keyname := t.passphraseKeyname()
	if !t.ManagePassphrase {
		t.Log().Infof("leave key %s in %s", keyname, sec.Path())
		return nil
	}
	t.Log().Infof("remove key %s in %s", keyname, sec.Path())
	return sec.RemoveKey(keyname)
}

func (t *T) passphraseNew() ([]byte, error) {
	sec, err := t.sec()
	if err != nil {
		return nil, err
	}
	keyname := t.passphraseKeyname()
	if !sec.HasKey(keyname) {
		b := genPassphrase()
		if err := sec.AddKey(keyname, b); err != nil {
			return nil, err
		}
	}
	return sec.DecodeKey(keyname)
}

func (t *T) passphraseStrict() ([]byte, error) {
	sec, err := t.sec()
	if err != nil {
		return nil, err
	}
	if !sec.Path().Exists() {
		return nil, fmt.Errorf("%s does not exist", sec.Path())
	}
	keyname := t.passphraseKeyname()
	if !sec.HasKey(keyname) {
		return nil, fmt.Errorf("%s:%s does not exist", sec.Path(), keyname)
	}
	return sec.DecodeKey(keyname)
}

func (t *T) verifyPassphrase(force bool) error {
	if force {
		return nil
	}
	if _, err := t.passphraseStrict(); err != nil {
		return fmt.Errorf("abort crypt deactivate, so you can backup the device that we won't be able to activate again: %s. restore the key or use --force to skip this safeguard", err)
	}
	return nil
}

func (t *T) sec() (object.Sec, error) {
	p, err := t.secPath()
	if err != nil {
		return nil, err
	}
	v, err := object.NewSec(p)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (t *T) exists(ctx context.Context) (bool, error) {
	dev := t.getDev()
	if dev == "" {
		return false, nil
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cryptsetup),
		command.WithVarArgs("isLuks", dev),
		command.WithIgnoredExitCodes(0, 1, 4),
	)
	if err := cmd.Run(); err != nil {
		return false, err
	}
	if cmd.ExitCode() != 0 {
		return false, nil
	}
	return true, nil
}

func (t *T) isUp(ctx context.Context) (bool, error) {
	if v, err := t.exists(ctx); err != nil {
		return false, err
	} else if !v {
		return false, nil
	}
	dev := t.exposedDevice()
	if dev == nil {
		return false, nil
	}
	return file.Exists(dev.String()), nil
}

func (t *T) activate(ctx context.Context) error {
	devp := t.getDev()
	if devp == "" {
		return fmt.Errorf("abort luksOpen: no dev")
	}
	name := t.getName()
	if name == "" {
		return fmt.Errorf("abort luksOpen: no name")
	}
	b, err := t.passphraseStrict()
	if err != nil {
		return err
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cryptsetup),
		command.WithVarArgs("luksOpen", devp, name, "-"),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	stdin, err := cmd.Cmd().StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := io.WriteString(stdin, string(b)); err != nil {
		return err
	} else {
		stdin.Close()
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *T) deactivate(ctx context.Context, force bool) error {
	name := t.getName()
	if name == "" {
		return nil
	}
	dev := t.exposedDevice()
	if dev == nil {
		return nil
	}
	if err := t.verifyPassphrase(force); err != nil {
		return err
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cryptsetup),
		command.WithVarArgs("luksClose", dev.String(), name),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *T) Start(ctx context.Context) error {
	if v, err := t.isUp(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s is already up", t.exposedDevpath())
		return nil
	}
	if err := t.activate(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.deactivate(ctx, true)
	})
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	devPath := t.exposedDevpath()
	if !file.Exists(devPath) {
		t.Log().Infof("%s is already down", devPath)
		return nil
	}
	if err := t.removeHolders(ctx); err != nil {
		return err
	}
	udevadm.Settle()
	force := actioncontext.IsForce(ctx)
	return t.deactivate(ctx, force)
}

func (t *T) removeHolders(ctx context.Context) error {
	return t.exposedDevice().RemoveHolders(ctx)
}

func (t *T) getDev() string {
	return t.Dev
}

func (t *T) getName() string {
	if t.Name != "" {
		return t.Name
	}
	dev := t.getDev()
	return filepath.Base(dev) + "-crypt"
}

func (t *T) Status(ctx context.Context) status.T {
	if v, err := t.isUp(ctx); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if v {
		return status.Up
	}
	return status.Down
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.getName()
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	dev := t.getDev()
	if dev == "" {
		return fmt.Errorf("no dev")
	}
	name := t.getName()
	if name == "" {
		return fmt.Errorf("no name")
	}
	var (
		b   []byte
		err error
	)
	if v, err := t.exists(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("%s is already luks formatted", dev)
		return nil
	}
	if t.ManagePassphrase {
		b, err = t.passphraseNew()
	} else {
		b, err = t.passphraseStrict()
	}
	if err != nil {
		return err
	}
	args := []string{
		"luksFormat",
		"--hash", "sha512",
		"--key-size", "512",
		"--cipher", "aes-xts-plain64",
		"--type", "luks2",
		"--batch-mode",
	}
	if t.FormatLabel != "" {
		args = append(args, "--label", t.FormatLabel)
	}
	args = append(args, dev, "-")
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cryptsetup),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	stdin, err := cmd.Cmd().StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := io.WriteString(stdin, string(b)); err != nil {
		return err
	} else {
		stdin.Close()
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	dev := t.getDev()
	if dev == "" {
		return nil
	}
	if v, err := t.exists(ctx); err != nil {
		return err
	} else if !v {
		t.Log().Infof("%s already erased", dev)
		return nil
	}
	if err := t.erase(ctx, dev); err != nil {
		return err
	}
	t.forgetPassphrase()
	return nil
}

func (t *T) erase(ctx context.Context, dev string) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cryptsetup),
		command.WithVarArgs("luksErase", "--batch-mode", dev),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	err := cmd.Run()
	if err != nil {
		return err
	}
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	v, err := t.exists(ctx)
	return provisioned.FromBool(v), err
}

func (t *T) exposedDevpath() string {
	name := t.getName()
	if name == "" {
		return ""
	}
	return fmt.Sprintf("/dev/mapper/%s", name)
}

func (t *T) exposedDevice() *device.T {
	devpath := t.exposedDevpath()
	if devpath == "" {
		return nil
	}
	dev := device.New(devpath, device.WithLogger(t.Log()))
	return &dev
}

// CurrentSize implements resource.Sizer.
//
// It is what the mapped device hands out, which is less than the device under
// it holds: the header sits at the front of it.
func (t *T) CurrentSize(ctx context.Context) (int64, error) {
	dev := t.exposedDevice()
	if dev == nil {
		return 0, fmt.Errorf("%s is not open, so its size cannot be read", t.getName())
	}
	return dev.Size()
}

// ResizePlan implements resource.Resizer.
//
// What it asks of the device below is the size wanted plus the header in
// front of it. The header offset is written when the volume is formatted and
// does not move, so it is read rather than measured as the gap between the
// two: that gap is the growth itself while a resize is in flight, and asking
// for it again compounds every pass.
func (t *T) ResizePlan(ctx context.Context, to int64) (int64, error) {
	offset, err := t.headerOffset(ctx)
	if err != nil {
		return 0, err
	}
	return sizeconv.RoundUp(to+offset, 512), nil
}

// Resize implements resource.Resizer.
//
// The passphrase is handed over the same way opening it does: a luks2 volume
// derives its key again to resize, and asks for it on the terminal otherwise.
func (t *T) Resize(ctx context.Context, to int64) error {
	name := t.getName()
	if name == "" {
		return fmt.Errorf("abort resize: no name")
	}
	b, err := t.passphraseStrict()
	if err != nil {
		return err
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cryptsetup),
		command.WithVarArgs("resize", "--size", fmt.Sprintf("%d", to/512), "--key-file", "-", name),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	stdin, err := cmd.Cmd().StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := io.WriteString(stdin, string(b)); err != nil {
		return err
	} else {
		stdin.Close()
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

// headerOffset is where the data begins on the device below, which is what
// the header takes from it.
func (t *T) headerOffset(ctx context.Context) (int64, error) {
	name := t.getName()
	if name == "" {
		return 0, fmt.Errorf("abort status: no name")
	}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(cryptsetup),
		command.WithVarArgs("status", name),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
		command.WithBufferedStdout(),
	)
	b, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(key) != "offset" {
			continue
		}
		field, _, _ := strings.Cut(strings.TrimSpace(value), " ")
		sectors, err := strconv.ParseInt(field, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%s: parse offset %s: %w", name, field, err)
		}
		return sectors * 512, nil
	}
	return 0, fmt.Errorf("%s reports no offset", name)
}

func (t *T) ExposedDevices(ctx context.Context) device.L {
	dev := t.exposedDevice()
	if dev == nil {
		return device.L{}
	}
	return device.L{*dev}
}

func (t *T) ReservableDevices(ctx context.Context) device.L {
	return t.SubDevices(ctx)
}

func (t *T) SubDevices(ctx context.Context) device.L {
	devp := t.getDev()
	if devp == "" {
		return device.L{}
	}
	dev := device.New(devp, device.WithLogger(t.Log()))
	return device.L{dev}
}
