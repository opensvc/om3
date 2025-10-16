package nmon

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/pubsub"
)

// onJoinRequest handle JoinRequest to update cluster config with new node.
//
// If error occurs publish msgbus.JoinIgnored, or msgbus.JoinError:
//
// - publish msgbus.JoinIgnored,join-node=node (the node already exists in cluster nodes)
// - publish msgbus.JoinError,join-node=node (update cluster config object fails)
func (t *Manager) onJoinRequest(c *msgbus.JoinRequest) {
	nodes := cluster.ConfigData.Get().Nodes
	node := c.Node
	labels := []pubsub.Label{
		{"node", hostname.Hostname()},
		{"join-node", node},
	}
	t.log.Infof("join request for node %s", node)
	if slices.Contains(nodes, node) {
		t.log.Debugf("join request ignored already member")
		t.publisher.Pub(&msgbus.JoinIgnored{Node: node}, labels...)
	} else if err := t.addClusterNode(node); err != nil {
		t.log.Warnf("join request denied: %s", err)
		t.publisher.Pub(&msgbus.JoinError{Node: node, Reason: err.Error()}, labels...)
	}
}

// addClusterNode adds node to cluster config
func (t *Manager) addClusterNode(node string) error {
	t.log.Debugf("adding cluster node %s", node)
	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}
	op := keyop.New(key.New("cluster", "nodes"), keyop.Append, node, 0)
	if err := ccfg.Config().Set(*op); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	return nil
}

// onLeaveRequest handle LeaveRequest to update cluster config with node removal.
//
// If an error occurs, publish msgbus.LeaveIgnored, or msgbus.LeaveError:
//
// - publish msgbus.LeaveIgnored,leave-node=node (the node is not a cluster nodes)
// - publish msgbus.LeaveError,leave-node=node (update cluster config object fails)
func (t *Manager) onLeaveRequest(c *msgbus.LeaveRequest) {
	nodes := cluster.ConfigData.Get().Nodes
	node := c.Node
	labels := []pubsub.Label{
		{"node", hostname.Hostname()},
		{"leave-node", node},
	}
	t.log.Infof("leave request for node %s", node)
	if !slices.Contains(nodes, node) {
		t.log.Debugf("leave request ignored for not cluster member")
		t.publisher.Pub(&msgbus.LeaveIgnored{Node: node}, labels...)
	} else if err := t.removeClusterNode(node); err != nil {
		t.log.Warnf("leave request denied: %s", err)
		t.publisher.Pub(&msgbus.LeaveError{Node: node, Reason: err.Error()}, labels...)
	}
}

// removeClusterNode removes node from cluster config
func (t *Manager) removeClusterNode(node string) error {
	t.log.Debugf("removing cluster node %s", node)
	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}
	op := keyop.New(key.New("cluster", "nodes"), keyop.Remove, node, 0)
	if err := ccfg.Config().Set(*op); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	return nil
}

// onHeartbeatRotateRequest handle heartbeat secret rotate to update cluster hb config
// with the next secret.
//
// If an error occurs, publish msgbus.HeartbeatRotateError:
func (t *Manager) onHeartbeatRotateRequest(c *msgbus.HeartbeatRotateRequest) {
	if t.hbSecretRotating {
		t.log.Warnf("heartbeat rotate request ignored already rotating")
		t.publisher.Pub(&msgbus.HeartbeatRotateError{Reason: "already rotating", ID: c.ID}, t.labelLocalhost)
		return
	}
	t.log.Infof("heartbeat rotate request")

	version, currentSecret, nextVersion, _ := cluster.ConfigData.Get().Heartbeat.Secrets()
	nextSecret := strings.ReplaceAll(uuid.New().String(), "-", "")
	nextVersion = max(version, nextVersion) + 1
	value := fmt.Sprintf("%d:%s %d:%s", version, currentSecret, nextVersion, nextSecret)

	if err := t.setClusterHeartbeatSecret(value); err != nil {
		t.log.Warnf("heartbeat rotate request failed: %s", err)
		t.publisher.Pub(&msgbus.HeartbeatRotateError{Reason: err.Error(), ID: c.ID}, t.labelLocalhost)
		return
	}
	t.hbSecretRotating = true
	t.hbSecretRotatingAt = time.Now()
	t.hbSecretRotatingUUID = c.ID
}

func (t *Manager) setClusterHeartbeatSecret(value string) error {
	t.log.Debugf("setting hb_secret")
	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}
	op := keyop.New(key.New("cluster", "hb_secret"), keyop.Set, value, 0)
	if err := ccfg.Config().Set(*op); err != nil {
		return err
	}
	return nil
}

func (t *Manager) onHeartbeatConfigUpdated(c *msgbus.HeartbeatConfigUpdated) {
	t.hbSecretSigByNodename[c.Nodename] = c.Value.SecretSig
	if t.hbSecretRotating {
		t.hbRotatingCheck()
	}
}

// hbRotatingCheck handles the heartbeat secret rotation process, ensuring all
// nodes are synchronized before applying changes.
// It monitors timeout, evaluates node synchronization, and updates the heartbeat
// secret if conditions are met.
func (t *Manager) hbRotatingCheck() {
	if !t.hbSecretRotating {
		return
	}
	if time.Now().After(t.hbSecretRotatingAt.Add(15 * time.Second)) {
		t.log.Warnf("heartbeat rotate request timed out")
		t.publisher.Pub(&msgbus.HeartbeatRotateError{Reason: "timed out", ID: t.hbSecretRotatingUUID}, t.labelLocalhost)
		t.hbSecretRotating = false
		t.hbSecretRotatingUUID = uuid.UUID{}
		return
	}
	expectedSig := t.hbSecretSigByNodename[t.localhost]
	if expectedSig == "" {
		return
	}
	count := 0
	waitingL := make([]string, 0)
	for peer, sig := range t.hbSecretSigByNodename {
		if sig != expectedSig {
			waitingL = append(waitingL, peer)
		}
		count++
	}
	if len(waitingL) > 0 {
		t.log.Infof("heartbeat rotate waiting for nodes %s", waitingL)
		return
	}
	if count == len(t.clusterConfig.Nodes) {
		cConfig := cluster.ConfigData.Get()
		_, _, nextVersion, nextSecret := cConfig.Heartbeat.Secrets()
		if nextSecret == "" {
			t.log.Warnf("heartbeat rotate failed: next secret is empty")
			t.publisher.Pub(&msgbus.HeartbeatRotateError{Reason: "next secret is empty", ID: t.hbSecretRotatingUUID}, t.labelLocalhost)
			t.hbSecretRotating = false
			t.hbSecretRotatingUUID = uuid.UUID{}
			return
		}
		t.log.Infof("heartbeat rotate prepared")
		s := fmt.Sprintf("%d:%s", nextVersion, nextSecret)
		if err := t.setClusterHeartbeatSecret(s); err != nil {
			t.log.Warnf("heartbeat rotate failed: %s", err)
			t.publisher.Pub(&msgbus.HeartbeatRotateError{Reason: err.Error(), ID: t.hbSecretRotatingUUID}, t.labelLocalhost)
			t.hbSecretRotating = false
			t.hbSecretRotatingUUID = uuid.UUID{}
			return
		}
		t.log.Infof("heartbeat rotate success, next secret gen: %d", nextVersion)
		t.publisher.Pub(&msgbus.HeartbeatRotateSuccess{ID: t.hbSecretRotatingUUID}, t.labelLocalhost)
		t.hbSecretRotating = false
		t.hbSecretRotatingUUID = uuid.UUID{}
		return
	}
}
