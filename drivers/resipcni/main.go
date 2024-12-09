//go:build linux

package resipcni

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
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/core/actionresdeps"
	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/resip"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/file"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/google/uuid"
	"github.com/vishvananda/netns"
)

type (
	T struct {
		resource.T

		Path naming.Path
		DNS  []string

		// config
		Expose     []string `json:"expose"`
		NetNS      string   `json:"netns"`
		NSDev      string   `json:"nsdev"`
		Network    string   `json:"network"`
		CNIConfig  string
		CNIPlugins string
		ObjectID   uuid.UUID
		ObjectFQDN string
		WaitDNS    *time.Duration `json:"wait_dns"`
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

func (t T) pluginFile(plugin string) string {
	candidates := []string{
		t.CNIPlugins,
		"/usr/lib/cni",
		"/usr/libexec/cni",
	}
	for _, s := range candidates {
		bin := filepath.Join(s, plugin)
		if file.Exists(bin) {
			return bin
		}
	}
	return ""
}

// getCNINetNS returns the value of the CNI_NETNS env var of cni commands
func (t T) getCNINetNS() (string, error) {
	if t.NetNS != "" {
		return t.getResourceNSPath()
	} else {
		return t.getObjectNSPIDFile()
	}
}

func (t T) getCNINetNSCtx(ctx context.Context) (string, error) {
	if t.NetNS != "" {
		return t.getResourceNSPathCtx(ctx)
	} else {
		return t.getObjectNSPIDFile()
	}
}

// getCNIContainerID returns the value of the CNI_CONTAINERID env var of cni commands
func (t T) getCNIContainerID() (string, error) {
	if t.NetNS != "" {
		return t.getResourceNSPID()
	} else {
		return t.getObjectNSPID()
	}
}

func (t T) objectNSPID() string {
	return t.ObjectID.String()
}

func (t T) objectNSPIDFile() string {
	return "/var/run/netns/" + t.objectNSPID()
}

func (t T) getObjectNSPID() (string, error) {
	return t.objectNSPID(), nil
}

func (t T) getObjectNSPIDFile() (string, error) {
	return t.objectNSPIDFile(), nil
}

func (t T) getResourceNSPID() (string, error) {
	r := t.GetObjectDriver().ResourceByID(t.NetNS)
	if r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	}
	i, ok := r.(resource.PIDer)
	if !ok {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a pid", t.NetNS)
	}
	return fmt.Sprint(i.PID()), nil
}

func (t T) getResourceNSPathCtx(ctx context.Context) (string, error) {
	r := t.GetObjectDriver().ResourceByID(t.NetNS)
	if r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	}
	if o, ok := r.(resource.NetNSPathCtxer); ok {
		return o.NetNSPathCtx(ctx)
	} else if o, ok := r.(resource.NetNSPather); ok {
		return o.NetNSPath()
	}
	return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a netns path", t.NetNS)
}

func (t T) getResourceNSPath() (string, error) {
	r := t.GetObjectDriver().ResourceByID(t.NetNS)
	if r == nil {
		return "", fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	}
	i, ok := r.(resource.NetNSPather)
	if !ok {
		return "", fmt.Errorf("resource %s pointed by the netns keyword does not expose a netns path", t.NetNS)
	}
	return i.NetNSPath()
}

func (t T) getNS() (ns.NetNS, error) {
	if path, err := t.getCNINetNS(); err != nil {
		return nil, err
	} else if path == "" {
		return nil, nil
	} else {
		return ns.GetNS(path)
	}
}

func (t T) getNSCtx(ctx context.Context) (ns.NetNS, error) {
	if path, err := t.getCNINetNSCtx(ctx); err != nil {
		return nil, err
	} else if path == "" {
		return nil, nil
	} else {
		return ns.GetNS(path)
	}
}

func (t T) hasNetNS() bool {
	if t.NetNS != "" {
		return true
	}
	if _, err := netns.GetFromPath(t.objectNSPIDFile()); err != nil {
		return false
	}
	return true
}

func (t T) addObjectNetNS() error {
	if t.NetNS != "" {
		// the container is expected to already have a netns. don't even care to log info.
		return nil
	}
	nsPID := t.objectNSPID()
	if t.hasNetNS() {
		t.Log().Infof("netns %s already added", nsPID)
		return nil
	}
	t.Log().Infof("create new netns %s", nsPID)
	if _, err := netns.NewNamed(nsPID); err != nil {
		return err
	}
	return nil
}

func (t T) delObjectNetNS() error {
	if t.NetNS != "" {
		// the container is expected to already have a netns. don't even care to log info.
		return nil
	}
	nsPIDFile := t.objectNSPIDFile()
	if !t.hasNetNS() {
		t.Log().Infof("netns %s already deleted", nsPIDFile)
		return nil
	}
	_ = netns.DeleteNamed(t.objectNSPID())
	return nil
}

func (t T) purgeCNIVarDir() error {
	pattern := fmt.Sprintf("/var/lib/cni/networks/%s/*.*.*.*", t.Network)
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, p := range paths {
		if err := t.purgeCNIVarFile(p); err != nil {
			return err
		}
	}
	return nil
}

func (t T) purgeCNIVarFile(p string) error {
	buff, err := os.ReadFile(p)
	if err != nil {
		return err
	}

	line := strings.Fields(string(buff))[0]
	_, err = strconv.Atoi(line)
	if _, err := strconv.Atoi(line); err != nil {
		runNetNSFile := fmt.Sprintf("/run/netns/%s", line)
		if _, err := os.Stat(runNetNSFile); err == nil || !errors.Is(err, os.ErrNotExist) {
			// the process is still alive, don't remove
			return nil
		}
	} else {
		pidFile := fmt.Sprintf("/proc/%s", line)
		if _, err := os.Stat(pidFile); err == nil || !errors.Is(err, os.ErrNotExist) {
			// the process is still alive, don't remove
			return nil
		}
	}
	if err = os.Remove(p); err == nil {
		t.Log().Infof("removed %s: %s no longer exist", p, line)
	} else if err != nil {
		return err
	}
	return nil
}

func (t T) purgeCNIVarFileWithIP(ip net.IP) error {
	p := fmt.Sprintf("/var/lib/cni/networks/%s/%s", t.Network, ip)
	err := os.Remove(p)
	switch {
	case err == nil:
		t.Log().Infof("removed %s", p)
		return nil
	case errors.Is(err, os.ErrNotExist):
		return nil
	default:
		return err
	}
}

func (t *T) StatusInfo() map[string]interface{} {
	data := make(map[string]interface{})
	if ip, _, err := t.ipNet(); (err == nil) && (len(ip) > 0) {
		data["ipaddr"] = ip.String()
	}
	data["expose"] = t.Expose
	/*
	   if self.container:
	       if self.container.vm_hostname != self.container.name:
	           data["hostname"] = self.container.vm_hostname
	       else:
	           data["hostname"] = self.container.name
	       if self.dns_name_suffix:
	           data["hostname"] += self.dns_name_suffix
	*/
	return data
}

func (t T) ActionResourceDeps() []actionresdeps.Dep {
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
	actionrollback.Register(ctx, func() error {
		return t.delObjectNetNS()
	})
	if err := t.start(); err != nil {
		return err
	}
	if err := resip.WaitDNSRecord(ctx, t.WaitDNS, t.ObjectFQDN, t.DNS); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.stop()
	})
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if t.Status(ctx) == status.Down {
		t.Log().Infof("already down")
		return nil
	}
	if err := t.stop(); err != nil {
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
	dev, err := t.currentGuestDev(netConf.IPAM.Subnet, netns)
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

func (t T) Label() string {
	var s string
	if ip, ipnet, _ := t.ipNet(); ipnet != nil {
		ones, _ := ipnet.Mask.Size()
		s = fmt.Sprintf("%s %s/%d in %s", t.Network, ip, ones, t.NetNS)
	} else {
		s = fmt.Sprintf("%s in %s", t.Network, t.NetNS)
	}
	return s
}

func (t T) LabelCtx(ctx context.Context) string {
	var s string
	if ip, ipnet, _ := t.ipNetCtx(ctx); ipnet != nil {
		ones, _ := ipnet.Mask.Size()
		s = fmt.Sprintf("%s %s/%d in %s", t.Network, ip, ones, t.NetNS)
	} else {
		s = fmt.Sprintf("%s in %s", t.Network, t.NetNS)
	}
	return s
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) LinkTo() string {
	return t.NetNS
}

func (t T) ipNet() (net.IP, *net.IPNet, error) {
	var (
		ipnet *net.IPNet
		netip net.IP
	)
	netns, err := t.getNS()
	if err != nil {
		return netip, ipnet, err
	}
	return t.nsIPNet(netns)
}

func (t T) ipNetCtx(ctx context.Context) (net.IP, *net.IPNet, error) {
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

func (t T) nsIPNet(netns ns.NetNS) (net.IP, *net.IPNet, error) {
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
	_, ref, err := net.ParseCIDR(netConf.IPAM.Subnet)
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

func (t T) netConfFile() string {
	return filepath.Join(t.CNIConfig, t.Network+".conf")
}

func (t T) netConfBytes() ([]byte, error) {
	s := t.netConfFile()
	return os.ReadFile(s)
}

type (
	NetConf struct {
		Type string
		IPAM IPAM
	}
	IPAM struct {
		Subnet string
	}
)

func (t T) netConf() (NetConf, error) {
	data := NetConf{}
	b, err := t.netConfBytes()
	if err != nil {
		return data, err
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return data, err
	}
	return data, nil
}

func (t T) stop() error {
	ip, _, _ := t.ipNet()
	netConf, err := t.netConf()
	if err != nil {
		return err
	}
	plugin := t.pluginFile(netConf.Type)
	if plugin == "" {
		return fmt.Errorf("plugin %s not found", netConf.Type)
	}
	bin := t.pluginFile(netConf.Type)

	netns, err := t.getNS()
	if err != nil {
		return err
	}

	containerID, err := t.getCNIContainerID()
	if err != nil {
		return err
	}

	dev, err := t.currentGuestDev(netConf.IPAM.Subnet, netns)
	if err != nil {
		return err
	}
	env := []string{
		"CNI_COMMAND=DEL",
		fmt.Sprintf("CNI_CONTAINERID=%s", containerID),
		fmt.Sprintf("CNI_NETNS=%s", netns.Path()),
		fmt.Sprintf("CNI_IFNAME=%s", dev),
		fmt.Sprintf("CNI_PATH=%s", filepath.Dir(plugin)),
	}

	cmd := command.New(
		command.WithName(bin),
		command.WithEnv(env),
		command.WithLogger(t.Log()),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
	)

	// {"name": "noop-test", "cniVersion": "0.3.1", ...}
	stdinData, err := t.netConfBytes()
	if err != nil {
		return err
	}
	cmd.Cmd().Stdin = bytes.NewReader(stdinData)
	t.Log().
		Attr("cmd", cmd.Cmd().String()).
		Attr("input", string(stdinData)).
		Attr("env", env).
		Infof("del cni network %s ip from container %s interface %s", t.Network, containerID, dev)
	err = cmd.Run()
	if outB := cmd.Stdout(); len(outB) > 0 {
		var resp response
		if err := json.Unmarshal(outB, &resp); err == nil && resp.Code != 0 {
			msg := fmt.Sprintf("cni error code %d: %s", resp.Code, resp.Msg)
			t.Log().Errorf(msg)
			return fmt.Errorf(msg)
		} else {
			t.Log().Infof(string(outB))
		}
	}
	if errB := cmd.Stderr(); len(errB) > 0 {
		t.Log().Infof(string(errB))
	}
	if err != nil {
		return err
	}
	if t.purgeCNIVarFileWithIP(ip); err != nil {
		return err
	}
	return nil
}

func (t T) start() error {
	netConf, err := t.netConf()
	if err != nil {
		return err
	}
	plugin := t.pluginFile(netConf.Type)
	if plugin == "" {
		return fmt.Errorf("plugin %s not found", netConf.Type)
	}
	bin := t.pluginFile(netConf.Type)

	cniNetNS, err := t.getCNINetNS()
	if err != nil {
		return err
	}

	dev, err := t.newGuestDev(cniNetNS)
	if err != nil {
		return err
	}

	containerID, err := t.getCNIContainerID()
	if err != nil {
		return err
	}

	env := []string{
		"CNI_COMMAND=ADD",
		fmt.Sprintf("CNI_CONTAINERID=%s", containerID),
		fmt.Sprintf("CNI_NETNS=%s", cniNetNS),
		fmt.Sprintf("CNI_IFNAME=%s", dev),
		fmt.Sprintf("CNI_PATH=%s", filepath.Dir(plugin)),
	}

	// {"name": "noop-test", "cniVersion": "0.3.1", ...}
	stdinData, err := t.netConfBytes()
	if err != nil {
		return err
	}
	run := func() error {
		var outB, errB []byte
		cmd := command.New(
			command.WithName(bin),
			command.WithEnv(env),
			command.WithLogger(t.Log()),
			command.WithBufferedStdout(),
			command.WithBufferedStderr(),
		)
		t.Log().
			Attr("cmd", cmd.Cmd().String()).
			Attr("input", string(stdinData)).
			Attr("env", env).
			Infof("add cni network %s ip from container %s interface %s", t.Network, containerID, dev)

		cmd.Cmd().Stdin = bytes.NewReader(stdinData)
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
		return fmt.Errorf("cni error code %d msg %s: %w", resp.Code, resp.Msg, err)
	}

	err = run()
	switch {
	case err == nil:
	case errors.Is(err, ErrNoIPAddrAvail), errors.Is(err, ErrDupIPAlloc):
		t.Log().Infof("clean allocations and retry: %s", err)
		t.purgeCNIVarDir()
		t.stop() // clean run leftovers (container veth name provided (eth12) already exists)
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

func (t T) currentGuestDev(cidr string, netns ns.NetNS) (string, error) {
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

func (t T) newGuestDev(path string) (string, error) {
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
