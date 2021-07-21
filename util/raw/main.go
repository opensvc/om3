package raw

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/device"
	"opensvc.com/opensvc/util/devicedriver"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/funcopt"
)

const (
	raw string = "raw"
)

var (
	regexpQueryLine = regexp.MustCompile(`/dev/raw/raw([0-9]+):  bound to major ([0-9]+), minor ([0-9]+)`)
	ErrExist        = errors.New("raw device is already bound")
)

type (
	T struct {
		log *zerolog.Logger
	}
	Entry struct {
		Index     int
		BDevMajor int
		BDevMinor int
	}
	Entries []Entry
)

var (
	probed bool = false
)

func CDevPath(i int) string {
	return fmt.Sprintf("/dev/raw/raw%d", i)
}

func (t Entry) CDevPath() string {
	return CDevPath(t.Index)
}

func (t Entry) BDevPath() string {
	sys := fmt.Sprintf("/sys/dev/block/%d:%d", t.BDevMajor, t.BDevMinor)
	p, err := os.Readlink(sys)
	if err != nil {
		return ""
	}
	return "/dev/" + filepath.Base(p)
}

func New(opts ...funcopt.O) *T {
	t := T{}
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

func (t T) modprobe() error {
	if probed {
		return nil
	}
	if file.ExistsAndDir("/sys/class/raw") {
		probed = true
		return nil
	}
	cmd := command.New(
		command.WithName("modprobe"),
		command.WithVarArgs("raw"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
	)
	probed = true
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func RawMajor() int {
	l := devicedriver.DriverMajors("raw")
	if len(l) == 0 {
		return 0
	}
	return int(l[0])
}

//
// NextMinor returns the next available raw device driver free minor.
// It must be called from a locked code section.
//
func (t T) NextMinor() int {
	data, err := t.Data()
	if err != nil {
		return 0
	}
	return data.NextMinor()
}

func (t Entries) NextMinor() int {
	for i := 1; i < 2^20; i++ {
		if !t.HasIndex(i) {
			return i
		}
	}
	return 0
}

func (t T) Data() (Entries, error) {
	data := make(Entries, 0)
	if err := t.modprobe(); err != nil {
		return data, err
	}
	cmd := command.New(
		command.WithName(raw),
		command.WithVarArgs("-qa"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
		command.WithEnv([]string{"LANG=C"}),
	)
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(bytes.NewReader(cmd.Stdout()))
	for sc.Scan() {
		subs := regexpQueryLine.FindStringSubmatch(sc.Text())
		if len(subs) != 4 {
			continue
		}
		i, err := strconv.Atoi(subs[1])
		if err != nil {
			continue
		}
		major, err := strconv.Atoi(subs[2])
		if err != nil {
			continue
		}
		minor, err := strconv.Atoi(subs[3])
		if err != nil {
			continue
		}
		data = append(data, Entry{
			Index:     i,
			BDevMajor: major,
			BDevMinor: minor,
		})
	}
	return data, nil
}

func (t T) Has(bDevPath string) (bool, error) {
	data, err := t.Data()
	if err != nil {
		return false, err
	}
	if e := data.BDevPath(bDevPath); e != nil {
		return true, nil
	}
	return false, nil
}

func (t T) Bind(bDevPath string) (int, error) {
	p := "/var/lock/opensvc.raw.lock"
	lock := flock.New(p, "", fcntllock.New)
	timeout, err := time.ParseDuration("20s")
	if err != nil {
		return 0, err
	}
	err = lock.Lock(timeout, "")
	if err != nil {
		return 0, err
	}
	defer func() { _ = lock.UnLock() }()
	return t.lockedBind(bDevPath)
}

func (t T) lockedBind(bDevPath string) (int, error) {
	data, err := t.Data()
	if err != nil {
		return 0, err
	}
	if e := data.BDevPath(bDevPath); e != nil {
		return e.Index, errors.Wrapf(ErrExist, "%s -> %s", bDevPath, e.CDevPath())
	}
	m := data.NextMinor()
	if m == 0 {
		return 0, fmt.Errorf("unable to allocate a free minor")
	}
	cDevPath := fmt.Sprintf("/dev/raw/raw%d", m)
	cmd := command.New(
		command.WithName(raw),
		command.WithVarArgs(cDevPath, bDevPath),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	err = cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("%s Run error %v", cmd, err)
	}
	if cmd.ExitCode() != 0 {
		return 0, fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return m, nil
}

func (t T) UnbindBDevPath(bDevPath string) error {
	data, err := t.Data()
	if err != nil {
		return err
	}
	e := data.BDevPath(bDevPath)
	if e == nil {
		t.log.Info().Msgf("%s already unbound from its raw device", bDevPath)
		return nil
	}
	cDevPath := e.CDevPath()
	return t.Unbind(cDevPath)
}

func (t T) UnbindMinor(minor int) error {
	cDevPath := CDevPath(minor)
	return t.Unbind(cDevPath)
}

func (t T) Unbind(cDevPath string) error {
	cmd := command.New(
		command.WithName(raw),
		command.WithVarArgs(cDevPath, "0", "0"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("%s Run error %v", cmd, err)
	}
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil

}

func (t Entries) BDev(major, minor int) *Entry {
	for _, e := range t {
		if e.BDevMajor == major && e.BDevMinor == minor {
			return &e
		}
	}
	return nil
}

func (t Entries) Index(i int) *Entry {
	for _, e := range t {
		if e.Index == i {
			return &e
		}
	}
	return nil
}

func (t Entries) CDevPath(s string) *Entry {
	for _, e := range t {
		if e.CDevPath() == s {
			return &e
		}
	}
	return nil
}

func (t Entries) BDevPath(s string) *Entry {
	dev := device.New(s)
	major, err := dev.Major()
	if err != nil {
		return nil
	}
	minor, err := dev.Minor()
	if err != nil {
		return nil
	}
	return t.BDev(int(major), int(minor))
}

func (t Entries) HasBDevPath(s string) bool {
	e := t.CDevPath(s)
	return e != nil
}

func (t Entries) HasBDev(major, minor int) bool {
	return t.BDev(major, minor) != nil
}

func (t Entries) HasCDevPath(s string) bool {
	return t.CDevPath(s) != nil
}

func (t Entries) HasIndex(i int) bool {
	return t.Index(i) != nil
}
