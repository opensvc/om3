package object

import (
	"opensvc.com/opensvc/util/asset"
	"opensvc.com/opensvc/util/key"
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
	data.UIDS, _ = asset.Users()
	data.GIDS, _ = asset.Groups()
	data.Hardware, _ = asset.Hardware()
	data.LAN, _ = asset.GetLANS()

	// from config only
	data.SecZone = t.assetValueFromProbe("node.sec_zone", "security zone", probe, nil)

	return data
}
