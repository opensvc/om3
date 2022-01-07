package resipcni

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os/exec"
	"path/filepath"

	"opensvc.com/opensvc/core/actionresdeps"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/file"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/google/uuid"
	"github.com/vishvananda/netns"
)

const (
	driverGroup = drivergroup.IP
	driverName  = "cni"
)

type (
	T struct {
		resource.T

		// config
		NetNS      string `json:"netns"`
		NSDev      string `json:"nsdev"`
		Network    string `json:"network"`
		CNIConfig  string
		CNIPlugins string
		ObjectID   uuid.UUID
	}

	Addrs []net.Addr
)

func init() {
	resource.Register(driverGroup, driverName, New)
}

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

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddContext([]manifest.Context{
		{
			Key:  "cni_plugins",
			Attr: "CNIPlugins",
			Ref:  "cni.plugins",
		},
		{
			Key:  "cni_config",
			Attr: "CNIConfig",
			Ref:  "cni.config",
		},
		{
			Key:  "object_id",
			Attr: "ObjectID",
			Ref:  "object.id",
		},
	}...)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "network",
			Attr:     "Network",
			Scopable: true,
			Default:  "default",
			Example:  "my-weave-net",
			Text:     "The name of the CNI network to plug into. The default network is created using the host-local bridge plugin if no existing configuration already exists.",
		},
		{
			Option:   "nsdev",
			Attr:     "NSDev",
			Scopable: true,
			Default:  "eth12",
			Aliases:  []string{"ipdev"},
			Example:  "front",
			Text:     "The interface name in the container namespace.",
		},
		{
			Option:   "netns",
			Attr:     "NetNS",
			Scopable: true,
			Aliases:  []string{"container_rid"},
			Example:  "container#0",
			Text:     "The resource id of the container to plumb the ip into.",
		},
	}...)
	return m
}

func (t T) getNSPID() (string, error) {
	if t.NetNS == "" {
		return t.NSPIDFile(), nil
	}
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

func (t T) getNS() (ns.NetNS, error) {
	if t.NetNS == "" {
		return ns.GetNS(t.NSPIDFile())
	}
	r := t.GetObjectDriver().ResourceByID(t.NetNS)
	if r == nil {
		return nil, fmt.Errorf("resource %s pointed by the netns keyword not found", t.NetNS)
	}
	i, ok := r.(resource.NetNSPather)
	if !ok {
		return nil, fmt.Errorf("resource %s pointed by the netns keyword does not expose a netns path", t.NetNS)
	}
	path, err := i.NetNSPath()
	if err != nil {
		return nil, err
	}
	return ns.GetNS(path)
}

func (t T) NSPID() string {
	return t.ObjectID.String()
}

func (t T) NSPIDFile() string {
	return "/var/run/netns/" + t.NSPID()
}

func (t T) hasNetNS() bool {
	if t.NetNS != "" {
		return true
	}
	if _, err := netns.GetFromPath(t.NSPIDFile()); err != nil {
		return false
	}
	return true
}

func (t T) addNetNS() error {
	if t.NetNS != "" {
		// the container is expected to already have a netns. don't even care to log info.
		return nil
	}
	nsPIDFile := t.NSPIDFile()
	if t.hasNetNS() {
		t.Log().Info().Msgf("netns %s already added", nsPIDFile)
		return nil
	}
	if _, err := netns.NewNamed(nsPIDFile); err != nil {
		return err
	}
	return nil
}

func (t T) delNetNS() error {
	if t.NetNS != "" {
		// the container is expected to already have a netns. don't even care to log info.
		return nil
	}
	if !t.hasNetNS() {
		t.Log().Info().Msgf("netns %s already deleted", t.NSPIDFile())
		return nil
	}
	_ = netns.DeleteNamed(t.NSPID())
	return nil
}

func (t *T) StatusInfo() map[string]interface{} {
	data := make(map[string]interface{})
	if ip, _, err := t.ipnet(); err == nil {
		data["ipaddr"] = ip.String()
	}
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
		{Action: "start", Kind: actionresdeps.KindSelect, A: t.RID(), B: t.NetNS},
		{Action: "start", Kind: actionresdeps.KindSelect, A: t.NetNS, B: t.RID()},
		{Action: "stop", Kind: actionresdeps.KindSelect, A: t.NetNS, B: t.RID()},
		{Action: "start", Kind: actionresdeps.KindAct, A: t.RID(), B: t.NetNS},
		{Action: "stop", Kind: actionresdeps.KindAct, A: t.NetNS, B: t.RID()},
	}
}

func (t *T) Start(ctx context.Context) error {
	if t.Status(ctx) == status.Up {
		t.Log().Info().Msg("already up")
		return nil
	}
	if err := t.start(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.stop()
	})
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	if _, err := t.netConf(); err != nil {
		t.StatusLog().Warn(fmt.Sprint(err))
	}
	hasNetNS := t.hasNetNS()
	if t.NetNS == "" && !hasNetNS {
		return status.Down
	}
	if _, ipnet, err := t.ipnet(); err != nil {
		t.StatusLog().Warn("%s", err)
		return status.Undef
	} else if ipnet != nil {
		return status.Up
	} else {
		return status.Down
	}
}

func (t T) Label() string {
	var s string
	if ip, ipnet, err := t.ipnet(); err == nil {
		ones, _ := ipnet.Mask.Size()
		s = fmt.Sprintf("%s %s/%d", t.Network, ip, ones)
	} else {
		s = fmt.Sprintf("%s", t.Network)
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

func (t T) ipnet() (net.IP, *net.IPNet, error) {
	var (
		ipnet *net.IPNet
		netip net.IP
	)
	netns, err := t.getNS()
	if err != nil {
		return netip, ipnet, err
	}
	if err := netns.Do(func(_ ns.NetNS) error {
		if iface, err := net.InterfaceByName(t.NSDev); err != nil {
			return err
		} else if addrs, err := iface.Addrs(); err != nil {
			return err
		} else if len(addrs) == 0 {
			return nil
		} else if netip, ipnet, err = net.ParseCIDR(addrs[0].String()); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return netip, ipnet, err
	}
	return netip, ipnet, nil
}

func (t T) netConfFile() string {
	return filepath.Join(t.CNIConfig, t.Network+".conf")
}

func (t T) netConfBytes() ([]byte, error) {
	s := t.netConfFile()
	return ioutil.ReadFile(s)
}

func (t T) netConf() (types.NetConf, error) {
	data := types.NetConf{}
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
	if err := t.delNetNS(); err != nil {
		return err
	}
	return nil
}

func (t T) start() error {
	if err := t.addNetNS(); err != nil {
		return err
	}
	netConf, err := t.netConf()
	if err != nil {
		return err
	}
	plugin := t.pluginFile(netConf.Type)
	if plugin == "" {
		return fmt.Errorf("plugin %s not found", netConf.Type)
	}
	bin := t.pluginFile(netConf.Type)
	cmd := exec.Command(bin)

	nsPath, err := t.getNS()
	if err != nil {
		return err
	}

	nsPID, err := t.getNSPID()
	if err != nil {
		return err
	}

	//args := fmt.Sprintf("DEBUG=%s;FOO=BAR", debugFileName)
	cmd.Env = []string{
		"CNI_COMMAND=ADD",
		fmt.Sprintf("CNI_CONTAINERID=%s", nsPID),
		fmt.Sprintf("CNI_NETNS=%s", nsPath),
		fmt.Sprintf("CNI_IFNAME=%s", t.NSDev),
		fmt.Sprintf("CNI_PATH=%s", filepath.Dir(plugin)),
		// Keep this last
		//"CNI_ARGS=" + args,
	}

	// `{"name": "noop-test", "some":"stdin-json", "cniVersion": "0.3.1"}`
	stdinData, err := t.netConfBytes()
	if err != nil {
		return err
	}
	cmd.Stdin = bytes.NewReader(stdinData)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
