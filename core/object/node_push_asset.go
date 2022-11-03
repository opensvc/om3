package object

import (
	"fmt"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/version"
	"opensvc.com/opensvc/util/asset"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/san"
)

type (
	// prober is responsible for a bunch of asset properties, and is
	// able to report them one by one.
	// Most prober use a command output, syscall, file content cache
	// where the properties can be found (ex: dmidecode)
	prober interface {
		Get(string) (interface{}, error)
	}
)

func (t Node) assetValueFromProbe(kw string, title string, probe prober, dflt interface{}) (data asset.Value) {
	data.Title = title
	k := key.Parse(kw)
	if t.MergedConfig().HasKey(k) {
		data.Source = asset.SrcConfig
		s, err := t.MergedConfig().Eval(k)
		if err == nil {
			data.Value = s
		}
		return
	}
	if probe != nil {
		s, err := probe.Get(k.Option)
		if err == nil {
			data.Source = asset.SrcProbe
			data.Value = s
			return
		}
		if !errors.Is(err, asset.ErrIgnore) {
			data.Error = fmt.Sprint(err)
		}
	}
	data.Source = asset.SrcDefault
	data.Value = dflt
	return
}

func (t Node) assetAgentVersion() (data asset.Value) {
	data.Title = "agent version"
	data.Source = asset.SrcProbe
	data.Value = version.Version
	return
}

func (t Node) assetNodename() (data asset.Value) {
	data.Title = "nodename"
	data.Source = asset.SrcProbe
	data.Value = hostname.Hostname()
	return
}

func (t Node) assetValueClusterID() (data asset.Value) {
	k := key.T{Section: "cluster", Option: "id"}
	data.Title = "cluster id"
	data.Source = asset.SrcProbe
	data.Value, _ = t.MergedConfig().Eval(k)
	return
}

// PushAsset assembles the asset inventory data.
// Each entry value comes from:
// * overrides (in config)
// * probes
// * default (code)
func (t Node) PushAsset() (asset.Data, error) {
	data, err := t.getAsset()
	if err != nil {
		return data, err
	}
	if err := t.pushAsset(data); err != nil {
		return data, err
	}
	return data, nil
}

func (t Node) getAsset() (asset.Data, error) {
	data := asset.NewData()

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
	data.HBA, _ = san.GetInitiators()
	data.Targets, _ = san.GetPaths()

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

func (t Node) pushAsset(data asset.Data) error {
	hn := hostname.Hostname()
	hba := func() []interface{} {
		vars := []string{
			"nodename",
			"hba_id",
			"hba_type",
		}
		vals := make([][]string, len(data.HBA))
		for i, e := range data.HBA {
			vals[i] = []string{
				hn,
				e.Name,
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
				e.Initiator.Name,
				e.Target.Name,
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
		for _, av := range data.Values() {
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
	client, err := t.CollectorFeedClient()
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
