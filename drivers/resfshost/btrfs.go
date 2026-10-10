package resfshost

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

func (t *T) isBtrfs() bool {
	return t.Type == "btrfs"
}

func cleanSubvol(s string) string {
	return strings.Trim(filepath.Clean("/"+s), "/")
}

// btrfsMountOptions returns the options a btrfs is mounted with: mnt_opt,
// and the subvol keyword as the subvol= option.
//
// A subvol= option of mnt_opt naming the same subvolume is dropped, as the
// keyword sets it. One naming another subvolume is a conflict, which
// btrfsConflicts refuses before anything is mounted.
func (t *T) btrfsMountOptions() string {
	if t.Subvol == "" {
		return t.MountOptions
	}
	l := make([]string, 0)
	for _, option := range strings.Split(t.MountOptions, ",") {
		if option == "" || strings.HasPrefix(option, "subvol=") {
			continue
		}
		l = append(l, option)
	}
	return strings.Join(append(l, "subvol="+cleanSubvol(t.Subvol)), ",")
}

// mkfsOptions returns the options a filesystem is formatted with: mkfs_opt,
// and the label keyword of a btrfs as -L when mkfs_opt sets none.
func (t *T) mkfsOptions() []string {
	if !t.isBtrfs() || t.BtrfsLabel == "" || mkfsLabel(t.MKFSOptions) != "" {
		return t.MKFSOptions
	}
	return append(slices.Clone(t.MKFSOptions), "-L", t.BtrfsLabel)
}

// mkfsLabel returns the label the mkfs options set, "" when none.
func mkfsLabel(args []string) string {
	for i, arg := range args {
		switch {
		case (arg == "-L" || arg == "--label") && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(arg, "--label="):
			return strings.TrimPrefix(arg, "--label=")
		case strings.HasPrefix(arg, "-L") && len(arg) > 2:
			return arg[2:]
		}
	}
	return ""
}

// btrfsConflicts returns why the subvol and label keywords disagree with
// mnt_opt and mkfs_opt, none when they agree.
//
// A disagreement is refused rather than settled by the keyword: the subvolume
// a btrfs resource mounts is the one its sync replicates, and mounting
// another one on a misconfiguration would run the service on the wrong data.
func (t *T) btrfsConflicts() []error {
	if !t.isBtrfs() {
		return nil
	}
	l := make([]error, 0)
	if t.Subvol != "" {
		want := cleanSubvol(t.Subvol)
		for _, option := range strings.Split(t.MountOptions, ",") {
			switch {
			case strings.HasPrefix(option, "subvol="):
				if got := cleanSubvol(strings.TrimPrefix(option, "subvol=")); got != want {
					l = append(l, fmt.Errorf("mnt_opt %s names another subvolume than the subvol keyword %s: remove one", option, t.Subvol))
				}
			case strings.HasPrefix(option, "subvolid="):
				l = append(l, fmt.Errorf("mnt_opt %s names a subvolume by id, and the subvol keyword %s names one by path: remove one", option, t.Subvol))
			}
		}
	}
	if t.BtrfsLabel != "" {
		if got := mkfsLabel(t.MKFSOptions); got != "" && got != t.BtrfsLabel {
			l = append(l, fmt.Errorf("mkfs_opt sets the label %s, and the label keyword %s: remove one", got, t.BtrfsLabel))
		}
	}
	return l
}
