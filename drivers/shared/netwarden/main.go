// Package netwarden restricts the network traffic of what a resource runs
// to what its net_rules keyword says.
//
// It holds the keyword, for the drivers of different groups taking it, and
// applies the filter its rules resolve to on the systemd slice of the
// process group of the resource: the IPAddressAllow and IPAddressDeny
// properties, which systemd enforces on every unit of the slice, the service
// an application is started in or the scope of a container, the one its
// engine makes or the one om starts a lxc container in. systemd knows no
// order: it is given the two lists letting through what the ordered rules
// do, see netrule.Filter.Flat.
//
// A guest sends its packets through network interfaces no cgroup carries
// the traffic of: its filter is applied on them, by nftables, see Netdev.
//
// systemd attaches the filter to the units, never to the slice: a process
// written into the group, in no unit, is not filtered. And where it can not
// attach it, it runs the unit unfiltered. So a driver setting a filter runs
// what it starts in a unit of the slice, and verifies the filter is
// attached, to refuse a start that would run unfiltered.
package netwarden

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/netrule"
	"github.com/opensvc/om3/v3/util/cgroupbpf"
	"github.com/opensvc/om3/v3/util/pg"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// T is held by the drivers taking the keyword, in a field named
	// Net: the keyword sets its attribute by that name.
	T struct {
		// Rules are the rules saying what the resource may exchange
		// packets with, and what it may not, one per line, the first
		// one matching deciding.
		Rules string `json:"net_rules"`
	}

	// Target is the resource a filter is applied to.
	Target struct {
		// Path is the object of the resource: its namespace is the
		// one ./ and . stand for in the rules.
		Path naming.Path

		// PG is the process group of the resource, whose slice takes
		// the filter, with the cgroup backend.
		PG *pg.Config

		// Backend is what applies the filter: the cgroup one, on the
		// slice of PG, when not named, and the netdev one, on Devs,
		// the network interfaces of a guest.
		Backend netrule.Backend
		Devs    []string

		// VarDir returns the directory of the resource the filter
		// applied is recorded in, for its status to compare with. It is
		// asked for when a record is read or written, not before.
		VarDir func() string

		// Log is the logger of the resource.
		Log *plog.Logger

		// DNS returns the nameservers what the resource runs resolves
		// through, which the dns rule and the nameservers allowed
		// without a rule name. Not set, they are the ones of the
		// resolver of the node, as for a command run on it. It is asked
		// for when the rules are resolved, not before. An error says
		// they could not be told, as for a network the configuration
		// no longer has, the ones found being returned all the same.
		DNS func() ([]netip.Addr, error)
	}

	// Driver is implemented by the drivers taking the keyword, for the
	// daemon to apply rules changed under the resource they run: the
	// daemon reads the configuration of the object as an action does,
	// and asks each resource what it is given.
	Driver interface {
		// NetRules returns the rules of the resource as its
		// configuration has them now, and what its filter is applied
		// to.
		NetRules() (T, Target)

		// NetLiveIssue returns what keeps a filter from being set
		// under what the resource already runs, having started with no
		// rule, nil when nothing does: what would have refused its
		// start with them.
		NetLiveIssue(ctx context.Context) error
	}

	// backend is what the filter asks of what applies it.
	backend interface {
		// Check says why no filter can be applied, nil when one can.
		Check() error

		// Set applies the filter, before what the resource runs is
		// started.
		Set(filter netrule.Filter) error

		// Current returns the filter in force, in the form the
		// backend holds one, see Form.
		Current() (netrule.Filter, error)

		// Form returns the filter given in the form the backend
		// holds it, to compare with the one in force: the steps as
		// they are for one judging a packet step after step, and
		// the allowed and denied lists saying the same for one
		// knowing no order.
		Form(filter netrule.Filter) netrule.Filter

		// HasFilter says a filter is in force.
		HasFilter() bool

		// HasMembers says something may run filtered: a unit of the
		// slice, an interface of the guest.
		HasMembers() bool

		// HasProcs says something runs where no filter reaches it.
		HasProcs() bool

		// Name names what the filter is set on, and Subject says it
		// as the subject of a sentence.
		Name() string
		Subject() string

		// Busy says why the filter is kept when the resource stops
		// with something still there to filter.
		Busy() string
	}

	// sliceBackend is the cgroup backend: the slice of a process group.
	sliceBackend struct {
		slice
		id string
	}

	// netdevBackend is the netdev backend: the interfaces of a guest.
	netdevBackend struct {
		Netdev
	}

	// slice is what the filter asks of the systemd slice of a process
	// group, which a pg.Config answers.
	slice interface {
		CheckIPFilter() error
		SetIPFilter(allow, deny []string) error
		IPFilter() (allow, deny []string, err error)
		HasIPFilter() bool
		HasMembers() bool
		HasProcs() bool
		Dir() string
	}
)

const (
	// Option is the option of the keyword.
	Option = netrule.Keyword

	// prefix starts what the logs and the status say of the filter.
	prefix = "netwarden: "

	// appliedFile is the name of the record of the filter applied.
	appliedFile = "netwarden.json"

	// NoUnitWarning is what the status of a resource given rules while
	// it runs says, and the daemon logs, when what it runs is in no unit
	// of the slice of its process group: an app or a task started with no
	// rule, and no systemd_properties, or a lxc container started by an
	// agent making no scope of it, which systemd filters nothing of.
	NoUnitWarning = "the rules apply at the next start: what the resource runs is in no unit of the slice"
)

var (
	//go:embed text
	fs embed.FS

	// sliceOf returns the slice of a process group, and is replaced by
	// the tests, which set no property of the systemd of the host.
	sliceOf = func(p *pg.Config) slice { return p }

	// querier asks the kernel the programs of a cgroup, and is replaced
	// by the tests.
	querier cgroupbpf.Querier = cgroupbpf.Kernel{}

	// cgroupDir returns the cgroup directory of a process, and is
	// replaced by the tests.
	cgroupDir = cgroupbpf.Dir

	// localView returns the view the rules resolve over, and is replaced
	// by the tests.
	localView = netrule.LocalView

	// lookupObjects asks the daemon the addresses of the objects the
	// rules name, and which objects they are, and is replaced by the
	// tests.
	lookupObjects = netrule.LookupObjects
)

// appliedFile is where the filter applied to the resource is recorded.
func (t Target) appliedFile() string {
	if t.VarDir == nil {
		return ""
	}
	return filepath.Join(t.VarDir(), appliedFile)
}

// backend returns what applies the filter of the resource, nil for one with
// no process group to set it on.
func (t Target) backend() backend {
	if t.Backend == netrule.BackendNetdev {
		return netdevBackend{Netdev{Devs: t.Devs}}
	}
	if t.PG == nil {
		return nil
	}
	return sliceBackend{slice: sliceOf(t.PG), id: t.PG.ID}
}

func (t sliceBackend) Check() error {
	if err := t.CheckIPFilter(); err != nil {
		return err
	}
	if !hasCapability() {
		return fmt.Errorf("the node lacks the %s capability, a unified cgroup hierarchy whose kernel attaches network filters, as of the last scan", NodeCapability)
	}
	return nil
}

func (t sliceBackend) Set(filter netrule.Filter) error {
	if filter.HasPorts() {
		return fmt.Errorf("the filter of a slice knows no port")
	}
	allow, deny := filter.Flat()
	return t.SetIPFilter(netrule.Strings(allow), netrule.Strings(deny))
}

// Current reads the filter the slice holds: its two lists.
func (t sliceBackend) Current() (netrule.Filter, error) {
	allow, deny, err := t.IPFilter()
	if err != nil {
		return netrule.Filter{}, err
	}
	allowed, err := netrule.ParsePrefixes(allow)
	if err != nil {
		return netrule.Filter{}, err
	}
	denied, err := netrule.ParsePrefixes(deny)
	if err != nil {
		return netrule.Filter{}, err
	}
	return netrule.FlatFilter(allowed, denied), nil
}

// Form returns the two lists systemd is given for the filter.
func (t sliceBackend) Form(filter netrule.Filter) netrule.Filter { return filter.Flattened() }

func (t sliceBackend) HasFilter() bool { return t.HasIPFilter() }
func (t sliceBackend) Name() string    { return filepath.Base(t.id) }
func (t sliceBackend) Subject() string { return "the slice " + t.Name() }
func (t sliceBackend) Busy() string    { return "the slice still holds processes" }

func (t netdevBackend) Set(filter netrule.Filter) error           { return t.Load(filter) }
func (t netdevBackend) Form(filter netrule.Filter) netrule.Filter { return filter }
func (t netdevBackend) HasProcs() bool                            { return false }
func (t netdevBackend) Name() string                              { return strings.Join(t.Devs, " ") }
func (t netdevBackend) Subject() string                           { return "the nft filter of " + t.Name() }
func (t netdevBackend) Busy() string                              { return "its interfaces are still there" }

// Keywords returns the definition of the keyword, in the keyword group
// given, for a resource whose filter the cgroup backend applies: each driver
// lists it with the keywords it sits beside.
func Keywords(group string) []*keywords.Keyword {
	return KeywordsOf(group, netrule.BackendCgroup)
}

// KeywordsOf returns the definition of the keyword for a resource whose
// filter the backend given applies, which says the rules it takes.
func KeywordsOf(group string, backend netrule.Backend) []*keywords.Keyword {
	return []*keywords.Keyword{
		{
			Attr:     "Net.Rules",
			Option:   Option,
			Group:    group,
			Scopable: true,
			Example:  "allow namespace . allow dns deny cluster deny network lan allow any",
			Validate: netrule.Validator(Option, backend),
			Text:     keywords.NewText(fs, "text/kw/net_rules"),
		},
	}
}

// Enabled says the resource has rules: what it runs is to run in a unit of
// the slice of its process group, filtered.
func (t T) Enabled() bool {
	return strings.TrimSpace(t.Rules) != ""
}

// Parse returns the rules of the keyword, and the error of a rule the
// grammar refuses or the backend given makes no filter of.
func (t T) Parse(backend netrule.Backend) (netrule.Rules, error) {
	return netrule.ParseKeyword(t.Rules, backend)
}

// Check says whether the filter of the resource can be applied on its
// process group, before anything is started: nil for a resource with no
// rule, else the reason a start is refused.
//
// A start refused is one that would run unfiltered: on a node without
// systemd or whose kernel attaches no filter to a cgroup, for a group that
// is no slice, or one delegated to the systemd instance of a user, as the
// one of a rootless container. For a guest, on a node without nft, or whose
// kernel filters nothing of what an interface sends to it.
func (t T) Check(target Target) error {
	if !t.Enabled() {
		return nil
	}
	if _, err := t.Parse(target.Backend); err != nil {
		return fmt.Errorf("%s%w", prefix, err)
	}
	b := target.backend()
	if b == nil {
		return fmt.Errorf("%s%s needs a process group: %w", prefix, Option, pg.ErrNoSlice)
	}
	if err := b.Check(); err != nil {
		return fmt.Errorf("%s%s can not be applied: %w", prefix, Option, err)
	}
	return nil
}

// Record returns the record of the rules of the resource, resolved to
// nothing yet: what a start resolves over what the daemon tells it, and the
// daemon over what it hears, for a resource given rules while it runs.
func (t T) Record(target Target) netrule.Record {
	rec := netrule.Record{
		NetRules:  netrule.TextsOf(t.Rules),
		Namespace: target.Path.Namespace,
		Backend:   target.Backend,
	}
	if target.Backend == netrule.BackendNetdev {
		rec.Devs = append([]string{}, target.Devs...)
	} else if target.PG != nil {
		rec.PG = target.PG.ID
	}
	if target.DNS != nil {
		// Never nil: no nameserver of its own is not the ones of
		// the node.
		addrs, err := target.DNS()
		if err != nil && target.Log != nil {
			target.Log.Warnf("netwarden: the nameservers of the resource: %s", err)
		}
		rec.DNS = append([]netip.Addr{}, addrs...)
	}
	return rec
}

// Apply sets the filter the rules resolve to on the slice of the process
// group of the resource, before what it runs is started there, and says
// whether a filter is in force, to verify once it is.
//
// A resource with no rule has the filter an earlier configuration left on
// its slice lifted. Rules letting everything through for now, as an allow
// any after the deny of a network not created yet, set no filter on a
// slice, and what the resource runs is in a unit all the same, for a filter
// set later to reach it.
func (t T) Apply(target Target) (bool, error) {
	if !t.Enabled() {
		return false, t.Clear(target)
	}
	if err := t.Check(target); err != nil {
		return false, err
	}
	view, told, err := t.view(target)
	if err != nil {
		return false, fmt.Errorf("%s%w", prefix, err)
	}
	rec, notes, err := t.Record(target).Refresh(view)
	if err != nil {
		return false, fmt.Errorf("%s%w", prefix, err)
	}
	for _, note := range notes {
		if !told && (note.Rule.Kind == netrule.KindObject || note.Rule.Kind == netrule.KindNamespace) {
			// Said once for all of them: nobody told the objects,
			// which is not their not existing.
			continue
		}
		target.Log.Warnf("%s%s", prefix, note)
	}
	filter := rec.Filter
	b := target.backend()
	if b.Form(filter).IsZero() {
		target.Log.Infof("%sthe rules deny no address for now: nothing is filtered", prefix)
		if err := t.lift(target); err != nil {
			return false, err
		}
		return false, t.record(target, rec)
	}
	if err := b.Set(filter); err != nil {
		return false, fmt.Errorf("%s%w", prefix, err)
	}
	// systemd adds to the lists it holds when asked the wrong way: what
	// is held now is read back rather than trusted.
	current, err := b.Current()
	if err != nil {
		return false, fmt.Errorf("%s%w", prefix, err)
	}
	if form := b.Form(filter); !current.Equal(form) {
		if rec.Foreign.Any() {
			return false, fmt.Errorf("%s%s does not hold the filter it was set: %s", prefix, b.Subject(), shown(rec))
		}
		return false, fmt.Errorf("%s%s holds %s after it was set %s", prefix, b.Subject(), current.Describe(), form.Describe())
	}
	target.Log.Infof("%s%s on %s", prefix, shown(rec), b.Name())
	return true, t.record(target, rec)
}

// view returns the view the rules of the resource resolve over: the
// networks and the nodes as the configuration of the node has them, and the
// addresses of the objects the rules name as the daemon knows them, with
// which objects they are. It says whether the daemon told them.
//
// With the daemon down, the resource starts all the same: its rules naming
// objects and namespaces resolve to nothing, the allow ones letting nothing
// through and the deny ones dropping nothing, until the daemon is back and
// resolves them.
func (t T) view(target Target) (netrule.View, bool, error) {
	view, err := localView()
	if err != nil {
		return nil, false, err
	}
	rules, err := t.Parse(target.Backend)
	if err != nil {
		return nil, false, err
	}
	objects, namespaces := netrule.References(target.Path.Namespace, rules)
	if len(objects) == 0 && len(namespaces) == 0 {
		return netrule.WithObjects(view, nil), true, nil
	}
	data, identities, err := lookupObjects(objects, namespaces)
	if err != nil {
		target.Log.Warnf("%sthe daemon does not tell the addresses of the objects: %s: the rules naming objects and namespaces resolve to nothing until it does", prefix, err)
		return netrule.WithObjects(view, nil), false, nil
	}
	// The addresses are recorded with the objects publishing them, each
	// its own, as the daemon knows them: a holder of its path alone would
	// be lifted by the record of a delete of an object the path named
	// before, and one given the newest object of the path kept allowed
	// past the delete of the object publishing it, until the daemon
	// resolves the rules again.
	return netrule.WithAddrIdentities(netrule.WithObjects(view, data), identities), true, nil
}

// shown says the filter of a record as the status and the logs of the
// resource show it: the addresses of the objects of another namespace
// counted, not listed, the reader of the resource being no reader of theirs,
// and so the ones read from a key of a sec, which the reader of a resource
// may not decode.
func shown(rec netrule.Record) string {
	if !rec.Foreign.Any() {
		return rec.Filter.Describe()
	}
	return rec.Public.Describe() + rec.Foreign.Describe()
}

// Verify says the filter of the resource is attached to the cgroup of the
// process pid, which the resource started: it runs in a unit of the slice
// of the process group, and the kernel filters its packets.
//
// A container the engine placed with its cgroupfs driver is in no unit:
// its cgroup is not under the slice, or under it with no filter attached.
func Verify(target Target, pid int) error {
	dir, err := cgroupDir(pid)
	if err != nil {
		return fmt.Errorf("%sthe cgroup of the process %d: %w", prefix, pid, err)
	}
	base := sliceOf(target.PG).Dir()
	if !strings.HasPrefix(dir, base+string(filepath.Separator)) {
		return fmt.Errorf("%sthe process %d runs in the cgroup %s, in no unit of the slice %s the filter is set on: %w", prefix, pid, dir, filepath.Base(target.PG.ID), cgroupbpf.ErrNotAttached)
	}
	if err := cgroupbpf.Verify(querier, dir, base); err != nil {
		return fmt.Errorf("%s%w", prefix, err)
	}
	return nil
}

// Clear lifts the filter of the slice of the process group of the resource,
// once what it ran is stopped, and forgets the filter applied.
//
// The filter is kept while the slice holds a unit, as the one of the
// processes an application left running at its stop: lifting it would
// unfilter them.
func (t T) Clear(target Target) error {
	b := target.backend()
	if b == nil {
		return nil
	}
	if b.HasFilter() && b.HasMembers() {
		target.Log.Infof("%sthe filter of %s is kept: %s", prefix, b.Name(), b.Busy())
		return nil
	}
	if err := t.lift(target); err != nil {
		return err
	}
	if target.VarDir == nil {
		return nil
	}
	if err := os.Remove(target.appliedFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%s%w", prefix, err)
	}
	return nil
}

// lift empties the two lists of a slice a filter was set on, or removes the
// tables of the interfaces of a guest. The properties of a slice are
// forgotten with the group, when the object stops.
func (t T) lift(target Target) error {
	b := target.backend()
	if !b.HasFilter() {
		return nil
	}
	if err := b.Set(netrule.Filter{}); err != nil {
		return fmt.Errorf("%s%w", prefix, err)
	}
	target.Log.Infof("%slifted the filter of %s", prefix, b.Name())
	return nil
}

// record writes the filter applied, with what it was resolved from.
func (t T) record(target Target, rec netrule.Record) error {
	if target.VarDir == nil {
		return nil
	}
	return rec.Save(target.appliedFile())
}

// Status returns what the status of a running resource says of its filter:
// the filter in force, and a warning when it is not the one recorded, or
// when the rules of the configuration are not the ones it was resolved
// from.
//
// The daemon applies the rules changed under a running resource, so the
// status warns of what the daemon does not: rules it refuses, whose filter
// it leaves as it was, rules given to a command that runs in no unit of the
// slice, where systemd filters nothing, and rules it has not applied yet, as
// when it is down.
//
// The filter in force is compared with the one recorded rather than
// resolved again: a status is evaluated often, and resolving reads the
// configuration of the node and the names of its peers. The record is the
// one of the start, which the daemon writes again each time the rules, or
// what they name, change, with the filter it sets then.
//
// The nameservers the dns rule names are the ones the resource was told at
// its start, which the daemon resolves over ever since: a running guest or
// container keeps the resolver it was given, whatever its networks name
// now. The status warns when they are no longer the ones a start would
// tell, the filter then allowing nameservers the configuration no longer
// names, and not the ones it names, until the resource restarts.
//
// The addresses the rules naming another namespace resolve to are counted,
// not shown: who reads the status of the resource is no reader of the
// objects of that namespace.
func (t T) Status(target Target) (infos, warnings []string) {
	b := target.backend()
	if !t.Enabled() || b == nil || target.VarDir == nil {
		return nil, nil
	}
	warn := func(format string, a ...any) {
		warnings = append(warnings, prefix+fmt.Sprintf(format, a...))
	}
	rec, err := netrule.LoadRecord(target.appliedFile())
	switch {
	case errors.Is(err, os.ErrNotExist):
		// Rules set after the start. The first two say why the daemon
		// leaves the resource unfiltered, the last that it has not
		// been there yet.
		if _, err := t.Parse(target.Backend); err != nil {
			warn("the rules are refused, and the resource runs unfiltered: %s", err)
		} else if !b.HasMembers() && b.HasProcs() {
			warn("%s", NoUnitWarning)
		} else {
			warn("%s was set after the resource started, and the daemon has not applied it yet: it runs unfiltered until then", Option)
		}
		return
	case err != nil:
		warn("%s", err)
		return
	}
	// The rules are compared, not the text of the keyword: the same
	// rules laid out another way are the ones the filter was resolved
	// from.
	if !rec.HasRules(t.Rules) {
		if _, err := t.Parse(target.Backend); err != nil {
			warn("the rules changed since the filter was set are refused, and the filter is kept: %s", err)
		} else {
			warn("%s changed, and the daemon has not applied it yet: the filter in force is the one of the rules before", Option)
		}
	}
	// A resource resolving through the nameservers of the node records
	// none, and is not told any: nothing to compare. Nameservers that
	// could not be told are no change of them: the error is the one to
	// fix, and the start that meets it says it.
	if rec.DNS != nil && target.DNS != nil {
		if now, err := target.DNS(); err == nil && !sameAddrs(rec.DNS, now) {
			warn("the nameservers of the resource changed since it started, %s where it was told %s: the filter allows the ones it was told until it restarts, and a rule \"allow address <nameserver>\" allows a new one meanwhile, the daemon applying it with no restart", describeAddrs(now), describeAddrs(rec.DNS))
		}
	}
	current, err := b.Current()
	if err != nil {
		warn("%s", err)
		return
	}
	if form := b.Form(rec.Filter); !current.Equal(form) {
		if !rec.Foreign.Any() {
			warn("%s holds %s, not the %s applied", b.Subject(), current.Describe(), form.Describe())
		} else {
			warn("%s does not hold the filter applied: %s", b.Subject(), shown(rec))
		}
		return
	}
	infos = append(infos, prefix+shown(rec))
	if n := rec.Kept.Count(netrule.ActionAllow); n > 0 {
		infos = append(infos, fmt.Sprintf("%s%d allowed address(es) kept, of objects whose status is not known for now", prefix, n))
	}
	if n := rec.Kept.Count(netrule.ActionDeny); n > 0 {
		infos = append(infos, fmt.Sprintf("%s%d denied address(es) kept, of objects no longer publishing them", prefix, n))
	}
	return
}

// sameAddrs says the two lists hold the same addresses, in any order, an
// address listed twice counting once.
func sameAddrs(a, b []netip.Addr) bool {
	has := func(l []netip.Addr, addr netip.Addr) bool {
		return slices.ContainsFunc(l, func(o netip.Addr) bool { return o.Unmap().WithZone("") == addr.Unmap().WithZone("") })
	}
	for _, addr := range a {
		if !has(b, addr) {
			return false
		}
	}
	for _, addr := range b {
		if !has(a, addr) {
			return false
		}
	}
	return true
}

// describeAddrs says a list of addresses, and none when it is empty.
func describeAddrs(l []netip.Addr) string {
	if len(l) == 0 {
		return "none"
	}
	words := make([]string, len(l))
	for i, addr := range l {
		words[i] = addr.String()
	}
	return strings.Join(words, " ")
}
