package volfile

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func probeXattr(dir string) error {
	p := filepath.Join(dir, ".probe")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		return err
	}
	defer func() { _ = os.Remove(p) }()
	return unix.Setxattr(p, "user.probe", []byte("1"), 0)
}
