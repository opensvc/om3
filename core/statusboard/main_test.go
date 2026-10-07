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

// notesOf returns the notes of the board, from their heading.
func notesOf(t *testing.T, board string) string {
	i := strings.Index(board, "\n notes\n")
	require.GreaterOrEqual(t, i, 0, "no notes in:\n%s", board)
	return board[i+1:]
}

// lineOf returns the line of the board starting with the row name.
func lineOf(t *testing.T, board, name string) string {
	for _, l := range strings.Split(board, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), name) {
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
	assert.Contains(t, lineOf(t, board, "app#1"), "up")
	assert.Contains(t, lineOf(t, board, "app#2"), "down")
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
	assert.Contains(t, notes, "¹  error disk#2   n1 n3", "the nodes of the same message share its note")
	assert.Contains(t, lineOf(t, board, "disk#2"), "warn ¹")
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
	assert.Contains(t, strings.SplitN(board, "\n", 2)[0], "warn 2/1", "the om mon up/expected counter")
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
		d.Object.ActorStatus = &object.ActorStatus{Avail: avail, Topology: topology.Failover, UpInstancesCount: 1}
		return Render(d, 100)
	}
	gray := rawconfig.Colorize.Secondary("down")
	red := rawconfig.Colorize.Error("down")
	assert.Contains(t, render(status.Up), gray)
	assert.NotContains(t, render(status.Up), red)
	assert.Contains(t, render(status.Down), red)
}
