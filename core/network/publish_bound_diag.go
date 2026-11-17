//go:build linux

package network

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// The inet_diag messages of the kernel, from linux/inet_diag.h.
const (
	// inetDiagReqV2Len is the length of an inet_diag_req_v2: family,
	// protocol, extensions, pad, states and the 48 bytes of the socket id.
	inetDiagReqV2Len = 56

	// inetDiagMsgLen is the length of an inet_diag_msg: family, state,
	// timer, retransmits, the 48 bytes of the socket id, expires, rqueue,
	// wqueue, uid and inode.
	inetDiagMsgLen = 72

	// inetDiagSKV6Only is the attribute of an ipv6 socket saying its
	// IPV6_V6ONLY option, the kernel sending it for every ipv6 socket.
	inetDiagSKV6Only = 11
)

// diagProtocols are the ip protocols of the socket protocols.
var diagProtocols = map[string]uint8{
	"tcp": unix.IPPROTO_TCP,
	"udp": unix.IPPROTO_UDP,
}

// diagV6Sockets reads the ipv6 sockets of a protocol holding a port, their
// IPV6_V6ONLY option included, which the socket tables of /proc do not say.
// Replaced by the tests.
var diagV6Sockets = sockDiagV6Sockets

// sockDiagV6Sockets dumps the ipv6 sockets of a protocol holding a port
// through the sock_diag netlink interface of the kernel.
func sockDiagV6Sockets(proto string) ([]boundSocket, error) {
	protocol, ok := diagProtocols[proto]
	if !ok {
		return nil, fmt.Errorf("sock_diag: unsupported protocol %s", proto)
	}
	var state uint8
	if _, err := fmt.Sscanf(boundStates[proto], "%X", &state); err != nil {
		return nil, fmt.Errorf("sock_diag: %s state: %w", proto, err)
	}
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, unix.NETLINK_SOCK_DIAG)
	if err != nil {
		return nil, os.NewSyscallError("socket", err)
	}
	defer func() { _ = unix.Close(fd) }()
	if err := unix.Sendto(fd, inetDiagRequest(protocol, state), 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, os.NewSyscallError("sendto", err)
	}
	var l []boundSocket
	buf := make([]byte, 64*1024)
	for {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			return nil, os.NewSyscallError("recvfrom", err)
		}
		done, err := parseInetDiagMessages(buf[:n], state, &l)
		if err != nil {
			return nil, err
		}
		if done {
			return l, nil
		}
	}
}

// inetDiagRequest is the netlink message asking the dump of the ipv6 sockets
// of a protocol in a state.
func inetDiagRequest(protocol, state uint8) []byte {
	b := make([]byte, unix.SizeofNlMsghdr+inetDiagReqV2Len)
	binary.NativeEndian.PutUint32(b[0:4], uint32(len(b)))
	binary.NativeEndian.PutUint16(b[4:6], unix.SOCK_DIAG_BY_FAMILY)
	binary.NativeEndian.PutUint16(b[6:8], unix.NLM_F_REQUEST|unix.NLM_F_DUMP)
	req := b[unix.SizeofNlMsghdr:]
	req[0] = unix.AF_INET6
	req[1] = protocol
	binary.NativeEndian.PutUint32(req[4:8], 1<<state)
	return b
}

// parseInetDiagMessages appends to l the sockets of the netlink messages of
// b in the state asked, and says the dump is done.
func parseInetDiagMessages(b []byte, state uint8, l *[]boundSocket) (bool, error) {
	for len(b) >= unix.SizeofNlMsghdr {
		msgLen := int(binary.NativeEndian.Uint32(b[0:4]))
		msgType := binary.NativeEndian.Uint16(b[4:6])
		if msgLen < unix.SizeofNlMsghdr || msgLen > len(b) {
			return false, fmt.Errorf("sock_diag: invalid message length %d", msgLen)
		}
		data := b[unix.SizeofNlMsghdr:msgLen]
		switch msgType {
		case unix.NLMSG_DONE:
			return true, nil
		case unix.NLMSG_ERROR:
			if len(data) < 4 {
				return false, fmt.Errorf("sock_diag: truncated error message")
			}
			if errno := int32(binary.NativeEndian.Uint32(data[0:4])); errno != 0 {
				return false, os.NewSyscallError("sock_diag", unix.Errno(-errno))
			}
			return true, nil
		case unix.SOCK_DIAG_BY_FAMILY:
			s, ok, err := parseInetDiagMsg(data, state)
			if err != nil {
				return false, err
			}
			if ok {
				*l = append(*l, s)
			}
		}
		if a := nlmAlign(msgLen); a < len(b) {
			b = b[a:]
		} else {
			break
		}
	}
	return false, nil
}

// parseInetDiagMsg reads the address, the port and the IPV6_V6ONLY option of
// an inet_diag_msg, and says the socket is in the state asked.
func parseInetDiagMsg(data []byte, state uint8) (boundSocket, bool, error) {
	if len(data) < inetDiagMsgLen {
		return boundSocket{}, false, fmt.Errorf("sock_diag: truncated socket message")
	}
	if data[0] != unix.AF_INET6 || data[1] != state {
		return boundSocket{}, false, nil
	}
	// The socket id: the ports and the addresses in network order.
	id := data[4:52]
	s := boundSocket{
		port: int(binary.BigEndian.Uint16(id[0:2])),
		addr: net.IP(append([]byte(nil), id[4:20]...)),
	}
	attrs := data[inetDiagMsgLen:]
	for len(attrs) >= unix.SizeofRtAttr {
		attrLen := int(binary.NativeEndian.Uint16(attrs[0:2]))
		attrType := binary.NativeEndian.Uint16(attrs[2:4])
		if attrLen < unix.SizeofRtAttr || attrLen > len(attrs) {
			return boundSocket{}, false, fmt.Errorf("sock_diag: invalid attribute length %d", attrLen)
		}
		if attrType == inetDiagSKV6Only && attrLen > unix.SizeofRtAttr {
			s.v6only = attrs[unix.SizeofRtAttr] != 0
		}
		if a := nlmAlign(attrLen); a < len(attrs) {
			attrs = attrs[a:]
		} else {
			break
		}
	}
	return s, true, nil
}

func nlmAlign(n int) int {
	return (n + unix.NLMSG_ALIGNTO - 1) &^ (unix.NLMSG_ALIGNTO - 1)
}
