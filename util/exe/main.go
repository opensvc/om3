package exe

import "os"

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
