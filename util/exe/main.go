package exe

import (
	"os"
	"path/filepath"
)

// IsExecOwner returns true is the file mode indicates the file is
// executable by its owner.
func IsExecOwner(mode os.FileMode) bool {
	return mode&0100 != 0
}

// IsExecGroup returns true is the file mode indicates the file is
// executable by its group.
func IsExecGroup(mode os.FileMode) bool {
	return mode&0010 != 0
}

// IsExecOther returns true is the file mode indicates the file is
// executable by other.
func IsExecOther(mode os.FileMode) bool {
	return mode&0001 != 0
}

// IsExecAny returns true is the file mode indicates the file is
// executable by owner, group or other.
func IsExecAny(mode os.FileMode) bool {
	return mode&0111 != 0
}

// IsExecAll returns true is the file mode indicates the file is
// executable by owner, group and other.
func IsExecAll(mode os.FileMode) bool {
	return mode&0111 == 0111
}

func FindExe(root string) []string {
	l := make([]string, 0)
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsDir() {
			return nil
		}
		if IsExecOwner(info.Mode().Perm()) {
			l = append(l, path)
			return nil
		}
		return nil
	})
	return l
}
