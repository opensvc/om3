package zfs

import (
	"bufio"
	"bytes"
	"strings"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	PoolStatusData struct {
		State  string
		Errors string
	}
	poolStatusOpts struct {
		Verbose bool
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
	l = append(l, "status")
	if t.Verbose {
		l = append(l, "-v")
	}
	return l
}

func parsePoolStatus(b []byte) PoolStatusData {
	data := PoolStatusData{}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "state:") {
			l := strings.SplitN(line, ":", 2)
			data.State = strings.TrimSpace(l[1])
		}
		if strings.HasPrefix(line, "errors:") {
			l := strings.SplitN(line, ":", 2)
			data.Errors = strings.TrimSpace(l[1])
		}
	}
	return data
}

func (t *Pool) Status(fopts ...funcopt.O) (PoolStatusData, error) {
	opts := &poolStatusOpts{}
	funcopt.Apply(opts, fopts...)
	args := append(poolStatusOptsToArgs(*opts), t.Name)
	cmd := command.New(
		command.WithName("zpool"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
	)
	b, err := cmd.Output()
	if err != nil {
		return PoolStatusData{}, err
	}
	return parsePoolStatus(b), nil
}
