package nmon

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/hbsecobject"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (t *Manager) onInstanceConfigUpdated(c *msgbus.InstanceConfigUpdated) {
	if c.Path != naming.SecHb {
		// unexpected since subscribe on SecHb
		t.log.Errorf("unexpected InstanceConfigUpdated for %s", c.Path)
		return
	}
	previousVersion := t.hbSecret.MainVersion()
	if t.hbSecretChecksumByNodename[c.Node] == c.Value.Checksum {
		return
	}
	t.hbSecretChecksumByNodename[c.Node] = c.Value.Checksum

	if err := t.setHbSecretFromFile(); err != nil {
		t.log.Warnf("can't analyse %s: %s", naming.SecHb, err)
		return
	}
	newVersion := t.hbSecret.MainVersion()
	t.publisher.Pub(&msgbus.HeartbeatSecretUpdated{Nodename: t.localhost, Value: *t.hbSecret.DeepCopy()}, t.labelLocalhost)
	if previousVersion != t.hbSecret.MainVersion() {
		t.log.Infof("heartbeat secret version changed from %d to %d", previousVersion, newVersion)
	}
	if !t.hbSecretRotating {
		return
	}
	t.hbRotatingCheck()
}

// onHeartbeatRotateRequest handle heartbeat secret rotate to update cluster hb config
// with the next secret.
//
// If an error occurs, publish msgbus.HeartbeatRotateError:
func (t *Manager) onHeartbeatRotateRequest(c *msgbus.HeartbeatRotateRequest) {
	logP := "heartbeat rotate request"
	onRefused := func(reason string) {
		err := fmt.Errorf(reason)
		t.log.Warnf("%s refused: %s", logP, err)
		t.publisher.Pub(&msgbus.HeartbeatRotateError{Reason: err.Error(), ID: c.ID}, t.labelLocalhost)
	}
	if t.hbSecretRotating {
		onRefused("already rotating")
		return
	}
	if len(t.livePeers) != len(t.clusterConfig.Nodes) {
		onRefused("some cluster nodes are offline")
		return
	}
	expectedChecksum := t.hbSecretChecksumByNodename[t.localhost]
	if expectedChecksum == "" {
		// secret version change been committed, we have to wait for the next event
		// HeartbeatSecretUpdated to avoid re-inserting the previous current secret.
		onRefused("not ready yet, initialising")
		return
	}

	notSameChecksum := make([]string, 0)
	for peer, checksum := range t.hbSecretChecksumByNodename {
		if checksum != expectedChecksum {
			notSameChecksum = append(notSameChecksum, peer)
		}
	}
	if len(notSameChecksum) > 0 {
		onRefused(fmt.Sprintf("not ready yet, found non matching heartbeat checksum on %s", notSameChecksum))
		return
	}

	version := t.hbSecret.MainVersion()
	nextVersion := t.hbSecret.AltSecretVersion()
	secret := t.hbSecret.MainSecret()

	if secret == "" {
		onRefused("current secret must be defined")
		return
	}
	nextSecret := strings.ReplaceAll(uuid.New().String(), "-", "")
	nextVersion = max(version, nextVersion) + 1

	t.log.Infof("%s candidate new secret version %d", logP, nextVersion)
	if err := hbsecobject.UpdateAlternate(nextVersion, nextSecret); err != nil {
		t.log.Errorf("%s: %s", logP, err)
		t.publisher.Pub(&msgbus.HeartbeatRotateError{Reason: err.Error(), ID: c.ID}, t.labelLocalhost)
		return
	}
	t.log.Debugf("%s wait for peer converge candidate new secret version %d", logP, nextVersion)
	t.hbSecretRotating = true
	t.hbSecretRotatingAt = time.Now()
	t.hbSecretRotatingUUID = c.ID
}

// hbRotatingCheck handles the heartbeat secret rotation process, ensuring all
// nodes are synchronized before applying changes.
// It monitors timeout, evaluates node synchronization, and updates the heartbeat
// secret if conditions are met.
func (t *Manager) hbRotatingCheck() {
	if !t.hbSecretRotating {
		return
	}
	logP := "heartbeat rotate request"
	onError := func(reason string) {
		err := fmt.Errorf(reason)
		t.log.Warnf("%s: %s", logP, err)
		t.publisher.Pub(&msgbus.HeartbeatRotateError{Reason: err.Error(), ID: t.hbSecretRotatingUUID}, t.labelLocalhost)
		t.hbSecretRotating = false
		t.hbSecretRotatingUUID = uuid.UUID{}
		return
	}
	if time.Now().After(t.hbSecretRotatingAt.Add(15 * time.Second)) {
		onError("timed out")
		return
	}
	expectedChecksum := t.hbSecretChecksumByNodename[t.localhost]
	if expectedChecksum == "" {
		return
	}
	count := 0
	waitingL := make([]string, 0)
	for peer, checksum := range t.hbSecretChecksumByNodename {
		if checksum != expectedChecksum {
			waitingL = append(waitingL, peer)
		}
		count++
	}
	if len(waitingL) > 0 {
		t.log.Debugf("%s waiting for peers: %s", logP, waitingL)
		return
	}
	if count == len(t.clusterConfig.Nodes) {
		version := t.hbSecret.MainVersion()
		nextVersion := t.hbSecret.AltSecretVersion()
		nextSecret := t.hbSecret.AltSecret()
		if nextSecret == "" {
			onError("next secret is empty")
			return
		}
		if version > nextVersion {
			onError(fmt.Sprintf("current version %d is greater than candidate version %d", version, nextVersion))
			return
		}
		t.log.Debugf("%s commiting version change %d -> %d", logP, version, nextVersion)
		t.hbSecret.Rotate()
		if err := hbsecobject.Set(*t.hbSecret.DeepCopy()); err != nil {
			onError(fmt.Sprintf("commit candidate version %d failed: %s", nextVersion, err))
			return
		}
		t.log.Infof("%s version is now %d", logP, nextVersion)
		t.publisher.Pub(&msgbus.HeartbeatRotateSuccess{ID: t.hbSecretRotatingUUID}, t.labelLocalhost)
		t.hbSecretRotating = false
		t.hbSecretRotatingUUID = uuid.UUID{}
		// reset localhost signature to prevent re-rotation accepted before the new signature is recreated
		delete(t.hbSecretChecksumByNodename, t.localhost)
		return
	}
}

func (t *Manager) setHbSecretFromFile() error {
	if sec, err := hbsecobject.Get(); err != nil {
		return err
	} else {
		t.hbSecret = *sec
	}
	return nil
}
