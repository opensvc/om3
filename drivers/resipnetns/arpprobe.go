//go:build linux

package resipnetns

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

const (
	arpRequest = 1
	arpReply   = 2

	// arpLen is the length of an arp packet of ipv4 over ethernet.
	arpLen = 28
)

// arpProbePacket returns the arp probe of ip sent from the hardware address
// hw, as RFC 5227 writes it: a request with no sender address, which no
// neighbor learns anything from, where a request sent from the address
// would have them take the address for the sender's.
func arpProbePacket(hw net.HardwareAddr, ip net.IP) []byte {
	b := make([]byte, arpLen)
	binary.BigEndian.PutUint16(b[0:2], 1)      // ethernet
	binary.BigEndian.PutUint16(b[2:4], 0x0800) // ipv4
	b[4], b[5] = 6, 4
	binary.BigEndian.PutUint16(b[6:8], arpRequest)
	copy(b[8:14], hw)
	// The sender address, b[14:18], and the target hardware address,
	// b[18:24], are left zero.
	copy(b[24:28], ip.To4())
	return b
}

// arpProbeAnswer returns the hardware address of the sender of the arp
// packet b when it is an answer to a probe of ip: a reply sent from ip. Nil
// for any other packet.
func arpProbeAnswer(b []byte, ip net.IP) net.HardwareAddr {
	if len(b) < arpLen || binary.BigEndian.Uint16(b[6:8]) != arpReply || !net.IP(b[14:18]).Equal(ip) {
		return nil
	}
	hw := make(net.HardwareAddr, 6)
	copy(hw, b[8:14])
	return hw
}

func htons(i uint16) uint16 {
	return i<<8 | i>>8
}

// sendARPProbes asks the segment of dev, twice within timeout, which host
// holds the ipv4 address ip, and returns the hardware address of the first
// answering, nil when none does. It runs in the network namespace of the
// calling thread.
func sendARPProbes(ip net.IP, dev string, timeout time.Duration) (net.HardwareAddr, error) {
	if ip.To4() == nil {
		return nil, fmt.Errorf("%s is no ipv4 address", ip)
	}
	iface, err := net.InterfaceByName(dev)
	if err != nil {
		return nil, err
	}
	proto := htons(unix.ETH_P_ARP)
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, int(proto))
	if err != nil {
		return nil, fmt.Errorf("arp socket: %w", err)
	}
	defer func() { _ = unix.Close(fd) }()
	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: proto, Ifindex: iface.Index}); err != nil {
		return nil, fmt.Errorf("arp socket bind to %s: %w", dev, err)
	}
	to := &unix.SockaddrLinklayer{Protocol: proto, Ifindex: iface.Index, Hatype: 1, Halen: 6}
	copy(to.Addr[:], []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	probe := arpProbePacket(iface.HardwareAddr, ip)
	const rounds = 2
	buf := make([]byte, 128)
	for i := 0; i < rounds; i++ {
		if err := unix.Sendto(fd, probe, 0, to); err != nil {
			return nil, fmt.Errorf("arp probe of %s on %s: %w", ip, dev, err)
		}
		deadline := time.Now().Add(timeout / rounds)
		for {
			left := time.Until(deadline)
			if left <= 0 {
				break
			}
			tv := unix.NsecToTimeval(left.Nanoseconds())
			if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv); err != nil {
				return nil, err
			}
			n, _, err := unix.Recvfrom(fd, buf, 0)
			switch {
			case errors.Is(err, unix.EAGAIN), errors.Is(err, unix.EINTR):
				continue
			case err != nil:
				return nil, fmt.Errorf("arp probe of %s on %s: %w", ip, dev, err)
			}
			if hw := arpProbeAnswer(buf[:n], ip); hw != nil {
				return hw, nil
			}
		}
	}
	return nil, nil
}
