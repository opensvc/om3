// Package chklag is the Linux bonding check driver: the MII status of each
// bond carrying an address and of its slaves, the link failures per hour of
// the slaves, and the number of slaves.
package chklag

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

const (
	// DriverGroup is the type of check driver.
	DriverGroup = "lag"
	// DriverName is the name of check driver.
	DriverName = "bonding"

	// cacheRefresh is how long a link failure count is kept as the base
	// of the rate: refreshed sooner, a burst of failures would be averaged
	// away before it is seen.
	cacheRefresh = 3600.
)

type (
	checker struct{}

	// failureCount is a link failure count, and the uptime it was read at.
	failureCount struct {
		uptime float64
		count  int64
	}
)

func init() {
	check.Register(&checker{})
}

func (t *checker) Check(ctx context.Context, objs []interface{}) (*check.ResultSet, error) {
	rs := check.NewResultSet()
	files, _ := filepath.Glob("/proc/net/bonding/*")
	if len(files) == 0 {
		return rs, nil
	}
	uptime, err := readUptime()
	if err != nil {
		return rs, err
	}
	for _, f := range files {
		bond := filepath.Base(f)
		if !hasAddress(bond) {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, r := range results(bond, b, uptime, cachedFailureCount) {
			r.DriverGroup = DriverGroup
			r.DriverName = DriverName
			rs.Push(r)
		}
	}
	return rs, nil
}

func hasAddress(name string) bool {
	intf, err := net.InterfaceByName(name)
	if err != nil {
		return false
	}
	addrs, err := intf.Addrs()
	return err == nil && len(addrs) > 0
}

func readUptime() (float64, error) {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0, fmt.Errorf("unexpected /proc/uptime content")
	}
	return strconv.ParseFloat(fields[0], 64)
}

// results reads a /proc/net/bonding file. failures returns the link failure
// count of a slave the rate is computed from, and records the current one.
func results(bond string, b []byte, uptime float64, failures func(bond, slave string, cur failureCount) failureCount) []check.Result {
	var (
		l      []check.Result
		inst   = bond
		slave  string
		slaves int64
	)
	for _, line := range strings.Split(string(b), "\n") {
		switch {
		case strings.HasPrefix(line, "Slave Interface:"):
			slaves++
			fields := strings.Fields(line)
			slave = fields[len(fields)-1]
			inst = bond + "." + slave
		case strings.HasPrefix(line, "MII Status:"):
			var v int64
			if !strings.HasSuffix(strings.TrimSpace(line), " up") {
				v = 1
			}
			l = append(l, check.Result{Instance: inst + ".mii_status", Value: v})
		case strings.HasPrefix(line, "Link Failure Count:") && slave != "":
			fields := strings.Fields(line)
			count, err := strconv.ParseInt(fields[len(fields)-1], 10, 64)
			if err != nil {
				continue
			}
			prev := failures(bond, slave, failureCount{uptime: uptime, count: count})
			if uptime <= prev.uptime {
				continue
			}
			perHour := 3600. * float64(count-prev.count) / (uptime - prev.uptime)
			l = append(l, check.Result{Instance: inst + ".link_failure_per_hour", Value: int64(math.Round(perHour))})
		}
	}
	l = append(l, check.Result{Instance: bond + ".paths", Value: slaves})
	return l
}

// cachedFailureCount returns the link failure count of the slave recorded
// last, or none when the node rebooted since, and records the current one
// when the recorded one is older than an hour.
func cachedFailureCount(bond, slave string, cur failureCount) failureCount {
	p := filepath.Join(rawconfig.Paths.Tmp, "check.lag."+bond+"."+slave)
	var prev failureCount
	if b, err := os.ReadFile(p); err == nil {
		var data [2]float64
		if json.Unmarshal(b, &data) == nil {
			prev = failureCount{uptime: data[0], count: int64(data[1])}
		}
	}
	if prev.uptime >= cur.uptime {
		// Rebooted: the counters started again.
		prev = failureCount{}
	}
	if cur.uptime-prev.uptime > cacheRefresh {
		if b, err := json.Marshal([2]float64{cur.uptime, float64(cur.count)}); err == nil {
			_ = os.MkdirAll(filepath.Dir(p), 0o700)
			_ = os.WriteFile(p, b, 0o600)
		}
	}
	return prev
}
