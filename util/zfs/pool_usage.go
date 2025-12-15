package zfs

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
)

type (
	// PoolUsage represents a parsed line of the df unix command
	PoolUsage struct {
		Size  int64
		Alloc int64
		Free  int64
	}
)

func parsePoolUsage(b []byte) (PoolUsage, error) {
	data := PoolUsage{}
	lines := strings.Split(string(b), "\n")
	if len(lines) != 3 {
		return data, fmt.Errorf("unexpected 'zpool get -H size,alloc,free' output: %s", string(b))
	}
	parseLine := func(line string) (int64, error) {
		l := strings.Fields(line)
		if len(l) < 3 {
			return 0, fmt.Errorf("unexpected number of elements in line: %s", line)
		}
		return strconv.ParseInt(l[2], 10, 64)
	}

	if i, err := parseLine(lines[0]); err == nil {
		data.Size = i
	} else {
		return PoolUsage{}, err
	}
	if i, err := parseLine(lines[1]); err == nil {
		data.Alloc = i
	} else {
		return PoolUsage{}, err
	}
	if i, err := parseLine(lines[2]); err == nil {
		data.Free = i
	} else {
		return PoolUsage{}, err
	}

	return data, nil
}

func (t *Pool) Usage(ctx context.Context, fopts ...funcopt.O) (PoolUsage, error) {
	opts := &poolStatusOpts{}
	funcopt.Apply(opts, fopts...)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("zpool"),
		command.WithVarArgs("get", "-H", "size,alloc,free", "-p", t.Name),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	b, err := cmd.Output()
	if err != nil {
		return PoolUsage{}, err
	}
	return parsePoolUsage(b)
}
