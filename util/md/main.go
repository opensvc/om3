package md

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/fcache"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/stringslice"
)

type (
	T struct {
		name string
		uuid string
		log  *zerolog.Logger
	}
)

var (
	mdadmConfigFileCache string
)

func New(name string, uuid string, opts ...funcopt.O) *T {
	t := T{
		name: name,
		uuid: uuid,
	}
	_ = funcopt.Apply(&t, opts...)
	return &t
}
func WithLogger(log *zerolog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.log = log
		return nil
	})
}

func (t T) detailState() (string, error) {
	buff, err := t.detail()
	if err != nil {
		return "", nil
	}
	scanner := bufio.NewScanner(strings.NewReader(buff))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "State :") {
			continue
		}
		l := strings.SplitN(line, " : ", 2)
		return l[1], nil
	}
	return "", fmt.Errorf("md state not found in details")
}

func (t T) detail() (string, error) {
	cmd := command.New(
		command.WithName(mdadm),
		command.WithVarArgs("--detail", t.devpathFromName()),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	if out, err := cmd.Output(); err != nil {
		return "", err
	} else {
		return string(out), nil
	}
}

func (t T) examineScanVerbose() (string, error) {
	cmd := command.New(
		command.WithName(mdadm),
		command.WithVarArgs("-E", "--scan", "-v"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
	)
	if out, err := fcache.Output(cmd, "mdadm-E-scan-v"); err != nil {
		return "", err
	} else {
		return string(out), nil
	}
}

func (t T) Remove() error {
	panic("not implemented")
	return nil
}

func (t T) IsActive() (bool, string, error) {
	if v, err := t.Exists(); !v {
		return false, "", err
	}
	s, err := t.detailState()
	if err != nil {
		return false, "", err
	}
	if len(s) == 0 {
		return false, "", nil
	}
	states := strings.Split(s, ", ")
	msg := ""
	if len(states) > 1 {
		msg = s
	}
	for _, state := range states {
		var inactive bool
		switch state {
		case "Not Started":
			inactive = true
		case "devpath does not exist":
			inactive = true
		case "unable to find a devpath for md":
			inactive = true
		case "unknown":
			inactive = true
		}
		if inactive {
			t.log.Debug().Msgf("status eval'ed down because: %s", s)
			return false, msg, nil
		}
	}
	return true, msg, nil
}

func (t T) Exists() (bool, error) {
	buff, err := t.examineScanVerbose()
	if err != nil {
		return false, err
	}
	if t.uuid != "" && strings.Contains(buff, "UUID="+t.uuid) {
		return true, nil
	}
	if t.name != "" && strings.Contains(buff, t.devpathFromName()) {
		return true, nil
	}
	return false, nil
}

func (t T) Devices() ([]*device.T, error) {
	l := make([]*device.T, 0)
	return l, nil
}

func (t T) UUID() string {
	return t.uuid
}

func (t T) Deactivate() error {
	if t.name == "" {
		return fmt.Errorf("name is required")
	}
	args := []string{"--stop", t.devpathFromName()}
	cmd := command.New(
		command.WithName(mdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	fcache.Clear("mdadm-E-scan-v")
	switch cmd.ExitCode() {
	case 0:
		// ok
	default:
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t T) Activate() error {
	if t.name == "" {
		return fmt.Errorf("name is required")
	}
	args := []string{"--assemble", t.devpathFromName(), "-u", t.uuid}
	cmd := command.New(
		command.WithName(mdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	fcache.Clear("mdadm-E-scan-v")
	switch cmd.ExitCode() {
	case 0:
		// ok
	case 2:
		t.log.Info().Msg("no changes were made to the array")
	default:
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t T) validateName() error {
	if t.name == "" {
		return fmt.Errorf("name is required")
	}
	if len(t.name) > 32 {
		return fmt.Errorf("device md name is too long, 32 chars max (name is %s)", t.name)
	}
	return nil
}

func (t T) devpathFromUUID() string {
	return "/dev/disk/by-id/md-uuid-" + t.uuid
}

func (t T) devpathFromName() string {
	return "/dev/md/" + t.name
}

func (t T) Create(level string, devs []string, spares int, layout string, chunk int64) error {
	args := []string{"--create", t.devpathFromName()}
	dataDevsCount := len(devs) - spares
	if dataDevsCount < 1 {
		return fmt.Errorf("at least 1 device must be set in the 'devs' provisioning")
	}
	if err := t.validateName(); err != nil {
		return err
	}
	cmd := command.New(
		command.WithName(mdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	fcache.Clear("mdadm-E-scan-v")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	// TODO: set t.uuid
	return nil
}

func mdadmConfigFile() string {
	if mdadmConfigFileCache == "" {
		if file.Exists("/etc/mdadm") {
			mdadmConfigFileCache = "/etc/mdadm/mdadm.conf"
		} else {
			mdadmConfigFileCache = "/etc/mdadm.conf"
		}
	}
	return mdadmConfigFileCache
}

func (t T) IsAutoActivated() bool {
	if t.uuid == "" {
		return false
	}
	cf := mdadmConfigFile()
	if !file.Exists(cf) {
		return true
	}
	if f, err := os.Open(cf); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			words := strings.Fields(line)
			if len(words) < 2 {
				continue
			}
			if words[0] == "AUTO" && stringslice.Has("-all", words) {
				return false
			}
			if words[0] == "ARRAY" && words[1] == "<ignore>" && stringslice.Has("UUID="+t.uuid, words) {
				return false
			}
		}
	}
	return true
}

func (t T) DisableAutoActivation() error {
	if t.uuid == "" {
		return nil
	}
	if !t.IsAutoActivated() {
		return nil
	}
	cf := mdadmConfigFile()
	t.log.Info().Msgf("disable auto-assemble in %s", cf)
	f, err := os.OpenFile(cf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.WriteString(fmt.Sprintf("ARRAY <ignore> UUID=%s\n", t.uuid)); err != nil {
		return err
	}
	return nil
}
