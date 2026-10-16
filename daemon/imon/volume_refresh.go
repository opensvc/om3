package imon

import (
	"fmt"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// volumePaths returns the vol objects the volume resources of the local
// instance use, as their status info names them.
func (t *Manager) volumePaths() naming.Paths {
	var l naming.Paths
	for _, rstat := range t.instStatus[t.localhost].Resources {
		if rstat.Type != "volume" {
			continue
		}
		s, ok := rstat.Info["path"].(string)
		if !ok {
			continue
		}
		p, err := naming.ParsePath(s)
		if err != nil || p == t.path || slices.Contains(l, p) {
			continue
		}
		l = append(l, p)
	}
	return l
}

// janitorVolumes subscribes to the status updates of the local instances
// of the vol objects the local instance uses, and unsubscribes from those
// it no longer uses: a volume resource relays what the local instance of
// its vol publishes, as a start blocker, so its status is stale once that
// instance changed.
func (t *Manager) janitorVolumes() {
	if t.volumeSeen == nil {
		t.volumeSeen = make(map[naming.Path]string)
	}
	paths := t.volumePaths()
	for _, p := range paths {
		if _, ok := t.volumeSeen[p]; ok {
			continue
		}
		t.log.Infof("subscribe to %s@%s instance status updates and deletes, used by a volume resource", p, t.localhost)
		t.sub.AddFilter(&msgbus.InstanceStatusUpdated{}, volumeLabels(p, t.localhost)...)
		t.sub.AddFilter(&msgbus.InstanceStatusDeleted{}, volumeLabels(p, t.localhost)...)
		t.volumeSeen[p] = ""
		if st := instance.StatusData.GetByPathAndNode(p, t.localhost); st != nil {
			t.volumeSeen[p] = volumeSignature(*st)
		}
	}
	for p := range t.volumeSeen {
		if slices.Contains(paths, p) {
			continue
		}
		t.log.Infof("unsubscribe from %s@%s instance status updates and deletes, no longer used by a volume resource", p, t.localhost)
		t.sub.DelFilter(&msgbus.InstanceStatusUpdated{}, volumeLabels(p, t.localhost)...)
		t.sub.DelFilter(&msgbus.InstanceStatusDeleted{}, volumeLabels(p, t.localhost)...)
		delete(t.volumeSeen, p)
	}
}

// volumeLabels are the labels of the filters watching the instance of a vol
// on a node. The namespace label makes them filters of their own: a relation
// watching the same instance subscribes with the path and node labels only,
// and the removal of a filter removes every filter of the same labels.
func volumeLabels(p naming.Path, node string) []pubsub.Label {
	return []pubsub.Label{{"namespace", p.Namespace}, {"path", p.String()}, {"node", node}}
}

// onVolumeInstanceStatusDeleted refreshes the local instance status when
// the local instance of a vol object it uses is gone.
func (t *Manager) onVolumeInstanceStatusDeleted(c *msgbus.InstanceStatusDeleted) {
	if c.Node != t.localhost {
		return
	}
	if _, ok := t.volumeSeen[c.Path]; !ok {
		return
	}
	t.volumeSeen[c.Path] = ""
	t.scheduleEventRefresh("volume " + c.Path.String() + " deleted")
}

// onVolumeInstanceStatusUpdated refreshes the local instance status when
// the local instance of a vol object it uses changed what its volume
// resource relays.
func (t *Manager) onVolumeInstanceStatusUpdated(c *msgbus.InstanceStatusUpdated) {
	if c.Node != t.localhost {
		return
	}
	last, ok := t.volumeSeen[c.Path]
	if !ok {
		return
	}
	sig := volumeSignature(c.Value)
	if sig == last {
		return
	}
	t.volumeSeen[c.Path] = sig
	t.scheduleEventRefresh("volume " + c.Path.String() + " changed")
}

// volumeSignature sums up what a volume resource relays of the instance of
// its vol: the avail and overall status, the status of its resources, and
// what would refuse their start.
func volumeSignature(st instance.Status) string {
	l := []string{st.Avail.String(), st.Overall.String()}
	for rid, rstat := range st.Resources {
		s := fmt.Sprintf("%s=%s", rid, rstat.Status)
		if rstat.StartBlocker != nil {
			s += fmt.Sprintf(" blocked by %s", rstat.StartBlocker.Reason)
		}
		l = append(l, s)
	}
	slices.Sort(l[2:])
	return strings.Join(l, ",")
}
