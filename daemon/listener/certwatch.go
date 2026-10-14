package listener

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// certRenewInterval is the interval the speaker checks the expiry of the
// certificates of system/sec/ca and system/sec/cert at, besides the events.
const certRenewInterval = 24 * time.Hour

// startCertWatch keeps the certificates of system/sec/ca and system/sec/cert
// valid, the one of system/sec/cert naming the cluster nodes, and the
// listeners presenting the current one.
//
// The speaker renews the certificate of system/sec/cert when the cluster
// nodes change, when system/sec/cert changes, as when alt_names is set, when
// system/sec/ca changes, as when it is renewed, when it becomes the speaker,
// as on its start, and daily for its expiry: it is issued for the names of
// alt_names, {clusternodes} by default, so a node joining is named in it,
// and a node leaving is not anymore. It renews the certificate of
// system/sec/ca before, with the same key, so the certificates it signed
// are verified by the renewed one. A single node renews, as the
// certificates are the ones of all the nodes, and the others receive them
// as any configuration.
//
// Every node installs the certificate files again when system/sec/cert
// changes, renewed here or anywhere else: the inet listener presents them,
// read again when they change, and the clients of the daemon read them.
// Every node installs the files of the cluster authorities again when one
// of them changes, or the cluster ca keyword does, and initializes the
// authentication strategies again, which verify with them.
func (t *T) startCertWatch(ctx context.Context) {
	localhost := hostname.Hostname()
	labelLocalhost := pubsub.Label{"node", localhost}
	sub := pubsub.SubFromContext(ctx, "daemon.listener.cert")
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.NodeStatusUpdated{}, labelLocalhost)
	sub.Start()
	go func() {
		defer func() {
			if err := sub.Stop(); err != nil {
				t.log.Warnf("cert watch: subscription stop: %s", err)
			}
		}()
		ticker := time.NewTicker(certRenewInterval)
		defer ticker.Stop()
		isLeader := func() bool {
			status := node.StatusData.GetByNode(localhost)
			return status != nil && status.IsLeader
		}
		caSecPaths := clusterCASecPaths()
		// The speaker may have been elected before the subscription.
		isSpeaker := false
		if isLeader() {
			isSpeaker = true
			t.renewCA(ctx)
			t.renewCert(ctx)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if isLeader() {
					t.renewCA(ctx)
					t.renewCert(ctx)
				}
			case i := <-sub.C:
				switch m := i.(type) {
				case *msgbus.InstanceConfigUpdated:
					switch {
					case m.Path == certPath:
						t.reinstallCertFiles()
						// alt_names may have changed: a renewal
						// rewrites the sec too, and comes back here
						// with a certificate naming them, which is
						// not due.
						if isLeader() {
							t.renewCert(ctx)
						}
					case m.Path == caPath:
						t.reinstallCaFiles()
						// The chain of the certificate holds the
						// certificate of the ca.
						if isLeader() {
							t.renewCert(ctx)
						}
					case slices.Contains(caSecPaths, m.Path.String()):
						t.reinstallCaFiles()
					}
				case *msgbus.ClusterConfigUpdated:
					if l := m.Value.CASecPaths; !slices.Equal(l, caSecPaths) {
						caSecPaths = slices.Clone(l)
						t.reinstallCaFiles()
					}
					if len(m.NodesAdded) == 0 && len(m.NodesRemoved) == 0 {
						continue
					}
					if isLeader() {
						t.renewCert(ctx)
					}
				case *msgbus.NodeStatusUpdated:
					if m.Value.IsLeader && !isSpeaker {
						t.renewCA(ctx)
						t.renewCert(ctx)
					}
					isSpeaker = m.Value.IsLeader
				}
			}
		}
	}()
}

// reinstallCaFiles installs the files of the cluster authorities again, and
// initializes the authentication strategies again with them.
func (t *T) reinstallCaFiles() {
	clusterName, err := getClusterName()
	if err != nil {
		t.log.Warnf("cert watch: install %s files: %s", caPath, err)
		return
	}
	if err := t.installCaFiles(clusterName); err != nil {
		t.log.Warnf("cert watch: install %s files: %s", caPath, err)
		return
	}
	daemonauth.RefreshStrategies()
}

// renewCA renews the certificate of system/sec/ca with the same key, within
// the renewal window of its validity. A certificate it did not issue, as one
// installed from elsewhere, is kept.
//
// A system/sec/ca without validity, as one bootstrapped before it was set,
// has it set first: its certificate was issued for the 1 year default.
func (t *T) renewCA(ctx context.Context) {
	sec, err := object.NewSec(caPath, object.WithVolatile(false))
	if err != nil {
		t.log.Warnf("cert watch: %s: %s", caPath, err)
		return
	}
	if sec.Config().GetString(key.Parse("acme.directory")) != "" {
		return
	}
	validityKey := key.New("DEFAULT", "validity")
	if !sec.Config().HasKey(validityKey) {
		t.log.Infof("cert watch: %s: set validity = %s", caPath, caValidity)
		if err := sec.Config().Set(*keyop.New(validityKey, keyop.Set, caValidity, 0)); err != nil {
			t.log.Warnf("cert watch: %s: set validity: %s", caPath, err)
			return
		}
	}
	var opts object.CertificateRenewOptions
	if validity := sec.Config().GetDuration(validityKey); validity != nil {
		opts.RenewBefore = caRenewBefore(*validity)
	}
	result, err := sec.RenewCertificate(ctx, opts)
	switch {
	case errors.Is(err, object.ErrCertificateNotIssued):
		t.log.Infof("cert watch: %s: %s", caPath, err)
	case err != nil:
		t.log.Warnf("cert watch: %s: renew: %s", caPath, err)
	case result.Renewed:
		t.log.Infof("cert watch: %s: renewed until %s: %s", caPath, result.NotAfter.Local().Format(time.RFC3339), result.Reason)
	default:
		t.log.Debugf("cert watch: %s: not renewed: %s", caPath, result.Reason)
	}
}

// clusterCASecPaths returns the secs of the cluster ca keyword, as the
// cluster config cache says.
func clusterCASecPaths() []string {
	if cfg := cluster.ConfigData.Get(); cfg != nil {
		return slices.Clone(cfg.CASecPaths)
	}
	return nil
}

// reinstallCertFiles installs the certificate files of system/sec/cert again.
func (t *T) reinstallCertFiles() {
	clusterName, err := getClusterName()
	if err != nil {
		t.log.Warnf("cert watch: install %s files: %s", certPath, err)
		return
	}
	if err := t.installCertFiles(clusterName); err != nil {
		t.log.Warnf("cert watch: install %s files: %s", certPath, err)
	}
}

// renewCert renews the certificate of system/sec/cert when its names are not
// the ones alt_names evaluates to. A certificate it did not issue, as one
// installed from elsewhere, is kept: its names are what the operator chose.
//
// A system/sec/cert without alt_names, as one migrated from a v2 cluster, or
// with the alt_names the bootstrap set before {clusternodes}, the hostname of
// the node bootstrapping it, has it set first.
func (t *T) renewCert(ctx context.Context) {
	sec, err := object.NewSec(certPath, object.WithVolatile(false))
	if err != nil {
		t.log.Warnf("cert watch: %s: %s", certPath, err)
		return
	}
	if sec.Config().GetString(key.Parse("acme.directory")) != "" {
		// Renewed from its directory, by the acme tasks.
		return
	}
	altNames := key.New("DEFAULT", "alt_names")
	hasAltNames := sec.Config().HasKey(altNames)
	if was := sec.Config().Get(altNames); !hasAltNames || isLegacyCertAltNames(was, clusterNodes()) {
		if hasAltNames {
			// Said, for an operator who chose this name to set it back.
			t.log.Infof("cert watch: %s: set alt_names = %s, was %s", certPath, certAltNames, was)
		} else {
			t.log.Infof("cert watch: %s: set alt_names = %s", certPath, certAltNames)
		}
		if err := sec.Config().Set(*keyop.New(altNames, keyop.Set, certAltNames, 0)); err != nil {
			t.log.Warnf("cert watch: %s: set alt_names: %s", certPath, err)
			return
		}
	}
	result, err := sec.RenewCertificate(ctx, object.CertificateRenewOptions{})
	switch {
	case errors.Is(err, object.ErrCertificateNotIssued):
		t.log.Infof("cert watch: %s: %s", certPath, err)
	case err != nil:
		t.log.Warnf("cert watch: %s: renew: %s", certPath, err)
	case result.Renewed:
		t.log.Infof("cert watch: %s: renewed for %v: %s", certPath, result.Domains, result.Reason)
	default:
		t.log.Debugf("cert watch: %s: not renewed: %s", certPath, result.Reason)
	}
}

// isLegacyCertAltNames says whether s, the alt_names of system/sec/cert, is
// the one its bootstrap set before {clusternodes}: the hostname of the node
// bootstrapping it, a cluster node. A certificate naming that node alone is
// never due when the cluster nodes change.
func isLegacyCertAltNames(s string, nodes []string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t,{") {
		return false
	}
	return slices.ContainsFunc(nodes, func(node string) bool {
		return strings.EqualFold(node, s)
	})
}

// clusterNodes returns the cluster nodes, as the cluster config cache says.
func clusterNodes() []string {
	if cfg := cluster.ConfigData.Get(); cfg != nil {
		return cfg.Nodes
	}
	return nil
}
