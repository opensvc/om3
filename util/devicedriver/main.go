package devicedriver

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/rs/zerolog"
)

type (
	device interface {
		Path() string
	}
	setLogger interface {
		SetLog(*plog.Logger)
	}
	logT struct {
		log *plog.Logger
	}
	Loop struct {
		logT
	}
	DeviceMapper struct {
		logT
	}
)

var (
	procDevicesCache map[uint32]string
)

func (t *logT) SetLog(log *plog.Logger) {
	t.log = log
}

func WithLogger(log *plog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(setLogger)
		t.SetLog(log)
		return nil
	})
}

func NewFromMajor(major uint32, opts ...funcopt.O) interface{} {
	if procDevicesCache == nil {
		procDevicesCache = ProcDevices()
	}
	if name, ok := procDevicesCache[major]; !ok {
		return nil
	} else {
		return NewFromName(name)
	}
}

func NewFromName(name string, opts ...funcopt.O) interface{} {
	switch name {
	case "loop":
		t := NewLoop()
		_ = funcopt.Apply(t, opts...)
		return t
	case "device-mapper":
		t := NewDeviceMapper()
		_ = funcopt.Apply(t, opts...)
		return t
	default:
		return nil
	}
}

func DriverMajors(s string) []uint32 {
	l := make([]uint32, 0)
	for i, n := range ProcDevices() {
		if n == s {
			l = append(l, i)
		}
	}
	return l
}

func ProcDevices() map[uint32]string {
	m := make(map[uint32]string)
	f, err := os.Open("/proc/devices")
	if err != nil {
		return m
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		l := strings.Fields(scanner.Text())
		if len(l) != 2 {
			continue
		}
		major, err := strconv.ParseUint(l[0], 10, 32)
		if err != nil {
			continue
		}
		m[uint32(major)] = l[1]
	}
	return m
}

func NewLoop() *Loop {
	t := Loop{}
	return &t
}

func (t Loop) Remove(dev device) error {
	cmd := command.New(
		command.WithName("losetup"),
		command.WithVarArgs("-d", dev.Path()),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	exitCode := cmd.ExitCode()
	if exitCode != 0 {
		return fmt.Errorf("%s returned %d", cmd, exitCode)
	}
	return nil
}

func NewDeviceMapper() *DeviceMapper {
	t := DeviceMapper{}
	return &t
}

func (t DeviceMapper) Remove(dev device) error {
	panic("not implemented")
}
