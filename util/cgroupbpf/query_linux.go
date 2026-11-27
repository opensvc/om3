//go:build linux

package cgroupbpf

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// queryAttr is the part of union bpf_attr the BPF_PROG_QUERY command reads,
// padded to the size the kernel checks the rest of to be zero.
type queryAttr struct {
	targetFd        uint32
	attachType      uint32
	queryFlags      uint32
	attachFlags     uint32
	progIDs         uint64
	progCnt         uint32
	_               uint32
	progAttachFlags uint64
	linkIDs         uint64
	linkAttachFlags uint64
	revision        uint64
}

// Kernel is the querier asking the kernel, with the bpf system call.
type Kernel struct{}

// Effective implements Querier. No program id is asked for: the kernel
// writes their count and returns.
func (Kernel) Effective(dir string, direction Direction) (int, error) {
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}
	defer func() { _ = unix.Close(fd) }()
	attr := queryAttr{
		targetFd:   uint32(fd),
		queryFlags: unix.BPF_F_QUERY_EFFECTIVE,
	}
	switch direction {
	case Ingress:
		attr.attachType = unix.BPF_CGROUP_INET_INGRESS
	default:
		attr.attachType = unix.BPF_CGROUP_INET_EGRESS
	}
	_, _, errno := unix.Syscall(unix.SYS_BPF, unix.BPF_PROG_QUERY, uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr))
	runtime.KeepAlive(&attr)
	if errno != 0 {
		return 0, errno
	}
	return int(attr.progCnt), nil
}

// Supported says the kernel answers the query on the root of the unified
// hierarchy: it has BPF for cgroups, and the caller may ask.
func Supported() bool {
	_, err := Kernel{}.Effective(mount, Egress)
	return err == nil
}
