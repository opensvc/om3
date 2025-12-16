package zfs

import (
	"bufio"
	"bytes"
	"context"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/funcopt"
)

type (
	PoolStatusData struct {
		State  string
		Errors string
		VDevs  poolStatusVDevs
	}
	poolStatusOpts struct {
		Verbose bool
	}
	poolStatusVDevs []poolStatusVDev
	poolStatusVDev  struct {
		Path  string
		Read  uint64
		Write uint64
		Cksum uint64
		Slow  uint64
	}
)

// PoolStatusWithVerbose displays verbose data error information, printing out a complete list of all data errors since the last complete pool scrub.
func PoolStatusWithVerbose() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolStatusOpts)
		t.Verbose = true
		return nil
	})
}

func poolStatusOptsToArgs(t poolStatusOpts) []string {
	l := make([]string, 0)
	l = append(l, "status", "-PL")
	if t.Verbose {
		l = append(l, "-v")
	}
	return l
}

func (t poolStatusVDevs) Paths() []string {
	l := make([]string, 0)
	for _, vd := range t {
		l = append(l, vd.Path)
	}
	return l
}

func parsePoolStatus(b []byte) PoolStatusData {
	data := PoolStatusData{}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	inConfig := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !inConfig {
			if strings.HasPrefix(line, "state:") {
				l := strings.SplitN(line, ":", 2)
				data.State = strings.TrimSpace(l[1])
				continue
			}
			if strings.HasPrefix(line, "errors:") {
				l := strings.SplitN(line, ":", 2)
				data.Errors = strings.TrimSpace(l[1])
				continue
			}
			if !inConfig && strings.HasPrefix(line, "config:") {
				inConfig = true
				continue
			}
		} else {
			if strings.HasPrefix(line, "/dev/") {
				words := strings.Fields(line)
				vd := poolStatusVDev{}
				n := len(words)
				switch {
				case n >= 4:
					vd.Path = words[0]
					if i, err := strconv.ParseUint(words[1], 10, 64); err == nil {
						vd.Read = i
					}
					if i, err := strconv.ParseUint(words[2], 10, 64); err == nil {
						vd.Write = i
					}
					if i, err := strconv.ParseUint(words[3], 10, 64); err == nil {
						vd.Cksum = i
					}
					fallthrough
				case n >= 5:
					if i, err := strconv.ParseUint(words[4], 10, 64); err == nil {
						vd.Slow = i
					}
				}
				if vd.Path != "" {
					data.VDevs = append(data.VDevs, vd)
				}
			}
		}
	}
	return data
}

func (t *Pool) Status(ctx context.Context, fopts ...funcopt.O) (PoolStatusData, error) {
	opts := &poolStatusOpts{}
	funcopt.Apply(opts, fopts...)
	args := append(poolStatusOptsToArgs(*opts), t.Name)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("zpool"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	b, err := cmd.Output()
	if err != nil {
		return PoolStatusData{}, err
	}
	return parsePoolStatus(b), nil
}

func (t *Pool) VDevPaths(ctx context.Context) ([]string, error) {
	if status, err := t.Status(ctx); err != nil {
		return nil, err
	} else {
		return status.VDevs.Paths(), nil
	}
}

func (t *Pool) VDevDevices(ctx context.Context) (device.L, error) {
	paths, err := t.VDevPaths(ctx)
	if err != nil {
		return device.L{}, err
	}
	l := make(device.L, len(paths))
	for i, path := range paths {
		l[i] = device.New(path)
	}
	return l, nil
}
