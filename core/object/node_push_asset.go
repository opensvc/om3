package object

import (
	"fmt"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/asset"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/render/tree"
)

type (
	// OptsNodePushAsset is the options of the PushAsset function.
	OptsNodePushAsset struct {
		Global OptsGlobal
	}

	AssetValue struct {
		Source string      `json:"source"`
		Title  string      `json:"title"`
		Value  interface{} `json:"value"`
		Error  string      `json:"error,omitempty"`
	}

	AssetData struct {
		ClusterID    AssetValue             `json:"cluster_id"`
		CPUModel     AssetValue             `json:"cpu_model"`
		CPUFreq      AssetValue             `json:"cpu_freq"`
		CPUThreads   AssetValue             `json:"cpu_threads"`
		CPUCores     AssetValue             `json:"cpu_cores"`
		CPUDies      AssetValue             `json:"cpu_dies"`
		BIOSVersion  AssetValue             `json:"bios_version"`
		OSVendor     AssetValue             `json:"os_vendor"`
		OSRelease    AssetValue             `json:"os_release"`
		OSKernel     AssetValue             `json:"os_kernel"`
		OSArch       AssetValue             `json:"os_arch"`
		OSName       AssetValue             `json:"os_name"`
		Serial       AssetValue             `json:"serial"`
		SPVersion    AssetValue             `json:"sp_version"`
		Enclosure    AssetValue             `json:"enclosure"`
		TZ           AssetValue             `json:"tz"`
		Manufacturer AssetValue             `json:"manufacturer"`
		Model        AssetValue             `json:"model"`
		MemBytes     AssetValue             `json:"mem_bytes"`
		MemSlots     AssetValue             `json:"mem_slots"`
		MemBanks     AssetValue             `json:"mem_banks"`
		ConnectTo    AssetValue             `json:"connect_to"`
		GIDS         []asset.Group          `json:"gids"`
		UIDS         []asset.User           `json:"uids"`
		Hardware     []asset.Device         `json:"hardware"`
		LAN          map[string][]asset.LAN `json:"lan"`
		FQDN         AssetValue             `json:"fqdn"`
		SecZone      AssetValue             `json:"sec_zone"`
		LastBoot     AssetValue             `json:"last_boot"`
		BootID       AssetValue             `json:"boot_id"`
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
	s, err := probe.Get(k.Option)
	if err == nil {
		data.Source = AssetSrcProbe
		data.Value = s
		return
	}
	data.Error = fmt.Sprint(err)
	data.Source = AssetSrcDefault
	data.Value = dflt
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
func (t Node) PushAsset() AssetData {
	data := NewAssetData()

	// from core
	data.ClusterID = t.assetValueClusterID()

	// from probe
	probe := asset.New()
	data.CPUModel = t.assetValueFromProbe("node.cpu_model", "cpu model", probe, nil)
	data.CPUFreq = t.assetValueFromProbe("node.cpu_freq", "cpu freq", probe, nil)
	data.CPUThreads = t.assetValueFromProbe("node.cpu_threads", "cpu threads", probe, nil)
	data.CPUCores = t.assetValueFromProbe("node.cpu_cores", "cpu cores", probe, nil)
	data.CPUDies = t.assetValueFromProbe("node.cpu_dies", "cpu dies", probe, nil)
	data.BIOSVersion = t.assetValueFromProbe("node.bios_version", "bios version", probe, nil)
	data.OSVendor = t.assetValueFromProbe("node.os_vendor", "os vendor", probe, nil)
	data.OSRelease = t.assetValueFromProbe("node.os_release", "os release", probe, nil)
	data.OSKernel = t.assetValueFromProbe("node.os_kernel", "os kernel", probe, nil)
	data.OSArch = t.assetValueFromProbe("node.os_arch", "os arch", probe, nil)
	data.OSName = t.assetValueFromProbe("node.os_name", "os name", probe, nil)
	data.Serial = t.assetValueFromProbe("node.serial", "serial", probe, nil)
	data.SPVersion = t.assetValueFromProbe("node.sp_version", "sp version", probe, nil)
	data.Enclosure = t.assetValueFromProbe("node.enclosure", "enclosure", probe, nil)
	data.TZ = t.assetValueFromProbe("node.tz", "timezone", probe, nil)
	data.Manufacturer = t.assetValueFromProbe("node.manufacturer", "manufacturer", probe, nil)
	data.Model = t.assetValueFromProbe("node.model", "model", probe, nil)
	data.MemBytes = t.assetValueFromProbe("node.mem_bytes", "mem bytes", probe, nil)
	data.MemSlots = t.assetValueFromProbe("node.mem_slots", "mem slots", probe, nil)
	data.MemBanks = t.assetValueFromProbe("node.mem_banks", "mem banks", probe, nil)
	data.ConnectTo = t.assetValueFromProbe("node.connect_to", "connect to", probe, nil)
	data.FQDN = t.assetValueFromProbe("node.fqdn", "fqdn", probe, nil)
	data.LastBoot = t.assetValueFromProbe("node.last_boot", "last boot", probe, nil)
	data.BootID = t.assetValueFromProbe("node.boot_id", "boot id", probe, nil)
	data.UIDS, _ = asset.Users()
	data.GIDS, _ = asset.Groups()
	data.Hardware, _ = asset.Hardware()
	data.LAN, _ = asset.GetLANS()

	// from config only
	data.SecZone = t.assetValueFromProbe("node.sec_zone", "security zone", probe, nil)

	return data
}

func (t AssetData) Render() string {
	tr := tree.New()
	tr.AddColumn().AddText(hostname.Hostname()).SetColor(rawconfig.Node.Color.Bold)
	tr.AddColumn().AddText("Value").SetColor(rawconfig.Node.Color.Bold)
	tr.AddColumn().AddText("Source").SetColor(rawconfig.Node.Color.Bold)

	node := func(v AssetValue) *tree.Node {
		n := tr.AddNode()
		n.AddColumn().AddText(v.Title).SetColor(rawconfig.Node.Color.Primary)
		n.AddColumn().AddText(fmt.Sprint(v.Value))
		n.AddColumn().AddText(v.Source)
		return n
	}

	_ = node(t.BIOSVersion)
	_ = node(t.ClusterID)
	_ = node(t.CPUModel)
	_ = node(t.CPUFreq)
	_ = node(t.CPUThreads)
	_ = node(t.CPUCores)
	_ = node(t.CPUDies)
	_ = node(t.OSVendor)
	_ = node(t.OSRelease)
	_ = node(t.OSKernel)
	_ = node(t.OSArch)
	_ = node(t.OSName)
	_ = node(t.Serial)
	_ = node(t.SPVersion)
	_ = node(t.Enclosure)
	_ = node(t.TZ)
	_ = node(t.Manufacturer)
	_ = node(t.Model)
	_ = node(t.MemBytes)
	_ = node(t.MemSlots)
	_ = node(t.MemBanks)
	_ = node(t.ConnectTo)
	_ = node(t.SecZone)
	_ = node(t.FQDN)
	_ = node(t.LastBoot)
	_ = node(t.BootID)

	n := tr.AddNode()
	n.AddColumn().AddText("hardware").SetColor(rawconfig.Node.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(len(t.Hardware)))
	n.AddColumn().AddText(AssetSrcProbe)
	for _, e := range t.Hardware {
		l := n.AddNode()
		l.AddColumn().AddText(e.Type + " " + e.Path)
		l.AddColumn().AddText(e.Description)
	}

	n = tr.AddNode()
	n.AddColumn().AddText("uids").SetColor(rawconfig.Node.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(len(t.UIDS)))
	n.AddColumn().AddText(AssetSrcProbe)

	n = tr.AddNode()
	n.AddColumn().AddText("gids").SetColor(rawconfig.Node.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(len(t.GIDS)))
	n.AddColumn().AddText(AssetSrcProbe)

	nbAddr := 0
	for _, v := range t.LAN {
		nbAddr = nbAddr + len(v)
	}
	n = tr.AddNode()
	n.AddColumn().AddText("ip addresses").SetColor(rawconfig.Node.Color.Primary)
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

	return tr.Render()
}
