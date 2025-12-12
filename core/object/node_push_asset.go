package object

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/asset"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/san"
	"github.com/opensvc/om3/v3/util/version"
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

// assetValueFromDefinedConfig return asset.property from config keyword eval
// when the keyword is present in current config or return asset.property from
// default value
func (t Node) assetValueFromDefinedConfig(kw string, title string, defaultValue interface{}) (data asset.Property) {
	data.Title = title
	k := key.Parse(kw)
	if t.MergedConfig().HasKey(k) {
		data.Source = asset.SrcConfig
		s, err := t.MergedConfig().Eval(k)
		if err != nil {
			data.Error = fmt.Sprint(err)
		} else {
			data.Value = s
		}
		return
	}
	data.Source = asset.SrcDefault
	data.Value = defaultValue
	return
}

// assetValueFromConfigEval return asset.property from config keyword eval
func (t Node) assetValueFromConfigEval(kw string, title string) (data asset.Property) {
	data.Title = title
	data.Source = asset.SrcConfig
	k := key.Parse(kw)
	s, err := t.MergedConfig().Eval(k)
	if err != nil {
		data.Error = fmt.Sprint(err)
	} else {
		data.Value = s
	}
	return
}

// assetValueFromDefinedConfigOrProbe return asset.property with the following evaluation order:
//
//	1- config keyword eval if the keyword is present in current config, or present
//	2- probe.Get
//	3- defaultValue
func (t Node) assetValueFromDefinedConfigOrProbe(kw string, title string, probe prober, defaultValue interface{}) (data asset.Property) {
	data.Title = title
	k := key.Parse(kw)
	if t.MergedConfig().HasKey(k) {
		data.Source = asset.SrcConfig
		s, err := t.MergedConfig().Eval(k)
		if err != nil {
			data.Error = fmt.Sprint(err)
		} else {
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
	data.Value = defaultValue
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
	filename := t.nodeSystemCacheFile()
	tryOpen := func() (*os.File, error) {
		return os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
	}
	file, err := tryOpen()
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			return err
		}
		file, err = tryOpen()
	}
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
	data.Properties.FQDN = t.assetValueFromDefinedConfigOrProbe("node.fqdn", "fqdn", probe, nil)
	data.Properties.OSName = t.assetValueFromDefinedConfigOrProbe("node.os_name", "os name", probe, nil)
	data.Properties.OSVendor = t.assetValueFromDefinedConfigOrProbe("node.os_vendor", "os vendor", probe, nil)
	data.Properties.OSRelease = t.assetValueFromDefinedConfigOrProbe("node.os_release", "os release", probe, nil)
	data.Properties.OSKernel = t.assetValueFromDefinedConfigOrProbe("node.os_kernel", "os kernel", probe, nil)
	data.Properties.OSArch = t.assetValueFromDefinedConfigOrProbe("node.os_arch", "os arch", probe, nil)
	data.Properties.MemBytes = t.assetValueFromDefinedConfigOrProbe("node.mem_bytes", "mem bytes", probe, nil)
	data.Properties.MemSlots = t.assetValueFromDefinedConfigOrProbe("node.mem_slots", "mem slots", probe, nil)
	data.Properties.MemBanks = t.assetValueFromDefinedConfigOrProbe("node.mem_banks", "mem banks", probe, nil)
	data.Properties.CPUFreq = t.assetValueFromDefinedConfigOrProbe("node.cpu_freq", "cpu freq", probe, nil)
	data.Properties.CPUThreads = t.assetValueFromDefinedConfigOrProbe("node.cpu_threads", "cpu threads", probe, nil)
	data.Properties.CPUCores = t.assetValueFromDefinedConfigOrProbe("node.cpu_cores", "cpu cores", probe, nil)
	data.Properties.CPUDies = t.assetValueFromDefinedConfigOrProbe("node.cpu_dies", "cpu dies", probe, nil)
	data.Properties.CPUModel = t.assetValueFromDefinedConfigOrProbe("node.cpu_model", "cpu model", probe, nil)
	data.Properties.BIOSVersion = t.assetValueFromDefinedConfigOrProbe("node.bios_version", "bios version", probe, nil)
	data.Properties.Serial = t.assetValueFromDefinedConfigOrProbe("node.serial", "serial", probe, nil)
	data.Properties.SPVersion = t.assetValueFromDefinedConfigOrProbe("node.sp_version", "sp version", probe, nil)
	data.Properties.Enclosure = t.assetValueFromDefinedConfigOrProbe("node.enclosure", "enclosure", probe, nil)
	data.Properties.TZ = t.assetValueFromDefinedConfigOrProbe("node.tz", "timezone", probe, nil)
	data.Properties.Manufacturer = t.assetValueFromDefinedConfigOrProbe("node.manufacturer", "manufacturer", probe, nil)
	data.Properties.Model = t.assetValueFromDefinedConfigOrProbe("node.model", "model", probe, nil)
	data.Properties.ConnectTo = t.assetValueFromDefinedConfigOrProbe("node.connect_to", "connect to", probe, "")
	data.Properties.LastBoot = t.assetValueFromDefinedConfigOrProbe("node.last_boot", "last boot", probe, nil)
	data.Properties.BootID = t.assetValueFromDefinedConfigOrProbe("node.boot_id", "boot id", probe, nil)
	data.UIDS, _ = asset.Users()
	data.GIDS, _ = asset.Groups()
	data.Hardware, _ = asset.Hardware()
	data.LAN, _ = asset.GetLANS()
	data.HBA, _ = san.GetInitiators()
	data.Targets, _ = san.GetPaths()

	// from config eval only
	data.Properties.NodeEnv = t.assetValueFromConfigEval("node.env", "environment")

	// from existing config key
	data.Properties.SecZone = t.assetValueFromDefinedConfig("node.sec_zone", "security zone", nil)
	data.Properties.AssetEnv = t.assetValueFromDefinedConfig("node.asset_env", "asset environment", nil)
	data.Properties.ListenerPort = t.assetValueFromDefinedConfig("listener.port", "listener port", nil)
	data.Properties.LocCountry = t.assetValueFromDefinedConfig("node.loc_country", "loc, country", nil)
	data.Properties.LocCity = t.assetValueFromDefinedConfig("node.loc_city", "loc, city", nil)
	data.Properties.LocBuilding = t.assetValueFromDefinedConfig("node.loc_building", "loc, building", nil)
	data.Properties.LocRoom = t.assetValueFromDefinedConfig("node.loc_room", "loc, room", nil)
	data.Properties.LocRack = t.assetValueFromDefinedConfig("node.loc_rack", "loc, rack", nil)
	data.Properties.LocAddr = t.assetValueFromDefinedConfig("node.loc_addr", "loc, address", nil)
	data.Properties.LocFloor = t.assetValueFromDefinedConfig("node.loc_floor", "loc, floor", nil)
	data.Properties.LocZIP = t.assetValueFromDefinedConfig("node.loc_zip", "loc, zip", nil)
	data.Properties.TeamInteg = t.assetValueFromDefinedConfig("node.team_integ", "team, integration", nil)
	data.Properties.TeamSupport = t.assetValueFromDefinedConfig("node.team_support", "team, support", nil)

	return data, nil
}

func (t Node) pushAsset(data asset.Data) error {
	var (
		req  *http.Request
		resp *http.Response

		ioReader io.Reader

		method = http.MethodPost
		path   = "/oc3/feed/system"
	)
	oc3, err := t.CollectorClient()
	if err != nil {
		return err
	}

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

	if b, err := json.MarshalIndent(gen, "  ", "  "); err != nil {
		return fmt.Errorf("encode request body: %w", err)
	} else {
		ioReader = bytes.NewBuffer(b)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultPostCollectorTimeout)
	defer cancel()

	req, err = oc3.NewRequestWithContext(ctx, method, path, ioReader)
	if err != nil {
		return fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}

	resp, err = oc3.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected collector response status code for %s %s: wanted %d got %d",
			method, path, http.StatusAccepted, resp.StatusCode)
	}

	return nil
}
