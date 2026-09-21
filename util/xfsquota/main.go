// Package xfsquota sets and reads the project quotas an xfs filesystem can put
// on a directory tree.
//
// A project is a number the filesystem stamps on a directory and everything
// under it. A block limit on that number is what bounds the tree, and what df
// reports for a path inside it, so a consumer sees the size it was given
// rather than the size of the filesystem holding it.
package xfsquota

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/findmnt"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	T struct {
		// mountPoint is the filesystem holding the trees, because a project
		// and its limit belong to a filesystem, not to a directory.
		mountPoint string
		log        *plog.Logger
	}
)

const xfsQuota = "xfs_quota"

func New(mountPoint string, opts ...funcopt.O) *T {
	t := T{mountPoint: mountPoint}
	_ = funcopt.Apply(&t, opts...)
	return &t
}

func WithLogger(log *plog.Logger) funcopt.O {
	return funcopt.F(func(i any) error {
		t := i.(*T)
		t.log = log
		return nil
	})
}

func (t *T) Log() *plog.Logger {
	return t.log
}

// CanHoldProjectQuota says whether a mount can hold project quotas.
//
// The filesystem has to be xfs and to have been mounted for it, which is the
// node's fstab rather than anything an object owns. Saying so plainly is
// better than a limit that is set and silently does nothing.
func CanHoldProjectQuota(mi findmnt.MountInfo) bool {
	if mi.FsType != "xfs" {
		return false
	}
	for _, opt := range strings.Split(mi.Options, ",") {
		switch opt {
		case "prjquota", "pquota":
			return true
		}
	}
	return false
}

// SetProject stamps a directory tree with a project id.
func (t *T) SetProject(ctx context.Context, dir string, id uint32) error {
	return t.run(ctx, fmt.Sprintf("project -s -p %s %d", dir, id))
}

// SetHardLimit bounds what the trees of a project may hold.
func (t *T) SetHardLimit(ctx context.Context, id uint32, size int64) error {
	return t.run(ctx, fmt.Sprintf("limit -p bhard=%d %d", size, id))
}

// Get is what a project holds and what it may hold, in bytes.
//
// A project with no limit is reported with a limit of zero, which is how an
// unbounded tree reads.
func (t *T) Get(ctx context.Context, id uint32) (used, hard int64, err error) {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(xfsQuota),
		command.WithVarArgs("-x", "-c", "report -p -N -b", t.mountPoint),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
		command.WithStdoutLogLevel(zerolog.TraceLevel),
		command.WithStderrLogLevel(zerolog.TraceLevel),
		command.WithBufferedStdout(),
	)
	if err := cmd.Run(); err != nil {
		return 0, 0, err
	}
	want := fmt.Sprintf("#%d", id)
	for _, line := range strings.Split(string(cmd.Stdout()), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != want {
			continue
		}
		// The report counts in 1k blocks.
		if used, err = strconv.ParseInt(fields[1], 10, 64); err != nil {
			return 0, 0, fmt.Errorf("project %d: parse used %s: %w", id, fields[1], err)
		}
		if hard, err = strconv.ParseInt(fields[3], 10, 64); err != nil {
			return 0, 0, fmt.Errorf("project %d: parse hard limit %s: %w", id, fields[3], err)
		}
		return used * 1024, hard * 1024, nil
	}
	return 0, 0, fmt.Errorf("%s holds no project %d", t.mountPoint, id)
}

func (t *T) run(ctx context.Context, arg string) error {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(xfsQuota),
		command.WithVarArgs("-x", "-c", arg, t.mountPoint),
		command.WithLogger(t.log),
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
