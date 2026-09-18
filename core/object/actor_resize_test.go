package object

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

const g = 1024 * 1024 * 1024

// fakeLink is one link of a chain, saying what it holds, what it needs from
// the link below, and what it cannot do.
type fakeLink struct {
	resource.T
	rid string
	has int64

	// needBelow, when set, is what this link asks of the link below for every
	// byte it is asked for: a raid6 of n devices asks each of them for
	// to/(n-2), which is needBelow = n-2.
	divideBy int64
}

func (t *fakeLink) RID() string           { return t.rid }
func (t *fakeLink) Manifest() *manifest.T { return nil }

func (t *fakeLink) CurrentSize(_ context.Context) (int64, error) { return t.has, nil }

func (t *fakeLink) ResizePlan(_ context.Context, to int64) (int64, error) {
	if t.divideBy > 1 {
		return to / t.divideBy, nil
	}
	return to, nil
}

func (t *fakeLink) Resize(_ context.Context, _ int64) error { return nil }

// sizerOnly reports a size and cannot change it, like a volume group.
type sizerOnly struct {
	resource.T
	rid string
	has int64
}

func (t *sizerOnly) RID() string                                  { return t.rid }
func (t *sizerOnly) CurrentSize(_ context.Context) (int64, error) { return t.has, nil }
func (t *sizerOnly) Manifest() *manifest.T                        { return nil }

// links wraps fake resources into a chain of one object, one resource per
// level, which is what a chain that neither crosses into another object nor
// fans out is.
func links(l ...resource.Driver) []resizeLevel {
	chain := make([]resizeLevel, len(l))
	for i, r := range l {
		chain[i] = resizeLevel{{r: r}}
	}
	return chain
}

// fanOut is a level resting on several resources at once, the way an array
// rests on each of its members.
func fanOut(head resource.Driver, members ...resource.Driver) []resizeLevel {
	level := make(resizeLevel, len(members))
	for i, r := range members {
		level[i] = resizeLink{r: r}
	}
	return []resizeLevel{{{r: head}}, level}
}

func rids(plan ResizePlan) []string {
	l := make([]string, len(plan.Steps))
	for i, step := range plan.Steps {
		l[i] = step.RID
	}
	return l
}

func plan(t *testing.T, chain []resource.Driver, size string) ResizePlan {
	t.Helper()
	change, err := sizeconv.ParseChange(size)
	require.NoError(t, err)
	p, err := buildResizePlan(context.Background(), links(chain...), change, naming.Path{}, ResizeOptions{})
	require.NoError(t, err)
	return p
}

// A chain grows from the bottom up: the space has to exist before anything is
// stretched onto it.
func TestAGrowIsAppliedFromTheBottomUp(t *testing.T) {
	chain := []resource.Driver{
		&fakeLink{rid: "fs#1", has: 10 * g},
		&fakeLink{rid: "disk#1", has: 10 * g},
	}
	p := plan(t, chain, "+1g")
	assert.Equal(t, []string{"disk#1", "fs#1"}, rids(p))
	for _, step := range p.Steps {
		assert.Equal(t, int64(11*g), step.To)
	}
}

// A resize only grows, so asking for less is nothing to do rather than a
// chain unwound from the top down.
func TestAskingForLessIsNothingToDo(t *testing.T) {
	chain := []resource.Driver{
		&fakeLink{rid: "fs#1", has: 10 * g},
		&fakeLink{rid: "disk#1", has: 10 * g},
	}
	p := plan(t, chain, "-1g")
	assert.Empty(t, p.Steps)
	assert.False(t, p.HasWork())
}

// A link asks the link below for the size it needs, which is not always the
// size it was asked for: a raid6 holding n devices needs to/(n-2) from each.
func TestALinkTranslatesTheSizeItAsksBelow(t *testing.T) {
	chain := []resource.Driver{
		&fakeLink{rid: "fs#1", has: 10 * g},
		&fakeLink{rid: "disk#md", has: 10 * g, divideBy: 4}, // raid6 of 6
		&fakeLink{rid: "disk#member", has: 2500 * 1024 * 1024},
	}
	p := plan(t, chain, "12g")
	assert.Equal(t, []string{"disk#member", "disk#md", "fs#1"}, rids(p))

	byRID := make(map[string]ResizeStep)
	for _, step := range p.Steps {
		byRID[step.RID] = step
	}
	assert.Equal(t, int64(12*g), byRID["fs#1"].To)
	assert.Equal(t, int64(12*g), byRID["disk#md"].To)
	assert.Equal(t, int64(3*g), byRID["disk#member"].To, "12g over a raid6 of 6 is 3g a member")
	assert.Contains(t, byRID["disk#md"].Comment, "asks 3gi of the link below")
}

// A chain holding one link that cannot be resized is refused whole, and the
// error names both the link that cannot and the resource the size was asked
// of, so nothing is left half resized and the answer is about what was asked.
func TestAChainWithALinkThatCannotResizeIsRefused(t *testing.T) {
	chain := []resource.Driver{
		&fakeLink{rid: "fs#1", has: 10 * g},
		&fakeLink{rid: "disk#1", has: 10 * g},
		&sizerOnly{rid: "disk#vg", has: 100 * g},
	}
	change, err := sizeconv.ParseChange("+1g")
	require.NoError(t, err)
	_, err = buildResizePlan(context.Background(), links(chain...), change, naming.Path{}, ResizeOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk#vg")
	assert.Contains(t, err.Error(), "cannot resize")
	assert.Contains(t, err.Error(), "fs#1")
}

// A size that leaves nothing is refused rather than applied.
func TestAResizeToNothingIsRefused(t *testing.T) {
	chain := []resource.Driver{&fakeLink{rid: "fs#1", has: 10 * g}}
	change, err := sizeconv.ParseChange("-10g")
	require.NoError(t, err)
	_, err = buildResizePlan(context.Background(), links(chain...), change, naming.Path{}, ResizeOptions{})
	assert.ErrorContains(t, err, "leaves nothing")
}

// A resize keeps the configuration describing the object, so the size it
// reached is written back to the keyword that asked for it. Two values say
// more than a number does and are left alone.
func TestWhichConfiguredSizesAResizeMayRewrite(t *testing.T) {
	cases := map[string]bool{
		"5g":             true,
		"5368709120":     true,
		"100%FREE":       false, // a policy the resize satisfies
		"10%":            false,
		"{DEFAULT.size}": false, // a reference to the keyword that owns it
		"{size}":         false,
		"":               false, // nothing was recorded, so nothing to keep accurate
	}
	for was, want := range cases {
		assert.Equalf(t, want, isRecordableSize(was), "isRecordableSize(%q)", was)
	}
}

// An array rests on each of its members at once. They are all asked for the
// same size, and all grown before the array that rests on them.
func TestAFanOutGrowsEveryMemberBeforeWhatRestsOnThem(t *testing.T) {
	// a raid6 of 4: the array hands out what 2 of them hold together
	chain := fanOut(
		&fakeLink{rid: "disk#5", has: 20 * g, divideBy: 2},
		&fakeLink{rid: "disk#1", has: 10 * g},
		&fakeLink{rid: "disk#2", has: 10 * g},
		&fakeLink{rid: "disk#3", has: 10 * g},
		&fakeLink{rid: "disk#4", has: 10 * g},
	)
	change, err := sizeconv.ParseChange("+4g")
	require.NoError(t, err)
	p, err := buildResizePlan(context.Background(), chain, change, naming.Path{}, ResizeOptions{})
	require.NoError(t, err)

	// every member first, the array last
	assert.Equal(t, []string{"disk#1", "disk#2", "disk#3", "disk#4", "disk#5"}, rids(p))

	// each member is asked for half of what the array was asked for
	for _, step := range p.Steps {
		if step.RID == "disk#5" {
			assert.Equal(t, int64(24*g), step.To)
		} else {
			assert.Equalf(t, int64(12*g), step.To, "%s", step.RID)
		}
	}
}

// A level is asked for what its most demanding parent needs, so no member is
// left short.
func TestAFanOutAsksForTheMostDemandingNeed(t *testing.T) {
	chain := []resizeLevel{
		{{r: &fakeLink{rid: "fs#1", has: 10 * g}}},
		{
			{r: &fakeLink{rid: "disk#1", has: 10 * g, divideBy: 2}},
			{r: &fakeLink{rid: "disk#2", has: 10 * g}},
		},
		{{r: &fakeLink{rid: "disk#9", has: 10 * g}}},
	}
	change, err := sizeconv.ParseChange("+2g")
	require.NoError(t, err)
	p, err := buildResizePlan(context.Background(), chain, change, naming.Path{}, ResizeOptions{})
	require.NoError(t, err)

	// disk#1 asks 6g of the level below, disk#2 asks 12g: the deepest level
	// is asked for 12g, the larger of the two.
	for _, step := range p.Steps {
		if step.RID == "disk#9" {
			assert.Equal(t, int64(12*g), step.To)
		}
	}
}

func TestWhichValuesNameTheKeywordHoldingTheirSize(t *testing.T) {
	for s, expected := range map[string]*key.T{
		// a value that is nothing but a reference names where its value lives
		"{DEFAULT.size}": {Section: "DEFAULT", Option: "size"},
		"{disk#1.size}":  {Section: "disk#1", Option: "size"},
		// everything else says more than that, and is left alone
		"":                  nil,
		"1g":                nil,
		"100%FREE":          nil,
		"{DEFAULT.size}+1g": nil,
		"x{DEFAULT.size}":   nil,
		"{{DEFAULT.size}}":  nil,
		"{nodename}":        nil,
		"{DEFAULT.}":        nil,
		"{.size}":           nil,
	} {
		t.Run(s, func(t *testing.T) {
			k, ok := referencedKey(s)
			if expected == nil {
				assert.False(t, ok)
				return
			}
			assert.True(t, ok)
			assert.Equal(t, *expected, k)
		})
	}
}

// spanningLink is a filesystem: it is grown onto the device under it rather
// than to a size of its own, and it reports that device as its size.
type spanningLink struct {
	fakeLink
}

func (t *spanningLink) ResizeSpansBelow() bool { return true }

func TestAFilesystemIsGrownWheneverSomethingBelowIt(t *testing.T) {
	// The device under it already holds the size asked for, because a chain
	// grows from the bottom up, so comparing sizes says there is nothing to
	// do of a filesystem that has not been grown at all.
	head := &spanningLink{fakeLink{rid: "fs#1", has: 10 * 1024 * 1024}}
	below := &fakeLink{rid: "disk#1", has: 5 * 1024 * 1024}
	p := plan(t, []resource.Driver{head, below}, "10m")

	require.Equal(t, []string{"disk#1", "fs#1"}, rids(p))
	assert.False(t, p.Steps[0].Skip, "the device grows")
	assert.False(t, p.Steps[1].Skip, "the filesystem is grown onto it")
}

func TestAFilesystemIsGrownOntoADeviceGrownByHand(t *testing.T) {
	// Nothing below has anything to do, and the filesystem reports the device
	// it is counted as, so no size says it is short of it. Asking it to take
	// up its device is what repairs that, and changes nothing when it already
	// does.
	head := &spanningLink{fakeLink{rid: "fs#1", has: 10 * 1024 * 1024}}
	below := &fakeLink{rid: "disk#1", has: 10 * 1024 * 1024}
	p := plan(t, []resource.Driver{head, below}, "10m")

	require.Equal(t, []string{"disk#1", "fs#1"}, rids(p))
	assert.True(t, p.Steps[0].Skip, "the device holds what is asked of it")
	assert.False(t, p.Steps[1].Skip, "the filesystem is asked to span it")
}

func TestALinkThatDoesNotSpanBelowIsStillSkipped(t *testing.T) {
	head := &fakeLink{rid: "disk#2", has: 10 * 1024 * 1024}
	below := &fakeLink{rid: "disk#1", has: 10 * 1024 * 1024}
	p := plan(t, []resource.Driver{head, below}, "10m")

	assert.True(t, p.Steps[0].Skip)
	assert.True(t, p.Steps[1].Skip)
	assert.False(t, p.HasWork())
}

// A resource short of its configured size by less than the compact rendering
// resolves printed as holding what it is configured to hold, which reads as a
// warning about nothing. The drbd of a volume keeps its metadata out of what
// it hands up, and is short by that much for ever.
func TestTheSizesAWarningCompares(t *testing.T) {
	for _, tc := range []struct {
		name                string
		current, configured int64
		held, target        string
	}{
		{"far apart", 96 * 1024 * 1024, 100 * 1024 * 1024, "96mi", "100mi"},
		{"a drbd short by its metadata", 366907392, 367001600, "366907392 bytes", "367001600 bytes"},
		{"a loop short by a sector", 34952192, 34952533, "34952192 bytes", "34952533 bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			held, target := resizeSizePair(tc.current, tc.configured)
			assert.Equal(t, tc.held, held)
			assert.Equal(t, tc.target, target)
			assert.NotEqual(t, held, target, "a warning naming the same size twice explains nothing")
		})
	}
}

// A chain crossing into several objects at once is one chain. An array over
// two volumes rests on both, and what each volume exposes is grown at the same
// depth as what the other does, so the levels of the two line up rather than
// following one another.
func TestMergeResizeLevels(t *testing.T) {
	p1 := naming.Path{Name: "v1-vol-1", Kind: naming.KindVol}
	p2 := naming.Path{Name: "v1-vol-2", Kind: naming.KindVol}
	link := func(p naming.Path, rid string) resizeLink {
		return resizeLink{path: p, r: &fakeLink{rid: rid}}
	}
	names := func(levels []resizeLevel) []string {
		l := make([]string, 0, len(levels))
		for _, level := range levels {
			s := ""
			for _, link := range level {
				if s != "" {
					s += " "
				}
				s += link.path.String() + ":" + link.r.RID()
			}
			l = append(l, s)
		}
		return l
	}

	a := []resizeLevel{{link(p1, "fs#1")}, {link(p1, "disk#0")}}
	b := []resizeLevel{{link(p2, "fs#1")}, {link(p2, "disk#0")}}
	assert.Equal(t, []string{
		"vol/v1-vol-1:fs#1 vol/v1-vol-2:fs#1",
		"vol/v1-vol-1:disk#0 vol/v1-vol-2:disk#0",
	}, names(mergeResizeLevels(a, b)), "the same depth of each is one level")

	// One chain deeper than the other keeps its depth: the shorter one ran
	// out of links, not the longer one of levels.
	c := []resizeLevel{{link(p2, "fs#1")}}
	assert.Equal(t, []string{
		"vol/v1-vol-1:fs#1 vol/v1-vol-2:fs#1",
		"vol/v1-vol-1:disk#0",
	}, names(mergeResizeLevels(a, c)))

	// Merging onto nothing is the chain itself, which is what the first
	// object entered merges onto.
	assert.Equal(t, names(a), names(mergeResizeLevels(nil, a)))
}
