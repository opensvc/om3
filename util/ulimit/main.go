package ulimit

import (
	"syscall"
	"time"

	"github.com/pkg/errors"
)

type Config struct {
	CPU     *time.Duration
	FSize   *int64
	Data    *int64
	Stack   *int64
	Core    *int64
	NoFile  *int64
	AS      *int64
	MemLock *int64
	NProc   *int64
	RSS     *int64
	VMem    *int64
}

func (c Config) Apply() error {
	if err := setDurationRlimit(c.CPU, syscall.RLIMIT_CPU); err != nil {
		return errors.Wrapf(err, "cpu")
	}
	if err := setSizeRlimit(c.FSize, syscall.RLIMIT_FSIZE); err != nil {
		return errors.Wrapf(err, "fsize")
	}
	if err := setSizeRlimit(c.Data, syscall.RLIMIT_DATA); err != nil {
		return errors.Wrapf(err, "data")
	}
	if err := setSizeRlimit(c.Stack, syscall.RLIMIT_STACK); err != nil {
		return errors.Wrapf(err, "stack")
	}
	if err := setSizeRlimit(c.Core, syscall.RLIMIT_CORE); err != nil {
		return errors.Wrapf(err, "core")
	}
	if err := setSizeRlimit(c.NoFile, syscall.RLIMIT_NOFILE); err != nil {
		return errors.Wrapf(err, "nofile")
	}
	if err := setSizeRlimit(c.AS, syscall.RLIMIT_AS); err != nil {
		return errors.Wrapf(err, "as")
	}
	if err := setNProc(c.NProc); err != nil {
		return errors.Wrapf(err, "nproc")
	}
	if err := setRss(c.RSS); err != nil {
		return errors.Wrapf(err, "rss")
	}
	if err := setMemLock(c.MemLock); err != nil {
		return errors.Wrapf(err, "memlock")
	}
	if err := setSizeRlimit(c.VMem, syscall.RLIMIT_AS); err != nil {
		return errors.Wrapf(err, "as via vmem")
	}
	return nil
}

func setDurationRlimit(d *time.Duration, what int) error {
	if d == nil {
		return nil
	}
	v := *d
	return setRlimit(uint64(v.Seconds()), what)
}

func setSizeRlimit(d *int64, what int) error {
	if d == nil {
		return nil
	}
	return setRlimit(uint64(*d), what)
}

func setRlimit(max uint64, what int) error {
	var rLimit syscall.Rlimit
	err := syscall.Getrlimit(what, &rLimit)
	if err != nil {
		return errors.Wrapf(err, "get_rlimit")
	}
	rLimit.Max = max
	if rLimit.Max < rLimit.Cur {
		rLimit.Cur = rLimit.Max
	}
	err = syscall.Setrlimit(what, &rLimit)
	if err != nil {
		return errors.Wrapf(err, "set_rlimit: %+v", rLimit)
	}
	return nil
}
