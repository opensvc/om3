//go:build linux

package resip

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/opensvc/om3/v3/core/actionrollback"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/plog"
)

// PublishModesRefused are the ip.netns modes the node does not reach the
// address in, so a port of the node can not be forwarded to it.
var PublishModesRefused = []string{"dedicated", "macvlan", "ipvlan-l2", "ipvlan-l3", "ipvlan-l3s"}

// PublishOwner is the resource the forwardings are recorded under on the
// node, in a directory of its object, and the comment of their rules.
func PublishOwner(p naming.Path, rid string) network.PublishOwner {
	return network.PublishOwner{Path: p, RID: rid}
}

// StartPublish forwards the ports of the node the publish keyword lists to
// addr, the address the resource plumbed, on the network subnet, and has a
// rollback remove them. A resource publishing nothing has nothing to do.
func StartPublish(ctx context.Context, log *plog.Logger, owner network.PublishOwner, publish []string, addr net.IP, subnet *net.IPNet) error {
	if len(publish) == 0 {
		return nil
	}
	publishes, err := network.ParsePublishes(publish)
	if err != nil {
		return err
	}
	if err := network.CheckForwarding(addr); err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	resolved, err := network.PublishStart(ctx, log, owner, addr, subnet, publishes)
	if err != nil {
		return err
	}
	log.Infof("published %s to %s", publishStrings(resolved), addr)
	actionrollback.Register(ctx, func(ctx context.Context) error {
		return network.PublishStop(ctx, log, owner)
	})
	return nil
}

// StopPublish removes the forwardings of the resource, the ones of a start
// made with a former configuration included.
func StopPublish(ctx context.Context, log *plog.Logger, owner network.PublishOwner) error {
	return network.PublishStop(ctx, log, owner)
}

// StatusPublish says in the status log of an address up what keeps its
// published ports from being forwarded, and returns the state the status
// takes: warn when they are not, else s.
func StatusPublish(slog resource.StatusLogger, owner network.PublishOwner, publish []string, addr net.IP, s status.T) status.T {
	if len(publish) == 0 || s != status.Up {
		return s
	}
	recorded, loaded, err := network.PublishStatus(owner)
	switch {
	case err != nil:
		slog.Warn("publish: %s", err)
		return status.Warn
	case !recorded:
		slog.Warn("publish: %s not forwarded", strings.Join(publish, " "))
		return status.Warn
	case !loaded:
		slog.Warn("publish: table %s not loaded", network.PublishTableName)
		return status.Warn
	}
	var off *network.ErrForwardingOff
	if err := network.CheckForwarding(addr); errors.As(err, &off) {
		slog.Warn("publish: forwarding off (%s=%s)", off.Sysctl, off.Value)
		return status.Warn
	} else if err != nil {
		slog.Warn("publish: %s", err)
		return status.Warn
	}
	return s
}

// PublishInfo returns the publish and expose values the status info of the
// resource reports, the ones the cluster DNS makes the SRV records of.
//
// The publishes are the ones the start resolved, their drawn host ports
// given, else the declared ones. The exposes are:
//
//   - none, for an expose set to "none";
//   - the expose keyword, when set, which replaces the published ports;
//   - else every published port with a host port, as the expose
//     "<host port>:<port>/<proto>" would: a port to draw not drawn yet
//     exposes nothing.
func PublishInfo(owner network.PublishOwner, publish, expose []string) ([]string, []string) {
	var resolved []network.Publish
	if len(publish) > 0 {
		resolved, _ = network.PublishResolved(owner)
	}
	return publishInfo(resolved, publish, expose)
}

// publishInfo is PublishInfo with the publishes the start resolved given.
func publishInfo(resolved []network.Publish, publish, expose []string) ([]string, []string) {
	published := publish
	if len(publish) > 0 {
		if len(resolved) > 0 {
			published = strings.Fields(publishStrings(resolved))
		} else if l, err := network.ParsePublishes(publish); err == nil {
			resolved = l
		}
	}
	switch {
	case len(expose) == 1 && expose[0] == exposeNone:
		return published, []string{}
	case len(expose) > 0:
		return published, expose
	}
	exposed := make([]string, 0, len(resolved))
	for _, p := range resolved {
		if p.Random() {
			continue
		}
		exposed = append(exposed, fmt.Sprintf("%d:%d/%s", p.HostPort, p.Port, p.Proto))
	}
	return published, exposed
}

func publishStrings(l []network.Publish) string {
	s := make([]string, len(l))
	for i, p := range l {
		s[i] = p.String()
	}
	return strings.Join(s, " ")
}
