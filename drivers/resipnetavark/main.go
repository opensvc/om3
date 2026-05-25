//go:build linux

package resipnetavark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/actionresdeps"
	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/drivers/resip"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/file"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/google/uuid"
	"github.com/vishvananda/netns"
)

var (
	NetavarkConfigDir = "/etc/containers/networks/"
)

type (
	T struct {
		resource.T
		resource.Restart

		Path naming.Path
		DNS  []string

		// config
		Expose        []string `json:"expose"`
		NetNS         string   `json:"netns"`
		NSDev         string   `json:"nsdev"`
		Network       string   `json:"network"`
		ObjectID      uuid.UUID
		ObjectFQDN    string
		WaitDNS       *time.Duration `json:"wait_dns"`
		DNSNameSuffix string
	}

	Addrs []net.Addr

	response struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
)

var (
	ErrNoIPAddrAvail = errors.New("no ip address available")
	ErrDupIPAlloc    = errors.New("duplicate ip allocation")
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) netavarkFile() string {
	candidates := []string{
		"/usr/lib/podman/netavark",
		"/usr/libexec/podman/netavark",
		"/usr/local/bin/netavark",
		"/usr/bin/netavark",
	}
	for _, s := range candidates {
		bin := s
		if file.Exists(bin) {
			return bin
		}
	}
	return ""
}

// NetNSPath returns the value of the netavark netns path
func (t *T) NetNSPath(ctx context.Context) (string, error) {
	if t.NetNS != "" {
		return t.getResourceNSPathCtx(ctx)
	} else {
		return t.getObjectNSPIDFile()
	}
}

// getNetavarkNetNS returns the value of the netavark netns path
func (t *T) getNetavarkNetNSCtx(ctx context.Context) (string, error) {
	if t.NetNS != "" {
		return t.getResourceNSPathCtx(ctx)
	} else {
		return t.getObjectNSPIDFile()
	}
}

// getNetavarkContainerID returns the value of the container ID for netavark commands
func (t *T) getNetavarkContainerID(ctx context.Context) (string, error) {
	if t.NetNS != "" {
		return t.getResourceNSPID(ctx)
	} else {
		return t.getObjectNSPID()
	}
}

func (t *T) objectNSPID() string {
	return t.ObjectID.String()
}

func (t *T) objectNSPIDFile() string {
	return "/var/run/netns/" + t.objectNSPID()
}

func (t *T) getObjectNSPID() (string, error) {
	return t.objectNSPID(), nil
}

func (t *T) getObjectNSPIDFile() (string, error) {
	return t.objectNSPIDFile(), nil
}

func (t *T) getResourceHostname(ctx context.Context) (string, error) {
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if i, ok := r.(resource.GetHostnamer); !ok {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a hostname", t.NetNS)
	} else {
		return i.GetHostname(), nil
	}
}

func (t *T) getResourceNSPID(ctx context.Context) (string, error) {
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if i, ok := r.(resource.PIDer); !ok {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a pid", t.NetNS)
	} else {
		return fmt.Sprint(i.PID(ctx)), nil
	}
}

func (t *T) getResourceNSPathCtx(ctx context.Context) (string, error) {
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if o, ok := r.(resource.NetNSPather); ok {
		return o.NetNSPath(ctx)
	} else {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a netns path", t.NetNS)
	}
}

func (t *T) getResourceNSPath(ctx context.Context) (string, error) {
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if i, ok := r.(resource.NetNSPather); !ok {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a netns path", t.NetNS)
	} else {
		return i.NetNSPath(ctx)
	}
}

func (t *T) getResourceContainerID(ctx context.Context) (string, error) {
	type containerIDer interface {
		ContainerID(context.Context) string
	}
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if i, ok := r.(containerIDer); !ok {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a ContainerID function", t.NetNS)
	} else {
		return i.ContainerID(ctx), nil
	}
}

func (t *T) getResourceContainerName(ctx context.Context) (string, error) {
	type containerNamer interface {
		ContainerName() string
	}
	if r := t.GetObjectDriver().ResourceByID(t.NetNS); r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	} else if i, ok := r.(containerNamer); !ok {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a ContainerName function", t.NetNS)
	} else {
		return i.ContainerName(), nil
	}
}

func (t *T) getNS(ctx context.Context) (ns.NetNS, error) {
	if path, err := t.NetNSPath(ctx); err != nil {
		return nil, err
	} else if path == "" {
		return nil, nil
	} else {
		return ns.GetNS(path)
	}
}

func (t *T) getNSCtx(ctx context.Context) (ns.NetNS, error) {
	if path, err := t.getNetavarkNetNSCtx(ctx); err != nil {
		return nil, err
	} else if path == "" {
		return nil, nil
	} else {
		return ns.GetNS(path)
	}
}

func (t *T) hasNetNS() bool {
	if t.NetNS != "" {
		return true
	}
	if _, err := netns.GetFromPath(t.objectNSPIDFile()); err != nil {
		return false
	}
	return true
}

func (t *T) purgeNetavarkVarWithNetNS(ns string) error {
	// Netavark doesn't use the same var directory structure as CNI
	// For netavark, we might need to clean up different files
	// This is a placeholder for netavark-specific cleanup
	return nil
}

func (t *T) addObjectNetNS() error {
	if t.NetNS != "" {
		// the container is expected to already have a netns. don't even care to log info.
		return nil
	}
	nsPID := t.objectNSPID()
	if t.hasNetNS() {
		t.Log().Infof("netns %s already added", nsPID)
		return nil
	}
	if err := t.purgeNetavarkVarWithNetNS(nsPID); err != nil {
		return err
	}
	cmd := command.New(
		command.WithName("ip"),
		command.WithVarArgs("netns", "add", nsPID),
		command.WithLogger(t.Log()),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithCommandLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) delObjectNetNS() error {
	if t.NetNS != "" {
		// the container is expected to already have a netns. don't even care to log info.
		return nil
	}
	nsPID := t.objectNSPID()
	if !t.hasNetNS() {
		t.Log().Infof("netns %s already deleted", nsPID)
		return nil
	}
	cmd := command.New(
		command.WithName("ip"),
		command.WithVarArgs("netns", "del", nsPID),
		command.WithLogger(t.Log()),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithCommandLogLevel(zerolog.InfoLevel),
	)
	return cmd.Run()
}

func (t *T) purgeNetavarkVarDir() error {
	// Netavark doesn't use the same var directory structure as CNI
	// This is a placeholder for netavark-specific cleanup
	return nil
}

func (t *T) purgeNetavarkVarFile(p string) error {
	// Netavark doesn't use the same var directory structure as CNI
	// This is a placeholder for netavark-specific cleanup
	return nil
}

func (t *T) purgeNetavarkVarFileWithIP(ip net.IP) error {
	// Netavark doesn't use the same var directory structure as CNI
	// This is a placeholder for netavark-specific cleanup
	return nil
}

// StatusInfo implements resource.StatusInfoer
func (t *T) StatusInfo(ctx context.Context) map[string]interface{} {
	data := make(map[string]interface{})
	if ip, _, err := t.ipNet(ctx); (err == nil) && (len(ip) > 0) {
		data["ipaddr"] = ip.String()
	}
	data["expose"] = t.Expose
	if hostname, _ := t.getResourceHostname(ctx); hostname != "" {
		if t.DNSNameSuffix != "" {
			hostname += t.DNSNameSuffix
		}
		data["hostname"] = hostname
	}
	return data
}

func (t *T) ActionResourceDeps() []actionresdeps.Dep {
	return []actionresdeps.Dep{
		{Action: "start", A: t.RID(), B: t.NetNS},
		{Action: "start", A: t.NetNS, B: t.RID()},
		{Action: "stop", A: t.NetNS, B: t.RID()},
	}
}

func (t *T) Start(ctx context.Context) error {
	if t.Status(ctx) == status.Up {
		t.Log().Infof("already up")
		return nil
	}
	if err := t.addObjectNetNS(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.delObjectNetNS()
	})
	if err := t.start(ctx); err != nil {
		return err
	}
	if err := resip.WaitDNSRecord(ctx, t.WaitDNS, t.ObjectFQDN, t.DNS); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		// Don't use start context to stop (it may be already deadlined)
		return t.stop(nil)
	})
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if t.Status(ctx) == status.Down {
		t.Log().Infof("already down")
		return nil
	}
	if err := t.stop(ctx); err != nil {
		return err
	}
	if err := t.delObjectNetNS(); err != nil {
		return err
	}
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	netConf, err := t.netConf()
	if err != nil {
		t.StatusLog().Warn(fmt.Sprint(err))
		return status.Undef
	}
	netns, err := t.getNSCtx(ctx)
	if err != nil {
		return status.Down
	}
	if netns == nil {
		return status.Down
	}
	dev, err := t.currentGuestDev(netConf.Subnets[0].Subnet, netns)
	if err != nil {
		t.StatusLog().Warn("%s", err)
		return status.Undef
	}
	if dev == "" {
		return status.Down
	} else {
		return status.Up
	}
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(ctx context.Context) string {
	var s string
	if ip, ipnet, _ := t.ipNet(ctx); ipnet != nil {
		ones, _ := ipnet.Mask.Size()
		s = fmt.Sprintf("%s %s/%d in %s", t.Network, ip, ones, t.NetNS)
	} else {
		s = fmt.Sprintf("%s in %s", t.Network, t.NetNS)
	}
	return s
}

func (t *T) LinkTo() string {
	return t.NetNS
}

func (t *T) ipNet(ctx context.Context) (net.IP, *net.IPNet, error) {
	var (
		ipnet *net.IPNet
		netip net.IP
	)
	netns, err := t.getNS(ctx)
	if err != nil {
		return netip, ipnet, err
	}
	return t.nsIPNet(netns)
}

func (t *T) ipNetCtx(ctx context.Context) (net.IP, *net.IPNet, error) {
	var (
		ipnet *net.IPNet
		netip net.IP
	)
	netns, err := t.getNSCtx(ctx)
	if err != nil {
		return netip, ipnet, err
	}
	return t.nsIPNet(netns)
}

func (t *T) nsIPNet(netns ns.NetNS) (net.IP, *net.IPNet, error) {
	var (
		ipnet *net.IPNet
		ip    net.IP
	)
	if netns == nil {
		return ip, ipnet, nil
	}
	netConf, err := t.netConf()
	if err != nil {
		return ip, ipnet, err
	}
	_, ref, err := net.ParseCIDR(netConf.Subnets[0].Subnet)
	if err != nil {
		return ip, ipnet, err
	}
	if err := netns.Do(func(_ ns.NetNS) error {
		ifaces, err := net.Interfaces()
		if err != nil {
			return err
		}
		for _, iface := range ifaces {
			if t.NSDev != "" && iface.Name == t.NSDev {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				return err
			}
			if len(addrs) == 0 {
				continue
			}
			for _, addr := range addrs {
				candidateIP, candidateIPNet, err := net.ParseCIDR(addr.String())
				if err != nil {
					return err
				}
				if ref.Contains(candidateIP) {
					ip = candidateIP
					ipnet = candidateIPNet
					return nil
				}
			}

		}
		return nil
	}); err != nil {
		return ip, ipnet, err
	}
	return ip, ipnet, nil
}

func (t *T) netConfFile() string {
	return filepath.Join(NetavarkConfigDir, t.Network+".json")
}

func (t *T) netConfBytes() ([]byte, error) {
	s := t.netConfFile()
	if s == "" {
		return nil, fmt.Errorf("netavark config file not found for network %s: %s", t.Network, s)
	}
	return os.ReadFile(s)
}

func (t *T) netRequestBytes(ctx context.Context, netNSPath string) ([]byte, error) {
	netConf, err := t.netConf()
	if err != nil {
		return nil, err
	}

	dev, err := t.newGuestDev(netNSPath)
	if err != nil {
		return nil, err
	}

	containerID, err := t.getResourceContainerID(ctx)
	if err != nil {
		return nil, err
	}

	containerName, err := t.getResourceContainerName(ctx)
	if err != nil {
		return nil, err
	}

	containerHostname, err := t.getResourceHostname(ctx)
	if err != nil {
		return nil, err
	}

	request := Request{
		ContainerID:       containerID,
		ContainerName:     containerName,
		ContainerHostname: containerHostname,
		Networks:          make(map[string]PerNetworkOptions),
		NetworkInfo:       make(map[string]Network),
	}

	request.NetworkInfo[t.Network] = netConf
	request.Networks[t.Network] = PerNetworkOptions{
		StaticIPs:     []net.IP{},
		Aliases:       []string{},
		StaticMAC:     "",
		InterfaceName: dev,
		Options:       map[string]string{},
	}

	b, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	return b, nil
}

type (
	Request struct {
		ContainerID       string                       `json:"container_id,omitempty"`
		ContainerName     string                       `json:"container_name,omitempty"`
		ContainerHostname string                       `json:"container_hostname,omitempty"`
		Networks          map[string]PerNetworkOptions `json:"networks,omitempty"`
		NetworkInfo       map[string]Network           `json:"network_info,omitempty"`
		PortMappings      []Portmapping                `json:"port_mappings,omitempty"`
		DNSServers        []string                     `json:"dns_servers,omitempty"`
	}
	PerNetworkOptions struct {
		StaticIPs     []net.IP          `json:"static_ips,omitempty"`
		Aliases       []string          `json:"aliases,omitempty"`
		StaticMAC     string            `json:"static_mac,omitempty"`
		InterfaceName string            `json:"interface_name,omitempty"`
		Options       map[string]string `json:"options,omitempty"`
	}
	Portmapping struct {
		ContainerPort uint16 `json:"container_port"`
		HostIP        string `json:"host_ip,omitempty"`
		HostPort      uint16 `json:"host_port,omitempty"`
		Protocol      string `json:"protocol,omitempty"`
		Range         uint16 `json:"range,omitempty"`
	}
	Network struct {
		DNSEnabled        bool              `json:"dns_enabled"`
		Driver            string            `json:"driver,omitempty"`
		ID                string            `json:"id,omitempty"`
		Internal          bool              `json:"internal"`
		IPv6Enabled       bool              `json:"ipv6_enabled"`
		Name              string            `json:"name,omitempty"`
		NetworkInterface  string            `json:"network_interface,omitempty"`
		Options           map[string]string `json:"options,omitempty"`
		IPAMOptions       map[string]string `json:"ipam_options,omitempty"`
		Subnets           []Subnet          `json:"subnets,omitempty"`
		Routes            []Route           `json:"routes,omitempty"`
		NetworkDNSServers []net.IP          `json:"network_dns_servers,omitempty"`
	}
	Route struct {
		Gateway     net.IP `json:"gateway,omitempty"`
		Destination string `json:"destination,omitempty"`
		Metric      uint32 `json:"metric,omitempty"`
	}
	Subnet struct {
		Subnet     string      `json:"subnet,omitempty"`
		Gateway    net.IP      `json:"gateway,omitempty"`
		LeaseRange *LeaseRange `json:"lease_range,omitempty"`
	}
	LeaseRange struct {
		StartIP net.IP `json:"start_ip,omitempty"`
		EndIP   net.IP `json:"end_ip,omitempty"`
	}
)

func (t *T) netConf() (Network, error) {
	data := Network{}
	b, err := t.netConfBytes()
	if err != nil {
		return data, err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return data, err
	}
	return data, nil
}

func (t *T) stop(ctx context.Context) error {
	if ctx == nil {
		// TODO: introduce t.StopTimeout and use context.WithTimeout ?
		ctx = context.Background()
	}
	ip, _, _ := t.ipNet(ctx)
	netConf, err := t.netConf()
	if err != nil {
		return err
	}
	bin := t.netavarkFile()
	if bin == "" {
		return fmt.Errorf("netavark binary not found")
	}

	netns, err := t.getNS(ctx)
	if err != nil {
		return err
	}

	containerID, err := t.getNetavarkContainerID(ctx)
	if err != nil {
		return err
	}

	dev, err := t.currentGuestDev(netConf.Subnets[0].Subnet, netns)
	if err != nil {
		return err
	}

	// Netavark uses different commands than CNI
	// It typically uses "netavark teardown" instead of CNI_COMMAND=DEL
	args := []string{
		"teardown",
		"--network", t.Network,
		"--container-id", containerID,
		"--netns", netns.Path(),
		"--ifname", dev,
	}

	cmd := command.New(
		command.WithContext(ctx),
		command.WithName(bin),
		command.WithArgs(args),
		command.WithLogger(t.Log()),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithCommandLogLevel(zerolog.InfoLevel),
	)

	err = cmd.Run()
	if err != nil {
		return err
	}
	if t.purgeNetavarkVarFileWithIP(ip); err != nil {
		return err
	}
	return nil
}

func (t *T) start(ctx context.Context) error {
	bin := t.netavarkFile()
	if bin == "" {
		return fmt.Errorf("netavark binary not found")
	}

	netNSPath, err := t.NetNSPath(ctx)
	if err != nil {
		return err
	}

	args := []string{
		"setup",
		netNSPath,
	}

	input, err := t.netRequestBytes(ctx, netNSPath)
	if err != nil {
		return err
	}

	run := func() error {
		var outB, errB []byte
		cmd := command.New(
			command.WithName(bin),
			command.WithArgs(args),
			command.WithLogger(t.Log()),
			command.WithBufferedStdout(),
			command.WithBufferedStderr(),
		)
		t.Log().Attr("input", input).Infof("%s %s", bin, strings.Join(args, " "))

		cmd.Cmd().Stdin = bytes.NewReader(input)
		err := cmd.Run()
		outB = cmd.Stdout()
		errB = cmd.Stderr()

		if len(outB) > 0 {
			t.Log().Infof(string(outB))
		}
		if len(errB) > 0 {
			t.Log().Infof(string(errB))
		}

		var resp response
		if err := json.Unmarshal(outB, &resp); err != nil {
			return err
		}
		if resp.Code == 0 {
			return nil
		}
		if strings.Contains(resp.Msg, "no IP addresses available") {
			return ErrNoIPAddrAvail
		}
		if strings.Contains(resp.Msg, "duplicate allocation") {
			return ErrDupIPAlloc
		}
		return fmt.Errorf("netavark error code %d msg %s: %w", resp.Code, resp.Msg, err)
	}

	err = run()
	switch {
	case err == nil:
	case errors.Is(err, ErrNoIPAddrAvail), errors.Is(err, ErrDupIPAlloc):
		t.Log().Infof("clean allocations and retry: %s", err)
		t.purgeNetavarkVarDir()

		// clean run leftovers (container veth name provided (eth12) already exists)
		// use nil context, start context may be deadlined
		t.stop(nil) // clean run leftovers (container veth name provided (eth12) already exists)
		err = run()
	default:
		t.Log().Errorf("%s", err)
	}
	return err
}

func getInterfaceAndAddr(ref *net.IPNet) (net.Interface, net.Addr, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return net.Interface{}, nil, err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return net.Interface{}, nil, err
		}
		for _, addr := range addrs {
			ip := net.ParseIP(strings.Split(addr.String(), "/")[0])
			if ref.Contains(ip) {
				return iface, addr, nil
			}
		}
	}
	return net.Interface{}, nil, nil
}

func (t *T) currentGuestDev(cidr string, netns ns.NetNS) (string, error) {
	if netns == nil {
		return "", fmt.Errorf("can't get current guest dev from nil netns")
	}
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}
	var s string
	if err := netns.Do(func(_ ns.NetNS) error {
		if iface, _, err := getInterfaceAndAddr(ipNet); err != nil {
			return err
		} else {
			s = iface.Name
		}
		return nil
	}); err != nil {
		return "", err
	}
	return s, nil
}

func (t *T) newGuestDev(path string) (string, error) {
	if t.NSDev != "" {
		return t.NSDev, nil
	}
	devs, err := getLinkStringsIn(path)
	if err != nil {
		return "", err
	}
	i := 0
	for {
		name := fmt.Sprintf("eth%d", i)
		if !slices.Contains(devs, name) {
			return name, nil
		}
		i = i + 1
		if i > math.MaxUint16 {
			break
		}
	}
	return "", fmt.Errorf("can't find a free link name")
}

func getLinkStringsIn(path string) ([]string, error) {
	l := make([]string, 0)
	buff, err := linkListIn(path)
	if err != nil {
		return l, err
	}
	for _, line := range strings.Split(buff, "\n") {
		words := strings.Fields(line)
		if len(words) < 2 {
			continue
		}
		if !strings.HasSuffix(words[1], ":") {
			continue
		}
		dev := strings.TrimRight(words[1], ":")
		dev = strings.Split(dev, "@")[0]
		l = append(l, dev)
	}
	return l, nil
}

func linkListIn(path string) (string, error) {
	args := []string{"nsenter", "--net=" + path, "ip", "link", "list"}
	cmd := command.New(
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithBufferedStdout(),
	)
	err := cmd.Run()
	return string(cmd.Stdout()), err
}
