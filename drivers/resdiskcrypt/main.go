//go:build linux

package resdiskcrypt

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/udevadm"

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
		Name             string `json:"name"`
		Dev              string `json:"dev"`
		ManagePassphrase bool   `json:"manage_passphrase"`
		Secret           string `json:"secret"`
		FormatLabel      string `json:"label"`
		Path             path.T `json:"path"`
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func genPassphrase() []byte {
	var s strings.Builder
	passLen := 256
	max := len(allCharSet)

	for i := 0; i < passLen; i++ {
		n := rand.Intn(max)
		s.WriteString(string(allCharSet[n]))
	}
	inRune := []rune(s.String())
	rand.Shuffle(len(inRune), func(i, j int) {
		inRune[i], inRune[j] = inRune[j], inRune[i]
	})
	return []byte(string(inRune))
}

func (t T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{"name", t.getName()},
		{"dev", t.getDev()},
		{"secret", t.Secret},
		{"label", t.FormatLabel},
		{"manage_passphrase", fmt.Sprint(t.ManagePassphrase)},
	}
	return m, nil
}

func (t T) secPath() (path.T, error) {
	return path.New(t.Secret, t.Path.Namespace, kind.Sec.String())
}

func (t T) passphraseKeyname() string {
	s := t.RID()
	s = strings.ReplaceAll(s, "#", "_")
	s = s + "_crypt_passphrase"
	return s
}

func (t T) forgetPassphrase() error {
	sec, err := t.sec()
	if err != nil {
		return err
	}
	keyname := t.passphraseKeyname()
	if !t.ManagePassphrase {
		t.Log().Info().Msgf("leave key %s in %s", keyname, sec.Path())
		return nil
	}
	t.Log().Info().Msgf("remove key %s in %s", keyname, sec.Path())
	return sec.RemoveKey(keyname)
}

func (t T) passphraseNew() ([]byte, error) {
	sec, err := t.sec()
	if err != nil {
		return nil, err
	}
	b := genPassphrase()
	keyname := t.passphraseKeyname()
	if err := sec.AddKey(keyname, b); err != nil {
		return nil, err
	}
	return sec.DecodeKey(keyname)
}

func (t T) passphraseStrict() ([]byte, error) {
	sec, err := t.sec()
	if err != nil {
		return nil, err
	}
	if !sec.Path().Exists() {
		return nil, fmt.Errorf("%s does not exist", sec.Path())
	}
	keyname := t.passphraseKeyname()
	if !sec.HasKey(keyname) {
		return nil, fmt.Errorf("%s does not exist", sec.Path())
	}
	return sec.DecodeKey(keyname)
}

func (t T) verifyPassphrase(force bool) error {
	if force {
		return nil
	}
	if _, err := t.passphraseStrict(); err != nil {
		return fmt.Errorf("abort crypt deactivate, so you can backup the device that we won't be able to activate again: %s. restore the key or use --force to skip this safeguard", err)
	}
	return nil
}

func (t T) sec() (object.Sec, error) {
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

func (t T) exists() (bool, error) {
	dev := t.getDev()
	if dev == "" {
		return false, nil
	}
	cmd := command.New(
		command.WithName(cryptsetup),
		command.WithVarArgs("isLuks", dev),
	)
	if err := cmd.Run(); err != nil {
		return false, err
	}
	if cmd.ExitCode() != 0 {
		return false, nil
	}
	return true, nil
}

func (t T) isUp() (bool, error) {
	if v, err := t.exists(); err != nil {
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

func (t T) activate() error {
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

func (t T) deactivate(force bool) error {
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

func (t T) Start(ctx context.Context) error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("%s is already up", t.exposedDevpath())
		return nil
	}
	if err := t.activate(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.deactivate(true)
	})
	return nil
}

func (t T) Stop(ctx context.Context) error {
	if v, err := t.isUp(); err != nil {
		return err
	} else if !v {
		t.Log().Info().Msgf("%s is already down", t.exposedDevpath())
		return nil
	}
	if err := t.removeHolders(); err != nil {
		return err
	}
	udevadm.Settle()
	force := actioncontext.IsForce(ctx)
	return t.deactivate(force)
}

func (t T) removeHolders() error {
	return t.exposedDevice().RemoveHolders()
}

func (t T) getDev() string {
	return t.Dev
}

func (t T) getName() string {
	if t.Name != "" {
		return t.Name
	}
	dev := t.getDev()
	return filepath.Base(dev) + "-crypt"
}

func (t *T) Status(ctx context.Context) status.T {
	if v, err := t.isUp(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	} else if v {
		return status.Up
	}
	return status.Down
}

func (t T) Label() string {
	return t.getName()
}

func (t T) ProvisionLeader(ctx context.Context) error {
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
	if v, err := t.exists(); err != nil {
		return err
	} else if v {
		t.Log().Info().Msgf("%s is already luks formated", dev)
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

func (t T) UnprovisionLeader(ctx context.Context) error {
	dev := t.getDev()
	if dev == "" {
		return nil
	}
	if v, err := t.exists(); err != nil {
		return err
	} else if !v {
		t.Log().Info().Msgf("%s already erased", dev)
		return nil
	}
	if err := t.erase(dev); err != nil {
		return err
	}
	t.forgetPassphrase()
	return nil
}

func (t T) erase(dev string) error {
	cmd := command.New(
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

func (t T) Provisioned() (provisioned.T, error) {
	v, err := t.exists()
	return provisioned.FromBool(v), err
}

func (t T) exposedDevpath() string {
	name := t.getName()
	if name == "" {
		return ""
	}
	return fmt.Sprintf("/dev/mapper/%s", name)
}

func (t T) exposedDevice() *device.T {
	devpath := t.exposedDevpath()
	if devpath == "" {
		return nil
	}
	dev := device.New(devpath, device.WithLogger(t.Log()))
	return &dev
}

func (t T) ExposedDevices() device.L {
	dev := t.exposedDevice()
	if dev == nil {
		return device.L{}
	}
	return device.L{*dev}
}

func (t *T) ReservableDevices() device.L {
	return t.SubDevices()
}

func (t T) SubDevices() device.L {
	devp := t.getDev()
	if devp == "" {
		return device.L{}
	}
	dev := device.New(devp, device.WithLogger(t.Log()))
	return device.L{dev}
}
