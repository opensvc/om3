package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Publish is a port of the node forwarded to a port of an address om
// plumbed, as the publish keyword of an ip resource declares it:
//
//	[<host addr>:][<host port>:]<port>[/<proto>]
//
// A publish naming no host port is given a free port of the node when the
// resource starts, kept while it runs.
//
// The forwarding is done by the node, against the address of the resource,
// so it works whatever runs in the namespace holding that address: no
// container engine flag is involved.
type Publish struct {
	// HostAddr restricts the forwarding to the packets sent to that
	// address of the node. Nil forwards the port of every address.
	HostAddr net.IP `json:"host_addr,omitempty"`

	// HostPort is the port of the node the clients connect to. Zero in a
	// declaration is a port of the node drawn at start.
	HostPort int `json:"host_port"`

	// Port is the port of the address of the resource the connections are
	// forwarded to.
	Port int `json:"port"`

	// Proto is tcp or udp.
	Proto string `json:"proto"`

	// Auto says the host port was drawn, as the declaration named none.
	Auto bool `json:"auto,omitempty"`
}

// Random says the declaration names no host port: one is drawn at start.
func (t Publish) Random() bool {
	return t.HostPort == 0
}

// Declared returns the publish as it was declared: a drawn host port is
// forgotten.
func (t Publish) Declared() Publish {
	if t.Auto {
		t.HostPort, t.Auto = 0, false
	}
	return t
}

// String returns the publish in the form ParsePublish reads.
func (t Publish) String() string {
	var addr string
	switch {
	case t.HostAddr == nil:
	case t.HostAddr.To4() != nil:
		addr = t.HostAddr.String() + ":"
	default:
		addr = "[" + t.HostAddr.String() + "]:"
	}
	switch {
	case !t.Random():
		return fmt.Sprintf("%s%d:%d/%s", addr, t.HostPort, t.Port, t.Proto)
	case addr != "":
		return fmt.Sprintf("%s:%d/%s", addr, t.Port, t.Proto)
	default:
		return fmt.Sprintf("%d/%s", t.Port, t.Proto)
	}
}

// sameAddr says two publishes forward the same addresses of the node.
func (t Publish) sameAddr(o Publish) bool {
	if t.HostAddr == nil || o.HostAddr == nil {
		return t.HostAddr == nil && o.HostAddr == nil
	}
	return t.HostAddr.Equal(o.HostAddr)
}

// conflicts says two publishes claim the same port of the node: the same
// protocol and port, on addresses that overlap, an unset address standing
// for all of them.
func (t Publish) conflicts(o Publish) bool {
	if t.Random() || o.Random() {
		// A port to draw claims none yet, and is drawn free.
		return false
	}
	if t.Proto != o.Proto || t.HostPort != o.HostPort {
		return false
	}
	return t.HostAddr == nil || o.HostAddr == nil || t.HostAddr.Equal(o.HostAddr)
}

// ParsePublish reads one publish declaration:
//
//	25489:80             node port 25489 to port 80 of the address, tcp
//	25489:80/udp         the same in udp
//	80                   a free port of the node, drawn at start, to port 80
//	10.0.0.5:8443:443    only the packets sent to 10.0.0.5
//	10.0.0.5::443        a free port of 10.0.0.5, drawn at start
//	[fd00::5]:8443:443   an ipv6 address of the node, in brackets
func ParsePublish(s string) (Publish, error) {
	p := Publish{Proto: "tcp"}
	spec := s
	if i := strings.LastIndex(spec, "/"); i >= 0 {
		p.Proto = spec[i+1:]
		spec = spec[:i]
	}
	switch p.Proto {
	case "tcp", "udp":
	default:
		return p, fmt.Errorf("publish %q: protocol %q is not tcp or udp", s, p.Proto)
	}
	hasAddr := false
	if strings.HasPrefix(spec, "[") {
		i := strings.Index(spec, "]:")
		if i < 0 {
			return p, fmt.Errorf("publish %q: an ipv6 address of the node is written [<addr>]:[<host port>]:<port>", s)
		}
		ip := net.ParseIP(spec[1:i])
		if ip == nil || ip.To4() != nil {
			return p, fmt.Errorf("publish %q: %q is not an ipv6 address", s, spec[1:i])
		}
		p.HostAddr, hasAddr = ip, true
		spec = spec[i+2:]
	}
	l := strings.Split(spec, ":")
	switch {
	case hasAddr && len(l) == 2:
	case !hasAddr && len(l) == 3:
		ip := net.ParseIP(l[0])
		if ip == nil || ip.To4() == nil {
			return p, fmt.Errorf("publish %q: %q is not an ipv4 address, an ipv6 one is written in brackets", s, l[0])
		}
		p.HostAddr = ip.To4()
		l = l[1:]
	case !hasAddr && len(l) == 2:
		if l[0] == "" {
			return p, fmt.Errorf("publish %q: expect [<host addr>:][<host port>:]<port>[/<proto>]", s)
		}
	case !hasAddr && len(l) == 1:
		l = []string{"", l[0]}
	default:
		return p, fmt.Errorf("publish %q: expect [<host addr>:][<host port>:]<port>[/<proto>]", s)
	}
	var err error
	if l[0] != "" {
		if p.HostPort, err = parsePort(l[0]); err != nil {
			return p, fmt.Errorf("publish %q: host port: %w", s, err)
		}
	}
	if p.Port, err = parsePort(l[1]); err != nil {
		return p, fmt.Errorf("publish %q: port: %w", s, err)
	}
	return p, nil
}

func parsePort(s string) (int, error) {
	i, err := strconv.Atoi(s)
	if err != nil || i < 1 || i > 65535 {
		return 0, fmt.Errorf("%q is not a port number between 1 and 65535", s)
	}
	return i, nil
}

// ParsePublishes reads the declarations of a resource, and refuses two of
// them claiming the same port of the node, or drawing one for the same port
// of the address on the same addresses of the node.
func ParsePublishes(l []string) ([]Publish, error) {
	publishes := make([]Publish, 0, len(l))
	for _, s := range l {
		p, err := ParsePublish(s)
		if err != nil {
			return nil, err
		}
		for _, o := range publishes {
			if p.conflicts(o) {
				return nil, fmt.Errorf("publish %s and %s claim the same port of the node", o, p)
			}
			if p.Random() && o.Random() && p.Proto == o.Proto && p.Port == o.Port && p.sameAddr(o) {
				return nil, fmt.Errorf("publish %s is declared twice", p)
			}
		}
		publishes = append(publishes, p)
	}
	return publishes, nil
}

// ValidatePublish validates the value of a publish keyword, a whitespace
// separated list of declarations.
func ValidatePublish(value string) error {
	_, err := ParsePublishes(strings.Fields(value))
	return err
}
