package object

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/ssrathi/go-attr"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/version"
	"opensvc.com/opensvc/util/asset"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/render/tree"
	"opensvc.com/opensvc/util/san"
)

type (
	AssetValue struct {
		Name   string      `json:"-"`
		Source string      `json:"source"`
		Title  string      `json:"title"`
		Value  interface{} `json:"value"`
		Error  string      `json:"error,omitempty"`
	}

	AssetData struct {
		Nodename     AssetValue `json:"nodename"`
		FQDN         AssetValue `json:"fqdn"`
		Version      AssetValue `json:"version"`
		OSName       AssetValue `json:"os_name"`
		OSVendor     AssetValue `json:"os_vendor"`
		OSRelease    AssetValue `json:"os_release"`
		OSKernel     AssetValue `json:"os_kernel"`
		OSArch       AssetValue `json:"os_arch"`
		MemBytes     AssetValue `json:"mem_bytes"`
		MemSlots     AssetValue `json:"mem_slots"`
		MemBanks     AssetValue `json:"mem_banks"`
		CPUFreq      AssetValue `json:"cpu_freq"`
		CPUThreads   AssetValue `json:"cpu_threads"`
		CPUCores     AssetValue `json:"cpu_cores"`
		CPUDies      AssetValue `json:"cpu_dies"`
		CPUModel     AssetValue `json:"cpu_model"`
		Serial       AssetValue `json:"serial"`
		Model        AssetValue `json:"model"`
		Manufacturer AssetValue `json:"manufacturer"`
		BIOSVersion  AssetValue `json:"bios_version"`
		SPVersion    AssetValue `json:"sp_version"`
		NodeEnv      AssetValue `json:"node_env"`
		AssetEnv     AssetValue `json:"asset_env"`
		ListenerPort AssetValue `json:"listener_port"`
		ClusterID    AssetValue `json:"cluster_id"`
		Enclosure    AssetValue `json:"enclosure"`
		TZ           AssetValue `json:"tz"`
		ConnectTo    AssetValue `json:"connect_to"`
		SecZone      AssetValue `json:"sec_zone"`
		LastBoot     AssetValue `json:"last_boot"`
		BootID       AssetValue `json:"boot_id"`
		LocCountry   AssetValue `json:"loc_country"`
		LocCity      AssetValue `json:"loc_city"`
		LocBuilding  AssetValue `json:"loc_building"`
		LocRoom      AssetValue `json:"loc_room"`
		LocRack      AssetValue `json:"loc_rack"`
		LocAddr      AssetValue `json:"loc_addr"`
		LocFloor     AssetValue `json:"loc_floor"`
		LocZIP       AssetValue `json:"loc_zip"`
		TeamInteg    AssetValue `json:"team_integ"`
		TeamSupport  AssetValue `json:"team_support"`

		GIDS     []asset.Group          `json:"gids"`
		UIDS     []asset.User           `json:"uids"`
		Hardware []asset.Device         `json:"hardware"`
		LAN      map[string][]asset.LAN `json:"lan"`
		HBA      []san.HostBusAdapter   `json:"hba"`
		Targets  []san.Path             `json:"targets"`
	}

	Prober interface {
		Get(string) (interface{}, error)
	}
)

const (
	AssetSrcProbe   string = "probe"
	AssetSrcDefault string = "default"
	AssetSrcConfig  string = "config"
)

func (t AssetData) AssetValues() []AssetValue {
	l := make([]AssetValue, 0)
	m, _ := attr.Values(t)
	for k, v := range m {
		av, ok := v.(AssetValue)
		if !ok {
			continue
		}
		av.Name, _ = attr.GetTag(t, k, "json")
		l = append(l, av)
	}
	return l
}

func (t Node) assetValueFromProbe(kw string, title string, probe Prober, dflt interface{}) (data AssetValue) {
	data.Title = title
	k := key.Parse(kw)
	if t.MergedConfig().HasKey(k) {
		data.Source = AssetSrcConfig
		s, err := t.MergedConfig().Eval(k)
		if err == nil {
			data.Value = s
		}
		return
	}
	if probe != nil {
		s, err := probe.Get(k.Option)
		if err == nil {
			data.Source = AssetSrcProbe
			data.Value = s
			return
		}
		if !errors.Is(err, asset.ErrIgnore) {
			data.Error = fmt.Sprint(err)
		}
	}
	data.Source = AssetSrcDefault
	data.Value = dflt
	return
}

func (t Node) assetAgentVersion() (data AssetValue) {
	data.Title = "agent version"
	data.Source = AssetSrcProbe
	data.Value = version.Version
	return
}

func (t Node) assetNodename() (data AssetValue) {
	data.Title = "nodename"
	data.Source = AssetSrcProbe
	data.Value = hostname.Hostname()
	return
}

func (t Node) assetValueClusterID() (data AssetValue) {
	k := key.T{Section: "cluster", Option: "id"}
	data.Title = "cluster id"
	data.Source = AssetSrcProbe
	data.Value, _ = t.MergedConfig().Eval(k)
	return
}

func NewAssetData() AssetData {
	t := AssetData{}
	return t
}

//
// PushAsset assembles the asset inventory data.
// Each entry value comes from:
// * overrides (in config)
// * probes
// * default (code)
//
func (t Node) PushAsset() (AssetData, error) {
	data, err := t.getAsset()
	if err != nil {
		return data, err
	}
	if err := t.pushAsset(data); err != nil {
		return data, err
	}
	return data, nil
}

func (t Node) getAsset() (AssetData, error) {
	data := NewAssetData()

	// from core
	data.ClusterID = t.assetValueClusterID()
	data.Nodename = t.assetNodename()
	data.Version = t.assetAgentVersion()

	// from probe
	probe := asset.New()
	data.FQDN = t.assetValueFromProbe("node.fqdn", "fqdn", probe, nil)
	data.OSName = t.assetValueFromProbe("node.os_name", "os name", probe, nil)
	data.OSVendor = t.assetValueFromProbe("node.os_vendor", "os vendor", probe, nil)
	data.OSRelease = t.assetValueFromProbe("node.os_release", "os release", probe, nil)
	data.OSKernel = t.assetValueFromProbe("node.os_kernel", "os kernel", probe, nil)
	data.OSArch = t.assetValueFromProbe("node.os_arch", "os arch", probe, nil)
	data.MemBytes = t.assetValueFromProbe("node.mem_bytes", "mem bytes", probe, nil)
	data.MemSlots = t.assetValueFromProbe("node.mem_slots", "mem slots", probe, nil)
	data.MemBanks = t.assetValueFromProbe("node.mem_banks", "mem banks", probe, nil)
	data.CPUFreq = t.assetValueFromProbe("node.cpu_freq", "cpu freq", probe, nil)
	data.CPUThreads = t.assetValueFromProbe("node.cpu_threads", "cpu threads", probe, nil)
	data.CPUCores = t.assetValueFromProbe("node.cpu_cores", "cpu cores", probe, nil)
	data.CPUDies = t.assetValueFromProbe("node.cpu_dies", "cpu dies", probe, nil)
	data.CPUModel = t.assetValueFromProbe("node.cpu_model", "cpu model", probe, nil)
	data.BIOSVersion = t.assetValueFromProbe("node.bios_version", "bios version", probe, nil)
	data.Serial = t.assetValueFromProbe("node.serial", "serial", probe, nil)
	data.SPVersion = t.assetValueFromProbe("node.sp_version", "sp version", probe, nil)
	data.Enclosure = t.assetValueFromProbe("node.enclosure", "enclosure", probe, nil)
	data.TZ = t.assetValueFromProbe("node.tz", "timezone", probe, nil)
	data.Manufacturer = t.assetValueFromProbe("node.manufacturer", "manufacturer", probe, nil)
	data.Model = t.assetValueFromProbe("node.model", "model", probe, nil)
	data.ConnectTo = t.assetValueFromProbe("node.connect_to", "connect to", probe, "")
	data.LastBoot = t.assetValueFromProbe("node.last_boot", "last boot", probe, nil)
	data.BootID = t.assetValueFromProbe("node.boot_id", "boot id", probe, nil)
	data.UIDS, _ = asset.Users()
	data.GIDS, _ = asset.Groups()
	data.Hardware, _ = asset.Hardware()
	data.LAN, _ = asset.GetLANS()
	data.HBA, _ = san.HostBusAdapters()
	data.Targets, _ = san.Paths()

	// from config only
	data.SecZone = t.assetValueFromProbe("node.sec_zone", "security zone", nil, nil)
	data.NodeEnv = t.assetValueFromProbe("node.env", "environment", nil, nil)
	data.AssetEnv = t.assetValueFromProbe("node.asset_env", "asset environment", nil, nil)
	data.ListenerPort = t.assetValueFromProbe("listener.port", "listener port", nil, nil)
	data.LocCountry = t.assetValueFromProbe("node.loc_country", "loc, country", nil, nil)
	data.LocCity = t.assetValueFromProbe("node.loc_city", "loc, city", nil, nil)
	data.LocBuilding = t.assetValueFromProbe("node.loc_building", "loc, building", nil, nil)
	data.LocRoom = t.assetValueFromProbe("node.loc_room", "loc, room", nil, nil)
	data.LocRack = t.assetValueFromProbe("node.loc_rack", "loc, rack", nil, nil)
	data.LocAddr = t.assetValueFromProbe("node.loc_addr", "loc, address", nil, nil)
	data.LocFloor = t.assetValueFromProbe("node.loc_floor", "loc, floor", nil, nil)
	data.LocZIP = t.assetValueFromProbe("node.loc_zip", "loc, zip", nil, nil)
	data.TeamInteg = t.assetValueFromProbe("node.team_integ", "team, integration", nil, nil)
	data.TeamSupport = t.assetValueFromProbe("node.team_support", "team, support", nil, nil)

	return data, nil
}

func (t AssetData) Render() string {
	tr := tree.New()
	tr.AddColumn().AddText(hostname.Hostname()).SetColor(rawconfig.Color.Bold)
	tr.AddColumn().AddText("Value").SetColor(rawconfig.Color.Bold)
	tr.AddColumn().AddText("Source").SetColor(rawconfig.Color.Bold)

	node := func(v AssetValue) *tree.Node {
		val := ""
		if v.Value != nil {
			val = fmt.Sprint(v.Value)
		}
		n := tr.AddNode()
		n.AddColumn().AddText(v.Title).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(val)
		n.AddColumn().AddText(v.Source)
		return n
	}

	_ = node(t.Nodename)
	_ = node(t.FQDN)
	_ = node(t.Version)
	_ = node(t.OSName)
	_ = node(t.OSVendor)
	_ = node(t.OSRelease)
	_ = node(t.OSKernel)
	_ = node(t.OSArch)
	_ = node(t.MemBytes)
	_ = node(t.MemSlots)
	_ = node(t.MemBanks)
	_ = node(t.CPUFreq)
	_ = node(t.CPUThreads)
	_ = node(t.CPUCores)
	_ = node(t.CPUDies)
	_ = node(t.CPUModel)
	_ = node(t.Serial)
	_ = node(t.Model)
	_ = node(t.Manufacturer)
	_ = node(t.BIOSVersion)
	_ = node(t.SPVersion)
	_ = node(t.NodeEnv)
	_ = node(t.AssetEnv)
	_ = node(t.Enclosure)
	_ = node(t.ListenerPort)
	_ = node(t.ClusterID)
	_ = node(t.TZ)
	_ = node(t.ConnectTo)
	_ = node(t.SecZone)
	_ = node(t.LastBoot)
	_ = node(t.BootID)
	_ = node(t.LocCountry)
	_ = node(t.LocCity)
	_ = node(t.LocBuilding)
	_ = node(t.LocRoom)
	_ = node(t.LocRack)
	_ = node(t.LocAddr)
	_ = node(t.LocFloor)
	_ = node(t.LocZIP)
	_ = node(t.TeamInteg)
	_ = node(t.TeamSupport)

	n := tr.AddNode()
	n.AddColumn().AddText("hardware").SetColor(rawconfig.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(len(t.Hardware)))
	n.AddColumn().AddText(AssetSrcProbe)
	for _, e := range t.Hardware {
		l := n.AddNode()
		l.AddColumn().AddText(e.Type + " " + e.Path)
		l.AddColumn().AddText(e.Class + ": " + e.Description)
	}

	n = tr.AddNode()
	n.AddColumn().AddText("uids").SetColor(rawconfig.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(len(t.UIDS)))
	n.AddColumn().AddText(AssetSrcProbe)

	n = tr.AddNode()
	n.AddColumn().AddText("gids").SetColor(rawconfig.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(len(t.GIDS)))
	n.AddColumn().AddText(AssetSrcProbe)

	nbAddr := 0
	for _, v := range t.LAN {
		nbAddr = nbAddr + len(v)
	}
	n = tr.AddNode()
	n.AddColumn().AddText("ip addresses").SetColor(rawconfig.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(nbAddr))
	n.AddColumn().AddText(AssetSrcProbe)
	for _, v := range t.LAN {
		for _, e := range v {
			s := e.Address
			if e.Mask != "" {
				s = s + "/" + e.Mask
			}
			l := n.AddNode()
			l.AddColumn().AddText(s)
			l.AddColumn().AddText(e.Intf)
		}
	}

	n = tr.AddNode()
	n.AddColumn().AddText("host bus adapters").SetColor(rawconfig.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(len(t.HBA)))
	n.AddColumn().AddText(AssetSrcProbe)
	for _, v := range t.HBA {
		l := n.AddNode()
		l.AddColumn().AddText(v.ID)
		l.AddColumn().AddText(v.Type)
	}

	return tr.Render()
}

func (t Node) pushAsset(data AssetData) error {
	//hn := hostname.Hostname()
	hba := func() []interface{} {
		vars := []string{
			"nodename",
			"hba_id",
			"hba_type",
		}
		vals := make([][]string, len(data.HBA))
		for i, e := range data.HBA {
			vals[i] = []string{
				e.Host,
				e.ID,
				e.Type,
			}
		}
		return []interface{}{vars, vals}
	}
	targets := func() []interface{} {
		vars := []string{
			"hba_id",
			"tgt_id",
		}
		vals := make([][]string, len(data.Targets))
		for i, e := range data.Targets {
			vals[i] = []string{
				e.HostBusAdapter.ID,
				e.TargetPort.ID,
			}
		}
		return []interface{}{vars, vals}
	}
	lan := func() []interface{} {
		vars := []string{
			"mac",
			"intf",
			"type",
			"addr",
			"mask",
			"flag_deprecated",
		}
		vals := make([][]string, 0)
		for mac, ips := range data.LAN {
			for _, ip := range ips {
				val := []string{
					mac,
					ip.Intf,
					ip.Type,
					ip.Address,
					ip.Mask,
					fmt.Sprint(ip.FlagDeprecated),
				}
				vals = append(vals, val)
			}
		}
		return []interface{}{vars, vals}
	}
	uids := func() []interface{} {
		vars := []string{
			"user_name",
			"user_id",
		}
		vals := make([][]string, len(data.UIDS))
		for i, e := range data.UIDS {
			vals[i] = []string{
				e.Name,
				fmt.Sprint(e.ID),
			}
		}
		return []interface{}{vars, vals}
	}
	gids := func() []interface{} {
		vars := []string{
			"group_name",
			"group_id",
		}
		vals := make([][]string, len(data.GIDS))
		for i, e := range data.GIDS {
			vals[i] = []string{
				e.Name,
				fmt.Sprint(e.ID),
			}
		}
		return []interface{}{vars, vals}
	}
	props := func() []interface{} {
		vars := make([]string, 0)
		vals := make([]string, 0)
		for _, av := range data.AssetValues() {
			if av.Name == "boot_id" {
				continue
			}
			vars = append(vars, av.Name)
			val := ""
			if av.Value != nil {
				val = fmt.Sprint(av.Value)
			}
			vals = append(vals, val)
		}
		return []interface{}{vars, vals}
	}
	client, err := t.collectorFeedClient()
	if err != nil {
		return err
	}
	gen := make(map[string]interface{})
	gen["hardware"] = data.Hardware
	gen["hba"] = hba()
	gen["targets"] = targets()
	gen["lan"] = lan()
	gen["uids"] = uids()
	gen["gids"] = gids()
	if response, err := client.Call("insert_generic", gen); err != nil {
		return err
	} else if response.Error != nil {
		return errors.Errorf("rpc: %s %s", response.Error.Message, response.Error.Data)
	}

	args := props()
	if response, err := client.Call("update_asset", args...); err != nil {
		return err
	} else if response.Error != nil {
		return errors.Errorf("rpc: %s %s", response.Error.Message, response.Error.Data)
	}
	return nil
}
