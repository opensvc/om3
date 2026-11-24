// Package wakeonlan builds and sends Wake-on-LAN magic packets.
//
// A magic packet is 6 bytes of 0xFF followed by the 6 bytes of the target
// MAC address repeated 16 times, sent in a UDP datagram to a broadcast
// address of the target LAN.
package wakeonlan

import (
	"fmt"
	"net"
	"strconv"
	"syscall"
	"time"
)

const (
	// DefaultPort is the UDP port the magic packet is sent to by default,
	// the "discard" port most Wake-on-LAN tools use.
	DefaultPort = 9

	macRepeat  = 16
	headerSize = 6
	packetSize = headerSize + macRepeat*6

	sendTimeout = 5 * time.Second
)

// ParseMAC parses a 48-bit MAC address, with ':', '-' or no separator.
func ParseMAC(s string) (net.HardwareAddr, error) {
	mac, err := net.ParseMAC(s)
	if err != nil {
		return nil, fmt.Errorf("invalid mac address %q", s)
	}
	if len(mac) != 6 {
		return nil, fmt.Errorf("invalid mac address %q: not 48-bit", s)
	}
	return mac, nil
}

// ParseBroadcast parses an IPv4 broadcast address.
func ParseBroadcast(s string) (net.IP, error) {
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() == nil {
		return nil, fmt.Errorf("invalid broadcast address %q: not an ipv4 address", s)
	}
	return ip.To4(), nil
}

// ValidatePort returns an error if port is not a valid UDP port.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d: not in 1-65535", port)
	}
	return nil
}

// MagicPacket returns the magic packet waking the node with the mac address.
func MagicPacket(mac net.HardwareAddr) []byte {
	b := make([]byte, 0, packetSize)
	for i := 0; i < headerSize; i++ {
		b = append(b, 0xFF)
	}
	for i := 0; i < macRepeat; i++ {
		b = append(b, mac...)
	}
	return b
}

// Send sends the magic packet of the mac address to the broadcast address
// and port. The egress interface is the one the routing table selects for
// the broadcast address.
func Send(mac, broadcast string, port int) error {
	hw, err := ParseMAC(mac)
	if err != nil {
		return err
	}
	ip, err := ParseBroadcast(broadcast)
	if err != nil {
		return err
	}
	if err := ValidatePort(port); err != nil {
		return err
	}
	conn, err := dialBroadcast(net.JoinHostPort(ip.String(), strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("wake %s via %s:%d: %w", hw, ip, port, err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetWriteDeadline(time.Now().Add(sendTimeout)); err != nil {
		return fmt.Errorf("wake %s via %s:%d: %w", hw, ip, port, err)
	}
	if _, err := conn.Write(MagicPacket(hw)); err != nil {
		return fmt.Errorf("wake %s via %s:%d: %w", hw, ip, port, err)
	}
	return nil
}

// dialBroadcast returns a UDP connection to addr allowed to send to a
// broadcast address.
func dialBroadcast(addr string) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout: sendTimeout,
		Control: func(network, address string, c syscall.RawConn) error {
			var sockErr error
			if err := c.Control(func(fd uintptr) {
				sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
			}); err != nil {
				return err
			}
			return sockErr
		},
	}
	return dialer.Dial("udp4", addr)
}

// interfaceAddrs returns the addresses of the local interfaces, replaced in
// tests.
var interfaceAddrs = net.InterfaceAddrs

// BroadcastFor returns the broadcast address of the local interface on the
// ipv4 subnet of ip, so a magic packet sent to it reaches the lan of ip.
func BroadcastFor(ip net.IP) (net.IP, error) {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("%s is not an ipv4 address", ip)
	}
	addrs, err := interfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.To4() == nil || ipnet.IP.IsLoopback() {
			continue
		}
		mask := ipnet.Mask
		if len(mask) == net.IPv6len {
			mask = mask[12:]
		}
		if ones, _ := net.IPMask(mask).Size(); ones >= 31 {
			// point-to-point subnets have no broadcast address
			continue
		}
		if !ipnet.Contains(ip4) {
			continue
		}
		network := ipnet.IP.To4().Mask(mask)
		broadcast := make(net.IP, net.IPv4len)
		for i := range broadcast {
			broadcast[i] = network[i] | ^mask[i]
		}
		return broadcast, nil
	}
	return nil, fmt.Errorf("no local interface on the subnet of %s", ip)
}
