// Package statusboard renders the status of an object cluster-wide, for a
// reader to grasp its issues at a sight: a board of the resources by node,
// the instance states above it, and below it the notes saying what needs
// attention.
//
// The resources are rows, in the order the actions run them, grouped by
// subset, and the nodes are columns. A cell holds the status of the resource
// on the node, the flags departing from the usual, and the marker of the
// notes about it. What the label of a resource says the same on every node
// stays in the label column, and what differs, as a node-scoped address,
// goes on rows of its own under it, "@<node> <value>". The
// resource logs, long lines that differ per node, are notes the markers
// point to, so the board keeps its width.
package statusboard

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	"github.com/opensvc/om3/v3/core/colorstatus"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/placement"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceset"
	"github.com/opensvc/om3/v3/core/status"
)

type (
	// row is a line of the board: a label part, the cells of the nodes,
	// and a description.
	row struct {
		name  string
		typ   string
		cells []string
		desc  string

		// heading is a row naming a part of the board, in bold: the
		// instance row, and the resources one above the resource rows.
		heading bool

		// blank is an empty line between the parts of the board.
		blank bool
	}

	// note is an entry of the notes: a message about a resource on the
	// nodes it was logged on, or about the object.
	note struct {
		// id is the creation rank of the note, which the cells name it by
		// until the notes are numbered.
		id     int
		marker string
		level  resource.Level
		rid    string
		nodes  []string
		text   string
	}

	board struct {
		digest object.Digest
		nodes  []string
		states map[string]instance.States
		rows   []row
		notes  []*note

		// noteByKey finds the note of a resource message, to give the
		// nodes logging the same one the same marker.
		noteByKey map[string]*note
	}
)

const (
	// infoMax is the number of info notes listed, beyond which they are
	// counted: the warnings and errors stay on the screen.
	infoMax = 10
)

var (
	regexpANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

	superscripts = []rune("⁰¹²³⁴⁵⁶⁷⁸⁹")
)

// Render returns the board of the object status, its notes wrapped to width
// columns, or as wide as they need when width is 0.
func Render(digest object.Digest, width int) string {
	t := newBoard(digest)
	t.load()
	return t.render(width)
}

func newBoard(digest object.Digest) *board {
	t := &board{
		digest:    digest,
		states:    digest.Instances.ByNode(),
		noteByKey: make(map[string]*note),
	}
	// The nodes in the order of the object scope, which is the order of
	// its placement policy reads them in, then the nodes not in the scope
	// that report an instance anyway.
	for _, nodename := range digest.Object.Scope {
		if _, ok := t.states[nodename]; ok {
			t.nodes = append(t.nodes, nodename)
		}
	}
	others := make([]string, 0)
	for nodename := range t.states {
		if !slices.Contains(t.nodes, nodename) {
			others = append(others, nodename)
		}
	}
	sort.Strings(others)
	t.nodes = append(t.nodes, others...)
	return t
}

func (t *board) load() {
	t.loadInstanceRows()
	first := len(t.rows)
	t.loadResourceRows()
	if len(t.rows) > first {
		// The resources are listed under their heading, as the instance
		// states are under the instance row. An object with no resource
		// has no heading for them.
		for i := first; i < len(t.rows); i++ {
			t.rows[i].name = "  " + t.rows[i].name
		}
		heading := []row{{blank: true}, {name: "resources", heading: true}}
		t.rows = append(t.rows[:first], append(heading, t.rows[first:]...)...)
	}
	t.loadObjectNotes()
	t.numberNotes()
}

// cellsOf returns the cells of a row, one per node, computed by fn for the
// nodes having an instance, and "·" for the others.
func (t *board) cellsOf(fn func(instance.States) string) []string {
	cells := make([]string, len(t.nodes))
	for i, nodename := range t.nodes {
		cells[i] = fn(t.states[nodename])
	}
	return cells
}

func (t *board) anyNode(fn func(instance.States) bool) bool {
	for _, nodename := range t.nodes {
		if fn(t.states[nodename]) {
			return true
		}
	}
	return false
}

// nodesWith returns the nodes whose instance fn says yes about.
func (t *board) nodesWith(fn func(instance.States) bool) []string {
	l := make([]string, 0)
	for _, nodename := range t.nodes {
		if fn(t.states[nodename]) {
			l = append(l, nodename)
		}
	}
	return l
}

func (t *board) loadInstanceRows() {
	excess := t.digest.Object.ActorStatus.ExcessInstances() > 0
	t.rows = append(t.rows, row{name: "instance", heading: true, cells: t.cellsOf(func(s instance.States) string {
		if excess && s.Status.Avail == status.Up {
			// One of the instances up beyond what the topology allows.
			return rawconfig.Colorize.Error(s.Status.Avail.String())
		}
		return t.statusText(s.Status.Avail)
	})})
	t.rows = append(t.rows, row{name: "  monitor", cells: t.cellsOf(func(s instance.States) string {
		switch {
		case s.Monitor.UpdatedAt.IsZero():
			return rawconfig.Colorize.Warning("no-monitor")
		case s.Monitor.State == instance.MonitorStateIdle:
			return rawconfig.Colorize.Secondary(s.Monitor.State.String())
		case strings.HasSuffix(s.Monitor.State.String(), "failed"):
			return rawconfig.Colorize.Error(s.Monitor.State.String())
		default:
			return rawconfig.Colorize.Primary(s.Monitor.State.String())
		}
	})})
	if t.anyNode(hasExpect) {
		t.rows = append(t.rows, row{name: "  expect", cells: t.cellsOf(func(s instance.States) string {
			l := make([]string, 0)
			if isGlobalExpect(s.Monitor.GlobalExpect) {
				l = append(l, ">"+s.Monitor.GlobalExpect.String())
			}
			if isLocalExpect(s.Monitor.LocalExpect) {
				l = append(l, s.Monitor.LocalExpect.String())
			}
			if len(l) == 0 {
				return rawconfig.Colorize.Secondary("·")
			}
			return strings.Join(l, " ")
		})})
	}
	if t.anyNode(func(s instance.States) bool { return s.Status.IsFrozen() || !s.Node.FrozenAt.IsZero() }) {
		t.rows = append(t.rows, row{name: "  frozen", cells: t.cellsOf(func(s instance.States) string {
			l := make([]string, 0)
			if s.Status.IsFrozen() {
				l = append(l, "frozen")
			}
			if !s.Node.FrozenAt.IsZero() {
				l = append(l, "node-frozen")
			}
			if len(l) == 0 {
				return rawconfig.Colorize.Secondary("·")
			}
			return rawconfig.Colorize.Frozen(strings.Join(l, " "))
		})})
	}
	if t.anyNode(func(s instance.States) bool { return s.Status.IsStopped() }) {
		t.rows = append(t.rows, row{name: "  stopped", cells: t.cellsOf(func(s instance.States) string {
			if s.Status.IsStopped() {
				return rawconfig.Colorize.Frozen("stopped")
			}
			return rawconfig.Colorize.Secondary("·")
		})})
	}
	if t.anyNode(func(s instance.States) bool {
		return isActor(s) && (s.Status.Provisioned == provisioned.False || s.Status.Provisioned == provisioned.Mixed)
	}) {
		t.rows = append(t.rows, row{name: "  provisioned", cells: t.cellsOf(func(s instance.States) string {
			v := s.Status.Provisioned
			switch v {
			case provisioned.True:
				return rawconfig.Colorize.Secondary(v.String())
			case provisioned.NotApplicable:
				return rawconfig.Colorize.Secondary(v.String())
			default:
				return rawconfig.Colorize.Error(v.String())
			}
		})})
	}
	if t.anyNode(func(s instance.States) bool { return s.Monitor.OrchestrationID != uuid.Nil }) {
		t.rows = append(t.rows, row{name: "  orchestration", cells: t.cellsOf(func(s instance.States) string {
			if s.Monitor.OrchestrationID == uuid.Nil {
				return rawconfig.Colorize.Secondary("·")
			}
			return s.Monitor.OrchestrationID.String()[:8]
		})})
	}
}

// statusText is a status of an instance or a resource as the board colors
// it: a down one in gray where the object is up, as an instance down on a
// node of a failover object running elsewhere, and in red where it is not,
// as om mon colors its instance icons.
func (t *board) statusText(st status.T) string {
	if st == status.Down {
		if obj := t.digest.Object.ActorStatus; obj != nil && obj.Avail == status.Up {
			return rawconfig.Colorize.Secondary(st.String())
		}
	}
	return colorstatus.Sprint(st, rawconfig.Colorize)
}

func isActor(s instance.States) bool {
	return s.Config.ActorConfig != nil
}

func isGlobalExpect(v instance.MonitorGlobalExpect) bool {
	return v != instance.MonitorGlobalExpectNone && v != instance.MonitorGlobalExpectInit
}

func isLocalExpect(v instance.MonitorLocalExpect) bool {
	return v != instance.MonitorLocalExpectNone && v != instance.MonitorLocalExpectInit
}

func hasExpect(s instance.States) bool {
	return isGlobalExpect(s.Monitor.GlobalExpect) || isLocalExpect(s.Monitor.LocalExpect)
}

// resourceOrder returns the resources of the object in the order the
// actions run them, from the instance knowing the most of them, followed by
// the ones only other instances know.
func (t *board) resourceOrder() []resource.Status {
	var ref []resource.Status
	for _, nodename := range t.nodes {
		s := t.states[nodename]
		if l := s.Status.SortedResources(); len(l) > len(ref) {
			ref = l
		}
	}
	seen := make(map[string]bool)
	for _, r := range ref {
		seen[r.ResourceID.Name] = true
	}
	for _, nodename := range t.nodes {
		s := t.states[nodename]
		for _, r := range s.Status.SortedResources() {
			if !seen[r.ResourceID.Name] {
				seen[r.ResourceID.Name] = true
				ref = append(ref, r)
			}
		}
	}
	return ref
}

func (t *board) loadResourceRows() {
	order := t.resourceOrder()
	lastSubset := ""
	for _, r := range order {
		rid := r.ResourceID.Name
		prefix := ""
		subset := r.Subset
		if subset != "" {
			name := resourceset.T{Name: subset, DriverGroup: r.ResourceID.DriverGroup()}.String()
			if name != lastSubset {
				t.loadSubsetRow(name)
			}
			lastSubset = name
			// The resources of a subset are indented under its row.
			prefix = "  "
		} else {
			lastSubset = ""
		}
		t.loadResourceRow(prefix, rid, r.Type, func(s instance.States) (resource.Status, bool) {
			rs, ok := s.Status.Resources[rid]
			return rs, ok
		}, func(s instance.States, rs resource.Status) string {
			return flags(rid, s, rs)
		})
		if r.ResourceID.DriverGroup().String() == "container" {
			t.loadEncapRows(rid)
		}
	}
}

// loadSubsetRow adds the row of a subset, marked "//" when its resources
// run in parallel, its resources listed under it.
func (t *board) loadSubsetRow(name string) {
	parallel := ""
	for _, nodename := range t.nodes {
		s := t.states[nodename]
		if isActor(s) {
			if cfg, ok := s.Config.Subsets[name]; ok && cfg.Parallel {
				parallel = "//"
			}
		}
	}
	t.rows = append(t.rows, row{
		name:  name,
		cells: make([]string, len(t.nodes)),
		desc:  parallel,
	})
}

// loadEncapRows adds the rows of the resources run inside a container, as
// its encapsulated agent reports them.
func (t *board) loadEncapRows(container string) {
	order := make([]resource.Status, 0)
	seen := make(map[string]bool)
	for _, nodename := range t.nodes {
		s := t.states[nodename]
		encap, ok := s.Status.Encap[container]
		if !ok {
			continue
		}
		for _, r := range encap.SortedResources() {
			if !seen[r.ResourceID.Name] {
				seen[r.ResourceID.Name] = true
				order = append(order, r)
			}
		}
	}
	for _, r := range order {
		rid := r.ResourceID.Name
		// The resources a container runs are indented under its row.
		prefix := "  "
		t.loadResourceRow(prefix, rid, r.Type, func(s instance.States) (resource.Status, bool) {
			encap, ok := s.Status.Encap[container]
			if !ok {
				return resource.Status{}, false
			}
			rs, ok := encap.Resources[rid]
			return rs, ok
		}, func(s instance.States, rs resource.Status) string {
			return flags(rid, s, rs)
		})
	}
}

// loadResourceRow adds the row of a resource, and the rows of the parts of
// its label too long for the cells.
func (t *board) loadResourceRow(prefix, rid, typ string, get func(instance.States) (resource.Status, bool), flagsOf func(instance.States, resource.Status) string) {
	labels := make(map[string]string)
	for _, nodename := range t.nodes {
		if rs, ok := get(t.states[nodename]); ok {
			labels[nodename] = rs.Label
		}
	}
	common, varying := splitLabels(t.nodes, labels)
	if _, short, ok := strings.Cut(typ, "."); ok {
		typ = short
	}
	common = strings.TrimSpace(strings.TrimPrefix(common, typ))
	cells := t.cellsOf(func(s instance.States) string {
		rs, ok := get(s)
		if !ok {
			return rawconfig.Colorize.Secondary("·")
		}
		st := t.statusText(rs.Status)
		markers := make([]string, 0)
		for _, e := range rs.Log {
			markers = append(markers, t.noteOf(rid, s.Node.Name, e).placeholder())
		}
		return joinCell(st, flagsOf(s, rs), uniq(markers))
	})
	t.rows = append(t.rows, row{name: prefix + rid, typ: typ, cells: cells, desc: common})
	// The parts of the label differing by node, one row per distinct
	// value, naming the nodes having it.
	byValue := make(map[string][]string)
	values := make([]string, 0)
	for _, nodename := range t.nodes {
		v, ok := varying[nodename]
		if !ok || v == "" {
			continue
		}
		// A device the label joins to its mount point with a "@", the
		// mount point shared, stands alone on its row.
		v = strings.TrimSuffix(v, "@")
		if _, ok := byValue[v]; !ok {
			values = append(values, v)
		}
		byValue[v] = append(byValue[v], nodename)
	}
	// In the description column, the nodes as a scoped keyword names
	// them, "@<node>", then their value.
	for _, v := range values {
		scopes := make([]string, len(byValue[v]))
		for i, nodename := range byValue[v] {
			scopes[i] = "@" + nodename
		}
		t.rows = append(t.rows, row{desc: strings.Join(scopes, " ") + " " + v, cells: make([]string, len(t.nodes))})
	}
}

// words cuts a label in words: at the spaces, and after a "@", which joins
// a device to its mount point, so the part the nodes share, as the mount
// point, is told from the part they do not, as the device. A word ending
// with "@" is glued to the next one when the words are joined back.
func words(label string) []string {
	l := make([]string, 0)
	for _, w := range strings.Fields(label) {
		for {
			i := strings.Index(w, "@")
			if i < 0 || i == len(w)-1 {
				break
			}
			l = append(l, w[:i+1])
			w = w[i+1:]
		}
		l = append(l, w)
	}
	return l
}

func joinWords(l []string) string {
	var b strings.Builder
	for i, w := range l {
		if i > 0 && !strings.HasSuffix(l[i-1], "@") {
			b.WriteString(" ")
		}
		b.WriteString(w)
	}
	return b.String()
}

// splitLabels returns the words of the labels the nodes share, in the order
// of the first label, and by node the words of its label the others do not
// all have.
func splitLabels(nodes []string, labels map[string]string) (string, map[string]string) {
	varying := make(map[string]string)
	if len(labels) == 0 {
		return "", varying
	}
	byNode := make(map[string][]string)
	var ref []string
	for _, nodename := range nodes {
		l, ok := labels[nodename]
		if !ok {
			continue
		}
		byNode[nodename] = words(l)
		if ref == nil {
			ref = byNode[nodename]
		}
	}
	isCommon := func(w string) bool {
		for _, l := range byNode {
			if !slices.Contains(l, w) {
				return false
			}
		}
		return true
	}
	common := make([]string, 0)
	for _, w := range ref {
		if isCommon(w) {
			common = append(common, w)
		}
	}
	for nodename, l := range byNode {
		own := make([]string, 0)
		for _, w := range l {
			if !isCommon(w) {
				own = append(own, w)
			}
		}
		varying[nodename] = joinWords(own)
	}
	return joinWords(common), varying
}

// flags returns the flags of a resource on a node departing from the usual:
// R running, M monitored, D disabled, O optional, E encap, P not
// provisioned, S standby, and the restarts remaining, + for 10 or more, or X
// stopped on purpose, in the order the instance status flags have them.
func flags(rid string, s instance.States, rs resource.Status) string {
	l := make([]string, 0)
	if s.Status.Running.Has(rid) {
		l = append(l, "R")
	}
	if rs.IsMonitored {
		l = append(l, "M")
	}
	if rs.IsDisabled {
		l = append(l, "D")
	}
	if rs.IsOptional {
		l = append(l, "O")
	}
	if rs.IsEncap {
		l = append(l, "E")
	}
	if rs.IsProvisioned.State == provisioned.False {
		l = append(l, rawconfig.Colorize.Error("P"))
	}
	if rs.IsStandby {
		l = append(l, "S")
	}
	if isActor(s) {
		restart := 0
		if rcfg := s.Config.Resources.Get(rid); rcfg != nil {
			restart = rcfg.Restart
		}
		remaining := 0
		if rmon := s.Monitor.Resources.Get(rid); rmon != nil && rmon.Restart != nil {
			remaining = rmon.Restart.Remaining
		}
		// As the instance status flags say it: X stopped, the restarts
		// remaining, + for 10 or more, and nothing without restarts.
		if f := rs.RestartFlag(restart, remaining); f != "." {
			l = append(l, f)
		}
	}
	return strings.Join(l, "")
}

func joinCell(st, flags string, markers []string) string {
	s := st
	if flags != "" {
		s += " " + flags
	}
	if len(markers) > 0 {
		s += " " + strings.Join(markers, "")
	}
	return s
}

func uniq(l []string) []string {
	out := make([]string, 0, len(l))
	for _, s := range l {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// noteOf returns the note of a message a resource logged on a node, the one
// of the same message logged on another node if any.
func (t *board) noteOf(rid, nodename string, e resource.StatusLogEntry) *note {
	key := rid + "\x00" + string(e.Level) + "\x00" + e.Message
	n, ok := t.noteByKey[key]
	if !ok {
		n = &note{id: len(t.notes), level: e.Level, rid: rid, text: e.Message}
		t.noteByKey[key] = n
		t.notes = append(t.notes, n)
	}
	if !slices.Contains(n.nodes, nodename) {
		n.nodes = append(n.nodes, nodename)
	}
	return n
}

// loadObjectNotes adds the notes about the object and its instances, which
// no resource marker points to.
func (t *board) loadObjectNotes() {
	add := func(level resource.Level, subject string, nodes []string, text string) {
		t.notes = append(t.notes, &note{id: len(t.notes), level: level, rid: subject, nodes: nodes, text: text})
	}
	if obj := t.digest.Object.ActorStatus; obj.ExcessInstances() > 0 {
		up := t.nodesWith(func(s instance.States) bool { return s.Status.Avail == status.Up })
		add(resource.ErrorLevel, "instances", up, fmt.Sprintf(
			"%d instances up, %s allows %d: the shared resources may be written by several nodes at once. Stop the instances in excess.",
			obj.UpInstancesCount, obj.Topology, obj.AllowedInstances()))
	}
	if obj := t.digest.Object.ActorStatus; obj != nil {
		if obj.PlacementState == placement.NonOptimal {
			leaders := t.nodesWith(func(s instance.States) bool { return s.Monitor.IsLeader })
			text := "non-optimal: the instances are not running on the nodes the placement policy prefers"
			if len(leaders) > 0 {
				text = fmt.Sprintf("non-optimal: the placement policy prefers %s", strings.Join(leaders, " "))
			}
			add(resource.WarnLevel, "placement", nil, text)
		}
	}
	// An overall status in warn no resource note explains, as a resource
	// in warn logging nothing: a note says it, for the issue mark to point
	// to one.
	if obj := t.digest.Object.ActorStatus; obj != nil && obj.Overall == status.Warn && !t.hasResourceIssueNote() {
		l := t.nodesWith(func(s instance.States) bool { return s.Status.Overall == status.Warn })
		add(resource.WarnLevel, "overall", l, "resources are in warn")
	}
	if !t.digest.IsCompat {
		add(resource.ErrorLevel, "agents", nil, "the nodes run incompatible agent versions")
	}
	if l := t.nodesWith(func(s instance.States) bool { return s.Monitor.UpdatedAt.IsZero() }); len(l) > 0 {
		add(resource.WarnLevel, "monitor", l, "no instance monitor: the daemon is down, or has not started monitoring the instance")
	}
	if l := t.nodesWith(func(s instance.States) bool { return strings.HasSuffix(s.Monitor.State.String(), "failed") }); len(l) > 0 {
		for _, nodename := range l {
			add(resource.ErrorLevel, "monitor", []string{nodename}, fmt.Sprintf("%s: the orchestration gave up, clear the instance to retry", t.states[nodename].Monitor.State))
		}
	}
	if l := t.nodesWith(func(s instance.States) bool { return s.Status.IsFrozen() }); len(l) > 0 {
		add(resource.InfoLevel, "frozen", l, "the daemon takes no initiative on the instance")
	}
	if l := t.nodesWith(func(s instance.States) bool { return s.Status.IsStopped() }); len(l) > 0 {
		add(resource.InfoLevel, "stopped", l, "stopped on purpose: the daemon does not start it back")
	}
	if l := t.nodesWith(func(s instance.States) bool {
		return isActor(s) && (s.Status.Provisioned == provisioned.False || s.Status.Provisioned == provisioned.Mixed)
	}); len(l) > 0 {
		add(resource.ErrorLevel, "provisioned", l, "the instance is not provisioned, or partly")
	}
}

// hasResourceIssueNote says whether a resource logged a warning or an
// error.
func (t *board) hasResourceIssueNote() bool {
	for _, n := range t.noteByKey {
		if n.level == resource.ErrorLevel || n.level == resource.WarnLevel {
			return true
		}
	}
	return false
}

// numberNotes orders the notes by level, errors first, and gives the markers
// to the notes of the resources.
func (t *board) numberNotes() {
	rank := map[resource.Level]int{resource.ErrorLevel: 0, resource.WarnLevel: 1, resource.InfoLevel: 2}
	sort.SliceStable(t.notes, func(i, j int) bool {
		return rank[t.notes[i].level] < rank[t.notes[j].level]
	})
	i := 0
	for _, n := range t.notes {
		if _, ok := t.noteByKey[n.rid+"\x00"+string(n.level)+"\x00"+n.text]; !ok {
			continue
		}
		i++
		n.marker = superscript(i)
	}
	// The markers were taken when the cells were made, before the notes
	// were numbered: put the numbers in the cells now.
	for ri := range t.rows {
		for ci := range t.rows[ri].cells {
			t.rows[ri].cells[ci] = t.renumber(t.rows[ri].cells[ci])
		}
	}
}

// placeholder names the note in a cell until the notes are numbered.
func (n *note) placeholder() string {
	return fmt.Sprintf("\x01%d\x02", n.id)
}

var regexpPlaceholder = regexp.MustCompile("\x01([0-9]+)\x02")

// renumber replaces the note placeholders of a cell by their markers,
// colored by the level of the note.
func (t *board) renumber(cell string) string {
	byID := make(map[int]*note, len(t.notes))
	for _, n := range t.notes {
		byID[n.id] = n
	}
	return regexpPlaceholder.ReplaceAllStringFunc(cell, func(m string) string {
		var id int
		_, _ = fmt.Sscanf(m[1:len(m)-1], "%d", &id)
		n, ok := byID[id]
		if !ok {
			return ""
		}
		switch n.level {
		case resource.ErrorLevel:
			return rawconfig.Colorize.Error(n.marker)
		case resource.WarnLevel:
			return rawconfig.Colorize.Warning(n.marker)
		default:
			return rawconfig.Colorize.Secondary(n.marker)
		}
	})
}

func superscript(n int) string {
	s := ""
	for _, c := range fmt.Sprint(n) {
		s += string(superscripts[c-'0'])
	}
	return s
}

func (t *board) render(width int) string {
	var b strings.Builder
	header := t.headerLine()

	// The widths of the columns: the name, a column per node, the type.
	nameW, typW := 0, 0
	nodeW := make([]int, len(t.nodes))
	for i, nodename := range t.nodes {
		nodeW[i] = visibleWidth(t.nodeHeader(nodename))
	}
	for _, r := range t.rows {
		nameW = max(nameW, runewidth.StringWidth(r.name))
		typW = max(typW, runewidth.StringWidth(r.typ))
		for i, c := range r.cells {
			nodeW[i] = max(nodeW[i], visibleWidth(c))
		}
	}
	descStart := 1 + nameW + 2 + typW + 2
	for _, w := range nodeW {
		descStart += w + 2
	}
	if width <= 0 {
		width = max(visibleWidth(header), descStart+20)
	}

	b.WriteString(header + "\n")
	b.WriteString("\n")

	pad := func(s string, w int) string {
		return s + strings.Repeat(" ", max(0, w-visibleWidth(s)))
	}
	// The columns: the name, a column per node, the type of the
	// resource, and its description.
	line := func(name, typ string, cells []string, desc string) {
		s := " " + pad(name, nameW) + "  "
		for i := range nodeW {
			c := ""
			if i < len(cells) {
				c = cells[i]
			}
			s += pad(c, nodeW[i]) + "  "
		}
		s += pad(typ, typW) + "  " + desc
		b.WriteString(strings.TrimRight(s, " ") + "\n")
	}
	nodeHeaders := make([]string, len(t.nodes))
	for i, nodename := range t.nodes {
		nodeHeaders[i] = t.nodeHeader(nodename)
	}
	line("", "", nodeHeaders, "")
	for _, r := range t.rows {
		if r.blank {
			b.WriteString("\n")
			continue
		}
		name := r.name
		if r.heading {
			name = rawconfig.Colorize.Bold(name)
		}
		line(name, r.typ, r.cells, r.desc)
	}
	if len(t.notes) > 0 {
		b.WriteString("\n " + rawconfig.Colorize.Bold("notes") + "\n")
		b.WriteString(t.renderNotes(width))
	}
	return b.String()
}

func (t *board) headerLine() string {
	d := t.digest
	l := []string{rawconfig.Colorize.Bold(d.Path.String())}
	if obj := d.Object.ActorStatus; obj != nil {
		avail := colorstatus.Sprint(obj.Avail, rawconfig.Colorize)
		if obj.Avail != status.NotApplicable {
			// The instances up over the ones expected, as om mon counts
			// them, an error when beyond what the topology allows.
			count := fmt.Sprintf("%d/%d", obj.UpInstancesCount, obj.ExpectedInstances())
			if obj.ExpectedInstances() == 0 {
				// No instance count is expected, as of a flex with no
				// target: the instances up alone, as om mon says it.
				count = fmt.Sprint(obj.UpInstancesCount)
			}
			if obj.ExcessInstances() > 0 {
				count = rawconfig.Colorize.Error(count)
			}
			avail = avail + t.issueMark() + " " + count
		} else {
			avail += t.issueMark()
		}
		l = append(l, avail)
		facts := make([]string, 0)
		if s := obj.Topology.String(); s != "" {
			facts = append(facts, s)
		}
		if obj.Orchestrate != "" {
			facts = append(facts, obj.Orchestrate)
		}
		if s := obj.PlacementPolicy.String(); s != "" {
			facts = append(facts, s)
		}
		if len(facts) > 0 {
			l = append(l, rawconfig.Colorize.Secondary(strings.Join(facts, " · ")))
		}
	}
	return strings.Join(l, "   ")
}

// issueMark is the "!" om mon puts after the avail of an object with issues,
// here when a note is a warning or an error, the notes saying which: red
// when one is an error, orange otherwise.
func (t *board) issueMark() string {
	mark := ""
	for _, n := range t.notes {
		switch n.level {
		case resource.ErrorLevel:
			return rawconfig.Colorize.Error("!")
		case resource.WarnLevel:
			mark = rawconfig.Colorize.Warning("!")
		}
	}
	return mark
}

// nodeHeader is the header of the column of a node: its name. Where the
// object is up is the instance row, just below, and which node the placement
// prefers is said by the placement note, when it matters.
func (t *board) nodeHeader(nodename string) string {
	return rawconfig.Colorize.Bold(nodename)
}

func (t *board) renderNotes(width int) string {
	var b strings.Builder
	infos := 0
	for _, n := range t.notes {
		if n.level == resource.InfoLevel {
			infos++
		}
	}
	listedInfos := 0
	for _, n := range t.notes {
		if n.level == resource.InfoLevel {
			if infos > infoMax && listedInfos >= infoMax {
				continue
			}
			listedInfos++
		}
		// The level is written out, the color only adding to it: a
		// board read without colors, as piped or with --color=no, still
		// tells an error from a warning.
		var mark string
		switch n.level {
		case resource.ErrorLevel:
			mark = rawconfig.Colorize.Error("error")
		case resource.WarnLevel:
			mark = rawconfig.Colorize.Warning("warn ")
		default:
			mark = rawconfig.Colorize.Secondary("info ")
		}
		head := fmt.Sprintf(" %-3s%s %s", n.marker, mark, rawconfig.Colorize.Bold(n.rid))
		nodes := ""
		switch {
		case len(n.nodes) == 0:
		case len(n.nodes) == len(t.nodes) && len(t.nodes) > 1:
			nodes = "all"
		default:
			nodes = strings.Join(n.nodes, " ")
		}
		if nodes != "" {
			head += "   " + nodes
		}
		b.WriteString(head + "\n")
		text := n.text
		if n.level == resource.InfoLevel {
			text = rawconfig.Colorize.Secondary(text)
		}
		for _, l := range wrap(text, width-7) {
			b.WriteString("       " + l + "\n")
		}
	}
	if infos > infoMax {
		fmt.Fprintf(&b, " %s\n", rawconfig.Colorize.Secondary(fmt.Sprintf("· %d more info notes: see -o json, or instance status", infos-infoMax)))
	}
	return b.String()
}

// wrap cuts the text in lines no wider than width, at the spaces, keeping
// the line breaks the text has. A word wider than width is not cut.
func wrap(text string, width int) []string {
	if width < 20 {
		width = 20
	}
	lines := make([]string, 0)
	for _, paragraph := range strings.Split(text, "\n") {
		current := ""
		for _, w := range strings.Fields(paragraph) {
			switch {
			case current == "":
				current = w
			case visibleWidth(current)+1+visibleWidth(w) > width:
				lines = append(lines, current)
				current = w
			default:
				current += " " + w
			}
		}
		lines = append(lines, current)
	}
	return lines
}

func visibleWidth(s string) int {
	return runewidth.StringWidth(regexpANSI.ReplaceAllString(s, ""))
}

// documentSeparator is the line between the renderings of several objects.
const documentSeparator = "---"

// JoinDocuments joins the renderings of several objects, as their boards or
// their instance status trees, with a "---" line between each, set apart by
// empty lines.
func JoinDocuments(l []string) string {
	for i := range l {
		l[i] = strings.TrimRight(l[i], "\n") + "\n"
	}
	return strings.Join(l, "\n"+documentSeparator+"\n\n")
}

// RenderTerminalList returns the boards of the objects, separated by a
// "---" line, as RenderTerminal renders each of them.
func RenderTerminalList(digests []object.Digest) string {
	l := make([]string, len(digests))
	for i, d := range digests {
		l[i] = RenderTerminal(d)
	}
	return JoinDocuments(l)
}

// RenderTerminal returns the board as wide as the terminal attached to the
// standard output, or the COLUMNS environment variable says, as wide as it
// needs otherwise.
func RenderTerminal(digest object.Digest) string {
	return Render(digest, outputColumns())
}

func outputColumns() int {
	fd := int(os.Stdout.Fd())
	if term.IsTerminal(fd) {
		if c, _, err := term.GetSize(fd); err == nil && c > 0 {
			return c - 1
		}
	}
	if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
		return c - 1
	}
	return 0
}
