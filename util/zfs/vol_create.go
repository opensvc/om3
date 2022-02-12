package zfs

import (
	"fmt"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/args"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/sizeconv"
)

type (
	volCreateOpts struct {
		Name      string
		Size      uint64
		BlockSize uint64
		Args      []string
	}
)

// VolCreateWithArgs defines the shlex splitted list of arguments to prepend
// to the command.
func VolCreateWithArgs(l []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*volCreateOpts)
		if t.Args == nil {
			t.Args = make([]string, 0)
		}
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

func volCreateOptsToArgs(t volCreateOpts) []string {
	a := args.New()
	a.Append("create", "-V")
	if t.BlockSize > 0 {
		a.DropOption("-b")
		a.Append("-b", sizeconv.ExactBSizeCompact(float64(t.Size)))
	}

	// zvol create -V <options> <size> <name>
	//                ^^^^^^^^^
	if t.Args != nil {
		a.Append(t.Args...)
	}

	// zvol create -V <options> <size> <name>
	//                          ^^^^^^
	a.Append(sizeconv.ExactBSizeCompact(float64(t.Size)))

	// zvol create -V <options> <size> <name>
	//                                 ^^^^^^
	a.Append(t.Name)
	return a.Get()
}

func (t *Vol) Create(fopts ...funcopt.O) error {
	opts := &volCreateOpts{Name: t.Name}
	funcopt.Apply(opts, fopts...)
	args := volCreateOptsToArgs(*opts)
	fmt.Println(opts, args)
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
