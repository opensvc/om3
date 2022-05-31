//go:build linux
// +build linux

package resipcni

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"

	"opensvc.com/opensvc/core/actionresdeps"
	"opensvc.com/opensvc/core/actionrollback"
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/command"
	"opensvc.com/opensvc/util/file"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/google/uuid"
	"github.com/vishvananda/netns"
)

const (
	driverGroup = driver.GroupIP
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
	m.AddKeyword(manifest.ProvisioningKeywords...)
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

// getCNINetNS returns the value of the CNI_NETNS env var of cni commands
func (t T) getCNINetNS() (string, error) {
	if t.NetNS != "" {
		return t.getResourceNSPath()
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
		t.Log().Info().Msgf("netns %s already added", nsPID)
		return nil
	}
	t.Log().Info().Msgf("create new netns %s", nsPID)
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
		t.Log().Info().Msgf("netns %s already deleted", nsPIDFile)
		return nil
	}
	_ = netns.DeleteNamed(t.objectNSPID())
	return nil
}

func (t *T) StatusInfo() map[string]interface{} {
	data := make(map[string]interface{})
	if ip, _, err := t.ipNet(); err == nil {
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
	if err := t.addObjectNetNS(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.delObjectNetNS()
	})
	if err := t.start(); err != nil {
		return err
	}
	actionrollback.Register(ctx, func() error {
		return t.stop()
	})
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if t.Status(ctx) == status.Down {
		t.Log().Info().Msg("already down")
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
	if _, err := t.netConf(); err != nil {
		t.StatusLog().Warn(fmt.Sprint(err))
	}
	netns, err := t.getNS()
	if err != nil {
		return status.Down
	}
	if netns == nil {
		return status.Down
	}
	if netip, ipnet, err := t.nsIPNet(netns); err != nil {
		t.StatusLog().Warn("%s", err)
		return status.Undef
	} else if ipnet == nil {
		t.StatusLog().Warn("%s not found", t.NSDev)
		return status.Down
	} else if len(netip) == 0 {
		t.StatusLog().Info("ip not found")
		return status.Down
	} else {
		return status.Up
	}
}

func (t T) Label() string {
	var s string
	if ip, ipnet, _ := t.ipNet(); ipnet != nil {
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

func (t T) nsIPNet(netns ns.NetNS) (net.IP, *net.IPNet, error) {
	var (
		ipnet *net.IPNet
		netip net.IP
	)
	if netns == nil {
		return netip, ipnet, nil
	}
	if err := netns.Do(func(_ ns.NetNS) error {
		var iface *net.Interface
		ifaces, err := net.Interfaces()
		if err != nil {
			return err
		}
		for _, i := range ifaces {
			if i.Name == t.NSDev {
				// found
				iface = &i
				break
			}
		}
		if iface == nil {
			// not found. not an error, because we want a clean Down state
			return nil
		}
		if addrs, err := iface.Addrs(); err != nil {
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

	containerID, err := t.getCNIContainerID()
	if err != nil {
		return err
	}

	env := []string{
		"CNI_COMMAND=DEL",
		fmt.Sprintf("CNI_CONTAINERID=%s", containerID),
		fmt.Sprintf("CNI_NETNS=%s", cniNetNS),
		fmt.Sprintf("CNI_IFNAME=%s", t.NSDev),
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
	t.Log().Info().
		Stringer("cmd", cmd.Cmd()).
		Str("input", string(stdinData)).
		Strs("env", env).
		Msg("run")
	err = cmd.Run()
	if outB := cmd.Stdout(); len(outB) > 0 {
		t.Log().Info().Msg(string(outB))
	}
	if errB := cmd.Stderr(); len(errB) > 0 {
		t.Log().Info().Msg(string(errB))
	}
	if err != nil {
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

	containerID, err := t.getCNIContainerID()
	if err != nil {
		return err
	}

	env := []string{
		"CNI_COMMAND=ADD",
		fmt.Sprintf("CNI_CONTAINERID=%s", containerID),
		fmt.Sprintf("CNI_NETNS=%s", cniNetNS),
		fmt.Sprintf("CNI_IFNAME=%s", t.NSDev),
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
	t.Log().Info().
		Stringer("cmd", cmd.Cmd()).
		Str("input", string(stdinData)).
		Strs("env", env).
		Msg("run")
	err = cmd.Run()
	if outB := cmd.Stdout(); len(outB) > 0 {
		t.Log().Info().Msg(string(outB))
	}
	if errB := cmd.Stderr(); len(errB) > 0 {
		t.Log().Info().Msg(string(errB))
	}
	if err != nil {
		return err
	}
	return nil

}
