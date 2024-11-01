package ulimit

import (
	"fmt"
	"syscall"
	"time"
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

func (c Config) NeedApply() bool {
	if c.CPU != nil {
		return true
	}
	if c.FSize != nil {
		return true
	}
	if c.Data != nil {
		return true
	}
	if c.Stack != nil {
		return true
	}
	if c.Core != nil {
		return true
	}
	if c.NoFile != nil {
		return true
	}
	if c.AS != nil {
		return true
	}
	if c.MemLock != nil {
		return true
	}
	if c.NProc != nil {
		return true
	}
	if c.RSS != nil {
		return true
	}
	if c.VMem != nil {
		return true
	}
	return false
}

func (c Config) Apply() error {
	if err := setDurationRlimit(c.CPU, syscall.RLIMIT_CPU); err != nil {
		return fmt.Errorf("cpu: %w", err)
	}
	if err := setSizeRlimit(c.FSize, syscall.RLIMIT_FSIZE); err != nil {
		return fmt.Errorf("fsize: %w", err)
	}
	if err := setSizeRlimit(c.Data, syscall.RLIMIT_DATA); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if err := setSizeRlimit(c.Stack, syscall.RLIMIT_STACK); err != nil {
		return fmt.Errorf("stack: %w", err)
	}
	if err := setSizeRlimit(c.Core, syscall.RLIMIT_CORE); err != nil {
		return fmt.Errorf("core: %w", err)
	}
	if err := setSizeRlimit(c.NoFile, syscall.RLIMIT_NOFILE); err != nil {
		return fmt.Errorf("nofile: %w", err)
	}
	if err := setSizeRlimit(c.AS, syscall.RLIMIT_AS); err != nil {
		return fmt.Errorf("as: %w", err)
	}
	if err := setNProc(c.NProc); err != nil {
		return fmt.Errorf("nproc: %w", err)
	}
	if err := setRss(c.RSS); err != nil {
		return fmt.Errorf("rss: %w", err)
	}
	if err := setMemLock(c.MemLock); err != nil {
		return fmt.Errorf("memlock: %w", err)
	}
	if err := setSizeRlimit(c.VMem, syscall.RLIMIT_AS); err != nil {
		return fmt.Errorf("as via vmem: %w", err)
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
		return fmt.Errorf("get_rlimit: %w", err)
	}
	rLimit.Max = max
	if rLimit.Max < rLimit.Cur {
		rLimit.Cur = rLimit.Max
	}
	err = syscall.Setrlimit(what, &rLimit)
	if err != nil {
		return fmt.Errorf("set_rlimit: %+v: %w", rLimit, err)
	}
	return nil
}
