package df

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type (
	// Entry represents a parsed line of the df unix command
	Entry struct {
		Device      string
		MountPoint  string
		UsedPercent int64

		// In bytes
		Total int64
		Used  int64
		Free  int64
	}
)

// Usage executes and parses a df command
func Usage() ([]Entry, error) {
	b, err := doDFUsage()
	if err != nil {
		return nil, err
	}
	return parseUsage(b)
}

// Inode executes and parses a df command
func Inode() ([]Entry, error) {
	b, err := doDFInode()
	if err != nil {
		return nil, err
	}
	return parseInode(b)
}

// ContainingMountUsage executes and parses a df command for the mount point containing the path
func ContainingMountUsage(p string) ([]Entry, error) {
	pp := p
	for {
		if _, err := os.Stat(pp); errors.Is(err, os.ErrNotExist) {
			if pp = filepath.Dir(pp); pp == "" {
				return nil, fmt.Errorf("failed to find a mount containing %s", p)
			} else {
				continue
			}
		} else if err != nil {
			return nil, err
		}
		if b, err := doDFUsage(pp); err == nil {
			return parseUsage(b)
		} else {
			return nil, err
		}
	}
}

// MountUsage executes and parses a df command for a mount point
func MountUsage(mnt string) ([]Entry, error) {
	b, err := doDFUsage(mnt)
	if err != nil {
		return nil, err
	}
	return parseUsage(b)
}

// TypeMountUsage executes and parses a df command for a mount point and a fstype
func TypeMountUsage(fstype string, mnt string) ([]Entry, error) {
	b, err := doDFUsage(typeOption, fstype, mnt)
	if err != nil {
		return nil, err
	}
	return parseUsage(b)
}

// HasTypeMount return true if df has 'mnt' mount point with type 'fstype'
// else return false
func HasTypeMount(fstype string, mnt string) bool {
	l, err := TypeMountUsage(fstype, mnt)
	if err != nil {
		return false
	}
	return len(l) > 0
}

func doDF(args []string) ([]byte, error) {
	df, err := exec.LookPath(dfPath)
	if err != nil {
		return nil, err
	}
	cmd := &exec.Cmd{
		Path: df,
		Args: args,
	}
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return b, nil
}
