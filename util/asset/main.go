package asset

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ssrathi/go-attr"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/render/tree"
	"opensvc.com/opensvc/util/san"
)

var (
	ErrNotImpl = fmt.Errorf("not implemented")
	ErrIgnore  = fmt.Errorf("ignore")
)

type (
	T struct{}

	Value struct {
		Name   string      `json:"-"`
		Source string      `json:"source"`
		Title  string      `json:"title"`
		Value  interface{} `json:"value"`
		Error  string      `json:"error,omitempty"`
	}

	Data struct {
		Nodename     Value `json:"nodename"`
		FQDN         Value `json:"fqdn"`
		Version      Value `json:"version"`
		OSName       Value `json:"os_name"`
		OSVendor     Value `json:"os_vendor"`
		OSRelease    Value `json:"os_release"`
		OSKernel     Value `json:"os_kernel"`
		OSArch       Value `json:"os_arch"`
		MemBytes     Value `json:"mem_bytes"`
		MemSlots     Value `json:"mem_slots"`
		MemBanks     Value `json:"mem_banks"`
		CPUFreq      Value `json:"cpu_freq"`
		CPUThreads   Value `json:"cpu_threads"`
		CPUCores     Value `json:"cpu_cores"`
		CPUDies      Value `json:"cpu_dies"`
		CPUModel     Value `json:"cpu_model"`
		Serial       Value `json:"serial"`
		Model        Value `json:"model"`
		Manufacturer Value `json:"manufacturer"`
		BIOSVersion  Value `json:"bios_version"`
		SPVersion    Value `json:"sp_version"`
		NodeEnv      Value `json:"node_env"`
		AssetEnv     Value `json:"asset_env"`
		ListenerPort Value `json:"listener_port"`
		ClusterID    Value `json:"cluster_id"`
		Enclosure    Value `json:"enclosure"`
		TZ           Value `json:"tz"`
		ConnectTo    Value `json:"connect_to"`
		SecZone      Value `json:"sec_zone"`
		LastBoot     Value `json:"last_boot"`
		BootID       Value `json:"boot_id"`
		LocCountry   Value `json:"loc_country"`
		LocCity      Value `json:"loc_city"`
		LocBuilding  Value `json:"loc_building"`
		LocRoom      Value `json:"loc_room"`
		LocRack      Value `json:"loc_rack"`
		LocAddr      Value `json:"loc_addr"`
		LocFloor     Value `json:"loc_floor"`
		LocZIP       Value `json:"loc_zip"`
		TeamInteg    Value `json:"team_integ"`
		TeamSupport  Value `json:"team_support"`

		GIDS     []Group              `json:"gids"`
		UIDS     []User               `json:"uids"`
		Hardware []Device             `json:"hardware"`
		LAN      map[string][]LAN     `json:"lan"`
		HBA      []san.HostBusAdapter `json:"hba"`
		Targets  []san.Path           `json:"targets"`
	}

	Device struct {
		Path        string `json:"path"`
		Description string `json:"description"`
		Class       string `json:"class"`
		Driver      string `json:"driver"`
		Type        string `json:"type"`
	}

	Group struct {
		ID   int    `json:"gid"`
		Name string `json:"groupname"`
	}

	User struct {
		ID   int    `json:"uid"`
		Name string `json:"username"`
	}

	LAN struct {
		Address        string `json:"addr"`
		FlagDeprecated bool   `json:"flag_deprecated"`
		Intf           string `json:"intf"`
		Mask           string `json:"mask"`
		Type           string `json:"type"`
	}
)

const (
	SrcProbe   = "probe"
	SrcDefault = "default"
	SrcConfig  = "config"
)

func NewData() Data {
	t := Data{}
	return t
}

func (t Data) Render() string {
	tr := tree.New()
	tr.AddColumn().AddText(hostname.Hostname()).SetColor(rawconfig.Color.Bold)
	tr.AddColumn().AddText("Value").SetColor(rawconfig.Color.Bold)
	tr.AddColumn().AddText("Source").SetColor(rawconfig.Color.Bold)

	node := func(v Value) *tree.Node {
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
	n.AddColumn().AddText(SrcProbe)
	for _, e := range t.Hardware {
		l := n.AddNode()
		l.AddColumn().AddText(e.Type + " " + e.Path)
		l.AddColumn().AddText(e.Class + ": " + e.Description)
	}

	n = tr.AddNode()
	n.AddColumn().AddText("uids").SetColor(rawconfig.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(len(t.UIDS)))
	n.AddColumn().AddText(SrcProbe)

	n = tr.AddNode()
	n.AddColumn().AddText("gids").SetColor(rawconfig.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(len(t.GIDS)))
	n.AddColumn().AddText(SrcProbe)

	nbAddr := 0
	for _, v := range t.LAN {
		nbAddr = nbAddr + len(v)
	}
	n = tr.AddNode()
	n.AddColumn().AddText("ip addresses").SetColor(rawconfig.Color.Primary)
	n.AddColumn().AddText(fmt.Sprint(nbAddr))
	n.AddColumn().AddText(SrcProbe)
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
	n.AddColumn().AddText(SrcProbe)
	for _, v := range t.HBA {
		l := n.AddNode()
		l.AddColumn().AddText(v.ID)
		l.AddColumn().AddText(v.Type)
	}

	return tr.Render()
}

func (t Data) Values() []Value {
	l := make([]Value, 0)
	m, _ := attr.Values(t)
	for k, v := range m {
		av, ok := v.(Value)
		if !ok {
			continue
		}
		av.Name, _ = attr.GetTag(t, k, "json")
		l = append(l, av)
	}
	return l
}

func TZ() (string, error) {
	now := time.Now()
	return now.Format("-07:00"), nil
}

func GetLANS() (map[string][]LAN, error) {
	m := make(map[string][]LAN)
	intfs, err := net.Interfaces()
	if err != nil {
		return m, err
	}
	for _, intf := range intfs {
		addrs, err := intf.Addrs()
		if err != nil {
			continue
		}
		mcastAddrs, err := intf.MulticastAddrs()
		if err == nil {
			addrs = append(addrs, mcastAddrs...)
		}
		for _, addr := range addrs {
			e := LAN{}
			e.Intf = intf.Name
			l := strings.Split(addr.String(), "/")
			switch len(l) {
			case 1:
				// mcast
				e.Address = l[0]
			case 2:
				// ucast
				e.Address = l[0]
				e.Mask = l[1]
			default:
				continue
			}
			if strings.Contains(e.Address, ":") {
				e.Type = "ipv6"
			} else {
				e.Type = "ipv4"
			}
			hwAddr := intf.HardwareAddr.String()
			if _, ok := m[hwAddr]; !ok {
				m[hwAddr] = make([]LAN, 0)
			}
			m[hwAddr] = append(m[hwAddr], e)
		}
	}
	return m, nil
}

func ConnectTo() (string, error) {
	// TODO: port gcloud address detection ?
	return "", ErrIgnore
}

func Users() ([]User, error) {
	l := make([]User, 0)
	data, err := parseColumned("/etc/passwd")
	if err != nil {
		return l, err
	}
	for _, lineSlice := range data {
		if len(lineSlice) < 3 {
			continue
		}
		uid, err := strconv.Atoi(lineSlice[2])
		if err != nil {
			continue
		}
		l = append(l, User{
			Name: lineSlice[0],
			ID:   uid,
		})
	}
	return l, nil
}

func Groups() ([]Group, error) {
	l := make([]Group, 0)
	data, err := parseColumned("/etc/group")
	if err != nil {
		return l, err
	}
	for _, lineSlice := range data {
		if len(lineSlice) < 3 {
			continue
		}
		gid, err := strconv.Atoi(lineSlice[2])
		if err != nil {
			continue
		}
		l = append(l, Group{
			Name: lineSlice[0],
			ID:   gid,
		})
	}
	return l, nil
}

func parseColumned(p string) ([][]string, error) {
	l := make([][]string, 0)
	file, err := os.Open(p)
	if err != nil {
		return l, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')

		// skip all line starting with #
		if equal := strings.Index(line, "#"); equal < 0 {
			lineSlice := strings.FieldsFunc(line, func(divide rune) bool {
				return divide == ':'
			})
			l = append(l, lineSlice)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return l, err
		}
	}
	return l, nil
}
