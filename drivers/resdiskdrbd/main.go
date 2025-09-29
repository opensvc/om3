//go:build linux

package resdiskdrbd

import (
	"context"
	// Necessary to use go:embed
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/actionrollback"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/network"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/drivers/resdisk"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/drbd"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
)

type (
	T struct {
		resdisk.T
		Path     naming.Path `json:"path"`
		Nodes    []string    `json:"nodes"`
		Res      string      `json:"res"`
		Disk     string      `json:"disk"`
		MaxPeers int         `json:"max_peers"`
		Addr     string      `json:"addr"`
		Port     int         `json:"port"`
		Network  string      `json:"network"`
		Template string      `json:"template"`
	}
	DRBDDriver interface {
		Adjust(context.Context) error
		Attach(context.Context) error
		Connect(context.Context, string) error
		ConnStates(context.Context) (drbd.CStateM, error)
		ConnState(context.Context, string) (string, error)
		CreateMD(context.Context, int) error
		DetachForce(context.Context) error
		Disconnect(context.Context) error
		DiskStates(context.Context) ([]string, error)
		Down(context.Context) error
		HasMD(context.Context) (bool, error)
		IsDefined(context.Context) (bool, error)
		Primary(context.Context) error
		PrimaryForce(context.Context) error
		Role(context.Context) (string, error)
		Secondary(context.Context) error
		Up(context.Context) error
		WipeMD(context.Context) error
		WaitCState(ctx context.Context, nodeID string, timeout time.Duration, candidates ...string) (string, error)
		WaitConnectingOrConnected(ctx context.Context, nodeID string) (string, error)
		StartConnections(context.Context, ...string) error
		Show(ctx context.Context) (drbd.DrbdShow, error)
	}

	// ResTemplateData represents template data for a resource configuration, it is exported (public)
	// to help template designers to use it.
	//
	// It defines the available fields that template designers can use.
	// Example usage in a template:
	//     resource {{.Name}} {
	//        {{range $node := .Nodes}}
	//        on {{$node.Name}} {
	//            device    {{$node.Device}};
	//            disk      {{$node.Disk}};
	//            meta-disk internal;
	//            address   {{$node.Addr}};
	//            node-id   {{$node.NodeId}};
	//        }
	//        {{end}}
	//        connection-mesh {
	//            hosts{{range $node := .Nodes}} {{$node.Name}}{{end}};
	//        }
	//        net {
	//            rr-conflict retry-connect;
	//        }
	//    }
	ResTemplateData struct {
		Name  string
		Nodes []NodeTemplateData
	}

	// NodeTemplateData represents a structure to hold template data for individual nodes,
	// including their name, address, device, and disk information.
	// It is exported (public) to help template designers to use it.
	NodeTemplateData struct {
		Name   string
		Addr   string
		Device string
		Disk   string
		NodeId int
	}
)

var (
	// waitConnectedTimeout defines the maximum duration to wait for a connection
	// state change to connected before timing out.
	waitConnectedTimeout = time.Second * 20

	waitDiskStatesDelay   = time.Second * 1
	waitDiskStatesTimeout = time.Second * 20

	MaxNodes = 32

	//go:embed text/template/res9
	resTemplateTextV9 string

	//go:embed text/template/res8
	resTemplateTextV8 string

	drbdCfgPath = naming.Path{Name: "drbd", Namespace: "system", Kind: naming.KindCfg}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t *T) Name() string {
	if t.Path.Namespace != "root" {
		return fmt.Sprintf(
			"%s.%s.%s",
			strings.ToLower(t.Path.Namespace),
			strings.Split(t.Path.Name, ".")[0],
			strings.ReplaceAll(t.RID(), "#", "."),
		)
	} else {
		return fmt.Sprintf(
			"%s.%s",
			strings.Split(t.Path.Name, ".")[0],
			strings.ReplaceAll(t.RID(), "#", "."),
		)
	}
}

func (t *T) Info(_ context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "res", Value: t.Res},
	}
	return m, nil
}

func (t *T) Connect(ctx context.Context, nodeID string) error {
	return t.drbd(ctx).Connect(ctx, nodeID)
}

// DownForce is called by the unprovisioner. Dataloss is not an issue there,
// so forced detach can be tried.
func (t *T) DownForce(ctx context.Context) error {
	dev := t.drbd(ctx)
	if err := dev.Disconnect(ctx); err != nil {
		return err
	}
	if err := dev.DetachForce(ctx); err != nil {
		return err
	}
	if err := dev.Down(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Down(ctx context.Context) error {
	dev := t.drbd(ctx)
	if err := dev.Down(ctx); err != nil {
		return err
	}
	// flush devtree caches
	return nil
}

// Up function brings up the DRBD device and waits until its state is stable and
// not in a diskless configuration.
func (t *T) Up(ctx context.Context) error {
	dev := t.drbd(ctx)
	if err := dev.Up(ctx); err != nil {
		return err
	}
	if err := t.waitForNonLocalDiskless(ctx, dev); err != nil {
		return err
	}
	// flush devtree caches
	return nil
}

func (t *T) GoSecondary(ctx context.Context) error {
	dev := t.drbd(ctx)
	role, err := dev.Role(ctx)
	if err != nil {
		return err
	}
	if role == "Secondary" {
		return nil
	}
	if err := dev.Secondary(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return dev.Primary(ctx)
	})
	return nil
}

func (t *T) isConfigured() bool {
	cf := drbd.ResConfigFile(t.Res)
	return file.Exists(cf)
}

func (t *T) StopStandby(ctx context.Context) error {
	if !t.isConfigured() {
		t.Log().Infof("skip: resource not configured")
		return nil
	}
	dev := t.drbd(ctx)
	if ok, err := dev.IsDefined(ctx); err != nil {
		return err
	} else if !ok {
		t.Log().Infof("skip: resource not defined (for this host)")
		return nil
	}
	if err := dev.StartConnections(ctx); err != nil {
		return fmt.Errorf("start connection: %s", err)
	}
	return t.GoSecondary(ctx)
}

func (t *T) StartStandby(ctx context.Context) error {
	dev := t.drbd(ctx)
	if err := t.prepareUp(ctx, dev); err != nil {
		return err
	}
	if err := dev.StartConnections(ctx); err != nil {
		return fmt.Errorf("start connection: %s", err)
	}
	role, err := dev.Role(ctx)
	if err != nil {
		return err
	}
	if role == "Primary" {
		return nil
	}
	return dev.Secondary(ctx)
}

func (t *T) Start(ctx context.Context) error {
	if !t.isConfigured() {
		t.Log().Infof("skip: resource not configured")
		return nil
	}
	dev := t.drbd(ctx)
	if err := t.prepareUp(ctx, dev); err != nil {
		return err
	}
	if err := dev.StartConnections(ctx); err != nil {
		return fmt.Errorf("start connection: %s", err)
	}
	role, err := dev.Role(ctx)
	if err != nil {
		return err
	}
	if role == "Primary" {
		return nil
	}
	if err := dev.Primary(ctx); err != nil {
		return err
	}
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return dev.Secondary(ctx)
	})
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	if !t.isConfigured() {
		t.Log().Infof("skip: resource not configured")
		return nil
	}
	dev := t.drbd(ctx)
	if ok, err := dev.IsDefined(ctx); err != nil {
		return err
	} else if !ok {
		t.Log().Infof("skip: resource not defined (for this host)")
		return nil
	}
	return t.Down(ctx)
}

func (t *T) Shutdown(ctx context.Context) error {
	if !t.isConfigured() {
		t.Log().Infof("skip: resource not configured")
		return nil
	}
	dev := t.drbd(ctx)
	if ok, err := dev.IsDefined(ctx); err != nil {
		return err
	} else if !ok {
		t.Log().Infof("skip: resource not defined (for this host)")
		return nil
	}
	return t.DownForce(ctx)
}

func (t *T) removeHolders() error {
	for _, dev := range t.ExposedDevices() {
		if err := dev.RemoveHolders(); err != nil {
			return nil
		}
	}
	return nil
}

func (t *T) StatusInfo(context.Context) map[string]any {
	return map[string]any{
		"res": t.Res,
	}
}

func (t *T) Status(ctx context.Context) status.T {
	dev := t.drbd(ctx)
	isDefined, err := dev.IsDefined(ctx)
	if err != nil {
		t.StatusLog().Error("defined: %s", err)
		return status.Undef
	}
	if !isDefined {
		return status.Down
	}
	role, err := dev.Role(ctx)
	if err != nil {
		t.StatusLog().Error("role: %s", err)
		return status.Undef
	}
	t.StatusLog().Info(role)

	states, err := dev.DiskStates(ctx)
	if err != nil {
		t.StatusLog().Error("dstates: %s", err)
		return status.Undef
	}
	resourceStatus := status.Undef
	for i, state := range states {
		if i == 0 {
			switch state {
			case "Diskless", "DUnknown", "Unconfigured":
				resourceStatus = status.Down
			}
		} else {
			switch state {
			case "UpToDate":
			default:
				t.StatusLog().Warn("unexpected drbd resource %s/%d state: %s", t.Res, i, state)
			}
		}
	}
	if resourceStatus != status.Undef {
		return resourceStatus
	}

	switch role {
	case "Primary":
		return status.Up
	case "Secondary":
		return status.StandbyUp
	default:
		t.StatusLog().Warn("unexpected drbd resource %s role: %s", t.Res, role)
		return status.Warn
	}
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.Res
}

// UnprovisionStop is a noop to avoid calling the normal Stop before unprovision
func (t *T) UnprovisionStop(_ context.Context) error {
	return nil
}

// ProvisionStart is a noop to avoid calling the normal Start after provision
func (t *T) ProvisionStart(_ context.Context) error {
	return nil
}

func (t *T) getDRBDAllocations() (map[string]api.DRBDAllocation, error) {
	allocations := make(map[string]api.DRBDAllocation)
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	for _, nodename := range t.Nodes {
		resp, err := c.GetNodeDRBDAllocationWithResponse(context.Background(), nodename)
		switch {
		case err != nil:
			return nil, err
		case resp.StatusCode() == 500:
			return nil, fmt.Errorf("get node %s drbd allocations: %s", nodename, resp.JSON500)
		case resp.StatusCode() == 200:
			allocations[nodename] = *resp.JSON200
		default:
			return nil, fmt.Errorf("get node %s drbd allocations: unexpected status code %d", nodename, resp.StatusCode())
		}
	}
	return allocations, nil
}

func (t *T) formatConfig(wr io.Writer, text string, data ResTemplateData) error {
	if text == "" {
		return fmt.Errorf("empty template")
	}
	templ, err := template.New("res").Parse(text)
	if err != nil {
		return err
	}
	return templ.Execute(wr, data)
}

func (t *T) getTemplateData(allocations map[string]api.DRBDAllocation) (ResTemplateData, error) {
	nodesData := make([]NodeTemplateData, 0)
	obj := t.GetObject().(object.Configurer)
	for nodeID, nodename := range t.Nodes {
		var (
			disk, addr, addrDrbd string

			port int
		)
		allocation, ok := allocations[nodename]
		if !ok {
			return ResTemplateData{}, fmt.Errorf("drbd allocation for node %s not found", nodename)
		}
		if time.Now().After(allocation.ExpiredAt) {
			return ResTemplateData{}, fmt.Errorf("drbd allocation for node %s has expired", nodename)
		}
		dev := fmt.Sprintf("/dev/drbd%d", allocation.Minor)
		if s, err := obj.Config().EvalAs(key.T{Section: t.RID(), Option: "disk"}, nodename); err != nil {
			return ResTemplateData{}, err
		} else {
			disk = s.(string)
		}

		if s, err := obj.Config().EvalAs(key.T{Section: t.RID(), Option: "addr"}, nodename); (err != nil) || (s == "") {
			if ip, err := t.getNodeIP(nodename); err != nil {
				return ResTemplateData{}, err
			} else {
				addr = ip.String()
			}
		} else {
			addr = s.(string)
		}

		if i, err := obj.Config().EvalAs(key.T{Section: t.RID(), Option: "port"}, nodename); err != nil {
			// EvalAs will error because the port kw has no default
			port = allocation.Port
		} else {
			// TODO: remove to not let the user bug ?
			port = i.(int)
		}

		ip := net.ParseIP(addr)
		if ip.To4() == nil {
			addrDrbd = fmt.Sprintf("ipv6 [%s]:%d", ip, port)
		} else {
			addrDrbd = fmt.Sprintf("ipv4 %s:%d", ip, port)
		}

		nodeData := NodeTemplateData{
			Name:   nodename,
			Addr:   addrDrbd,
			Disk:   disk,
			Device: dev,
			NodeId: nodeID,
		}
		nodesData = append(nodesData, nodeData)
	}
	return ResTemplateData{Name: t.Res, Nodes: nodesData}, nil
}

func (t *T) getTemplateText() (string, error) {
	if t.Template == "" {
		if capabilities.Has("drivers.resource.disk.drbd.mesh") {
			return resTemplateTextV9, nil
		} else {
			return resTemplateTextV8, nil
		}
	}
	t.Log().Infof("creating resource configuration from the %s %s template", drbdCfgPath, t.Template)
	if drbdCfg, err := object.NewCfg(drbdCfgPath, object.WithVolatile(true)); err != nil {
		return "", fmt.Errorf("retrieve template object %s: %w", drbdCfg, err)
	} else if !drbdCfg.HasKey(t.Template) {
		return "", fmt.Errorf("missing template object %s key %s", drbdCfg, t.Template)
	} else if b, err := drbdCfg.DecodeKey(t.Template); err != nil {
		return "", fmt.Errorf("decode template object %s key %s: %w", drbdCfgPath, t.Template, err)
	} else {
		return string(b), nil
	}
}

func (t *T) getNodeIP(nodename string) (net.IP, error) {
	if t.Network != "" {
		return t.getNodeIPWithNetwork(nodename)
	} else {
		return t.getNodeIPWithGetAddrInfo(nodename)
	}
}

func (t *T) getNodeIPWithNetwork(nodename string) (net.IP, error) {
	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	nws := network.Networks(node)
	for _, nw := range nws {
		if nw.Name() != t.Network {
			continue
		}
		if ip, err := nw.NodeSubnetIP(nodename); err != nil {
			return nil, err
		} else {
			return ip, nil
		}
	}
	return nil, fmt.Errorf("node %s ip not found on network %s", nodename, t.Network)
}

func (t *T) getNodeIPWithGetAddrInfo(nodename string) (net.IP, error) {
	ips, err := net.LookupIP(nodename)
	if err != nil {
		return nil, err
	}
	n := len(ips)
	switch n {
	case 0:
		return nil, fmt.Errorf("name %s is unresolvable", nodename)
	case 1:
		// ok
	default:
		t.Log().Debugf("name %s is resolvables to %d address. Using the first.", nodename, n)
	}
	return ips[0], nil

}

// TODO: Acquire/Release cluster lock
func (t *T) lock(_ context.Context) error {
	//lockName := "drivers.resources.disk.drbd.allocate"
	return nil
}

// TODO: Acquire/Release cluster lock
func (t *T) unlock(_ context.Context) error {
	return nil
}

func (t *T) fetchConfigFromNode(nodename string) ([]byte, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	params := api.GetNodeDRBDConfigParams{
		Name: t.Res,
	}
	resp, err := c.GetNodeDRBDConfigWithResponse(context.Background(), nodename, &params)
	if err != nil {
		return nil, err
	} else if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected get node drbd config status code %s", resp.Status())
	}
	return resp.JSON200.Data, nil
}

func (t *T) fetchConfig() error {
	cf := drbd.ResConfigFile(t.Res)
	if file.Exists(cf) {
		t.Log().Infof("%s already exists", cf)
		return nil
	}
	for _, nodename := range t.Nodes {
		b, err := t.fetchConfigFromNode(nodename)
		if err != nil {
			continue
		}
		err = os.WriteFile(cf, b, os.ModePerm)
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("failed to fetch %s, tried node %s", cf, t.Nodes)
}

func (t *T) writeConfig(ctx context.Context) error {
	var templateText string
	cf := drbd.ResConfigFile(t.Res)
	if file.Exists(cf) {
		t.Log().Infof("%s already exists", cf)
		return nil
	}
	if err := t.lock(ctx); err != nil {
		return err
	}
	defer func() {
		_ = t.unlock(ctx)
	}()
	allocations, err := t.getDRBDAllocations()
	if err != nil {
		return err
	}
	templateData, err := t.getTemplateData(allocations)
	if err != nil {
		return err
	}

	templateText, err = t.getTemplateText()
	if err != nil {
		return fmt.Errorf("get template text: %w", err)
	}

	f, err := os.OpenFile(cf, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := t.formatConfig(f, templateText, templateData); err != nil {
		return err
	}
	b, err := os.ReadFile(cf)
	if err != nil {
		return err
	}
	if err := t.sendConfig(b, allocations); err != nil {
		return err
	}
	return nil
}

func (t *T) sendConfig(b []byte, allocations map[string]api.DRBDAllocation) error {
	for _, nodename := range t.Nodes {
		var allocationID uuid.UUID
		if nodename == hostname.Hostname() {
			continue
		}
		if a, ok := allocations[nodename]; ok {
			allocationID = a.ID
		} else {
			return fmt.Errorf("allocation id for node %s not found", nodename)
		}
		if err := t.sendConfigToNode(nodename, allocationID, b); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) sendConfigToNode(nodename string, allocationID uuid.UUID, b []byte) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.PostNodeDRBDConfigParams{
		Name: t.Res,
	}
	body := api.PostNodeDRBDConfigRequest{
		AllocationID: allocationID,
		Data:         b,
	}
	resp, err := c.PostNodeDRBDConfigWithResponse(context.Background(), nodename, &params, body)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		return nil
	case 400:
		return fmt.Errorf("%s", resp.JSON400)
	case 401:
		return fmt.Errorf("%s", resp.JSON401)
	case 403:
		return fmt.Errorf("%s", resp.JSON403)
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %s", resp.Status())
	}
}

// connectPeers establishes connections to all peer nodes except the current host and logs the connection attempts or errors.
func (t *T) connectPeers(ctx context.Context, nodeIDs ...string) error {
	var peerNodeID string
	data, err := t.drbd(ctx).Show(ctx)
	if err != nil {
		return fmt.Errorf("drbd %s show: %w", t.Res, err)
	}
	localNodeID := fmt.Sprintf("%d", data.ThisHost.NodeID)
	var errs error
	for _, c := range data.Connections {
		peerNodeID = fmt.Sprintf("%d", c.PeerNodeID)
		if c.Cstate == drbd.ConnStateConnected {
			continue
		}
		if len(nodeIDs) == 0 || slices.Contains(nodeIDs, peerNodeID) {
			peerNode := c.Net.Name
			t.Log().Infof("resource %s connecting peer %s to localhost with node-id %s", t.Res, peerNode, localNodeID)
			if err := t.connectPeer(ctx, peerNode, localNodeID); err != nil {
				errs = errors.Join(errs, fmt.Errorf("connect peer %s to localhost node-id %s: %w", peerNode, peerNodeID, err))
			}
		}
	}
	return errs
}

// PreMove promotes the res to primary on move destination.
// The drbd needs to allow-two-primaries for both the source and
// destination to be primary at the same time during the move.
func (t *T) PreMove(ctx context.Context, to string) error {
	return t.primaryPeer(ctx, to)
}

func (t *T) primaryPeer(ctx context.Context, nodename string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.PostNodeDRBDPrimaryParams{
		Name: t.Res,
	}
	resp, err := c.PostNodeDRBDPrimaryWithResponse(ctx, nodename, &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 204:
		return nil
	case 400:
		return fmt.Errorf("%s", resp.JSON400)
	case 401:
		return fmt.Errorf("%s", resp.JSON401)
	case 403:
		return fmt.Errorf("%s", resp.JSON403)
	case 404:
		return nil
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %s", resp.Status())
	}
}

func (t *T) connectPeer(ctx context.Context, nodename, peerNodeID string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.PostNodeDRBDConnectParams{
		Name:   t.Res,
		NodeId: &peerNodeID,
	}
	resp, err := c.PostNodeDRBDConnectWithResponse(ctx, nodename, &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 204:
		return nil
	case 400:
		return fmt.Errorf("%s", resp.JSON400)
	case 401:
		return fmt.Errorf("%s", resp.JSON401)
	case 403:
		return fmt.Errorf("%s", resp.JSON403)
	case 404:
		return nil
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %s", resp.Status())
	}
}

func (t *T) ProvisionAsFollower(ctx context.Context) error {
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.UnprovisionAsFollower(ctx)
	})
	if err := t.fetchConfig(); err != nil {
		return err
	}
	if err := t.provisionCommon(ctx); err != nil {
		return err
	}
	if err := t.drbd(ctx).StartConnections(ctx); err != nil {
		return err
	}
	if err := t.waitPeersConnected(ctx); err != nil {
		t.Log().Warnf("drbd %s wait for peer connected: %s", t.Res, err)
		t.Log().Infof("drbd %s try reconnect peers", t.Res)
		if err := t.connectPeers(ctx); err != nil {
			return fmt.Errorf("drbd %s connect peers: %w", t.Res, err)
		}
		if err := t.waitPeersConnected(ctx); err != nil {
			return fmt.Errorf("drbd %s wait for peer connected failed after retry: %w", t.Res, err)
		}
	}
	return nil
}

func (t *T) ProvisionAsLeader(ctx context.Context) error {
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return t.UnprovisionAsLeader(ctx)
	})
	if err := t.writeConfig(ctx); err != nil {
		return err
	}
	if err := t.provisionCommon(ctx); err != nil {
		return err
	}
	if err := t.drbd(ctx).PrimaryForce(ctx); err != nil {
		return err
	}
	return t.drbd(ctx).StartConnections(ctx)
}

func (t *T) provisionCommon(ctx context.Context) error {
	if err := t.CreateMD(ctx); err != nil {
		return err
	}
	if err := t.Up(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) WipeMD(ctx context.Context) error {
	if v, err := t.drbd(ctx).HasMD(ctx); err != nil {
		return err
	} else if !v {
		t.Log().Infof("resource %s already has no metadata", t.Res)
		return nil
	}
	return t.drbd(ctx).WipeMD(ctx)
}

func (t *T) maxPeers() int {
	v := t.MaxPeers
	nNodes := len(t.Nodes)

	// minValue could be nNodes-1, but we want to add a slot to allow a server
	// swap.
	minValue := nNodes
	if minValue < 1 {
		minValue = 1
	}
	maxValue := MaxNodes - 1
	if v == 0 {
		v = (nNodes * 2) - 1
	}
	if v < minValue {
		v = minValue
	}
	if v > maxValue {
		v = maxValue
	}
	return v
}

func (t *T) CreateMD(ctx context.Context) error {
	if v, err := t.drbd(ctx).HasMD(ctx); err != nil {
		return err
	} else if v {
		t.Log().Infof("resource %s already has metadata", t.Res)
		return nil
	}
	return t.drbd(ctx).CreateMD(ctx, t.maxPeers())
}

func (t *T) deleteConfig() error {
	cf := drbd.ResConfigFile(t.Res)
	err := os.Remove(cf)
	if os.IsNotExist(err) {
		t.Log().Infof("%s already deleted", cf)
		return nil
	} else if err != nil {
		return err
	} else {
		t.Log().Infof("deleted %s", cf)
		return nil
	}
}

func (t *T) UnprovisionAsLeader(ctx context.Context) error {
	return t.unprovisionCommon(ctx)
}

func (t *T) UnprovisionAsFollower(ctx context.Context) error {
	return t.unprovisionCommon(ctx)
}

func (t *T) unprovisionCommon(ctx context.Context) error {
	isDefined, err := t.drbd(ctx).IsDefined(ctx)
	if err != nil {
		return err
	}
	if isDefined {
		if err := t.DownForce(ctx); err != nil {
			return err
		}
		if err := t.WipeMD(ctx); err != nil {
			return err
		}
	} else {
		t.Log().Infof("resource already not defined")
	}
	if err := t.deleteConfig(); err != nil {
		return err
	}
	return nil
}

func (t *T) Provisioned() (provisioned.T, error) {
	if !t.isConfigured() {
		return provisioned.False, nil
	}
	// TODO: allow Provisioned(ctx context.Context) ?
	ctx := context.Background()
	hasMD, err := t.drbd(ctx).HasMD(ctx)
	if err != nil {
		t.Log().Debugf("drbd res is not configured")
		return provisioned.Undef, err
	}
	if !hasMD {
		t.Log().Debugf("drbd disk has no metadata")
		return provisioned.False, nil
	}
	return provisioned.True, nil
}

func (t *T) ExposedDevices() device.L {
	l := make(device.L, 0)
	dump, err := drbd.GetConfig()
	if err != nil {
		return l
	}
	r, ok := dump.GetResource(t.Res)
	if !ok {
		return l
	}
	host, ok := r.GetHost(hostname.Hostname())
	if !ok {
		return l
	}
	for _, volume := range host.Volumes {
		l = append(l, device.New(volume.Device.Path))
	}
	return l
}

func (t *T) SubDevices() device.L {
	l := make(device.L, 0)
	dump, err := drbd.GetConfig()
	if err != nil {
		return l
	}
	r, ok := dump.GetResource(t.Res)
	if !ok {
		return l
	}
	host, ok := r.GetHost(hostname.Hostname())
	if !ok {
		return l
	}
	for _, volume := range host.Volumes {
		l = append(l, device.New(volume.Disk))
	}
	return l
}

func (t *T) ReservableDevices() device.L {
	return t.SubDevices()
}

func (t *T) ClaimedDevices() device.L {
	return t.SubDevices()
}

func (t *T) connectAndWaitConnectingOrConnected(ctx context.Context, nodeID string) (string, error) {
	if err := t.Connect(ctx, nodeID); err != nil {
		return "", fmt.Errorf("drbd resource %s node-id %s: connect: %w", t.Res, nodeID, err)
	}
	if err := t.connectPeers(ctx, nodeID); err != nil {
		t.Log().Warnf("drbd resource %s node-id %s: connect peers: %s", t.Res, nodeID, err)
	}
	return t.drbd(ctx).WaitConnectingOrConnected(ctx, nodeID)
}

func (t *T) prepareUp(ctx context.Context, dev DRBDDriver) error {
	if ok, err := dev.IsDefined(ctx); err != nil {
		return err
	} else if !ok {
		if err := t.Up(ctx); err != nil {
			return err
		}
	}
	if err := t.waitForNonLocalDiskless(ctx, dev); err != nil {
		return err
	}
	return nil
}

// waitPeersConnected ensures all the DRBD resource peer connections transition
// to the "Connected" state, attempting reconnection if in "StandAlone".
func (t *T) waitPeersConnected(ctx context.Context) error {
	t.Log().Infof("drbd %s wait for peer cstate Connected", t.Res)
	dev := t.drbd(ctx)
	states, err := dev.ConnStates(ctx)
	if err != nil {
		return err
	}
	errC := make(chan error, len(states))
	for nodeID, state := range states {
		if state == drbd.ConnStateConnected {
			t.Log().Infof("drbd %s node-id %s is in Connected state", t.Res, nodeID)
			errC <- nil
			continue
		}
		go func(c chan error) {
			if err := t.waitConnected(ctx, nodeID); err != nil {
				c <- fmt.Errorf("node-id %s: %w", nodeID, err)
			} else {
				t.Log().Infof("drbd %s node-id %s is in Connected state", t.Res, nodeID)
				c <- nil
			}
		}(errC)
	}
	for range states {
		err = errors.Join(<-errC)
	}
	return err
}

// waitConnected ensures the DRBD resource transitions to the "Connected" state,
// attempting reconnection if in "StandAlone".
func (t *T) waitConnected(ctx context.Context, nodeID string) error {
	dev := t.drbd(ctx)
	state, err := dev.WaitCState(ctx, nodeID, waitConnectedTimeout, drbd.ConnStateStandAlone, drbd.ConnStateConnected)
	if err != nil {
		return err
	} else if state == drbd.ConnStateConnected {
		return nil
	}

	// state is StandAlone
	t.Log().Infof("drbd %s node-id %s is in StandAlone state, trying to connect", t.Res, nodeID)
	state, err = t.connectAndWaitConnectingOrConnected(ctx, nodeID)
	if err != nil {
		return err
	} else if state == drbd.ConnStateConnected {
		return nil
	}

	state, err = dev.WaitCState(ctx, nodeID, waitConnectedTimeout, drbd.ConnStateConnected)
	if err != nil {
		return err
	} else if state == drbd.ConnStateConnected {
		return nil
	}
	return fmt.Errorf("cstate %s is not %s", state, drbd.ConnStateConnected)
}

func (t *T) waitForNonLocalDiskless(ctx context.Context, dev DRBDDriver) error {
	check := func() (bool, error) {
		states, err := dev.DiskStates(ctx)
		if err != nil {
			return false, err
		}
		if len(states) == 0 {
			t.Log().Infof("waiting for drbd %s disk local dstate", t.Res)
			return false, nil
		}
		state := states[0]
		if state == "Diskless" || state == "DUnknown" {
			t.Log().Infof("drbd %s disk local dstate %s (%s) is not yet valid", t.Res, state, states)
			return false, nil
		}
		t.Log().Infof("drbd %s found local dstate %s from states: %s", t.Res, state, states)
		return true, nil
	}
	limit := time.Now().Add(waitDiskStatesTimeout)
	for {
		ok, err := check()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().Add(waitDiskStatesDelay).After(limit) {
			return fmt.Errorf("timeout waiting for localhost to have a known dstate")
		}
		time.Sleep(waitDiskStatesDelay)
	}
}

/*
func (t Path) Boot(ctx context.Context) error {
	return t.Stop(ctx)
}

func (t Path) PostSync() error {
	return nil
}

func (t Path) PreSync() error {
	return t.dumpCacheFile()
}

func (t Path) ToSync() []string {
	return []string{}
}

func (t Path) Resync(ctx context.Context) error {
	return t.drbd().Resync()
}
*/
