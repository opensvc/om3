package md

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/fcache"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	T struct {
		name string
		uuid string
		log  *plog.Logger
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
func WithLogger(log *plog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.log = log
		return nil
	})
}

func (t T) detailState(ctx context.Context) (string, error) {
	buff, err := t.detail(ctx)
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

func (t T) detailUUID(ctx context.Context) (string, error) {
	buff, err := t.detail(ctx)
	if err != nil {
		return "", nil
	}
	scanner := bufio.NewScanner(strings.NewReader(buff))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "UUID :") {
			continue
		}
		l := strings.SplitN(line, " : ", 2)
		if len(l) != 2 {
			return "", fmt.Errorf("md uuid unexpected format in details: %s", line)
		}
		return l[1], nil
	}
	return "", fmt.Errorf("md uuid not found in details")
}

func (t T) detail(ctx context.Context) (string, error) {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(mdadm),
		command.WithVarArgs("--detail", t.devpathFromName()),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
		command.WithBufferedStdout(),
	)
	if out, err := cmd.Output(); err != nil {
		return "", err
	} else {
		return string(out), nil
	}
}

func (t T) examineScanVerbose(ctx context.Context) (string, error) {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(mdadm),
		command.WithVarArgs("-E", "--scan", "-v"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
		command.WithBufferedStdout(),
	)
	if out, err := fcache.Output(cmd, "mdadm-E-scan-v"); err != nil {
		return "", err
	} else {
		return string(out), nil
	}
}

func (t T) Resync(ctx context.Context) error {
	buff, err := t.detail(ctx)
	if err != nil {
		return err
	}
	added := 0
	removed := strings.Count(buff, "removed")
	if removed == 0 {
		t.log.Infof("skip: no removed device")
		return nil
	}
	if !strings.Contains(buff, "Raid Level : raid1") {
		t.log.Infof("skip: non-raid1 md")
		return nil
	}
	v, _, err := t.IsActive(ctx)
	if err != nil {
		return err
	}
	if !v {
		t.log.Infof("skip: non-assembed md")
		return nil
	}
	scanner := bufio.NewScanner(strings.NewReader(buff))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "faulty") {
			l := strings.Fields(line)
			faultyDev := l[len(l)-1]
			if err := t.reAdd(ctx, faultyDev); err != nil {
				return err
			}
			added = added + 1
		}
	}
	if removed > added {
		return fmt.Errorf("no faulty device found to re-add to %s remaining %d removed legs", t.devpathFromUUID(), removed-added)
	}
	return nil
}

func (t T) Wipe(ctx context.Context) error {
	devs, err := t.Devices(ctx)
	if err != nil {
		return err
	}
	for _, d := range devs {
		if err := t.wipeDevice(ctx, d.Path()); err != nil {
			return err
		}
	}
	return nil
}

func (t T) Remove(ctx context.Context) error {
	return nil
}

func (t T) IsActive(ctx context.Context) (bool, string, error) {
	if v, err := t.Exists(ctx); !v {
		return false, "", err
	}
	s, err := t.detailState(ctx)
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
			t.log.Tracef("status eval'ed down because: %s", s)
			return false, msg, nil
		}
	}
	return true, msg, nil
}

func (t T) Exists(ctx context.Context) (bool, error) {
	buff, err := t.examineScanVerbose(ctx)
	if err != nil {
		return false, err
	}
	if t.uuid != "" && strings.Contains(buff, "UUID="+t.uuid) {
		return true, nil
	}
	if t.name != "" && strings.Contains(buff, t.devpathFromName()+" ") {
		return true, nil
	}
	return false, nil
}

func (t T) Devices(ctx context.Context) (device.L, error) {
	l := make(device.L, 0)
	if t.uuid == "" {
		return l, nil
	}
	buff, err := t.examineScanVerbose(ctx)
	if err != nil {
		return l, nil
	}
	scanner := bufio.NewScanner(strings.NewReader(buff))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "UUID="+t.uuid) {
			scanner.Scan()
			line := strings.TrimSpace(scanner.Text())
			words := strings.SplitN(line, "devices=", 2)
			if len(words) != 2 {
				break
			}
			for _, d := range strings.Split(words[1], ",") {
				l = append(l, device.New(d, device.WithLogger(t.log)))
			}
			break
		}
	}
	// The `mdadm -E --scan -v` command can return a list with duplicates,
	// like /dev/mpath0 /dev/sda /dev/sdb, where sda and sdb are paths of mpath0.
	// In this case we want to return only the top holders of the list.
	return l.HolderEndpoints()
}

func (t T) UUID() string {
	return t.uuid
}

func (t T) reAdd(ctx context.Context, devpath string) error {
	args := []string{"--re-add", t.devpathFromUUID(), devpath}
	cmd := command.New(
		command.WithContext(ctx),
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
		return fmt.Errorf("failed to re-add %s to %s", devpath, t.devpathFromUUID())
	}
	return nil
}

func (t T) wipeDevice(ctx context.Context, devpath string) error {
	args := []string{"--brief", "--zero-superblock", devpath}
	cmd := command.New(
		command.WithContext(ctx),
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

func (t T) Deactivate(ctx context.Context) error {
	if t.name == "" {
		return fmt.Errorf("name is required")
	}
	filterStderrFunc := func(s string) {
		switch {
		case strings.Contains(s, "stopped"):
			t.log.Infof(s)
			return
		}
		t.log.Errorf(s)
	}
	args := []string{"--stop", t.devpathFromName()}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(mdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithOnStderrLine(filterStderrFunc),
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

func (t T) Activate(ctx context.Context) error {
	if t.name == "" {
		return fmt.Errorf("name is required")
	}
	filterStderrFunc := func(s string) {
		switch {
		case strings.Contains(s, "has been started"):
			t.log.Infof(s)
			return
		}
		t.log.Errorf(s)
	}
	args := []string{"--assemble", t.devpathFromName(), "-u", t.uuid}
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(mdadm),
		command.WithArgs(args),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithOnStderrLine(filterStderrFunc),
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

func (t *T) Create(ctx context.Context, level string, devs []string, spares int, layout string, chunk *int64, bitmap string) error {
	dataDevsCount := len(devs) - spares
	if dataDevsCount < 1 {
		return fmt.Errorf("at least 1 device must be set in the 'devs' provisioning")
	}
	if err := t.validateName(); err != nil {
		return err
	}
	args := []string{"--create", t.devpathFromName(), "--force", "--quiet", "--metadata=default", "-n", strconv.Itoa(dataDevsCount)}
	if level != "" {
		args = append(args, "-l", level)
	}
	if spares > 0 {
		args = append(args, "-x", strconv.Itoa(spares))
	}
	if chunk != nil && *chunk > 0 {
		// convert to kb and round to the greater multiple of 4
		n := int(math.Round(float64(*chunk)/1024/4)) * 4
		c := strconv.Itoa(n)
		args = append(args, "-c", c)
	}
	if layout != "" {
		args = append(args, "-p", layout)
	}
	if bitmap == "none" || bitmap == "internal" {
		args = append(args, "--bitmap="+bitmap)
	}
	args = append(args, devs...)

	cmd := command.New(
		command.WithContext(ctx),
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
	if uuid, err := t.detailUUID(ctx); err != nil {
		return err
	} else {
		t.uuid = uuid
	}
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
			if words[0] == "AUTO" && slices.Contains(words, "-all") {
				return false
			}
			if words[0] == "ARRAY" && words[1] == "<ignore>" && slices.Contains(words, "UUID="+t.uuid) {
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
	t.log.Infof("disable auto-assemble in %s", cf)
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
