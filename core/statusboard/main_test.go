package statusboard

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/placement"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
)

func init() {
	color.NoColor = true
}

type res struct {
	rid, typ, label, subset string
	st                      status.T
	log                     []resource.StatusLogEntry
}

func newInstance(p naming.Path, nodename string, avail status.T, resources ...res) instance.States {
	s := instance.States{
		Path: p,
		Node: instance.Node{Name: nodename},
		Config: instance.Config{
			ActorConfig: &instance.ActorConfig{
				Subsets: instance.SubsetConfigs{"subset#app:workers": {Parallel: true}},
			},
		},
		Monitor: instance.Monitor{State: instance.MonitorStateIdle, UpdatedAt: time.Now()},
		Status: instance.Status{
			Avail:     avail,
			Resources: make(instance.ResourceStatuses),
		},
	}
	for _, r := range resources {
		s.Status.Resources[r.rid] = resource.Status{
			Label:  r.label,
			Type:   r.typ,
			Status: r.st,
			Subset: r.subset,
			Log:    r.log,
		}
	}
	return s
}

func newDigest(instances ...instance.States) object.Digest {
	p := instances[0].Path
	d := object.Digest{Path: p, IsCompat: true}
	for _, s := range instances {
		d.Object.Scope = append(d.Object.Scope, s.Node.Name)
		d.Instances = append(d.Instances, s)
	}
	return d
}

// notesOf returns the notes of the board, from their heading, colors aside
// to find it.
func notesOf(t *testing.T, board string) string {
	lines := strings.Split(board, "\n")
	for i, l := range lines {
		if regexpANSI.ReplaceAllString(l, "") == " notes" {
			return strings.Join(lines[i:], "\n")
		}
	}
	require.Failf(t, "no notes", "no notes in:\n%s", board)
	return ""
}

// noteLines returns the lines of the notes, their spaces collapsed, so the
// assertions read their words, whatever the width of the first column.
func noteLines(t *testing.T, board string) []string {
	l := make([]string, 0)
	for _, line := range strings.Split(notesOf(t, board), "\n") {
		l = append(l, strings.Join(strings.Fields(regexpANSI.ReplaceAllString(line, "")), " "))
	}
	return l
}

// lineOf returns the line of the board starting with the row name, colors
// aside.
func lineOf(t *testing.T, board, name string) string {
	for _, l := range strings.Split(board, "\n") {
		if strings.HasPrefix(strings.TrimSpace(regexpANSI.ReplaceAllString(l, "")), name) {
			return l
		}
	}
	require.Failf(t, "no line", "no line %q in:\n%s", name, board)
	return ""
}

func TestALabelPartDifferingByNodeGoesOnScopedRows(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	board := Render(newDigest(
		newInstance(p, "n1", status.Up, res{rid: "ip#0", typ: "ip.host", label: "host 10.0.0.1/24 br0", st: status.Up}),
		newInstance(p, "n2", status.Up, res{rid: "ip#0", typ: "ip.host", label: "host 10.0.0.2/24 br0", st: status.Up}),
	), 100)
	ip := lineOf(t, board, "ip#0")
	assert.Contains(t, ip, "br0", "the shared part is the label")
	assert.NotContains(t, ip, "10.0.0.1", "the varying part is not in the label")
	assert.Contains(t, board, "@n1 10.0.0.1/24")
	assert.Contains(t, board, "@n2 10.0.0.2/24")
}

func TestALongLabelPartDifferingByNodeGoesOnRowsOfItsOwn(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	a := "2001:41d0:b00:8800:2345:6789:2900:1/64"
	b := "2001:41d0:b00:8800:2345:6789:2900:2/64"
	board := Render(newDigest(
		newInstance(p, "n1", status.Up, res{rid: "ip#0", typ: "ip.netns", label: "netns " + a, st: status.Up}),
		newInstance(p, "n2", status.Down, res{rid: "ip#0", typ: "ip.netns", label: "netns " + b, st: status.Down}),
		newInstance(p, "n3", status.Down, res{rid: "ip#0", typ: "ip.netns", label: "netns " + b, st: status.Down}),
	), 120)
	hasRow := func(scopes, value string) bool {
		for _, l := range strings.Split(board, "\n") {
			if strings.TrimSpace(l) == scopes+" "+value {
				return true
			}
		}
		return false
	}
	assert.True(t, hasRow("@n1", a), "a row of @n1 with its address in:\n%s", board)
	assert.True(t, hasRow("@n2 @n3", b), "a row of @n2 @n3 with their address in:\n%s", board)
}

func TestASubsetRowNamesTheSubsetAndItsResourcesFollow(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	board := Render(newDigest(
		newInstance(p, "n1", status.Warn,
			res{rid: "app#1", typ: "app.simple", subset: "workers", st: status.Up},
			res{rid: "app#2", typ: "app.simple", subset: "workers", st: status.Down},
		),
	), 100)
	subset := lineOf(t, board, "subset#app:workers")
	assert.Equal(t, []string{"subset#app:workers", "//"}, strings.Fields(subset), "the name and the parallel mark, no status")
	assert.Contains(t, lineOf(t, board, "app#1"), "O", "up")
	assert.Contains(t, lineOf(t, board, "app#2"), "X", "down")
}

func TestTheNotesListErrorsFirstAndGroupTheNodesOfAMessage(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	split := resource.StatusLogEntry{Level: resource.ErrorLevel, Message: "split brain " + strings.Repeat("word ", 40)}
	info := resource.StatusLogEntry{Level: resource.InfoLevel, Message: "Secondary"}
	board := Render(newDigest(
		newInstance(p, "n1", status.Warn, res{rid: "disk#2", typ: "disk.drbd", st: status.Warn, log: []resource.StatusLogEntry{split}}),
		newInstance(p, "n2", status.StandbyUp, res{rid: "disk#2", typ: "disk.drbd", st: status.StandbyUp, log: []resource.StatusLogEntry{info}}),
		newInstance(p, "n3", status.Warn, res{rid: "disk#2", typ: "disk.drbd", st: status.Warn, log: []resource.StatusLogEntry{split}}),
	), 80)
	notes := notesOf(t, board)
	assert.Less(t, strings.Index(notes, "split brain"), strings.Index(notes, "Secondary"), "the error is listed first")
	assert.Contains(t, noteLines(t, board), "¹ error disk#2 n1 n3", "the nodes of the same message share its note")
	assert.Contains(t, lineOf(t, board, "disk#2"), "! ¹", "the warn icon and its marker")
	for _, l := range strings.Split(notes, "\n") {
		assert.LessOrEqual(t, len([]rune(l)), 80, "the note is wrapped: %q", l)
	}
}

func TestTheInfoNotesBeyondTheMaxAreCounted(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	resources := make([]res, 0)
	for i := 0; i < infoMax+3; i++ {
		resources = append(resources, res{
			rid: fmt.Sprintf("app#%d", i), typ: "app.simple", st: status.Up,
			log: []resource.StatusLogEntry{{Level: resource.InfoLevel, Message: fmt.Sprintf("info %d", i)}},
		})
	}
	board := Render(newDigest(newInstance(p, "n1", status.Up, resources...)), 100)
	assert.Contains(t, board, "3 more info notes")
}

func TestInstancesUpBeyondTheTopologyAreAnError(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	d := newDigest(
		newInstance(p, "n1", status.Up, res{rid: "volume#1", typ: "volume", st: status.Up}),
		newInstance(p, "n2", status.Up, res{rid: "volume#1", typ: "volume", st: status.Up}),
		newInstance(p, "n3", status.Down, res{rid: "volume#1", typ: "volume", st: status.Down}),
	)
	d.Object.ActorStatus = &object.ActorStatus{Avail: status.Warn, Topology: topology.Failover, UpInstancesCount: 2}
	board := Render(d, 100)
	assert.Equal(t, []string{"instance", "O", "O", "X", "2/1"}, strings.Fields(lineOf(t, board, "instance")), "the om mon up/expected counter ends the instance row")
	notes := notesOf(t, board)
	first := strings.Split(notes, "\n")[1]
	assert.Contains(t, first, "error instances   n1 n2", "the error is the first note, naming the nodes it is up on")
	assert.NotContains(t, lineOf(t, board, "n1"), "*", "no up marker in the node header")
}

func TestAnObjectWithNoResourceHasNoResourcesHeading(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	board := Render(newDigest(newInstance(p, "n1", status.NotApplicable)), 80)
	assert.NotContains(t, board, "resources")
	assert.False(t, strings.HasSuffix(board, "\n\n"), "no trailing empty line:\n%q", board)
}

func TestTheBoardsOfSeveralObjectsAreSeparated(t *testing.T) {
	p1 := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	p2 := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s2"}
	out := RenderTerminalList([]object.Digest{
		newDigest(newInstance(p1, "n1", status.Up)),
		newDigest(newInstance(p2, "n1", status.Up)),
	})
	assert.Equal(t, 1, strings.Count(out, "\n---\n"), "one separator between two boards:\n%s", out)
	assert.Less(t, strings.Index(out, "s1"), strings.Index(out, "---"))
	assert.Greater(t, strings.Index(out, "s2"), strings.Index(out, "---"))
}

// A resource down is an issue where the object is not up, and the normal
// state of the nodes the object does not run on where it is up.
func TestDownIsGrayWhereTheObjectIsUp(t *testing.T) {
	color.NoColor = false
	defer func() { color.NoColor = true }()
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	render := func(avail status.T) string {
		d := newDigest(
			newInstance(p, "n1", status.Up, res{rid: "fs#1", typ: "fs.ext4", st: status.Up}),
			newInstance(p, "n2", status.Down, res{rid: "fs#1", typ: "fs.ext4", st: status.Down}),
		)
		up := 0
		if avail == status.Up {
			up = 1
		}
		d.Object.ActorStatus = &object.ActorStatus{Avail: avail, Topology: topology.Failover, UpInstancesCount: up}
		return Render(d, 100)
	}
	gray := rawconfig.Colorize.Secondary("X")
	red := rawconfig.Colorize.Error("X")
	assert.Contains(t, render(status.Up), gray)
	assert.NotContains(t, render(status.Up), red)
	assert.Contains(t, render(status.Down), red)
}

func TestTheHeaderSaysTheStatusesAndTheStatesOfTheObject(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	s := newInstance(p, "n1", status.Up, res{rid: "fs#1", typ: "fs.ext4", st: status.Warn})
	s.Status.Overall = status.Warn
	d := newDigest(s)
	d.Object.ActorStatus = &object.ActorStatus{Avail: status.Up, Overall: status.Warn, Topology: topology.Failover, UpInstancesCount: 1, Provisioned: provisioned.Mixed, Frozen: "mixed"}
	board := Render(d, 100)
	lines := strings.Split(board, "\n")
	assert.Equal(t, "s1   failover", lines[0], "the path and the policies")
	assert.Equal(t, []string{"avail", "up"}, strings.Fields(lines[1]))
	assert.Equal(t, []string{"overall", "warn"}, strings.Fields(lines[2]))
	assert.Equal(t, []string{"state", "mixed-provisioned,", "mixed-frozen"}, strings.Fields(lines[3]))
	assert.Equal(t, "O!", strings.Fields(lineOf(t, board, "instance"))[1], "the om mon overall warn mark of the instance")
	// A resource in warn logging nothing: the overall warn gets a note.
	assert.Contains(t, noteLines(t, board), "· warn overall n1")
}

// A copy older than its contract allows is an error: the instance has the om
// mon "L" mark, and the resource a note, as the copy never received.
func TestARPOBreachIsAnError(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	breached := time.Now().Add(-time.Hour)
	s := newInstance(p, "n1", status.Down, res{rid: "sync#1", typ: "sync.rsync", st: status.NotApplicable})
	s.Status.RPOBreachedAt = breached
	rs := s.Status.Resources["sync#1"]
	rs.RPOBreachedAt = breached
	s.Status.Resources["sync#1"] = rs
	never := newInstance(p, "n2", status.Down, res{rid: "sync#1", typ: "sync.rsync", st: status.NotApplicable})
	rs = never.Status.Resources["sync#1"]
	rs.RPOBreachedAt = time.Unix(0, 0)
	never.Status.Resources["sync#1"] = rs
	board := Render(newDigest(s, never), 100)
	assert.Equal(t, "XL", strings.Fields(lineOf(t, board, "instance"))[1])
	notes := notesOf(t, board)
	assert.Contains(t, noteLines(t, board), "¹ error sync#1 n1")
	assert.Contains(t, notes, "rpo breached since")
	assert.Contains(t, notes, "no copy was ever received here")
	assert.NotContains(t, notes, "1970")
}

// The markers of a cell come in the order of the notes they point to.
func TestTheMarkersOfACellAreInOrder(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	log := []resource.StatusLogEntry{
		{Level: resource.InfoLevel, Message: "an info"},
		{Level: resource.ErrorLevel, Message: "an error"},
	}
	board := Render(newDigest(newInstance(p, "n1", status.Warn, res{rid: "app#1", typ: "app.simple", st: status.Warn, log: log})), 100)
	assert.Contains(t, lineOf(t, board, "app#1"), "! ¹²")
}

// A wrapped info note is colored line by line, no color code spanning lines.
func TestAWrappedNoteIsColoredLineByLine(t *testing.T) {
	color.NoColor = false
	defer func() { color.NoColor = true }()
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	log := []resource.StatusLogEntry{{Level: resource.InfoLevel, Message: strings.Repeat("word ", 40)}}
	board := Render(newDigest(newInstance(p, "n1", status.Up, res{rid: "app#1", typ: "app.simple", st: status.Up, log: log})), 60)
	for _, l := range strings.Split(notesOf(t, board), "\n") {
		if strings.Contains(l, "word") {
			assert.True(t, strings.HasSuffix(l, "\x1b[0m"), "the color closes on its line: %q", l)
		}
	}
}

func TestResourcesLoggingTheSameMessageShareANote(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	msg := []resource.StatusLogEntry{{Level: resource.InfoLevel, Message: "not evaluated (fs#1 is down)"}}
	board := Render(newDigest(newInstance(p, "n1", status.Down,
		res{rid: "app#1", typ: "app.forking", st: status.NotApplicable, log: msg},
		res{rid: "app#env", typ: "app.forking", st: status.NotApplicable, log: msg},
	)), 100)
	notes := notesOf(t, board)
	assert.Equal(t, 1, strings.Count(notes, "not evaluated"), "the text is written once:\n%s", notes)
	lines := noteLines(t, board)
	assert.Contains(t, lines, "¹ info app#1 n1")
	assert.Contains(t, lines, "info app#env n1", "the second head of the note, with no marker")
	assert.Contains(t, lineOf(t, board, "app#env"), "¹", "the resources share the marker")
}

func TestANoteNoCellPointsToStartsWithADot(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	s := newInstance(p, "n1", status.Down)
	s.Status.FrozenAt = time.Now()
	lines := noteLines(t, Render(newDigest(s), 100))
	assert.Contains(t, lines, "· info frozen n1", "a dot where a numbered note has its number: %v", lines)
}

// An instance down beside instances in excess is not the issue to stress:
// the excess is, which a note says.
func TestDownIsGrayBesideInstancesInExcess(t *testing.T) {
	color.NoColor = false
	defer func() { color.NoColor = true }()
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	d := newDigest(
		newInstance(p, "n1", status.Up),
		newInstance(p, "n2", status.Up),
		newInstance(p, "n3", status.Down),
	)
	d.Object.ActorStatus = &object.ActorStatus{Avail: status.Warn, Topology: topology.Failover, UpInstancesCount: 2}
	board := Render(d, 100)
	assert.Contains(t, lineOf(t, board, "instance"), rawconfig.Colorize.Secondary("X"))

	d.Object.ActorStatus = &object.ActorStatus{Avail: status.Warn, Topology: topology.Flex, UpInstancesCount: 2, Flex: &object.FlexStatus{Target: 3, Max: 3}}
	board = Render(d, 100)
	assert.Contains(t, lineOf(t, board, "instance"), rawconfig.Colorize.Error("X"), "a flex missing an instance")
}

// The instance the placement prefers is marked as om mon marks it, gray, and
// red when the object does not run there.
func TestTheHALeaderInstanceIsMarked(t *testing.T) {
	color.NoColor = false
	defer func() { color.NoColor = true }()
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	leader := newInstance(p, "n1", status.Up)
	leader.Monitor.IsHALeader = true
	d := newDigest(leader, newInstance(p, "n2", status.Up))
	d.Object.ActorStatus = &object.ActorStatus{Avail: status.Warn, Topology: topology.Failover, UpInstancesCount: 2}
	assert.Contains(t, lineOf(t, Render(d, 100), "instance"), rawconfig.Colorize.Secondary("^"))

	d.Object.ActorStatus.PlacementState = placement.NonOptimal
	assert.Contains(t, lineOf(t, Render(d, 100), "instance"), rawconfig.Colorize.Error("^"))
}

// The statuses are the om mon icons, and the instance icon carries the om
// mon marks of a frozen and of a stopped instance.
func TestTheStatusesAreTheOmMonIcons(t *testing.T) {
	p := naming.Path{Namespace: "root", Kind: naming.KindSvc, Name: "s1"}
	frozen := newInstance(p, "n1", status.StandbyUp,
		res{rid: "disk#1", typ: "disk.drbd", st: status.StandbyUp},
		res{rid: "disk#2", typ: "disk.drbd", st: status.StandbyDown},
		res{rid: "fs#1", typ: "fs.flag", st: status.Warn},
		res{rid: "app#1", typ: "app.simple", st: status.NotApplicable},
	)
	frozen.Status.FrozenAt = time.Now()
	stopped := newInstance(p, "n2", status.Down)
	stopped.Status.StoppedAt = time.Now()
	stopped.Node.FrozenAt = time.Now()
	board := Render(newDigest(frozen, stopped), 100)
	fields := func(name string) []string { return strings.Fields(lineOf(t, board, name)) }
	assert.Equal(t, []string{"instance", "o*", "X="}, fields("instance"))
	assert.Equal(t, "o", fields("disk#1")[1])
	assert.Equal(t, "x", fields("disk#2")[1])
	assert.Equal(t, "!", fields("fs#1")[1])
	assert.Equal(t, "/", fields("app#1")[1])
	assert.Contains(t, board, "n2*", "a frozen node")
	for _, l := range strings.Split(board, "\n") {
		assert.False(t, strings.HasPrefix(strings.TrimSpace(l), "frozen"), "no frozen row: %q", l)
	}
}

func TestANodeOfTheScopeReportingNoInstanceHasItsColumn(t *testing.T) {
	// A node of the scope with no instance data, as a node down since the
	// object was created, is shown as such rather than left out, which
	// would show a two-node object as a complete one-node one.
	p, _ := naming.ParsePath("svc1")
	d := newDigest(newInstance(p, "n1", status.Up, res{rid: "fs#1", typ: "fs.flag", st: status.Up}))
	d.Object.Scope = append(d.Object.Scope, "n2")
	board := Render(d, 80)
	assert.Equal(t, []string{"n1", "n2"}, strings.Fields(lineOf(t, board, "n1")))
	assert.Equal(t, []string{"instance", "O", "?"}, strings.Fields(regexpANSI.ReplaceAllString(lineOf(t, board, "instance"), "")))
	assert.Contains(t, noteLines(t, board), "· warn monitor n2")
}

func TestTheMarkersBeyondTheNinthAreLetters(t *testing.T) {
	defer func(v bool) { rawconfig.BoardLetters = v }(rawconfig.BoardLetters)

	rawconfig.BoardLetters = true
	l := make([]string, 0, 30)
	for n := 1; n <= 30; n++ {
		l = append(l, marker(n))
	}
	assert.Equal(t, "¹²³⁴⁵⁶⁷⁸⁹ᴬᴮᴰᴱᴳᴴᴵᴶᴷᴸᴹᴺᴼᴾᴿᵀᵁⱽᵂ²⁹³⁰", strings.Join(l, ""))

	rawconfig.BoardLetters = false
	assert.Equal(t, "⁹", marker(9))
	assert.Equal(t, "¹⁰", marker(10))
	assert.Equal(t, "²⁸", marker(28))
}
