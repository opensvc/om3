package poolshm

import (
	"fmt"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/core/pool"
	"opensvc.com/opensvc/util/df"
)

type (
	T struct {
		pool.T
	}
)

func init() {
	pool.Register("shm", NewPooler)
}

func NewPooler(name string) pool.Pooler {
	p := New(name)
	var i interface{} = p
	return i.(pool.Pooler)
}

func New(name string) *T {
	t := T{}
	t.Type = "shm"
	t.Name = name
	return &t
}

func (t T) Capabilities() []string {
	return []string{"rox", "rwx", "roo", "rwo", "blk"}
}

func (t T) usage() (df.Entry, error) {
	entries, err := df.MountUsage(t.path())
	if err != nil {
		return df.Entry{}, err
	}
	if len(entries) == 0 {
		return df.Entry{}, fmt.Errorf("not mounted")
	}
	return entries[0], nil
}

func (t *T) Status() pool.Status {
	errs := make([]string, 0)
	usage, err := t.usage()
	if err != nil {
		errs = append(errs, err.Error())
	}
	return pool.Status{
		Type:         t.Type,
		Name:         t.Name,
		Capabilities: t.Capabilities(),
		Head:         t.path(),
		Free:         float64(usage.Free) * 1024,
		Used:         float64(usage.Used) * 1024,
		Total:        float64(usage.Total) * 1024,
		Errors:       errs,
	}
}

func (t *T) mntOpt(size string) string {
	sizeOpt := fmt.Sprintf("size=%s", size)
	opts := t.Config().GetString(t.Key("mnt_opt"))
	if opts != "" {
		opts = strings.Join([]string{opts, sizeOpt}, ",")
	} else {
		opts = sizeOpt
	}
	return opts
}

func (t *T) loopFile(name string) string {
	return filepath.Join(t.path(), name+".img")
}

func (t *T) Translate(name string, size string, shared bool) []string {
	return []string{
		"fs#0.type=tmpfs",
		"fs#0.dev=none",
		"fs#0.mnt=" + pool.MountPointFromName(name),
		"fs#0.mnt_opt=" + t.mntOpt(size),
	}
}

func (t *T) BlkTranslate(name string, size string, shared bool) []string {
	return []string{
		"disk#0.type=loop",
		"disk#0.file=" + t.loopFile(name),
		"disk#0.size=" + size,
	}
}
