package resiphost

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resip"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/netif"

	"github.com/go-ping/ping"
)

const (
	tagNonRouted = "nonrouted"
)

type (
	T struct {
		resource.T

		Path       naming.Path
		ObjectFQDN string
		DNS        []string

		// config
		IPName       string         `json:"ipname"`
		IPDev        string         `json:"ipdev"`
		Netmask      string         `json:"netmask"`
		Network      string         `json:"network"`
		Gateway      string         `json:"gateway"`
		Provisioner  string         `json:"provisioner"`
		CheckCarrier bool           `json:"check_carrier"`
		Alias        bool           `json:"alias"`
		Expose       []string       `json:"expose"`
		WaitDNS      *time.Duration `json:"wait_dns"`

		// cache
		_ipaddr net.IP
		_ipmask net.IPMask
		_ipnet  *net.IPNet
	}

	Addrs []net.Addr
)

func New() resource.Driver {
	t := &T{}
	return t
}

// StatusInfo implements resource.StatusInfoer
func (t *T) StatusInfo(_ context.Context) map[string]interface{} {
	netmask, _ := t.ipmask().Size()
	data := make(map[string]interface{})
	data["expose"] = t.Expose
	data["ipaddr"] = t.ipaddr()
	data["ipdev"] = t.IPDev
	data["netmask"] = netmask
	return data
}

func (t *T) Start(ctx context.Context) error {
	if initialStatus := t.Status(ctx); initialStatus == status.Up {
		t.Log().Infof("%s is already up on %s", t.IPName, t.IPDev)
		return nil
	}
	if err := t.start(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.stop()
	})
	if err := t.arpAnnounce(); err != nil {
		return err
	}
	if err := resip.WaitDNSRecord(ctx, t.WaitDNS, t.ObjectFQDN, t.DNS); err != nil {
		return err
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if initialStatus := t.Status(ctx); initialStatus == status.Down {
		t.Log().Infof("%s is already down on %s", t.IPName, t.IPDev)
		return nil
	}
	if err := t.stop(); err != nil {
		return err
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	var (
		i       *net.Interface
		err     error
		addrs   Addrs
		carrier bool
	)
	ip := t.ipaddr()
	if t.IPName == "" {
		t.StatusLog().Warn("ipname not set")
		return status.NotApplicable
	}
	if t.IPDev == "" {
		t.StatusLog().Warn("ipdev not set")
		return status.NotApplicable
	}
	if i, err = t.netInterface(); err != nil {
		if fmt.Sprint(err.(*net.OpError).Unwrap()) == "no such network interface" {
			t.StatusLog().Warn("interface %s not found", t.IPDev)
		} else {
			t.StatusLog().Error("%s", err)
		}
		return status.Down
	}
	if t.CheckCarrier {
		if carrier, err = t.hasCarrier(); err == nil && carrier == false {
			t.StatusLog().Error("interface %s no-carrier.", t.IPDev)
			return status.Down
		}
	}
	if addrs, err = i.Addrs(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Down
	}
	if !addrs.Has(ip) {
		t.Log().Debugf("ip not found on intf")
		return status.Down
	}
	return status.Up
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return fmt.Sprintf("%s %s", t.ipnet(), t.IPDev)
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) Abort(ctx context.Context) bool {
	if t.Tags.Has(tagNonRouted) || t.IsActionDisabled() {
		return false // let start fail with an explicit error message
	}
	if t.ipaddr() == nil {
		return false // let start fail with an explicit error message
	}
	if initialStatus := t.Status(ctx); initialStatus == status.Up {
		return false // let start fail with an explicit error message
	}
	if t.CheckCarrier {
		if carrier, err := t.hasCarrier(); err == nil && carrier == false && !actioncontext.IsForce(ctx) {
			t.Log().Errorf("interface %s no-carrier.", t.IPDev)
			return true
		}
	}
	if t.abortPing() {
		return true
	}
	return false
}

func (t *T) hasCarrier() (bool, error) {
	return netif.HasCarrier(t.IPDev)
}

func (t *T) abortPing() bool {
	ip := t.ipaddr()
	pinger, err := ping.NewPinger(ip.String())
	if err != nil {
		t.Log().Errorf("abort: ping: %s", err)
		return true
	}
	pinger.Count = 5
	pinger.Timeout = 5 * time.Second
	pinger.Interval = time.Second
	t.Log().Infof("checking %s availability (5s)", ip)
	pinger.Run()
	return pinger.Statistics().PacketsRecv > 0
}

func (t *T) ipnet() *net.IPNet {
	if t._ipnet != nil {
		return t._ipnet
	}
	t._ipnet = t.getIPNet()
	return t._ipnet
}

func (t *T) ipaddr() net.IP {
	if t._ipaddr != nil {
		return t._ipaddr
	}
	t._ipaddr = t.getIPAddr()
	return t._ipaddr
}

func (t *T) ipmask() net.IPMask {
	if t._ipmask != nil {
		return t._ipmask
	}
	t._ipmask = t.getIPMask()
	return t._ipmask
}

func (t *T) getIPNet() *net.IPNet {
	return &net.IPNet{
		IP:   t.ipaddr(),
		Mask: t.ipmask(),
	}
}

func (t *T) getIPMask() net.IPMask {
	ip := t.ipaddr()
	bits := getIPBits(ip)
	if m, err := parseCIDRMask(t.Netmask, bits); err == nil {
		return m
	}
	if m, err := parseDottedMask(t.Netmask); err == nil {
		return m
	}
	// fallback to the mask of the first found ip on the intf
	if m, err := t.defaultMask(); err == nil {
		return m
	}
	return nil
}

func (t *T) defaultMask() (net.IPMask, error) {
	intf, err := t.netInterface()
	if err != nil {
		return nil, err
	}
	addrs, err := intf.Addrs()
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addr to guess mask from")
	}
	_, net, err := net.ParseCIDR(addrs[0].String())
	if err != nil {
		return nil, err
	}
	return net.Mask, nil
}

func (t *T) getIPAddr() net.IP {
	switch {
	case naming.IsValidFQDN(t.IPName) || hostname.IsValid(t.IPName):
		var (
			l   []net.IP
			err error
		)
		l, err = net.LookupIP(t.IPName)
		if err != nil {
			t.Log().Errorf("%s", err)
			return nil
		}
		n := len(l)
		switch n {
		case 0:
			t.Log().Errorf("ipname %s is unresolvable", t.IPName)
		case 1:
			// ok
		default:
			t.Log().Debugf("ipname %s is resolvables to %d address. Using the first.", t.IPName, n)
		}
		return l[0]
	default:
		return net.ParseIP(t.IPName)
	}
}

func (t *T) netInterface() (*net.Interface, error) {
	return net.InterfaceByName(t.IPDev)
}

func (t Addrs) Has(ip net.IP) bool {
	for _, addr := range t {
		listIP, _, _ := net.ParseCIDR(addr.String())
		if ip.Equal(listIP) {
			return true
		}
	}
	return false
}

func parseCIDRMask(s string, bits int) (net.IPMask, error) {
	if bits == 0 {
		return nil, errors.New("invalid bits: 0")
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return nil, fmt.Errorf("invalid element in dotted mask: %s", err)
	}
	return net.CIDRMask(i, bits), nil
}

func parseDottedMask(s string) (net.IPMask, error) {
	m := []byte{}
	l := strings.Split(s, ".")
	if len(l) != 4 {
		return nil, errors.New("invalid number of elements in dotted mask")
	}
	for _, e := range l {
		i, err := strconv.Atoi(e)
		if err != nil {
			return nil, fmt.Errorf("invalid element in dotted mask: %s", err)
		}
		m = append(m, byte(i))
	}
	return m, nil
}

func ipv4MaskString(m []byte) string {
	if len(m) != 4 {
		panic("ipv4Mask: len must be 4 bytes")
	}

	return fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
}

func getIPBits(ip net.IP) (bits int) {
	switch {
	case ip.To4() != nil:
		bits = 32
	case ip.To16() != nil:
		bits = 128
	}
	return
}

func (t *T) arpAnnounce() error {
	ip := t.ipaddr()
	if ip.IsLoopback() {
		t.Log().Debugf("skip arp announce on loopback address %s", ip)
		return nil
	}
	if ip.IsLinkLocalUnicast() {
		t.Log().Debugf("skip arp announce on link local unicast address %s", ip)
		return nil
	}
	if ip.To4() == nil {
		t.Log().Debugf("skip arp announce on non-ip4 address %s", ip)
		return nil
	}
	if i, err := t.netInterface(); err == nil && i.Flags&net.FlagLoopback != 0 {
		t.Log().Debugf("skip arp announce on loopback interface %s", t.IPDev)
		return nil
	}
	t.Log().Infof("send gratuitous arp to announce %s over %s", t.ipaddr(), t.IPDev)
	return t.arpGratuitous()
}

func (t *T) start() error {
	ipnet := t.ipnet()
	if ipnet.Mask == nil {
		err := fmt.Errorf("ipnet definition error: %s/%s", t.ipaddr(), t.ipmask())
		t.Log().Errorf("%s", err)
		return err
	}
	t.Log().Infof("add %s to %s", ipnet, t.IPDev)
	return netif.AddAddr(t.IPDev, ipnet)
}

func (t *T) stop() error {
	ipnet := t.ipnet()
	if ipnet.Mask == nil {
		err := fmt.Errorf("ipnet definition error: %s/%s", t.ipaddr(), t.ipmask())
		t.Log().Errorf("%s", err)
		return err
	}
	t.Log().Infof("delete %s from %s", t.ipnet(), t.IPDev)
	return netif.DelAddr(t.IPDev, t.ipnet())
}
