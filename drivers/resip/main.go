package resip

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/opensvc/om3/core/fqdn"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/stringset"
	"github.com/rs/zerolog"
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

func WaitDNSRecord(ctx context.Context, timeout *time.Duration, p path.T) error {
	if timeout == nil {
		return nil
	}
	if *timeout == 0 {
		return nil
	}
	logger := zerolog.Ctx(ctx)
	limit := time.Now().Add(*timeout)
	todo := stringset.New()
	clusterSection := rawconfig.ClusterSection()
	name := fqdn.New(p, clusterSection.Name).String()

	for _, dns := range strings.Fields(clusterSection.DNS) {
		todo.Add(dns)
	}
	if len(todo) == 0 {
		return nil
	}
	for {
		logger.Info().Msgf("wait for the %s record to be resolved by dns %s", name, todo.Slice())
		for dns, _ := range todo {
			if ips, err := lookupHostOnDNS(ctx, name, dns); err != nil {
				logger.Info().Err(err).Msgf("lookup %s record on dns %s", name, dns)
				todo.Remove(dns)
			} else if len(ips) > 0 {
				logger.Info().Msgf("lookup %s record on dns %s returns %v", name, dns, ips)
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
