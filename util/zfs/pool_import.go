package zfs

import (
	"fmt"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
)

type (
	poolImportOpts struct {
		CacheFile string
		Force     bool
		Quiet     bool
		Options   []string
		Devices   []string
	}
)

// PoolImportWithCacheFile reads configuration from the given cachefile that was created
// with the cachefile pool property.  This cachefile is used instead of searching for devices.
func PoolImportWithCacheFile(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolImportOpts)
		t.CacheFile = s
		return nil
	})
}

// PoolImportWithDevice uses device or searches for devices or files in dir.
// PoolImportWithDevice can be specified multiple times.
// PoolImportWithDevice incompatible with PoolImportWithCacheFile.
func PoolImportWithDevice(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolImportOpts)
		if t.Devices == nil {
			t.Devices = make([]string, 0)
		}
		t.Devices = append(t.Devices, s)
		return nil
	})
}

// PoolImportWithOption is a mount option to use when mounting datasets within the pool.
func PoolImportWithOption(option, value string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolImportOpts)
		if t.Options == nil {
			t.Options = make([]string, 0)
		}
		s := fmt.Sprintf("%s=%s", option, value)
		t.Options = append(t.Options, s)
		return nil
	})
}

// PoolImportWithForce forcefully unmounts all datasets, using the unmount -f command.
// This option is not supported on Linux.
func PoolImportWithForce() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolImportOpts)
		t.Force = true
		return nil
	})
}

// PoolImportWithQuiet sets debug level for error and output logging
func PoolImportWithQuiet() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*poolImportOpts)
		t.Quiet = true
		return nil
	})
}

func poolImportOptsToArgs(t poolImportOpts) []string {
	l := []string{"import"}
	if t.Force {
		l = append(l, "-f")
	}
	if t.CacheFile != "" {
		l = append(l, "-c", t.CacheFile)
	}
	if t.Options != nil {
		for _, s := range t.Options {
			l = append(l, "-o", s)
		}
	}
	if t.Devices != nil {
		for _, s := range t.Devices {
			l = append(l, "-d", s)
		}
	}
	return l
}

func (t *Pool) Import(fopts ...funcopt.O) error {
	opts := &poolImportOpts{}
	funcopt.Apply(opts, fopts...)
	args := append(poolImportOptsToArgs(*opts), t.Name)
	cmd := command.New(
		command.WithName("zpool"),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithLogger(t.Log),
	)
	err := cmd.Run()
	if err != nil && opts.Quiet {
		return err
	}
	t.Log.Infof("%s", cmd)
	stdout := string(cmd.Stdout())
	stderr := string(cmd.Stderr())
	if err != nil {
		if stdout != "" {
			t.Log.Infof(stdout)
		}
		if stderr != "" {
			t.Log.Errorf(stderr)
		}
	} else {
		if stdout != "" {
			t.Log.Infof(stdout)
		}
		if stderr != "" {
			t.Log.Errorf(stderr)
		}
	}
	return err
}
