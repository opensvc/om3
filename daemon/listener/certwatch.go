package listener

import (
	"context"
	"errors"

	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// startCertWatch keeps the certificate of system/sec/cert naming the cluster
// nodes, and the listeners presenting the current one.
//
// The speaker renews the certificate when the cluster nodes change, when
// system/sec/cert changes, as when alt_names is set, and when it becomes the
// speaker, as on its start: it is issued for the names of alt_names,
// {clusternodes} by default, so a node joining is named in it, and a node
// leaving is not anymore. A single node renews, as the certificate is
// one for all the nodes, and the others receive it as any configuration.
//
// Every node installs the certificate files again when system/sec/cert
// changes, renewed here or anywhere else: the inet listener presents them,
// read again when they change, and the clients of the daemon read them.
func (t *T) startCertWatch(ctx context.Context) {
	localhost := hostname.Hostname()
	labelLocalhost := pubsub.Label{"node", localhost}
	sub := pubsub.SubFromContext(ctx, "daemon.listener.cert")
	sub.AddFilter(&msgbus.InstanceConfigUpdated{}, labelLocalhost, pubsub.Label{"path", certPath.String()})
	sub.AddFilter(&msgbus.ClusterConfigUpdated{}, labelLocalhost)
	sub.AddFilter(&msgbus.NodeStatusUpdated{}, labelLocalhost)
	sub.Start()
	go func() {
		defer func() {
			if err := sub.Stop(); err != nil {
				t.log.Warnf("cert watch: subscription stop: %s", err)
			}
		}()
		// The speaker may have been elected before the subscription.
		isSpeaker := false
		if status := node.StatusData.GetByNode(localhost); status != nil && status.IsLeader {
			isSpeaker = true
			t.renewCert(ctx)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case i := <-sub.C:
				switch m := i.(type) {
				case *msgbus.InstanceConfigUpdated:
					t.reinstallCertFiles()
					// alt_names may have changed: a renewal
					// rewrites the sec too, and comes back here
					// with a certificate naming them, which is
					// not due.
					if status := node.StatusData.GetByNode(localhost); status != nil && status.IsLeader {
						t.renewCert(ctx)
					}
				case *msgbus.ClusterConfigUpdated:
					if len(m.NodesAdded) == 0 && len(m.NodesRemoved) == 0 {
						continue
					}
					if status := node.StatusData.GetByNode(localhost); status != nil && status.IsLeader {
						t.renewCert(ctx)
					}
				case *msgbus.NodeStatusUpdated:
					if m.Value.IsLeader && !isSpeaker {
						t.renewCert(ctx)
					}
					isSpeaker = m.Value.IsLeader
				}
			}
		}
	}()
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
// A system/sec/cert without alt_names, as the bootstrap of the clusters
// created before it set {clusternodes} made, has it set first.
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
	if !sec.Config().HasKey(altNames) {
		t.log.Infof("cert watch: %s: set alt_names = %s", certPath, certAltNames)
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
