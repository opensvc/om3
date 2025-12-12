package network

import (
	"math"

	"github.com/opensvc/om3/v3/core/clusterip"
)

type (
	Usage struct {
		Free int `json:"free"`
		Used int `json:"used"`
		Size int `json:"size"`
	}

	Status struct {
		Name    string      `json:"name"`
		Type    string      `json:"type"`
		Network string      `json:"network"`
		IPs     clusterip.L `json:"ips"`
		Errors  []string    `json:"errors,omitempty"`
		Usage
	}
	StatusList []Status
)

func NewStatus() Status {
	t := Status{}
	t.IPs = make(clusterip.L, 0)
	t.Errors = make([]string, 0)
	return t
}

func GetStatus(t Networker, ips clusterip.L) Status {
	data := NewStatus()
	data.Type = t.Type()
	data.Name = t.Name()
	data.Network = t.Network()
	if ips != nil {
		data.IPs = t.FilterIPs(ips)
		data.Usage.Used = len(data.IPs)
		if ipn, err := t.IPNet(); err == nil {
			ones, bits := ipn.Mask.Size()
			data.Usage.Size = int(math.Pow(2.0, float64(bits-ones)))
			data.Usage.Free = data.Usage.Size - data.Usage.Used
		}
	}
	return data
}

func NewStatusList() StatusList {
	return make(StatusList, 0)
}

func (t StatusList) Len() int {
	return len(t)
}

func (t StatusList) Less(i, j int) bool {
	return t[i].Name < t[j].Name
}

func (t StatusList) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t StatusList) Add(p Networker, ips clusterip.L) StatusList {
	s := GetStatus(p, ips)
	l := []Status(t)
	l = append(l, s)
	return StatusList(l)
}

func ShowNetworksByName(noder Noder, name string, ips clusterip.L) StatusList {
	l := NewStatusList()
	for _, p := range Networks(noder) {
		if name != "" && name != p.Name() {
			continue
		}
		l = l.Add(p, ips)
	}
	return l
}

func ShowNetworks(noder Noder, ips clusterip.L) StatusList {
	return ShowNetworksByName(noder, "", ips)
}
