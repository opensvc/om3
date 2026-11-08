package resip

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/plog"
)

type (
	sent struct {
		kind string
		ip   string
		dev  string
	}

	fakeSenders struct {
		sent   []sent
		slept  []time.Duration
		failOn int
	}
)

var errFake = errors.New("fake send error")

// fake replaces the senders and the wait between rounds for the time of the
// test. The send numbered failOn, from 1, fails.
func fake(t *testing.T, failOn int) *fakeSenders {
	f := &fakeSenders{failOn: failOn}
	record := func(kind string) func(net.IP, string) error {
		return func(ip net.IP, dev string) error {
			f.sent = append(f.sent, sent{kind: kind, ip: ip.String(), dev: dev})
			if len(f.sent) == f.failOn {
				return errFake
			}
			return nil
		}
	}
	saveARP, saveNA, saveSleep := sendGratuitousARP, sendNeighborAdvertisement, sleep
	sendGratuitousARP = record("arp")
	sendNeighborAdvertisement = record("na")
	sleep = func(d time.Duration) { f.slept = append(f.slept, d) }
	t.Cleanup(func() {
		sendGratuitousARP, sendNeighborAdvertisement, sleep = saveARP, saveNA, saveSleep
	})
	return f
}

// An announce sends a gratuitous arp for an ipv4 address and an unsolicited
// neighbor advertisement for an ipv6 one, Count rounds, Interval apart, each
// round in the namespace In runs it in.
func TestAnnounceSequence(t *testing.T) {
	f := fake(t, 0)
	rounds := 0
	err := Announce{
		Log:      plog.NewDefaultLogger(),
		Dev:      "eth0",
		IPs:      []net.IP{net.ParseIP("10.140.0.61"), net.ParseIP("fd00::61")},
		Count:    AnnounceCount,
		Interval: AnnounceInterval,
		In: func(f func() error) error {
			rounds++
			return f()
		},
	}.Send()
	assert.NoError(t, err)
	assert.Equal(t, 3, rounds)
	assert.Equal(t, []time.Duration{time.Second, time.Second}, f.slept)
	round := []sent{
		{kind: "arp", ip: "10.140.0.61", dev: "eth0"},
		{kind: "na", ip: "fd00::61", dev: "eth0"},
	}
	var want []sent
	for i := 0; i < 3; i++ {
		want = append(want, round...)
	}
	assert.Equal(t, want, f.sent)
}

// A zero Count announces once, without a wait, in the namespace of the
// caller when no In is given.
func TestAnnounceOnce(t *testing.T) {
	f := fake(t, 0)
	err := Announce{
		Log: plog.NewDefaultLogger(),
		Dev: "eth0",
		IPs: []net.IP{net.ParseIP("fd00::61")},
	}.Send()
	assert.NoError(t, err)
	assert.Empty(t, f.slept)
	assert.Equal(t, []sent{{kind: "na", ip: "fd00::61", dev: "eth0"}}, f.sent)
}

// A loopback or a link local address is not announced.
func TestAnnounceSkips(t *testing.T) {
	f := fake(t, 0)
	err := Announce{
		Log:   plog.NewDefaultLogger(),
		Dev:   "eth0",
		IPs:   []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("fe80::1"), net.ParseIP("169.254.1.1"), nil},
		Count: 3,
	}.Send()
	assert.NoError(t, err)
	assert.Empty(t, f.sent)
	assert.Empty(t, f.slept)
}

// The first failed send stops the announce and fails it, so the start fails.
func TestAnnounceStopsOnError(t *testing.T) {
	f := fake(t, 1)
	err := Announce{
		Log:      plog.NewDefaultLogger(),
		Dev:      "eth0",
		IPs:      []net.IP{net.ParseIP("10.140.0.61"), net.ParseIP("fd00::61")},
		Count:    3,
		Interval: time.Second,
	}.Send()
	assert.ErrorIs(t, err, errFake)
	assert.ErrorContains(t, err, "arping -i eth0 10.140.0.61")
	assert.Len(t, f.sent, 1)
	assert.Empty(t, f.slept)

	f = fake(t, 2)
	err = Announce{
		Log:   plog.NewDefaultLogger(),
		Dev:   "eth0",
		IPs:   []net.IP{net.ParseIP("10.140.0.61"), net.ParseIP("fd00::61")},
		Count: 3,
	}.Send()
	assert.ErrorIs(t, err, errFake)
	assert.Len(t, f.sent, 2)

	inErr := errors.New("enter netns")
	f = fake(t, 0)
	err = Announce{
		Log:   plog.NewDefaultLogger(),
		Dev:   "eth0",
		IPs:   []net.IP{net.ParseIP("10.140.0.61")},
		Count: 3,
		In:    func(func() error) error { return inErr },
	}.Send()
	assert.ErrorIs(t, err, inErr)
	assert.Empty(t, f.sent)
}
