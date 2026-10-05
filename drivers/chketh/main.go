// Package chketh is the eth check driver: the speed, duplex, auto-negotiation
// and link of the network interfaces carrying addresses, and of the bonding
// slaves, as ethtool reports them.
package chketh

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/check/helpers/checkexec"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "eth"
	// DriverName is the name of check driver.
	DriverName = "ethtool"
)

type (
	checker struct{}
)

var speedRe = regexp.MustCompile(`^([0-9]+)`)

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	if checkexec.Find("ethtool", "/sbin", "/usr/sbin") == "" {
		return rs, nil
	}
	for _, intf := range interfaces() {
		b, err := checkexec.OutputIn(ctx, "", []string{"/sbin", "/usr/sbin"}, "ethtool", intf)
		if errors.Is(err, checkexec.ErrNotFound) {
			return rs, nil
		} else if err != nil {
			continue
		}
		for _, r := range results(intf, parse(b)) {
			r.DriverGroup = DriverGroup
			r.DriverName = DriverName
			rs.Push(r)
		}
	}
	return rs, nil
}

// interfaces returns the bonding slaves, and the eth* and en* interfaces
// carrying an address.
func interfaces() []string {
	var l []string
	files, _ := filepath.Glob("/proc/net/bonding/*")
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		l = append(l, bondingSlaves(b)...)
	}
	intfs, _ := net.Interfaces()
	for _, intf := range intfs {
		if !strings.HasPrefix(intf.Name, "eth") && !strings.HasPrefix(intf.Name, "en") {
			continue
		}
		if addrs, err := intf.Addrs(); err != nil || len(addrs) == 0 {
			continue
		}
		if !slices.Contains(l, intf.Name) {
			l = append(l, intf.Name)
		}
	}
	return l
}

// bondingSlaves returns the slaves a /proc/net/bonding file lists.
func bondingSlaves(b []byte) []string {
	var l []string
	for _, line := range strings.Split(string(b), "\n") {
		if name, ok := strings.CutPrefix(line, "Slave Interface: "); ok {
			l = append(l, strings.TrimSpace(name))
		}
	}
	return l
}

// parse reads the settings of an ethtool output, keyed by their name, lower
// case, with spaces and dashes as underscores.
func parse(b []byte) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "\t") {
			continue
		}
		k, v, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if !ok {
			continue
		}
		k = strings.NewReplacer(" ", "_", "-", "_").Replace(strings.ToLower(k))
		m[k] = strings.TrimSpace(v)
	}
	return m
}

// results are the checks of an interface, from its ethtool settings: the
// speed and the auto-negotiation when the speed is known, the duplex when
// known, and the link.
func results(intf string, m map[string]string) []check.Result {
	var l []check.Result
	if speed, ok := m["speed"]; ok && !strings.Contains(speed, "Unknown") {
		if match := speedRe.FindStringSubmatch(speed); match != nil {
			v, _ := strconv.ParseInt(match[1], 10, 64)
			l = append(l, check.Result{Instance: intf + ".speed", Value: v, Unit: "Mb/s"})
		}
		l = append(l, check.Result{Instance: intf + ".autoneg", Value: boolValue(m["auto_negotiation"] == "on")})
	}
	if duplex, ok := m["duplex"]; ok && !strings.Contains(duplex, "Unknown") {
		l = append(l, check.Result{Instance: intf + ".duplex", Value: boolValue(duplex == "Full")})
	}
	if link, ok := m["link_detected"]; ok {
		l = append(l, check.Result{Instance: intf + ".link", Value: boolValue(link == "yes")})
	}
	return l
}

func boolValue(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
