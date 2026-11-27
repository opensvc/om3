package netwarden

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
)

// onInstanceChanged follows a change of the configuration or of the status
// of an instance, on any node, st being its status when the change is one
// of it.
//
// The configuration of a cfg or of a sec changes when one of its keys does:
// the resources whose rules read addresses from it have them resolved again,
// from the configuration this node holds of it, which the event of this node
// says is written. No event of a key is waited for, and no time of another
// node is compared: the datastore is read as it is here. The datastore may
// be one of another namespace sharing its keys with the one of the object:
// its share keyword is of the same configuration, read with the key, so a
// share taken back is followed as a key removed is.
//
// An instance of this node may have started or stopped a resource with a
// filter: the records of its object are read again, and its filters
// resolved again, a start having resolved its own before the daemon could
// tell it what changed since. And the object may be one rules name, the one
// an address kept was of, or one now publishing it: the resources concerned
// have their rules resolved again, and no other, an instance changing
// somewhere in the cluster at every moment.
//
// kind is the kind of the event, for the logs of an audit to tell what was
// weighed: one line by event, saying the resources it has resolved again
// and why, or that it is none of the filters of the node.
func (t *Manager) onInstanceChanged(kind string, p naming.Path, nodename string, st *instance.Status, changed bool) {
	if nodename == t.localhost && t.scan(p) {
		changed = true
	}
	if !changed {
		// Nothing a filter resolves to moved: said nothing of, in the
		// logs of an audit either, as the statuses are published at
		// every evaluation.
		return
	}
	t.resolveAgain(kind, p, nodename, st)
}

// resolveAgain resolves again the rules of the resources a change of an
// instance concerns, see onInstanceChanged, the records of its object read.
func (t *Manager) resolveAgain(kind string, p naming.Path, nodename string, st *instance.Status) {
	local := nodename == t.localhost
	var published []netip.Addr
	if st != nil {
		published = objectAddrs(map[string]*instance.Status{nodename: st}).All("")
	}
	concerned := make([]*entry, 0)
	reasons := make([]string, 0)
	for _, e := range t.sorted() {
		var reason string
		switch {
		case e.datastores[p]:
			if st != nil {
				// A status of a datastore changes no key.
				continue
			}
			reason = "reads a key of it"
		case e.names(p):
			reason = "names it"
		case e.keepsOf(p, published):
			reason = "keeps an address of it, or one it publishes"
		case local && e.path == p:
			reason = "a resource of it"
		default:
			continue
		}
		concerned = append(concerned, e)
		reasons = append(reasons, fmt.Sprintf("%s %s (%s)", e.path, e.rid, reason))
	}
	if len(concerned) == 0 {
		// An object no filter names, nor keeps an address of: none of
		// the filters' business, said nothing of.
		return
	}
	t.log.Debugf("%s %s@%s: resolve again %s", kind, p, nodename, strings.Join(reasons, ", "))
	for _, e := range concerned {
		t.refresh(e)
	}
}

// onNodeChanged follows a node the daemon starts or stops hearing of, and
// a node whose daemon leaves or joins the cluster, away saying which: what
// was not known of its instances is now, or the reverse, which decides the
// addresses the filters keep.
//
// The state is the one of the event, not the one the data of the daemon
// holds when the event is handled, which is ahead of it: the events after
// this one are judged by the state their node was in when they were
// published.
//
// A node in the state it was in is said nothing of, in the logs of an audit
// either: the nodes tell their state at every moment.
func (t *Manager) onNodeChanged(kind, nodename string, away bool) {
	if was, ok := t.away[nodename]; ok && was == away {
		return
	}
	t.away[nodename] = away
	keeping := make([]*entry, 0)
	for _, e := range t.sorted() {
		if e.keeps() {
			keeping = append(keeping, e)
		}
	}
	state := "tells its instances again"
	if away {
		state = "does not tell its instances any more, lost or its daemon away"
	}
	t.log.Debugf("%s %s: the node %s: resolve again the %d filter(s) keeping addresses, of %d", kind, nodename, state, len(keeping), len(t.entries))
	for _, e := range keeping {
		t.refresh(e)
	}
}

// isAway says a node did not tell its instances as of the last event of it,
// and as the data of the daemon has it for a node no event told of.
func (t *Manager) isAway(nodename string) bool {
	if away, ok := t.away[nodename]; ok {
		return away
	}
	return isNodeUnsure(nodename)
}

// onClusterChanged follows a change of the nodes and the networks of the
// cluster, which the rules naming them resolve to: the configuration of the
// node is read again, and every filter resolved again.
func (t *Manager) onClusterChanged(kind string) {
	t.base = nil
	t.resolveAll(kind)
}

// resolveAll resolves every filter again, for a change of the cluster of
// the kind given, over the configuration of the node as last read, or read
// now when a change of the cluster is not read yet.
func (t *Manager) resolveAll(kind string) {
	t.log.Debugf("%s: the nodes or the networks of the cluster may have changed: read the configuration of the node again, and resolve again the %d filter(s) of the node", kind, len(t.entries))
	t.refreshAll()
}

func (t *Manager) refreshAll() {
	for _, e := range t.sorted() {
		t.refresh(e)
	}
}

// sorted returns the entries in the order of their files, so the filters
// are set, and logged, in the same order every time.
func (t *Manager) sorted() []*entry {
	files := make([]string, 0, len(t.entries))
	for file := range t.entries {
		files = append(files, file)
	}
	slices.Sort(files)
	l := make([]*entry, 0, len(files))
	for _, file := range files {
		l = append(l, t.entries[file])
	}
	return l
}

// names says a rule of the resource names the object, or its namespace.
func (e *entry) names(p naming.Path) bool {
	return e.objects[p] || e.namespaces[p.Namespace]
}

// keeps says the filter of the resource keeps addresses its rules no longer
// resolve to: a node heard of again, or lost, may be what says what became
// of them.
func (e *entry) keeps() bool {
	return !e.rec.Kept.IsZero()
}

// keepsOf says the filter of the resource keeps an address of the object,
// or one of the addresses given, which another object publishes now: a
// change of that instance may be what says what became of it.
func (e *entry) keepsOf(p naming.Path, published []netip.Addr) bool {
	for _, kept := range e.rec.Kept {
		for addr, owner := range kept {
			if owner.Path == p.String() || slices.Contains(published, addr) {
				return true
			}
		}
	}
	return false
}

// scan reads the records of the resources of an object of the node, and
// indexes them: a record written by a start is added, and a record a stop
// removed is dropped. It says whether an entry was added, dropped or
// written anew.
func (t *Manager) scan(p naming.Path) bool {
	switch p.Kind {
	case naming.KindCfg, naming.KindSec, naming.KindUsr, naming.KindCcfg, naming.KindNscfg:
		// No resource, so no record: a node holds as many of them
		// as of the others, and each tells its configuration and its
		// status as the daemon starts.
		return false
	}
	changed := false
	files := recordFiles(p)
	for file, e := range t.entries {
		if e.path == p && !slices.Contains(files, file) {
			t.log.Debugf("%s %s: record gone, removed by a stop: dropped from the filters followed, %d left", p, e.rid, len(t.entries)-1)
			delete(t.entries, file)
			changed = true
		}
	}
	for _, file := range files {
		rec, err := netrule.LoadRecord(file)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				t.log.Warnf("%s: %s", p, err)
			}
			if _, ok := t.entries[file]; ok {
				delete(t.entries, file)
				changed = true
			}
			continue
		}
		e, ok := t.entries[file]
		if ok && sameStart(e.rec, rec) {
			continue
		}
		next := t.index(file, p, rec)
		t.entries[file] = next
		changed = true
		found := "record found, written by a start: followed"
		if ok {
			found = "record written again by a start: followed anew"
		}
		t.log.Debugf("%s %s: %s, %s on %s, naming %d object(s) and %d namespace(s), %d filter(s) followed", p, next.rid, found, rec.Filter.Describe(), where(rec), len(next.objects), len(next.namespaces), len(t.entries))
	}
	return changed
}

// sameStart says the two records are of the same start of a resource: the
// daemon rewrites the record it indexed, and a start writes a new one.
func sameStart(a, b netrule.Record) bool {
	return a.NetRules.Equal(b.NetRules) && a.PG == b.PG && a.Namespace == b.Namespace &&
		a.Backend == b.Backend && slices.Equal(a.Devs, b.Devs) &&
		slices.Equal(a.DNS, b.DNS) && a.Filter.Equal(b.Filter) && a.Held.Equal(b.Held) && a.Kept.Equal(b.Kept)
}

// index returns the entry of a record: its resource, and what its rules
// name.
func (t *Manager) index(file string, p naming.Path, rec netrule.Record) *entry {
	e := &entry{
		file:       file,
		path:       p,
		rid:        filepath.Base(filepath.Dir(file)),
		rec:        rec,
		objects:    make(map[naming.Path]bool),
		namespaces: make(map[string]bool),
		datastores: make(map[naming.Path]bool),
	}
	rules, err := rec.Rules()
	if err != nil {
		// Rules a newer om wrote, or a file damaged: nothing to follow.
		t.log.Warnf("%s %s: %s", p, e.rid, err)
		return e
	}
	objects, namespaces := netrule.References(rec.Namespace, rules)
	for _, o := range objects {
		e.objects[o] = true
	}
	for _, name := range namespaces {
		e.namespaces[name] = true
	}
	for _, ds := range netrule.Datastores(rec.Namespace, rules) {
		e.datastores[ds] = true
	}
	return e
}

// view returns the view the rules are resolved over, the configuration of
// the node read once until the cluster changes.
func (t *Manager) view() (netrule.View, error) {
	if t.base == nil {
		base, err := localView()
		if err != nil {
			return nil, err
		}
		t.base = base
	}
	waited := func(p naming.Path, nodename string) {
		t.log.Debugf("%s@%s: named by a rule, and its node does not tell its instances: waited for, the addresses it held kept", p, nodename)
	}
	return view{View: t.base, heard: t.heard, ghosts: t.ghosts, away: t.isAway, tombstones: t.tombstones, waited: waited}, nil
}

// refresh resolves the rules of a resource again, and sets the filter on its
// slice when it is not the one recorded.
//
// The record is written with the filter set, so the status of the resource
// compares the slice with it, and with the addresses held, the memory of the
// next time. A filter that could not be set leaves the record as it was: the
// status of the resource says the slice holds another, and the next change
// sets it again.
//
// The rules of the entry may be ones the configuration was found with, see
// replace and adopt: the record is then written with them even when they
// resolve to the filter in force, and the return value says it was.
func (t *Manager) refresh(e *entry) bool {
	v, err := t.view()
	if err != nil {
		t.log.Warnf("%s %s: %s", e.path, e.rid, err)
		return false
	}
	next, notes, err := e.rec.Refresh(v)
	if err != nil {
		t.log.Debugf("%s %s: not resolved, the filter of %s is left as it is: %s", e.path, e.rid, where(e.rec), err)
		return false
	}
	// Judged in the form the backend holds: the same lists of a slice
	// are not set again for steps that changed.
	changed := !inForce(next).Equal(inForce(e.rec))
	if changed || e.reapply || e.rules {
		// A datastore of another namespace that does not share its
		// keys, or no longer does: its addresses leave the filter,
		// the allowed and the denied ones, which nothing else says
		// to who runs the resource. Said when the filter moves or
		// is set again, not at every event resolving it.
		for _, note := range notes {
			if note.Refused {
				t.log.Warnf("%s %s: %s", e.path, e.rid, note)
			}
		}
	}
	if !changed && !e.reapply {
		switch {
		case e.rules:
			was := e.rec.Filter
			if !t.save(e, next) {
				return false
			}
			t.log.Infof("%s %s: rules changed: %s on %s, as it was", e.path, e.rid, was.Describe(), where(next))
			return true
		case !next.Held.Equal(e.rec.Held) || !next.Kept.Equal(e.rec.Kept):
			t.log.Debugf("%s %s: %s on %s, unchanged: %s, was %s", e.path, e.rid, next.Filter.Describe(), where(next), memory(next), memory(e.rec))
			return t.save(e, next)
		case !next.Filter.Equal(e.rec.Filter):
			// The steps moved, and not what the backend holds of
			// them: the record is what the status shows.
			t.log.Debugf("%s %s: %s on %s, in force as it is, was %s", e.path, e.rid, next.Filter.Describe(), where(next), e.rec.Filter.Describe())
			return t.save(e, next)
		}
		t.log.Debugf("%s %s: %s on %s, unchanged", e.path, e.rid, next.Filter.Describe(), where(next))
		return false
	}
	s := backendOf(next)
	if !s.HasMembers() {
		// Nothing runs in the slice: the record is one a stop did not
		// remove, as after a reboot, and the next start writes its own.
		t.log.Debugf("%s %s: nothing runs in %s, its record left by a stop that did not remove it: %s is not set", e.path, e.rid, where(next), next.Filter.Describe())
		e.reapply = false
		return false
	}
	if filtersNothing(next) && !s.HasFilter() {
		t.log.Debugf("%s %s: the rules deny no address for now, and %s holds no filter: nothing to set", e.path, e.rid, where(next))
		e.reapply = false
		return t.save(e, next)
	}
	if _, err := os.Stat(e.file); err != nil && !e.unrecorded {
		// Stopped since: the stop lifted the filter.
		t.log.Debugf("%s %s: record gone, the resource stopped since: %s is not set on %s", e.path, e.rid, next.Filter.Describe(), where(next))
		delete(t.entries, e.file)
		return false
	}
	if err := s.Set(next.Filter); err != nil {
		t.log.Errorf("%s %s: set the filter of %s: %s", e.path, e.rid, where(next), err)
		return false
	}
	if !next.Held.Equal(e.rec.Held) || !next.Kept.Equal(e.rec.Kept) {
		t.log.Debugf("%s %s: %s, was %s", e.path, e.rid, memory(next), memory(e.rec))
	}
	switch {
	case e.rules && changed:
		t.log.Infof("%s %s: rules changed: %s on %s, was %s", e.path, e.rid, next.Filter.Describe(), where(next), e.rec.Filter.Describe())
	case e.rules:
		t.log.Infof("%s %s: rules changed: %s on %s, as it was, set again as the daemon starts", e.path, e.rid, next.Filter.Describe(), where(next))
	case changed:
		t.log.Infof("%s %s: %s on %s, was %s", e.path, e.rid, next.Filter.Describe(), where(next), e.rec.Filter.Describe())
	default:
		t.log.Infof("%s %s: %s on %s, set again as the daemon starts", e.path, e.rid, next.Filter.Describe(), where(next))
	}
	e.reapply = false
	return t.save(e, next)
}

// memory says, in a line, the addresses a record holds of the objects its
// rules name, and the ones it keeps that they no longer resolve to: counted,
// not listed, a filter holding the addresses of a namespace at times.
func memory(rec netrule.Record) string {
	return fmt.Sprintf("%d allowed and %d denied address(es) of objects held, %d and %d kept",
		rec.Held.Count(netrule.VerbAllow), rec.Held.Count(netrule.VerbDeny), rec.Kept.Count(netrule.VerbAllow), rec.Kept.Count(netrule.VerbDeny))
}

// save writes the record of a resource, unless its stop removed it since,
// and says it did. The status of the instance read the record, and is told
// outdated.
func (t *Manager) save(e *entry, rec netrule.Record) bool {
	if _, err := os.Stat(e.file); err != nil && !e.unrecorded {
		delete(t.entries, e.file)
		return false
	}
	if err := rec.Save(e.file); err != nil {
		t.log.Warnf("%s %s: %s", e.path, e.rid, err)
		return false
	}
	e.rec = rec
	e.rules, e.unrecorded = false, false
	t.outdate(e)
	return true
}
