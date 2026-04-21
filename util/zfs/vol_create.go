package zfs

import (
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/args"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	volCreateOpts struct {
		Name      string
		Size      uint64
		BlockSize uint64
		Args      []string
	}
)

// VolCreateWithArgs defines the shlex split list of arguments to prepend
// to the command.
func VolCreateWithArgs(l []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*volCreateOpts)
		t.Args = append(t.Args, l...)
		return nil
	})
}

// VolCreateWithSize defines the size of the volume in bytes
func VolCreateWithSize(size uint64) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*volCreateOpts)
		t.Size = size
		return nil
	})
}

// VolCreateWithBlockSize defines the block size of the volume in bytes
func VolCreateWithBlockSize(size uint64) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*volCreateOpts)
		t.BlockSize = size
		return nil
	})
}

// createSizeString returns a string like 500m assumed to be a binary size,
// which is what zvol create expects.
func createSizeString(size uint64) string {
	s := sizeconv.ExactBSizeCompact(float64(size))
	return strings.TrimRight(s, "i")
}

func volCreateOptsToArgs(t volCreateOpts) []string {
	a := args.New()
	a.Append("create")

	// zvol create <options> -V <size> <name>
	//             ^^^^^^^^^
	a.Append(t.Args...)

	if t.BlockSize > 0 {
		a.DropOption("-b")
		a.DropOptionAndMatchingValue("-o", "^volblocksize=.*")
		a.Append("-b", sizeconv.ExactBSizeCompact(float64(t.BlockSize)))
	}

	// zvol create <options> -V <size> <name>
	//                          ^^^^^^
	a.Append("-V", createSizeString(t.Size))

	// zvol create <options> -V <size> <name>
	//                                 ^^^^^^
	a.Append(t.Name)
	return a.Get()
}

func (t *Vol) Create(fopts ...funcopt.O) error {
	opts := &volCreateOpts{Name: t.Name}
	funcopt.Apply(opts, fopts...)
	args := volCreateOptsToArgs(*opts)
	cmd := command.New(
		command.WithName("zfs"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}
