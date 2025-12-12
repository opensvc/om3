package zfs

import (
	"strings"
	"time"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/rs/zerolog"
)

type (
	volDestroyOpts struct {
		Force       bool
		BusyRetries int
	}
)

// VolDestroyWithForce forces an unmount of any file systems using the
// unmount -f command.  This option has no effect on non-file systems or
// unmounted file systems.
func VolDestroyWithForce() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*volDestroyOpts)
		t.Force = true
		return nil
	})
}

// VolDestroyWithBusyRetries is the number of retries when the destroy
// command reports "busy".
func VolDestroyWithBusyRetries(count int) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*volDestroyOpts)
		t.BusyRetries = count
		return nil
	})
}

func volDestroyOptsToArgs(t volDestroyOpts) []string {
	l := []string{"destroy"}
	if t.Force {
		l = append(l, "-f")
	}
	return l
}

func (t *Vol) Destroy(fopts ...funcopt.O) error {
	opts := &volDestroyOpts{}
	funcopt.Apply(opts, fopts...)
	args := append(volDestroyOptsToArgs(*opts), t.Name)
	cmd := command.New(
		command.WithName("zfs"),
		command.WithArgs(args),
		command.WithBufferedStderr(),
		command.WithLogger(t.Log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	for i := -1; i < opts.BusyRetries; i++ {
		err := cmd.Run()
		if err != nil {
			return nil
		}
		stderr := string(cmd.Stderr())
		if strings.Contains(stderr, "busy") {
			time.Sleep(time.Second)
			continue
		}
		return err

	}
	return nil
}
