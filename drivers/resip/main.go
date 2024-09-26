package resip

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/stringset"
)

func lookupHostOnDNS(ctx context.Context, name, dns string) ([]string, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, dns+":53")
		},
	}
	return r.LookupHost(ctx, name)
}

func WaitDNSRecord(ctx context.Context, timeout *time.Duration, name string, nameservers []string) error {
	if timeout == nil {
		return nil
	}
	if *timeout == 0 {
		return nil
	}
	logger := plog.Ctx(ctx)
	limit := time.Now().Add(*timeout)
	todo := stringset.New()

	for _, nameserver := range nameservers {
		todo.Add(nameserver)
	}
	if len(todo) == 0 {
		return nil
	}
	for {
		logger.Infof("wait for the %s record to be resolved by dns %s", name, todo.Slice())
		for dns := range todo {
			if ips, err := lookupHostOnDNS(ctx, name, dns); err != nil {
				logger.Infof("lookup %s record on dns %s: %s", name, dns, err)
				todo.Remove(dns)
			} else if len(ips) > 0 {
				logger.Infof("lookup %s record on dns %s returns %v", name, dns, ips)
				todo.Remove(dns)
			}
		}
		if len(todo) == 0 {
			break
		}
		if time.Now().After(limit) {
			return fmt.Errorf("timeout waiting for dns %s to resolve on %s", name, todo)
		}
		time.Sleep(300 * time.Millisecond)
	}
	return nil
}
