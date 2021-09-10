package ulimit

import (
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"opensvc.com/opensvc/util/sizeconv"
)

type Config struct {
	CPU     string
	FSize   string
	Data    string
	Stack   string
	Core    string
	NoFile  string
	AS      string
	MemLock string
	NProc   string
	RSS     string
	VMem    string
}

func (c Config) Apply() error {
	if err := setRlimit(c.CPU, syscall.RLIMIT_CPU); err != nil {
		return errors.Wrapf(err, "cpu")
	}
	if err := setRlimit(c.FSize, syscall.RLIMIT_FSIZE); err != nil {
		return errors.Wrapf(err, "fsize")
	}
	if err := setRlimit(c.Data, syscall.RLIMIT_DATA); err != nil {
		return errors.Wrapf(err, "data")
	}
	if err := setRlimit(c.Stack, syscall.RLIMIT_STACK); err != nil {
		return errors.Wrapf(err, "stack")
	}
	if err := setRlimit(c.Core, syscall.RLIMIT_CORE); err != nil {
		return errors.Wrapf(err, "core")
	}
	if err := setRlimit(c.NoFile, syscall.RLIMIT_NOFILE); err != nil {
		return errors.Wrapf(err, "nofile")
	}
	if err := setRlimit(c.AS, syscall.RLIMIT_AS); err != nil {
		return errors.Wrapf(err, "as")
	}
	if err := setRlimit(c.NProc, unix.RLIMIT_NPROC); err != nil {
		return errors.Wrapf(err, "nproc")
	}
	if err := setRlimit(c.RSS, unix.RLIMIT_RSS); err != nil {
		return errors.Wrapf(err, "rss")
	}
	if err := setRlimit(c.MemLock, unix.RLIMIT_MEMLOCK); err != nil {
		return errors.Wrapf(err, "memlock")
	}
	if err := setRlimit(c.VMem, syscall.RLIMIT_AS); err != nil {
		return errors.Wrapf(err, "as via vmem")
	}
	return nil
}

func setRlimit(s string, what int) error {
	if s == "" {
		return nil
	}
	max, err := sizeconv.FromSize(s)
	if err != nil {
		return errors.Wrapf(err, "sizeconv")
	}
	var rLimit syscall.Rlimit
	err = syscall.Getrlimit(what, &rLimit)
	if err != nil {
		return errors.Wrapf(err, "get_rlimit")
	}
	rLimit.Max = uint64(max)
	if rLimit.Max < rLimit.Cur {
		rLimit.Cur = rLimit.Max
	}
	err = syscall.Setrlimit(what, &rLimit)
	if err != nil {
		return errors.Wrapf(err, "set_rlimit: %+v", rLimit)
	}
	return nil
}
