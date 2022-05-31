//go:build linux
// +build linux

package resdiskcrypt

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os/exec"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/drivers/resdisk"
	"opensvc.com/opensvc/util/capabilities"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/converters"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/udevadm"

	"github.com/rs/zerolog"
)

const (
	driverGroup    = driver.GroupDisk
	driverName     = "crypt"
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

func capabilitiesScanner() ([]string, error) {
	if _, err := exec.LookPath(cryptsetup); err != nil {
		return []string{}, nil
	}
	return []string{"drivers.resource.disk.crypt"}, nil
}

func init() {
	capabilities.Register(capabilitiesScanner)
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword(resdisk.BaseKeywords...)
	m.AddKeyword(manifest.ProvisioningKeywords...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:      "name",
			Attr:        "Name",
			Scopable:    true,
			Text:        "The basename of the exposed device.",
			DefaultText: "The basename of the underlying device, suffixed with '-crypt'.",
			Example:     "{fqdn}-crypt",
		},
		{
			Option:   "dev",
			Attr:     "Dev",
			Scopable: true,
			Required: true,
			Text:     "The fullpath of the underlying block device.",
			Example:  "/dev/{fqdn}/lv1",
		},
		{
			Option:       "manage_passphrase",
			Attr:         "ManagePassphrase",
			Scopable:     true,
			Provisioning: true,
			Converter:    converters.Bool,
			Default:      "true",
			Text:         "By default, on provision the driver allocates a new random passphrase (256 printable chars), and forgets it on unprovision. If set to false, require a passphrase to be already present in the sec object to provision, and don't remove it on unprovision.",
		},
		{
			Option:   "secret",
			Attr:     "Secret",
			Scopable: true,
			Text:     "The name of the sec object hosting the crypt secrets. The sec object must be in the same namespace than the object defining the disk.crypt resource.",
			Default:  "{name}",
		},
		{
			Option:       "label",
			Attr:         "FormatLabel",
			Scopable:     true,
			Provisioning: true,
			Text:         "The label to set in the cryptsetup metadata writen on dev. A label helps admin understand the role of a device.",
			Default:      "{fqdn}",
		},
	}...)
	m.AddContext([]manifest.Context{
		{
			Key:  "path",
			Attr: "Path",
			Ref:  "object.path",
		},
	}...)
	return m
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

func (t T) Info() map[string]string {
	m := make(map[string]string)
	m["name"] = t.getName()
	m["dev"] = t.getDev()
	m["secret"] = t.Secret
	m["label"] = t.FormatLabel
	m["manage_passphrase"] = fmt.Sprint(t.ManagePassphrase)
	return m
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
		t.Log().Info().Msgf("leave key %s in %s", keyname, sec.Path)
		return nil
	}
	t.Log().Info().Msgf("remove key %s in %s", keyname, sec.Path)
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
	if !sec.Exists() {
		return nil, fmt.Errorf("%s does not exist", sec.Path)
	}
	keyname := t.passphraseKeyname()
	if !sec.HasKey(keyname) {
		return nil, fmt.Errorf("%s does not exist", sec.Path)
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

func (t T) sec() (*object.Sec, error) {
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
	return device.New(devpath, device.WithLogger(t.Log()))
}

func (t T) ExposedDevices() []*device.T {
	dev := t.exposedDevice()
	if dev == nil {
		return []*device.T{}
	}
	return []*device.T{t.exposedDevice()}
}

func (t T) SubDevices() []*device.T {
	devp := t.getDev()
	if devp == "" {
		return []*device.T{}
	}
	dev := device.New(devp, device.WithLogger(t.Log()))
	return []*device.T{dev}
}
