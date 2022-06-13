//go:build solaris

package ulimit

func setNProc(_ *int64) error {
	return nil
}

func setRss(_ *int64) error {
	return nil
}

func setMemLock(_ *int64) error {
	return nil
}
