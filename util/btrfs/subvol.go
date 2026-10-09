// Package btrfs reads what the btrfs commands say of the subvolumes of a
// filesystem.
package btrfs

import (
	"fmt"
	"strconv"
	"strings"
)

type (
	// Subvol is a subvolume, as "btrfs subvolume list -c -u -q -R" lists
	// it.
	Subvol struct {
		ID int64

		// CGen is the generation the subvolume was created at. It orders
		// the subvolumes of a filesystem by creation, a received one by the
		// time it was received.
		CGen int64

		// Path is relative to the root of the filesystem when listed on a
		// mount of its root, subvolid=5.
		Path string

		UUID         string
		ParentUUID   string
		ReceivedUUID string
	}
)

// none is the value btrfs lists an unset uuid as.
const none = "-"

// Identity is the identifier of the snapshot that is the same on both ends of
// a send: the uuid of the snapshot sent is the received uuid of its copy.
//
// A copy sent on to another node keeps the identity of the original, as a
// send names its parent by its received uuid when it has one, and a receive
// looks it up by received uuid then by uuid. So the snapshots two nodes hold
// in common are the ones with the same identity, whichever node was the
// source.
func (t Subvol) Identity() string {
	if t.ReceivedUUID != "" {
		return t.ReceivedUUID
	}
	return t.UUID
}

// IsReceived reports whether the subvolume was made by a receive.
func (t Subvol) IsReceived() bool {
	return t.ReceivedUUID != ""
}

// ListArgs are the arguments of the list command ParseList reads the output
// of: the subvolumes with their creation generation, uuids and paths, the
// read-only ones alone when readOnly.
func ListArgs(readOnly bool, path string) []string {
	args := []string{"subvolume", "list", "-c", "-u", "-q", "-R"}
	if readOnly {
		args = append(args, "-r")
	}
	return append(args, path)
}

// ParseList reads the output of "btrfs subvolume list -c -u -q -R", a line
// per subvolume:
//
//	ID 257 gen 8 cgen 8 top level 5 parent_uuid <u> received_uuid <u> uuid <u> path <path>
//
// The path is the end of the line, spaces included.
func ParseList(b []byte) ([]Subvol, error) {
	l := make([]Subvol, 0)
	for _, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		head, path, ok := strings.Cut(line, " path ")
		if !ok {
			return nil, fmt.Errorf("btrfs subvolume list: no path in %q", line)
		}
		fields := strings.Fields(head)
		var t Subvol
		t.Path = path
		for i := 0; i+1 < len(fields); i++ {
			switch fields[i] {
			case "ID":
				id, err := strconv.ParseInt(fields[i+1], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("btrfs subvolume list: id in %q: %w", line, err)
				}
				t.ID = id
			case "cgen":
				cgen, err := strconv.ParseInt(fields[i+1], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("btrfs subvolume list: cgen in %q: %w", line, err)
				}
				t.CGen = cgen
			case "uuid":
				t.UUID = unset(fields[i+1])
			case "parent_uuid":
				t.ParentUUID = unset(fields[i+1])
			case "received_uuid":
				t.ReceivedUUID = unset(fields[i+1])
			default:
				continue
			}
			i++
		}
		if t.ID == 0 || t.UUID == "" {
			return nil, fmt.Errorf("btrfs subvolume list: no id or uuid in %q", line)
		}
		l = append(l, t)
	}
	return l, nil
}

func unset(s string) string {
	if s == none {
		return ""
	}
	return s
}

// Under returns the subvolumes of l whose path is path or below it.
func Under(l []Subvol, path string) []Subvol {
	path = strings.Trim(path, "/")
	under := make([]Subvol, 0)
	for _, s := range l {
		if s.Path == path || strings.HasPrefix(s.Path, path+"/") {
			under = append(under, s)
		}
	}
	return under
}
