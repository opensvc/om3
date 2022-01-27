package network

import (
	"net"

	"github.com/vishvananda/netlink"
	"opensvc.com/opensvc/util/rttables"
)

type (
	Route struct {
		Nodename string     `json:"node"`
		Dev      string     `json:"dev"`
		Dst      *net.IPNet `json:"dst"`
		Gateway  net.IP     `json:"gw"`
		Table    string     `json:"table"`
	}
	Routes []Route
)

func (t Routes) Add() error {
	for _, r := range t {
		if err := r.Add(); err != nil {
			return err
		}
	}
	return nil
}

func (t Route) String() string {
	if t.Dst == nil {
		return ""
	}
	s := t.Dst.String()
	gw := t.Gateway.String()
	if gw != "" {
		s = s + " gw " + gw
	}
	if t.Dev != "" {
		s = s + " dev " + t.Dev
	}
	if t.Table != "" {
		s = s + " table " + t.Table
	}
	return s
}

func (t Route) Add() error {
	nlRoute := &netlink.Route{
		Dst: t.Dst,
		Gw:  t.Gateway,
	}
	if intf, err := net.InterfaceByName(t.Dev); err != nil {
		return err
	} else {
		nlRoute.LinkIndex = intf.Index
	}
	if i, err := rttables.Index(t.Table); err != nil {
		return err
	} else {
		nlRoute.Table = i
	}
	return netlink.RouteAdd(nlRoute)
}
