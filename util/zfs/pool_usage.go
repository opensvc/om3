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
	// PoolUsage is the space of a pool, as what is stored in it is told.
	PoolUsage struct {
		Size  int64
		Alloc int64
		Free  int64
	}
)

// Usable is what the datasets of the pool can hold, the size less what zfs
// keeps for itself: what the used and free of the pool are parts of.
func (t PoolUsage) Usable() int64 {
	return t.Alloc + t.Free
}

// parsePoolUsage reads the output of zfs get -Hp used,available.
func parsePoolUsage(b []byte) (PoolUsage, error) {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		return PoolUsage{}, fmt.Errorf("unexpected 'zfs get -Hp used,available' output: %s", string(b))
	}
	values := make([]int64, len(lines))
	for i, line := range lines {
		l := strings.Fields(line)
		if len(l) < 3 {
			return PoolUsage{}, fmt.Errorf("unexpected number of elements in line: %s", line)
		}
		v, err := strconv.ParseInt(l[2], 10, 64)
		if err != nil {
			return PoolUsage{}, err
		}
		values[i] = v
	}
	return PoolUsage{Alloc: values[0], Free: values[1]}, nil
}

// Usage returns the size of the pool, and what is used and free of it as its
// datasets see it, which is the question its callers ask: can a volume of a
// size be created there.
//
// What zpool get tells of them is another thing, the blocks written. A zvol
// reserves its whole size when created, and that reservation is no block of
// the pool until it is written: zpool says free the space no zfs create can
// have. The used of the root dataset counts the reservations, and available is
// what is left of them. Their sum is the size less what zfs keeps for itself,
// which is why Size stays what zpool says: a resize compares it to a device.
func (t *Pool) Usage(ctx context.Context, fopts ...funcopt.O) (PoolUsage, error) {
	opts := &poolStatusOpts{}
	funcopt.Apply(opts, fopts...)
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("zfs"),
		command.WithVarArgs("get", "-Hp", "used,available", t.Name),
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
	usage, err := parsePoolUsage(b)
	if err != nil {
		return PoolUsage{}, err
	}
	size, err := t.GetProperty("size")
	if err != nil {
		return PoolUsage{}, err
	}
	if usage.Size, err = strconv.ParseInt(size, 10, 64); err != nil {
		return PoolUsage{}, fmt.Errorf("zpool size %q: %w", size, err)
	}
	return usage, nil
}

// ExpandDevice makes the pool take the space a device of it has gained.
//
// A pool is not given a size: it is what its vdevs hold, less what zfs keeps
// for its labels and its reserve. So growing one is telling it to look at a
// device again, the way a filesystem is told to take up the device under it.
func (t *Pool) ExpandDevice(ctx context.Context, devpath string) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("zpool"),
		command.WithVarArgs("online", "-e", t.Name, devpath),
		command.WithLogger(t.Log),
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
