package resip

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Announce tells the neighbors of the link of Dev that its addresses
	// are on the mac of Dev: a gratuitous arp for an ipv4 address, an
	// unsolicited neighbor advertisement for an ipv6 one. Without it, the
	// neighbors of an address moved to another node, or to a new card,
	// keep sending to the mac it left until their cache entry expires.
	Announce struct {
		Log *plog.Logger

		// Dev is the name of the card holding the addresses.
		Dev string

		// IPs are the addresses to announce. A loopback or a link local one
		// is not announced.
		IPs []net.IP

		// Count is the number of announces of each address, Interval apart.
		// A zero Count announces once.
		Count    int
		Interval time.Duration

		// In runs each round of announces in the network namespace of Dev.
		// A nil In runs them in the namespace of the caller.
		In func(func() error) error
	}
)

const (
	// AnnounceCount is the number of announces a start sends, every
	// AnnounceInterval, as "arping -U -c 3" does: a neighbor missing the one
	// a start used to send kept the address on the mac of the namespace it
	// left, until its probe of the entry failed.
	AnnounceCount    = 3
	AnnounceInterval = time.Second
)

// ErrAnnounceUnsupported is the error of the senders of an operating system
// om does not announce an address on. An announce meeting it sends nothing
// more and warns, rather than fail the start of the address: the neighbors
// learn the address when they next resolve it.
var ErrAnnounceUnsupported = errors.New("not supported on this operating system")

var (
	// The senders and the wait between rounds, variables for the tests to
	// replace.
	sendGratuitousARP         = gratuitousARP
	sendNeighborAdvertisement = neighborAdvertisement
	sleep                     = time.Sleep
)

// Send announces the addresses Count times, Interval apart, and stops on its
// first error. An operating system om does not announce on is warned of, and
// is not an error.
func (t Announce) Send() error {
	ips := make([]net.IP, 0, len(t.IPs))
	for _, ip := range t.IPs {
		switch {
		case ip == nil:
		case ip.IsLoopback():
			t.Log.Tracef("skip announce of loopback address %s", ip)
		case ip.IsLinkLocalUnicast():
			t.Log.Tracef("skip announce of link local unicast address %s", ip)
		default:
			ips = append(ips, ip)
		}
	}
	if len(ips) == 0 {
		return nil
	}
	round := func() error {
		// The card is looked up in the namespace the round runs in.
		if i, err := net.InterfaceByName(t.Dev); err == nil && i.Flags&net.FlagLoopback != 0 {
			t.Log.Tracef("skip announce on loopback interface %s", t.Dev)
			return nil
		}
		for _, ip := range ips {
			if err := t.send(ip); err != nil {
				return err
			}
		}
		return nil
	}
	n := max(t.Count, 1)
	for i := 0; i < n; i++ {
		if i > 0 {
			sleep(t.Interval)
		}
		var err error
		if t.In == nil {
			err = round()
		} else {
			err = t.In(round)
		}
		if errors.Is(err, ErrAnnounceUnsupported) {
			t.Log.Warnf("%s not announced over %s: %s", ips, t.Dev, err)
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (t Announce) send(ip net.IP) error {
	if ip.To4() == nil {
		t.Log.Infof("send an unsolicited neighbor advertisement to announce %s over %s", ip, t.Dev)
		return sendNeighborAdvertisement(ip, t.Dev)
	}
	t.Log.Infof("send gratuitous arp to announce %s over %s", ip, t.Dev)
	if err := sendGratuitousARP(ip, t.Dev); err != nil {
		return fmt.Errorf("arping -i %s %s: %w", t.Dev, ip, err)
	}
	return nil
}
