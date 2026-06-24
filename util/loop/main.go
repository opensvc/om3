package loop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/sessioncache"
	"github.com/opensvc/om3/v3/util/udevadm"
)

const (
	losetup string = "losetup"
)

type (
	T struct {
		log *plog.Logger
	}

	Info struct {
		LoopDevices []InfoEntry `json:"loopdevices"`
	}

	InfoEntry struct {
		Name     string `json:"name"`      // "/dev/loop1"
		BackFile string `json:"back-file"` // "/var/lib/snapd/snaps/gnome-3-34-1804_66.snap"
	}

	InfoEntries []InfoEntry
)

func New(opts ...funcopt.O) *T {
	t := T{}
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

func (t T) FileExists(ctx context.Context, filePath string) (bool, error) {
	data, err := t.Data(ctx)
	if err != nil {
		return false, err
	}
	return data.HasFile(filePath), nil
}

func (t T) FileDelete(ctx context.Context, filePath string) error {
	i, err := t.FileGet(ctx, filePath)
	if err != nil {
		return err
	}
	if i == nil {
		return nil
	}
	return t.Delete(ctx, i.Name)
}

func (t T) FileGet(ctx context.Context, filePath string) (*InfoEntry, error) {
	data, err := t.Data(ctx)
	if err != nil {
		return nil, err
	}
	return data.File(filePath), nil
}

func (t T) Get(ctx context.Context, name string) (*InfoEntry, error) {
	data, err := t.Data(ctx)
	if err != nil {
		return nil, err
	}
	return data.Name(name), nil
}

func (t T) Data(ctx context.Context) (InfoEntries, error) {
	var (
		out     []byte
		err     error
		entries InfoEntries
	)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(losetup),
		command.WithVarArgs("-O", "NAME,BACK-FILE"), // The -J and -n options are not supported by losetup on RHEL 7.
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
		command.WithBufferedStdout(),
	)
	if out, err = sessioncache.Output(cmd, "losetup"); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return InfoEntries{}, nil
	}
	/*
		Output to parse: losetup -O NAME,BACK-FILE
		NAME       BACK-FILE
		/dev/loop0 /tmp/loopfile0 (deleted)
		/dev/loop1 /tmp/loopfile1
	*/
	for line := range strings.Lines(string(out)) {
		l := strings.Fields(line)
		if len(l) >= 2 {
			if l[0] == "NAME" {
				continue
			}
			entries = append(entries, InfoEntry{
				Name:     l[0],
				BackFile: l[1],
			})
		}
	}
	return entries, nil
}

func (t T) Add(ctx context.Context, filePath string) error {
	p := "/var/lock/opensvc.losetup.lock"
	lock := flock.New(p, "", fcntllock.New)
	if err := lock.Lock(20*time.Second, ""); err != nil {
		return err
	}
	defer func() { _ = lock.UnLock() }()
	return t.lockedAdd(ctx, filePath)
}

func (t T) lockedAdd(ctx context.Context, filePath string) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(losetup),
		command.WithVarArgs("-f", filePath),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	sessioncache.Clear("losetup")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil
}

func (t T) Delete(ctx context.Context, devPath string) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(losetup),
		command.WithVarArgs("-d", devPath),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	cmd.Run()
	sessioncache.Clear("losetup")
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	udevadm.Settle()
	limit := time.Now().Add(5 * time.Second)
	for {
		info, _ := t.Get(ctx, devPath)
		if info == nil {
			return nil
		}
		if time.Now().After(limit) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("losetup silently failed to delete %s", devPath)
}

func (t InfoEntries) Name(s string) *InfoEntry {
	for _, i := range t {
		if i.Name == s {
			return &i
		}
	}
	return nil
}

func (t InfoEntries) File(s string) *InfoEntry {
	for _, i := range t {
		if i.BackFile == s {
			return &i
		}
	}
	return nil
}

func (t InfoEntries) HasFile(s string) bool {
	return t.File(s) != nil
}
