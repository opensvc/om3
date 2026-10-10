package btrfs

import (
	"fmt"
	"path/filepath"
	"strings"
)

type (
	// Location is a subvolume of the btrfs filesystem of a label, as the
	// sync keywords name it: <label>:<subvol>.
	//
	// The label names the filesystem on every node: its device may change
	// from a node to another, its label does not. The subvolume is the path
	// from the root of the filesystem, the one an fs.btrfs resource mounts
	// with the subvol=<subvol> mount option.
	Location struct {
		Label  string
		Subvol string
	}
)

// ParseLocation reads a <label>:<subvol> value. The subvolume is not the
// root of the filesystem.
func ParseLocation(s string) (Location, error) {
	label, subvol, ok := strings.Cut(s, ":")
	if !ok || label == "" {
		return Location{}, fmt.Errorf("%q is not <label>:<subvol>", s)
	}
	subvol = strings.Trim(filepath.Clean("/"+subvol), "/")
	if subvol == "" || subvol == "." {
		return Location{}, fmt.Errorf("%q names the root of the filesystem, not a subvolume", s)
	}
	return Location{Label: label, Subvol: subvol}, nil
}

func (t Location) String() string {
	return t.Label + ":" + t.Subvol
}
