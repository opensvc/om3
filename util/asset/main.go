package asset

import (
	"net"
	"strings"
	"time"

	"github.com/zcalusic/sysinfo"
)

var (
	si          sysinfo.SysInfo
	initialized bool
)

type (
	T struct{}

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
		Address string `json:"addr"`
		//FlagDeprecated bool   `json:"flag_deprecated"`
		Intf string `json:"intf"`
		Mask string `json:"mask"`
		Type string `json:"type"`
	}
)

func New() *T {
	t := T{}
	if !initialized {
		si.GetSysInfo()
	}
	return &t
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
