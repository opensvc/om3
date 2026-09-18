package filesystems

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
)

type (
	TMPFS struct{ T }
)

func NewTMPFS() *TMPFS {
	return &TMPFS{T{fsType: "tmpfs", isVirtual: true}}
}

// SetSize changes how large the filesystem may grow.
//
// A tmpfs holds its own size: there is no device under it to enlarge first,
// and no filesystem structure to rewrite, so one remount does it in either
// direction. Made smaller than what it already holds it keeps the pages and
// reports itself full, which the kernel allows and a caller asked for.
func (t TMPFS) SetSize(ctx context.Context, mountPoint string, size int64) error {
	if mountPoint == "" {
		return fmt.Errorf("a tmpfs is resized through its mount point, and this one has none")
	}
	cmd := command.New(
		command.WithName("mount"),
		command.WithVarArgs("-o", fmt.Sprintf("remount,size=%d", size), mountPoint),
		command.WithContext(ctx),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
