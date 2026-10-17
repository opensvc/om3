package object

import (
	"context"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/hostname"
)

const (
	writesRunning = "running"
	writesStopped = "stopped"
	writesPrefix  = "writes_"
)

// writesFile is the record of whether the instance of nodename writes the
// data of the object: its content says running or stopped, and its
// modification time since when.
func writesFile(varDir, nodename string) string {
	return filepath.Join(varDir, writesPrefix+nodename)
}

// writesName is the name of the record of nodename in the var dir of the
// object. A node name is one word: one with a path separator names no record.
func writesName(nodename string) (string, error) {
	if nodename == "" || nodename == "." || nodename == ".." || strings.ContainsAny(nodename, `/\`) {
		return "", fmt.Errorf("invalid node name %q", nodename)
	}
	return writesPrefix + nodename, nil
}

// WritesStoppedAt is when the instances of the object of varDir stopped
// writing its data, the latest stop the records of its nodes say, and zero
// while one of them writes, or when none ever said.
//
// A copy of the data, or a snapshot of it, misses the writes made since it
// was taken, which end when the instance writing them stops: the copy of a
// stopped instance does not age.
//
// One instance writing is enough: a node coming back after it died records
// its stop at its return, later than the start of the node that took over.
func WritesStoppedAt(varDir string) time.Time {
	// The records are read inside the var dir only, a link pointing
	// out of it read as no record.
	root, err := os.OpenRoot(varDir)
	if err != nil {
		return time.Time{}
	}
	defer root.Close()
	names, err := iofs.Glob(root.FS(), writesPrefix+"*")
	if err != nil {
		return time.Time{}
	}
	var latest time.Time
	for _, name := range names {
		info, err := root.Stat(name)
		if err != nil {
			continue
		}
		b, err := root.ReadFile(name)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(string(b)) {
		case writesRunning:
			return time.Time{}
		case writesStopped:
			if info.ModTime().After(latest) {
				latest = info.ModTime()
			}
		}
	}
	// A stop recorded ahead of this clock, after the record of a peer
	// whose clock is ahead, counts no writes missed past now.
	if now := time.Now(); latest.After(now) {
		latest = now
	}
	return latest
}

// PeerWritesStopped records that the instance of a node the cluster lost no
// longer writes the data of the object of varDir, since at, when its record
// says it wrote: a dead node writes nothing, and says nothing either. The
// time it was lost is a little after its last write, which counts a little
// more of writes missed, not less. It says whether the record changed.
//
// The loss always stops a record saying the node writes: the time of the
// record comes from the clock of the peer, ahead of this one maybe, and the
// stop is then recorded right after it, so a status of the peer from before
// its loss, replayed from a cache, is older still and changes nothing.
func PeerWritesStopped(varDir, nodename string, at time.Time) (bool, error) {
	name, err := writesName(nodename)
	if err != nil {
		return false, err
	}
	root, err := os.OpenRoot(varDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	defer root.Close()
	info, err := root.Stat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	b, err := root.ReadFile(name)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(string(b)) != writesRunning {
		return false, nil
	}
	if !at.After(info.ModTime()) {
		at = info.ModTime().Add(time.Millisecond)
	}
	if err := root.WriteFile(name, []byte(writesStopped+"\n"), 0o644); err != nil {
		return false, err
	}
	if err := root.Chtimes(name, at, at); err != nil {
		return false, err
	}
	return true, nil
}

// PeerWrites records whether the instance of a peer writes the data of the
// object of varDir, as the status it publishes says, since at, when its
// record here says otherwise: a peer back after it was lost, or one whose
// record was not received. A peer never seen writing is not recorded as
// stopped. It says whether the record changed.
func PeerWrites(varDir, nodename string, data instance.Status) (bool, error) {
	return setWrites(varDir, nodename, IsWriting(data), data.UpdatedAt)
}

// ForgetWrites removes the record of the writes of a node no longer in the
// scope of the object of varDir: it writes nothing of the object.
func ForgetWrites(varDir, nodename string) error {
	name, err := writesName(nodename)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(varDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// setWrites records whether the instance of nodename writes the data of the
// object of varDir, since at, when its record says otherwise. A node never
// seen writing is not recorded as stopped. It says whether the record
// changed.
func setWrites(varDir, nodename string, writing bool, at time.Time) (bool, error) {
	return recordWritesIn(varDir, nodename, writing, at, false)
}

// recordWritesIn records whether the instance of nodename writes the data of
// the object of varDir, creating the var dir when create is set: the local
// instance records its own writes from its first status, which may come
// before anything else made the var dir. A record of a peer creates none: an
// object without one, as purged, has no instance here for a copy to judge.
func recordWritesIn(varDir, nodename string, writing bool, at time.Time, create bool) (bool, error) {
	name, err := writesName(nodename)
	if err != nil {
		return false, err
	}
	if create {
		if err := os.MkdirAll(varDir, 0o755); err != nil {
			return false, err
		}
	}
	root, err := os.OpenRoot(varDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	defer root.Close()
	if at.IsZero() {
		at = time.Now()
	}
	if info, err := root.Stat(name); err == nil && !at.After(info.ModTime()) {
		// A status older than the record, as one of a peer replayed
		// from a cache after the peer was lost, changes nothing.
		return false, nil
	}
	b, err := root.ReadFile(name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	was := strings.TrimSpace(string(b))
	var state string
	switch {
	case writing && was != writesRunning:
		state = writesRunning
	case !writing && was == writesRunning:
		state = writesStopped
	default:
		return false, nil
	}
	if err := root.WriteFile(name, []byte(state+"\n"), 0o644); err != nil {
		return false, err
	}
	if err := root.Chtimes(name, at, at); err != nil {
		return false, err
	}
	return true, nil
}

// IsWriting says whether the instance may write its data: an app, container
// or share resource of it runs, or says nothing of whether it does. A standby
// resource, running on every node, is not counted, nor a disabled one. What
// writes behind the back of opensvc is not counted either.
func IsWriting(data instance.Status) bool {
	for rid, rstat := range data.Resources {
		if rstat.IsStandby || rstat.IsDisabled {
			continue
		}
		id, err := resourceid.Parse(rid)
		if err != nil {
			continue
		}
		switch id.DriverGroup() {
		case driver.GroupApp, driver.GroupContainer, driver.GroupShare:
		default:
			continue
		}
		switch rstat.Status {
		case status.Down, status.StandbyDown:
		default:
			return true
		}
	}
	return false
}

// recordWrites records whether the instance writes its data, when that
// changed: the copies of the data on this node judge how far behind they are
// by it.
//
// The other nodes record it from the status this instance publishes, which
// reaches them by heartbeat: an action does not wait on a peer, which a node
// losing its network, as one failing over, would make it wait on.
func (t *actor) recordWrites(ctx context.Context, data instance.Status) {
	if t.IsVolatile() {
		// A volatile object, as one evaluated for a check, writes
		// nothing on disk.
		return
	}
	localhost := hostname.Hostname()
	writing := IsWriting(data)
	changed, err := recordWritesIn(t.VarDir(), localhost, writing, time.Now(), true)
	if err != nil {
		t.log.Warnf("record the writes: %s", err)
		return
	}
	if !changed {
		return
	}
	state := writesStopped
	if writing {
		state = writesRunning
	}
	t.log.Infof("the writes of the data are %s", state)
}
