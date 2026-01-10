package resiphost

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resip"
	"github.com/opensvc/om3/v3/util/duration"
	"github.com/opensvc/om3/v3/util/getaddr"
	"github.com/opensvc/om3/v3/util/netif"
	"github.com/opensvc/om3/v3/util/ping"
)

const (
	tagNonRouted = "nonrouted"
	maxIPAddrAge = 16 * time.Minute
)

type (
	T struct {
		resource.T

		Path       naming.Path
		ObjectFQDN string
		DNS        []string

		// config
		Name         string         `json:"name"`
		Dev          string         `json:"dev"`
		Netmask      string         `json:"netmask"`
		Network      string         `json:"network"`
		Gateway      string         `json:"gateway"`
		Provisioner  string         `json:"provisioner"`
		CheckCarrier bool           `json:"check_carrier"`
		Alias        bool           `json:"alias"`
		Expose       []string       `json:"expose"`
		WaitDNS      *time.Duration `json:"wait_dns"`

		// cache
		_ipaddr    net.IP
		_ipaddrAge time.Duration
		_ipmask    net.IPMask
		_ipnet     *net.IPNet
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
	data["dev"] = t.Dev
	data["netmask"] = netmask
	return data
}

func (t *T) getDevAndLabel() (string, string, error) {
	dev, idx := resip.SplitDevLabel(t.Dev)
	label := ""
	if idx == "" {
		if !t.Alias {
			// ip#0.dev = eth0
			// ip#0.alias = false
			// => allocate a label
			if s, err := resip.AllocateDevLabel(dev); err != nil {
				return "", "", err
			} else {
				label = s
			}
		}
	} else {
		// ip#0.dev = eth0:0
		label = t.Dev
	}
	return dev, label, nil
}

func (t *T) Start(ctx context.Context) error {
	if initialStatus := t.Status(ctx); initialStatus == status.Up {
		t.Log().Infof("%s is already up on %s", t.Name, t.Dev)
		return nil
	}
	if t._ipaddrAge > maxIPAddrAge {
		return fmt.Errorf("ip %s lookup issue, cache expired (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	} else if t._ipaddrAge > 0 {
		t.Log().Warnf("ip %s lookup issue, cache valid (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	}
	dev, label, err := t.getDevAndLabel()
	if err != nil {
		return err
	}
	if err := t.start(dev, label); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.stopAddr(ctx, dev)
	})
	if err := t.arpAnnounce(dev); err != nil {
		return err
	}
	if err := resip.WaitDNSRecord(ctx, t.WaitDNS, t.ObjectFQDN, t.DNS); err != nil {
		return err
	}
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if t._ipaddrAge > maxIPAddrAge {
		return fmt.Errorf("ip %s lookup issue, cache expired (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	} else if t._ipaddrAge > 0 {
		t.Log().Warnf("ip %s lookup issue, cache valid (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	}
	dev, _ := resip.SplitDevLabel(t.Dev)
	if err := t.stopAddr(ctx, dev); err != nil {
		return err
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	if t.Name == "" {
		t.StatusLog().Warn("name not set")
		return status.NotApplicable
	}
	if t.Dev == "" {
		t.StatusLog().Warn("dev not set")
		return status.NotApplicable
	}
	dev, _ := resip.SplitDevLabel(t.Dev)
	if t.statusOfCarrier(ctx, dev) == status.Down {
		return status.Down
	}
	return t.statusOfAddr(ctx, dev)
}

func (t *T) statusOfCarrier(ctx context.Context, dev string) status.T {
	if !t.CheckCarrier {
		return status.NotApplicable
	}
	if carrier, err := t.hasCarrier(); err == nil && carrier == false {
		t.StatusLog().Error("interface %s no-carrier.", dev)
		return status.Down
	} else if err != nil {
		t.StatusLog().Warn("carrier: %s", err)
		return status.Undef
	}
	return status.Up
}

func (t *T) statusOfAddr(ctx context.Context, dev string) status.T {
	var (
		i     *net.Interface
		err   error
		addrs Addrs
	)
	if t.Name == "" {
		return status.NotApplicable
	}
	ip := t.ipaddr()
	if t._ipaddrAge > maxIPAddrAge {
		t.StatusLog().Error("ip %s lookup issue, cache expired (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
		return status.Undef
	} else if t._ipaddrAge > 0 {
		t.StatusLog().Warn("ip %s lookup issue, cache valid (%s old)", t.Name, duration.FmtShortDuration(t._ipaddrAge))
	}
	if i, err = net.InterfaceByName(dev); err != nil {
		if fmt.Sprint(err.(*net.OpError).Unwrap()) == "no such network interface" {
			t.StatusLog().Warn("interface %s not found", dev)
		} else {
			t.StatusLog().Error("%s", err)
		}
		return status.Down
	}
	if addrs, err = i.Addrs(); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Down
	}
	if !addrs.Has(ip) {
		t.Log().Tracef("ip not found on intf")
		return status.Down
	}
	if t._ipaddrAge > 0 {
		return status.Warn
	}
	return status.Up
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t *T) Abort(ctx context.Context) bool {
	if t.Tags.Has(tagNonRouted) || t.IsActionDisabled() {
		return false // let start fail with an explicit error message
	}
	if t.ipaddr() == nil {
		return false // let start fail with an explicit error message
	}
	if t._ipaddrAge > maxIPAddrAge {
		return false // let start fail with an explicit error message
	}
	if initialStatus := t.Status(ctx); initialStatus == status.Up {
		return false // let start fail with an explicit error message
	}
	if t.CheckCarrier {
		if carrier, err := t.hasCarrier(); err == nil && carrier == false && !actioncontext.IsForce(ctx) {
			t.Log().Errorf("abort! interface %s no-carrier.", t.Dev)
			return true
		}
	}
	if t.abortPing() {
		return true
	}
	return false
}

func (t *T) hasCarrier() (bool, error) {
	return netif.HasCarrier(t.Dev)
}

func (t *T) abortPing() bool {
	timeout := 5 * time.Second
	ip := t.ipaddr().String()
	t.Log().Infof("abort? checking %s availability with ping (%s)", ip, timeout)
	isAlive, err := ping.Ping(ip, timeout)
	if err != nil {
		t.Log().Errorf("abort? ping failed: %s", err)
		return true
	}
	if isAlive {
		t.Log().Errorf("abort! %s is alive", ip)
		return true
	} else {
		t.Log().Tracef("abort? %s is not alive", ip)
		return false
	}
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
	ip, age, err := getaddr.Lookup(t.Name)
	if getaddr.IsErrManyAddr(err) {
		t.StatusLog().Warn("%s", err)
	}
	t._ipaddr = ip
	t._ipaddrAge = age
	return ip
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
	intf, err := net.InterfaceByName(t.Dev)
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

func (t *T) arpAnnounce(dev string) error {
	ip := t.ipaddr()
	if ip.IsLoopback() {
		t.Log().Tracef("skip arp announce on loopback address %s", ip)
		return nil
	}
	if ip.IsLinkLocalUnicast() {
		t.Log().Tracef("skip arp announce on link local unicast address %s", ip)
		return nil
	}
	if ip.To4() == nil {
		t.Log().Tracef("skip arp announce on non-ip4 address %s", ip)
		return nil
	}
	if i, err := net.InterfaceByName(dev); err == nil && i.Flags&net.FlagLoopback != 0 {
		t.Log().Tracef("skip arp announce on loopback interface %s", t.Dev)
		return nil
	}
	t.Log().Infof("send gratuitous arp to announce %s over %s", t.ipaddr(), dev)
	return t.arpGratuitous(dev)
}

func (t *T) ipmaskOnes() int {
	ones, _ := t.ipmask().Size()
	return ones
}

func (t *T) start(dev, label string) error {
	addr := fmt.Sprintf("%s/%d", t.ipaddr(), t.ipmaskOnes())
	return t.addrAdd(addr, dev, label)
}

func (t *T) stopAddr(ctx context.Context, dev string) error {
	if t.statusOfAddr(ctx, dev) == status.Down {
		t.Log().Infof("%s is already down on %s", t.Name, t.Dev)
		return nil
	}
	addr := fmt.Sprintf("%s/%d", t.ipaddr(), t.ipmaskOnes())
	return t.addrDel(addr, t.Dev)
}
