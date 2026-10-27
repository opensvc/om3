package zfs

import (
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
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

// volDestroyBusyDelay is the time between two tries of a destroy reported
// busy. A var for the tests.
var volDestroyBusyDelay = time.Second

func volDestroyOptsToArgs(t volDestroyOpts) []string {
	l := []string{"destroy"}
	if t.Force {
		l = append(l, "-f")
	}
	return l
}

// Destroy destroys the volume, a volume already gone being destroyed.
//
// A volume just released, as the zvol under a drbd device brought down, is
// reported busy by zfs for a moment: the destroy is tried again a second
// later, as many times as VolDestroyWithBusyRetries says. A destroy that
// fails is an error, the busy one once the retries are spent: the caller
// would otherwise report the volume unprovisioned and leave it behind.
func (t *Vol) Destroy(fopts ...funcopt.O) error {
	opts := &volDestroyOpts{}
	funcopt.Apply(opts, fopts...)
	args := append(volDestroyOptsToArgs(*opts), t.Name)
	var err error
	for i := 0; i <= opts.BusyRetries; i++ {
		if i > 0 {
			time.Sleep(volDestroyBusyDelay)
		}
		// A command runs once: a retry is another one.
		cmd := command.New(
			command.WithName("zfs"),
			command.WithArgs(args),
			command.WithBufferedStderr(),
			command.WithLogger(t.Log),
			command.WithCommandLogLevel(zerolog.InfoLevel),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.ErrorLevel),
		)
		err = cmd.Run()
		if err == nil {
			return nil
		}
		stderr := string(cmd.Stderr())
		if strings.Contains(stderr, "does not exist") {
			return nil
		}
		if !strings.Contains(stderr, "busy") {
			return err
		}
	}
	return err
}
