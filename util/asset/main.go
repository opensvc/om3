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

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/render/tree"
	"github.com/opensvc/om3/v3/util/san"
)

var (
	ErrNotImpl = fmt.Errorf("not implemented")
	ErrIgnore  = fmt.Errorf("ignore")
)

type (
	T struct{}

	Property struct {
		Name   string      `json:"-"`
		Source string      `json:"source"`
		Title  string      `json:"title"`
		Value  interface{} `json:"value"`
		Error  string      `json:"error,omitempty"`
	}

	Properties struct {
		Nodename     Property `json:"nodename"`
		FQDN         Property `json:"fqdn"`
		Version      Property `json:"version"`
		OSName       Property `json:"os_name"`
		OSVendor     Property `json:"os_vendor"`
		OSRelease    Property `json:"os_release"`
		OSKernel     Property `json:"os_kernel"`
		OSArch       Property `json:"os_arch"`
		MemBytes     Property `json:"mem_bytes"`
		MemSlots     Property `json:"mem_slots"`
		MemBanks     Property `json:"mem_banks"`
		CPUFreq      Property `json:"cpu_freq"`
		CPUThreads   Property `json:"cpu_threads"`
		CPUCores     Property `json:"cpu_cores"`
		CPUDies      Property `json:"cpu_dies"`
		CPUModel     Property `json:"cpu_model"`
		Serial       Property `json:"serial"`
		Model        Property `json:"model"`
		Manufacturer Property `json:"manufacturer"`
		BIOSVersion  Property `json:"bios_version"`
		SPVersion    Property `json:"sp_version"`
		NodeEnv      Property `json:"node_env"`
		AssetEnv     Property `json:"asset_env"`
		ListenerPort Property `json:"listener_port"`
		ClusterID    Property `json:"cluster_id"`
		Enclosure    Property `json:"enclosure"`
		TZ           Property `json:"tz"`
		ConnectTo    Property `json:"connect_to"`
		SecZone      Property `json:"sec_zone"`
		LastBoot     Property `json:"last_boot"`
		BootID       Property `json:"boot_id"`
		LocCountry   Property `json:"loc_country"`
		LocCity      Property `json:"loc_city"`
		LocBuilding  Property `json:"loc_building"`
		LocRoom      Property `json:"loc_room"`
		LocRack      Property `json:"loc_rack"`
		LocAddr      Property `json:"loc_addr"`
		LocFloor     Property `json:"loc_floor"`
		LocZIP       Property `json:"loc_zip"`
		TeamInteg    Property `json:"team_integ"`
		TeamSupport  Property `json:"team_support"`
	}

	Data struct {
		Properties Properties       `json:"properties"`
		GIDS       []Group          `json:"gids"`
		UIDS       []User           `json:"uids"`
		Hardware   []Device         `json:"hardware"`
		LAN        map[string][]LAN `json:"lan"`
		HBA        []san.Initiator  `json:"hba"`
		Targets    san.Paths        `json:"targets"`
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
	pn := tr.AddNode()
	pn.AddColumn().AddText("properties").SetColor(rawconfig.Color.Primary)

	node := func(v Property) *tree.Node {
		val := ""
		if v.Value != nil {
			val = fmt.Sprint(v.Value)
		}
		n := pn.AddNode()
		n.AddColumn().AddText(v.Title).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(val)
		n.AddColumn().AddText(v.Source)
		return n
	}

	_ = node(t.Properties.Nodename)
	_ = node(t.Properties.FQDN)
	_ = node(t.Properties.Version)
	_ = node(t.Properties.OSName)
	_ = node(t.Properties.OSVendor)
	_ = node(t.Properties.OSRelease)
	_ = node(t.Properties.OSKernel)
	_ = node(t.Properties.OSArch)
	_ = node(t.Properties.MemBytes)
	_ = node(t.Properties.MemSlots)
	_ = node(t.Properties.MemBanks)
	_ = node(t.Properties.CPUFreq)
	_ = node(t.Properties.CPUThreads)
	_ = node(t.Properties.CPUCores)
	_ = node(t.Properties.CPUDies)
	_ = node(t.Properties.CPUModel)
	_ = node(t.Properties.Serial)
	_ = node(t.Properties.Model)
	_ = node(t.Properties.Manufacturer)
	_ = node(t.Properties.BIOSVersion)
	_ = node(t.Properties.SPVersion)
	_ = node(t.Properties.NodeEnv)
	_ = node(t.Properties.AssetEnv)
	_ = node(t.Properties.Enclosure)
	_ = node(t.Properties.ListenerPort)
	_ = node(t.Properties.ClusterID)
	_ = node(t.Properties.TZ)
	_ = node(t.Properties.ConnectTo)
	_ = node(t.Properties.SecZone)
	_ = node(t.Properties.LastBoot)
	_ = node(t.Properties.BootID)
	_ = node(t.Properties.LocCountry)
	_ = node(t.Properties.LocCity)
	_ = node(t.Properties.LocBuilding)
	_ = node(t.Properties.LocRoom)
	_ = node(t.Properties.LocRack)
	_ = node(t.Properties.LocAddr)
	_ = node(t.Properties.LocFloor)
	_ = node(t.Properties.LocZIP)
	_ = node(t.Properties.TeamInteg)
	_ = node(t.Properties.TeamSupport)

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
		n1 := n.AddNode()
		n1.AddColumn().AddText(v.Name)
		n1.AddColumn().AddText(v.Type)
		sanPaths := t.Targets.WithInitiatorName(v.Name)
		if len(sanPaths) > 0 {
			n2 := n1.AddNode()
			n2.AddColumn().AddText("targets").SetColor(rawconfig.Color.Primary)
			for _, sanPath := range sanPaths {
				n3 := n2.AddNode()
				n3.AddColumn().AddText(sanPath.Target.Name)
			}
		}
	}

	return tr.Render()
}

func (t Data) Values() []Property {
	l := make([]Property, 0)
	m, _ := attr.Values(t.Properties)
	for k, v := range m {
		av, ok := v.(Property)
		if !ok {
			continue
		}
		av.Name, _ = attr.GetTag(t.Properties, k, "json")
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
