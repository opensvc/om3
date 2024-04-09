package object

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/asset"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/san"
	"github.com/opensvc/om3/util/version"
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

func (t Node) nodeSystemCacheFile() string {
	return filepath.Join(rawconfig.NodeVarDir(), "system.json")
}

func (t Node) assetValueFromProbe(kw string, title string, probe prober, dflt interface{}) (data asset.Property) {
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

func (t Node) assetAgentVersion() (data asset.Property) {
	data.Title = "agent version"
	data.Source = asset.SrcProbe
	data.Value = version.Version()
	return
}

func (t Node) assetNodename() (data asset.Property) {
	data.Title = "nodename"
	data.Source = asset.SrcProbe
	data.Value = hostname.Hostname()
	return
}

func (t Node) assetValueClusterID() (data asset.Property) {
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
	if err := t.dumpSystem(data); err != nil {
		return data, err
	}
	if err := t.pushAsset(data); err != nil {
		return data, err
	}
	return data, nil
}

func (t Node) dumpSystem(data asset.Data) error {
	file, err := os.OpenFile(t.nodeSystemCacheFile(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return json.NewEncoder(file).Encode(data)
}

func (t Node) LoadSystem() (asset.Data, error) {
	var data asset.Data
	file, err := os.Open(t.nodeSystemCacheFile())
	if err != nil {
		return data, err
	}
	defer func() { _ = file.Close() }()
	err = json.NewDecoder(file).Decode(&data)
	return data, err
}

func (t Node) getAsset() (asset.Data, error) {
	data := asset.NewData()

	// from core
	data.Properties.ClusterID = t.assetValueClusterID()
	data.Properties.Nodename = t.assetNodename()
	data.Properties.Version = t.assetAgentVersion()

	// from probe
	probe := asset.New()
	data.Properties.FQDN = t.assetValueFromProbe("node.fqdn", "fqdn", probe, nil)
	data.Properties.OSName = t.assetValueFromProbe("node.os_name", "os name", probe, nil)
	data.Properties.OSVendor = t.assetValueFromProbe("node.os_vendor", "os vendor", probe, nil)
	data.Properties.OSRelease = t.assetValueFromProbe("node.os_release", "os release", probe, nil)
	data.Properties.OSKernel = t.assetValueFromProbe("node.os_kernel", "os kernel", probe, nil)
	data.Properties.OSArch = t.assetValueFromProbe("node.os_arch", "os arch", probe, nil)
	data.Properties.MemBytes = t.assetValueFromProbe("node.mem_bytes", "mem bytes", probe, nil)
	data.Properties.MemSlots = t.assetValueFromProbe("node.mem_slots", "mem slots", probe, nil)
	data.Properties.MemBanks = t.assetValueFromProbe("node.mem_banks", "mem banks", probe, nil)
	data.Properties.CPUFreq = t.assetValueFromProbe("node.cpu_freq", "cpu freq", probe, nil)
	data.Properties.CPUThreads = t.assetValueFromProbe("node.cpu_threads", "cpu threads", probe, nil)
	data.Properties.CPUCores = t.assetValueFromProbe("node.cpu_cores", "cpu cores", probe, nil)
	data.Properties.CPUDies = t.assetValueFromProbe("node.cpu_dies", "cpu dies", probe, nil)
	data.Properties.CPUModel = t.assetValueFromProbe("node.cpu_model", "cpu model", probe, nil)
	data.Properties.BIOSVersion = t.assetValueFromProbe("node.bios_version", "bios version", probe, nil)
	data.Properties.Serial = t.assetValueFromProbe("node.serial", "serial", probe, nil)
	data.Properties.SPVersion = t.assetValueFromProbe("node.sp_version", "sp version", probe, nil)
	data.Properties.Enclosure = t.assetValueFromProbe("node.enclosure", "enclosure", probe, nil)
	data.Properties.TZ = t.assetValueFromProbe("node.tz", "timezone", probe, nil)
	data.Properties.Manufacturer = t.assetValueFromProbe("node.manufacturer", "manufacturer", probe, nil)
	data.Properties.Model = t.assetValueFromProbe("node.model", "model", probe, nil)
	data.Properties.ConnectTo = t.assetValueFromProbe("node.connect_to", "connect to", probe, "")
	data.Properties.LastBoot = t.assetValueFromProbe("node.last_boot", "last boot", probe, nil)
	data.Properties.BootID = t.assetValueFromProbe("node.boot_id", "boot id", probe, nil)
	data.UIDS, _ = asset.Users()
	data.GIDS, _ = asset.Groups()
	data.Hardware, _ = asset.Hardware()
	data.LAN, _ = asset.GetLANS()
	data.HBA, _ = san.GetInitiators()
	data.Targets, _ = san.GetPaths()

	// from config only
	data.Properties.SecZone = t.assetValueFromProbe("node.sec_zone", "security zone", nil, nil)
	data.Properties.NodeEnv = t.assetValueFromProbe("node.env", "environment", nil, nil)
	data.Properties.AssetEnv = t.assetValueFromProbe("node.asset_env", "asset environment", nil, nil)
	data.Properties.ListenerPort = t.assetValueFromProbe("listener.port", "listener port", nil, nil)
	data.Properties.LocCountry = t.assetValueFromProbe("node.loc_country", "loc, country", nil, nil)
	data.Properties.LocCity = t.assetValueFromProbe("node.loc_city", "loc, city", nil, nil)
	data.Properties.LocBuilding = t.assetValueFromProbe("node.loc_building", "loc, building", nil, nil)
	data.Properties.LocRoom = t.assetValueFromProbe("node.loc_room", "loc, room", nil, nil)
	data.Properties.LocRack = t.assetValueFromProbe("node.loc_rack", "loc, rack", nil, nil)
	data.Properties.LocAddr = t.assetValueFromProbe("node.loc_addr", "loc, address", nil, nil)
	data.Properties.LocFloor = t.assetValueFromProbe("node.loc_floor", "loc, floor", nil, nil)
	data.Properties.LocZIP = t.assetValueFromProbe("node.loc_zip", "loc, zip", nil, nil)
	data.Properties.TeamInteg = t.assetValueFromProbe("node.team_integ", "team, integration", nil, nil)
	data.Properties.TeamSupport = t.assetValueFromProbe("node.team_support", "team, support", nil, nil)

	return data, nil
}

func (t Node) pushAsset(data asset.Data) error {
	hba := func() []any {
		l := make([]any, len(data.HBA))
		for i, e := range data.HBA {
			l[i] = map[string]any{
				"hba_id":   e.Name,
				"hba_type": e.Type,
			}
		}
		return l
	}
	targets := func() []any {
		l := make([]any, len(data.Targets))
		for i, e := range data.Targets {
			l[i] = map[string]any{
				"hba_id": e.Initiator.Name,
				"tgt_id": e.Target.Name,
			}
		}
		return l
	}

	gen := make(map[string]any)

	gen["properties"] = data.Properties
	gen["hardware"] = data.Hardware
	gen["lan"] = data.LAN
	gen["uids"] = data.UIDS
	gen["gids"] = data.GIDS

	// Transformations
	gen["hba"] = hba()
	gen["targets"] = targets()

	url, err := t.Collector3RestAPIURL()
	if err != nil {
		return err
	}
	url.Path += "/daemon/system"
	b, err := json.MarshalIndent(gen, "  ", "  ")
	if err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(b))
	req.SetBasicAuth(hostname.Hostname(), rawconfig.GetNodeSection().UUID)
	req.Header.Add("Content-Type", "application/json")
	c := t.CollectorRestAPIClient()
	response, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 204 {
		return fmt.Errorf("unexpected %s %s response: %s", req.Method, req.URL, response.Status)
	}

	return nil
}
