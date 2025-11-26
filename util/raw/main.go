package raw

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"
	"github.com/rs/zerolog"

	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/devicedriver"
	"github.com/opensvc/om3/util/fcache"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/plog"
)

const (
	raw string = "raw"
)

var (
	regexpQueryLine = regexp.MustCompile(`/dev/raw/raw([0-9]+): {2}bound to major ([0-9]+), minor ([0-9]+)`)
	ErrExist        = errors.New("the raw device is already bound")
)

type (
	// T holds the actions for raw device
	T struct {
		log *plog.Logger
	}

	// Bind hold a raw bind detail
	Bind struct {
		Index     int
		BDevMajor int
		BDevMinor int
	}

	// Binds is a list of Bind
	Binds []Bind
)

var (
	probed bool = false
)

// cDevPath returns raw device path with Index 'i'
func cDevPath(i int) string {
	return fmt.Sprintf("/dev/raw/raw%d", i)
}

// CDevPath returns raw device path for a bind entry
func (b Bind) CDevPath() string {
	return cDevPath(b.Index)
}

// BDevPath returns block device path associated with a bind entry
func (b Bind) BDevPath() string {
	sys := fmt.Sprintf("/sys/dev/block/%d:%d", b.BDevMajor, b.BDevMinor)
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

func WithLogger(log *plog.Logger) funcopt.O {
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
	if v, err := file.ExistsAndDir("/sys/class/raw"); err != nil {
		return err
	} else if v {
		probed = true
		return nil
	}
	cmd := command.New(
		command.WithName("modprobe"),
		command.WithVarArgs("raw"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	probed = true
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func DriverMajor() int {
	l := devicedriver.DriverMajors("raw")
	if len(l) == 0 {
		return 0
	}
	return int(l[0])
}

// nextMinor returns the next available raw device driver free minor.
//
// It must be called from a locked code section.
func (t T) nextMinor() int {
	binds, err := t.QueryAll()
	if err != nil {
		return 0
	}
	return binds.nextMinor()
}

func (bl Binds) nextMinor() int {
	for i := 1; i < 2^20; i++ {
		if !bl.HasIndex(i) {
			return i
		}
	}
	return 0
}

// QueryAll returns list of current binds
func (t T) QueryAll() (Binds, error) {
	var (
		out []byte
		err error
	)
	data := make(Binds, 0)
	if err := t.modprobe(); err != nil {
		return data, err
	}
	cmd := command.New(
		command.WithName(raw),
		command.WithVarArgs("-qa"),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
		command.WithBufferedStdout(),
		command.WithEnv([]string{"LANG=C"}),
	)
	if out, err = fcache.Output(cmd, "raw"); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
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
		data = append(data, Bind{
			Index:     i,
			BDevMajor: major,
			BDevMinor: minor,
		})
	}
	return data, nil
}

// Find returns bind entry handled by raw if current raw devices handle block device 'path'
func (t T) Find(path string) (*Bind, error) {
	binds, err := t.QueryAll()
	if err != nil {
		return nil, err
	}
	return binds.FromBDevPath(path), nil
}

// HasBlockDev returns true if current raw devices handle block device 'path'
func (t T) HasBlockDev(path string) (bool, error) {
	bind, err := t.Find(path)
	if err != nil {
		return false, err
	}
	return bind != nil, nil
}

// Bind create a new raw device for block dev 'bDevPath'
//
// it returns device minor of created raw device
func (t T) Bind(bDevPath string) (int, error) {
	p := "/var/lock/opensvc.raw.lock"
	lock := flock.New(p, "", fcntllock.New)
	if err := lock.Lock(20*time.Second, ""); err != nil {
		return 0, err
	}
	defer func() { _ = lock.UnLock() }()
	return t.lockedBind(bDevPath)
}

func (t T) lockedBind(bDevPath string) (int, error) {
	data, err := t.QueryAll()
	if err != nil {
		return 0, err
	}
	if e := data.FromBDevPath(bDevPath); e != nil {
		return e.Index, fmt.Errorf("%w: %s -> %s", ErrExist, bDevPath, e.CDevPath())
	}
	m := data.nextMinor()
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
	fcache.Clear("raw")
	if err != nil {
		return 0, fmt.Errorf("%s run error %v", cmd, err)
	}
	if cmd.ExitCode() != 0 {
		return 0, fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return m, nil
}

// UnbindBDevPath unbind raw device associated with block device path 'bDevPath'
//
// It return nil if succeed or if no raw device for block 'bDevPath'
func (t T) UnbindBDevPath(bDevPath string) error {
	binds, err := t.QueryAll()
	if err != nil {
		return err
	}
	b := binds.FromBDevPath(bDevPath)
	if b == nil {
		t.log.Infof("%s already unbound from its raw device", bDevPath)
		return nil
	}
	cDevPath := b.CDevPath()
	return t.Unbind(cDevPath)
}

// UnbindMinor unbind raw device with 'minor'
func (t T) UnbindMinor(minor int) error {
	cDevPath := cDevPath(minor)
	return t.Unbind(cDevPath)
}

// Unbind unbinds raw device 'cDevPath'
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
	fcache.Clear("raw")
	if err != nil {
		return fmt.Errorf("%s run error %v", cmd, err)
	}
	if cmd.ExitCode() != 0 {
		return fmt.Errorf("%s error %d", cmd, cmd.ExitCode())
	}
	return nil

}

// BDev returns pointer to bind entry that match 'major' and 'minor' or returns nil
func (bl Binds) BDev(major, minor int) *Bind {
	for _, e := range bl {
		if e.BDevMajor == major && e.BDevMinor == minor {
			return &e
		}
	}
	return nil
}

func (bl Binds) Index(i int) *Bind {
	for _, e := range bl {
		if e.Index == i {
			return &e
		}
	}
	return nil
}

// FromCDevPath returns pointer to bind entry matching raw dev path 's' or returns nil
func (bl Binds) FromCDevPath(s string) *Bind {
	for _, b := range bl {
		if b.CDevPath() == s {
			return &b
		}
	}
	return nil
}

// FromBDevPath returns pointer to the bind entry that match block dev path 's' or returns nil
func (bl Binds) FromBDevPath(s string) *Bind {
	dev := device.New(s)
	major, err := dev.Major()
	if err != nil {
		return nil
	}
	minor, err := dev.Minor()
	if err != nil {
		return nil
	}
	return bl.BDev(int(major), int(minor))
}

// HasBDevPath returns true if a raw device is bound to block device 's'
func (bl Binds) HasBDevPath(s string) bool {
	return bl.FromBDevPath(s) != nil
}

// HasBDevMajorMinor returns true if a raw device is bound to block device 'major' and 'minor'
func (bl Binds) HasBDevMajorMinor(major, minor int) bool {
	return bl.BDev(major, minor) != nil
}

// HasCDevPath returns true if
func (bl Binds) HasCDevPath(s string) bool {
	return bl.FromCDevPath(s) != nil
}

func (bl Binds) HasIndex(i int) bool {
	return bl.Index(i) != nil
}
