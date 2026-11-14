package object

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/funcopt"
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

func (t *fakeLink) RID() string                                        { return t.rid }
func (t *fakeLink) Manifest() *manifest.T                              { return nil }
func (t *fakeLink) DriverID() driver.ID                                { return driver.NewID(driver.GroupDisk, "fake") }
func (t *fakeLink) Label(context.Context) string                       { return t.rid }
func (t *fakeLink) Status(context.Context) status.T                    { return status.Up }
func (t *fakeLink) Provisioned(context.Context) (provisioned.T, error) { return provisioned.True, nil }

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

func (t *sizerOnly) RID() string                     { return t.rid }
func (t *sizerOnly) DriverID() driver.ID             { return driver.NewID(driver.GroupDisk, "sizer") }
func (t *sizerOnly) Label(context.Context) string    { return t.rid }
func (t *sizerOnly) Status(context.Context) status.T { return status.Up }
func (t *sizerOnly) Provisioned(context.Context) (provisioned.T, error) {
	return provisioned.True, nil
}
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
	// A link that has to grow, and cannot, refuses the chain: nothing is
	// grown under a link that would refuse once reached.
	chain := []resource.Driver{
		&fakeLink{rid: "fs#1", has: 10 * g},
		&fakeLink{rid: "disk#1", has: 10 * g},
		&sizerOnly{rid: "disk#vg", has: 10 * g},
	}
	change, err := sizeconv.ParseChange("+1g")
	require.NoError(t, err)
	_, err = buildResizePlan(context.Background(), links(chain...), change, naming.Path{}, ResizeOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk#vg")
	assert.Contains(t, err.Error(), "cannot resize")
	assert.Contains(t, err.Error(), "fs#1")
}

// refusingLink cannot grow, as a raid0 array, and says so when asked.
type refusingLink struct {
	fakeLink
}

func (t *refusingLink) ResizePlan(context.Context, int64) (int64, error) {
	return 0, fmt.Errorf("a raid0 array cannot be grown onto members that grew")
}

// The walk stops at a level holding what is asked of it: a logical volume
// growing into the free space of its volume group asks nothing of the raid0
// under the group, which cannot grow, and the levels below are listed, not
// asked.
func TestTheWalkStopsAtALevelHoldingWhatIsAsked(t *testing.T) {
	chain := []resource.Driver{
		&fakeLink{rid: "fs#1", has: 12 * g},
		&fakeLink{rid: "disk#lv", has: 12 * g},
		&sizerOnly{rid: "disk#vg", has: 16 * g},
		&refusingLink{fakeLink{rid: "disk#md", has: 16 * g}},
	}
	p := plan(t, chain, "+2g")
	require.Len(t, p.Steps, 4)
	assert.Equal(t, []string{"disk#md", "disk#vg", "disk#lv", "fs#1"}, rids(p))
	assert.True(t, p.Steps[0].Skip)
	assert.Contains(t, p.Steps[0].Comment, "not asked")
	assert.True(t, p.Steps[1].Skip)
	assert.Contains(t, p.Steps[1].Comment, "already holds")
	assert.False(t, p.Steps[2].Skip)
	assert.Equal(t, int64(14*g), p.Steps[2].To)

	// Where the level has to grow, the link below it is asked, and refuses.
	chain[2] = &fakeLink{rid: "disk#vg", has: 12 * g}
	chain[3] = &refusingLink{fakeLink{rid: "disk#md", has: 12 * g}}
	change, err := sizeconv.ParseChange("+2g")
	require.NoError(t, err)
	_, err = buildResizePlan(context.Background(), links(chain...), change, naming.Path{}, ResizeOptions{})
	assert.ErrorContains(t, err, "raid0")
}

// A link asking more of a device no resource handles than it holds is
// refused, beyond the tolerance of the formulas rounding up.
func TestADeviceNothingGrowsRefusesANeedItDoesNotHold(t *testing.T) {
	former := deviceSize
	defer func() { deviceSize = former }()
	deviceSize = func(string) (int64, error) { return 16 * 1024 * 1024, nil }
	err := refuseShortDevices(36*1024*1024, []string{"/dev/sda"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "/dev/sda")
	assert.ErrorContains(t, err, "nothing below it can grow")
	assert.NoError(t, refuseShortDevices(16*1024*1024+512*1024, []string{"/dev/sda"}), "within the tolerance")
	assert.NoError(t, refuseShortDevices(36*1024*1024, nil), "no device unmanaged")
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

// recordOf builds a step recording the size a resource reached in the
// configuration of an object.
func recordOf(t *testing.T, kind naming.Kind, config, rid string, reached int64) *xconfig.T {
	t.Helper()
	return recordOfLimited(t, kind, config, rid, reached, 0)
}

// recordOfLimited is recordOf for a resize asked of the bottom of the chain,
// whose size limit is what DEFAULT.size can be recorded as at most.
func recordOfLimited(t *testing.T, kind naming.Kind, config, rid string, reached, limit int64) *xconfig.T {
	t.Helper()
	p, err := naming.NewPath("test", kind, "obj")
	require.NoError(t, err)
	opts := []funcopt.O{WithConfigData([]byte(config)), WithVolatile(true)}
	var a *actor
	switch kind {
	case naming.KindVol:
		o, err := NewVol(p, opts...)
		require.NoError(t, err)
		a = &o.actor
	default:
		o, err := NewSvc(p, opts...)
		require.NoError(t, err)
		a = &o.actor
	}
	step := ResizeStep{
		RID:   rid,
		r:     &fakeLink{rid: rid, has: reached},
		owner: a,
	}
	require.NoError(t, step.recordSize(context.Background(), limit))
	return a.config
}

// A pool sizes the volume it serves from the claim the namespace holds on it,
// and the cluster counts that claim from the size of the object. A
// configuration written before pools pointed the one at the other holds the
// same number twice, and recording the resource alone would leave the cluster
// counting a claim the storage has outgrown.
func TestASizeRecordedCarriesTheClaimItWasSizedFrom(t *testing.T) {
	const volConfig = `
pool = p1
size = 512mi

[disk#1]
type = loop
size = 512mi

[disk#3]
type = lv
size = 100mi
`
	cfg := recordOf(t, naming.KindVol, volConfig, "disk#1", 612*1024*1024)
	assert.Equal(t, "612mi", cfg.Get(key.T{Section: "disk#1", Option: "size"}))
	assert.Equal(t, "612mi", cfg.Get(key.T{Section: "DEFAULT", Option: "size"}),
		"the claim the volume was sized from follows the storage")

	cfg = recordOf(t, naming.KindVol, volConfig, "disk#3", 200*1024*1024)
	assert.Equal(t, "200mi", cfg.Get(key.T{Section: "disk#3", Option: "size"}))
	assert.Equal(t, "512mi", cfg.Get(key.T{Section: "DEFAULT", Option: "size"}),
		"a resource sized from something else does not carry the claim")

	// A volume served by no pool claims nothing of one.
	cfg = recordOf(t, naming.KindVol, `
size = 512mi

[disk#1]
type = loop
size = 512mi
`, "disk#1", 612*1024*1024)
	assert.Equal(t, "612mi", cfg.Get(key.T{Section: "disk#1", Option: "size"}))
	assert.Equal(t, "512mi", cfg.Get(key.T{Section: "DEFAULT", Option: "size"}))

	// The size of a service is not a claim on a pool.
	cfg = recordOf(t, naming.KindSvc, `
pool = p1
size = 512mi

[disk#1]
type = loop
size = 512mi
`, "disk#1", 612*1024*1024)
	assert.Equal(t, "612mi", cfg.Get(key.T{Section: "disk#1", Option: "size"}))
	assert.Equal(t, "512mi", cfg.Get(key.T{Section: "DEFAULT", Option: "size"}))
}

// A pool writing the size of its resource as a reference to the size of the
// volume is recorded where the value lives, and once.
func TestASizeRecordedThroughAReferenceLandsWhereItLives(t *testing.T) {
	cfg := recordOf(t, naming.KindVol, `
pool = p1
size = 512mi

[disk#1]
type = loop
size = {DEFAULT.size}
`, "disk#1", 612*1024*1024)
	assert.Equal(t, "{DEFAULT.size}", cfg.Get(key.T{Section: "disk#1", Option: "size"}),
		"the reference is kept, so a clone still sizes itself from the volume")
	assert.Equal(t, "612mi", cfg.Get(key.T{Section: "DEFAULT", Option: "size"}))
}

const mi = int64(1024 * 1024)

// keepingLink keeps a cut of what it rests on for its metadata, and hands out
// what is left in whole units, the way a volume group keeps its metadata and
// hands out whole extents, and a drbd resource keeps room for its bitmap.
type keepingLink struct {
	fakeLink
	keep, unit int64
	spans      bool
}

func (t *keepingLink) ResizePlan(_ context.Context, to int64) (int64, error) {
	return sizeconv.RoundUp(to, t.unit) + t.keep, nil
}

func (t *keepingLink) ResizeHandsOut(_ context.Context) (func(int64) int64, error) {
	return func(below int64) int64 {
		n := sizeconv.RoundDown(below-t.keep, t.unit)
		if n < 0 {
			return 0
		}
		return n
	}, nil
}

func (t *keepingLink) ResizeSpansBelow() bool { return t.spans }

// drbdPoolChain is the chain of a volume of a drbd pool of loop files, with
// the bottom holding loop: a filesystem on a drbd resource keeping 4m, on a
// logical volume taking its whole group, on a volume group keeping 4m of its
// loop file and handing out 4m extents.
func drbdPoolChain(loop int64) []resource.Driver {
	vg := sizeconv.RoundDown(loop-4*mi, 4*mi)
	return []resource.Driver{
		&spanningLink{fakeLink{rid: "fs#1", has: vg - 4*mi}},
		&keepingLink{fakeLink: fakeLink{rid: "disk#4", has: vg - 4*mi}, keep: 4 * mi, spans: true},
		&fakeLink{rid: "disk#3", has: vg},
		&keepingLink{fakeLink: fakeLink{rid: "disk#2", has: vg}, keep: 4 * mi, unit: 4 * mi, spans: true},
		&fakeLink{rid: "disk#1", has: loop},
	}
}

func bottomPlan(t *testing.T, chain []resource.Driver, size string) ResizePlan {
	t.Helper()
	change, err := sizeconv.ParseChange(size)
	require.NoError(t, err)
	p, err := buildResizePlan(context.Background(), links(chain...), change, naming.Path{}, ResizeOptions{Bottom: true})
	require.NoError(t, err)
	return p
}

func stepOf(t *testing.T, p ResizePlan, rid string) ResizeStep {
	t.Helper()
	for _, step := range p.Steps {
		if step.RID == rid {
			return step
		}
	}
	require.Failf(t, "no step", "%s is not in the plan", rid)
	return ResizeStep{}
}

// The size of the head is worked out from the size of the bottom, up the
// chain, from what each link hands out of what is below it. A link that
// cannot say, a filesystem or a logical volume of these fakes, hands out what
// is below it.
func TestTheHeadSizeIsWorkedOutFromTheBottom(t *testing.T) {
	chain := links(drbdPoolChain(128 * mi)...)
	head, err := headSizeFromBottom(context.Background(), chain, 172*mi, naming.Path{})
	require.NoError(t, err)
	assert.Equal(t, 164*mi, head, "172m less 4m of volume group, less 4m of drbd")

	head, err = headSizeFromBottom(context.Background(), chain, 170*mi, naming.Path{})
	require.NoError(t, err)
	assert.Equal(t, 160*mi, head, "a volume group hands out whole extents")

	head, err = headSizeFromBottom(context.Background(), links(&fakeLink{rid: "fs#1"}), 172*mi, naming.Path{})
	require.NoError(t, err)
	assert.Equal(t, 172*mi, head, "a chain of one link is its own bottom")

	// A level is grown as one, so it hands up the least of what its links
	// hand out.
	fan := []resizeLevel{
		{{r: &fakeLink{rid: "fs#1"}}},
		{{r: &keepingLink{fakeLink: fakeLink{rid: "disk#a"}, keep: 2 * mi}}, {r: &keepingLink{fakeLink: fakeLink{rid: "disk#b"}, keep: 4 * mi}}},
		{{r: &fakeLink{rid: "disk#1"}}},
	}
	head, err = headSizeFromBottom(context.Background(), fan, 100*mi, naming.Path{})
	require.NoError(t, err)
	assert.Equal(t, 96*mi, head)
}

// A volume asked for the size it is configured to hold, right after it was
// provisioned to it, has nothing to do: the configuration records the bottom
// of the chain, which the head never holds.
func TestAskingAVolumeForTheSizeItHoldsIsNothingToDo(t *testing.T) {
	p := bottomPlan(t, drbdPoolChain(128*mi), "128m")
	assert.False(t, p.HasWork(), "%s", p)
	assert.Equal(t, "nothing to do: every link already holds the size asked of it", p.String())
	assert.Len(t, p.Steps, 5, "the whole chain is listed")
	assert.Equal(t, 128*mi, p.Bottom)

	p = bottomPlan(t, drbdPoolChain(128*mi), "-1m")
	assert.False(t, p.HasWork(), "a resize only grows")

	// The same chain asked the same size of its head grows, every time,
	// which is what the configuration read as the size of the head did.
	assert.True(t, plan(t, drbdPoolChain(128*mi), "128m").HasWork())
}

// A volume grown to a size asks the bottom of its chain for that size, and
// once grown, asking for it again is nothing to do.
func TestAVolumeGrownToASizeHoldsItAtTheBottom(t *testing.T) {
	p := bottomPlan(t, drbdPoolChain(128*mi), "172m")
	require.True(t, p.HasWork())
	assert.Equal(t, []string{"disk#1", "disk#2", "disk#3", "disk#4", "fs#1"}, rids(p))
	assert.Equal(t, 172*mi, stepOf(t, p, "disk#1").To, "the bottom is asked the size given")
	assert.Equal(t, 164*mi, stepOf(t, p, "fs#1").To, "the head what the bottom hands up")
	assert.Equal(t, 172*mi, p.Bottom)
	assert.Contains(t, p.String(), "for the bottom of the chain to hold 172mi")

	// An amount to add is added to what the bottom holds.
	p = bottomPlan(t, drbdPoolChain(128*mi), "+44m")
	assert.Equal(t, 172*mi, stepOf(t, p, "disk#1").To)

	// Grown, the chain holds what it was asked, and asking again is
	// nothing to do.
	p = bottomPlan(t, drbdPoolChain(172*mi), "172m")
	assert.False(t, p.HasWork(), "%s", p)
}

// A bottom asked a size no level above can use the whole of, a volume group
// handing out whole extents, is asked no more than the head needs: the bottom
// lands under the size given, never over it beyond a rounding.
func TestTheBottomIsAskedNoMoreThanTheSizeGiven(t *testing.T) {
	for _, size := range []int64{129 * mi, 130 * mi, 172 * mi, 173*mi + 512, 1 << 34} {
		change := sizeconv.Change{Value: size}
		p, err := buildResizePlan(context.Background(), links(drbdPoolChain(128*mi)...), change, naming.Path{}, ResizeOptions{Bottom: true})
		require.NoError(t, err)
		if !p.HasWork() {
			continue
		}
		bottom := stepOf(t, p, "disk#1")
		if !bottom.Skip {
			assert.LessOrEqual(t, bottom.To, size, "asked %d", size)
		}
	}
}

// A size recorded in DEFAULT.size by a resize asked of the bottom of a chain
// is never more than that size, which is what the claim was checked for: a
// loop file rounded up to whole sectors is not the claim outgrown.
func TestASizeRecordedNeverOutgrowsTheSizeAsked(t *testing.T) {
	const reference = `
pool = p1
size = 1000000

[disk#1]
type = loop
size = {DEFAULT.size}
`
	cfg := recordOfLimited(t, naming.KindVol, reference, "disk#1", 1000448, 1000000)
	assert.Equal(t, "1000000", cfg.Get(key.T{Section: "DEFAULT", Option: "size"}))

	cfg = recordOfLimited(t, naming.KindVol, reference, "disk#1", 999424, 1000000)
	assert.Equal(t, int64(999424), *cfg.GetSize(key.T{Section: "DEFAULT", Option: "size"}), "less than asked is what is there")

	cfg = recordOf(t, naming.KindVol, reference, "disk#1", 1000448)
	assert.Equal(t, int64(1000448), *cfg.GetSize(key.T{Section: "DEFAULT", Option: "size"}), "a resize of a resource records what it reached")

	// A configuration holding the claim twice records the two together.
	cfg = recordOfLimited(t, naming.KindVol, `
pool = p1
size = 512mi

[disk#1]
type = loop
size = 512mi
`, "disk#1", 612*mi+512, 612*mi)
	assert.Equal(t, "612mi", cfg.Get(key.T{Section: "disk#1", Option: "size"}))
	assert.Equal(t, "612mi", cfg.Get(key.T{Section: "DEFAULT", Option: "size"}))

	// A resource not carrying the claim records what it reached.
	cfg = recordOfLimited(t, naming.KindVol, `
pool = p1
size = 512mi

[disk#3]
type = lv
size = 100mi
`, "disk#3", 200*mi, 150*mi)
	assert.Equal(t, "200mi", cfg.Get(key.T{Section: "disk#3", Option: "size"}))
}

// A chain crossing a replicated resource is refused when every stage would run
// on one node, unless there is nothing to run: asking a drbd volume for the
// size it holds strands nothing.
func TestALoneReplicaResizeWithNothingToDoIsNotRefused(t *testing.T) {
	home := naming.Path{Namespace: "test", Kind: naming.KindVol, Name: "data"}
	idle := ResizePlan{Stages: 2, Steps: []ResizeStep{{RID: "fs#1", Skip: true}}}
	assert.NoError(t, refuseLoneReplicaResize(idle, home, ResizeOptions{}))
	busy := ResizePlan{Stages: 2, Steps: []ResizeStep{{RID: "fs#1"}}}
	assert.Error(t, refuseLoneReplicaResize(busy, home, ResizeOptions{}))
	assert.NoError(t, refuseLoneReplicaResize(busy, home, ResizeOptions{Force: true}))
}

// A size above the configured one that grows nothing is refused, naming the
// smallest size that does: the volume group hands out whole extents of 4m, so
// a bottom of 130m hands up what a bottom of 128m does.
func TestASizeThatGrowsNothingIsRefused(t *testing.T) {
	ask := func(size string, configured int64) (ResizePlan, error) {
		change, err := sizeconv.ParseChange(size)
		require.NoError(t, err)
		return buildResizePlan(context.Background(), links(drbdPoolChain(128*mi)...), change, naming.Path{}, ResizeOptions{Bottom: true, Configured: configured})
	}
	_, err := ask("130m", 128*mi)
	var idle *ResizeGrowsNothingError
	require.ErrorAs(t, err, &idle)
	assert.Equal(t, 130*mi, idle.Asked)
	assert.Equal(t, 120*mi, idle.Head)
	assert.Equal(t, 132*mi, idle.Smallest, "the bottom size giving the group one more extent")
	assert.ErrorContains(t, err, "The smallest size growing it is 132mi")

	p, err := ask("132m", 128*mi)
	require.NoError(t, err)
	assert.True(t, p.HasWork(), "the smallest size named grows the volume")

	p, err = ask("128m", 128*mi)
	require.NoError(t, err)
	assert.False(t, p.HasWork(), "the configured size is nothing to do, not refused")

	// With no configured size to compare to, nothing is refused, and the
	// bottom is asked the size given, the head holding what it hands up.
	p, err = ask("130m", 0)
	require.NoError(t, err)
	assert.True(t, p.HasWork())
	assert.Equal(t, 130*mi, stepOf(t, p, "disk#1").To)
	assert.False(t, stepOf(t, p, "disk#1").Skip)
	assert.True(t, stepOf(t, p, "disk#3").Skip, "the lv above the group is not asked")
}

// The search for the smallest size growing a chain lands on a whole unit of
// the bottom, the first one handing up more than the head holds.
func TestTheSmallestGrowingBottom(t *testing.T) {
	handsUp := func(b int64) int64 { return sizeconv.RoundDown(b-4*mi, 4*mi) - 4*mi }
	smallest, ok := smallestGrowingBottom(handsUp, 130*mi, 120*mi, 512)
	require.True(t, ok)
	assert.Equal(t, 132*mi, smallest)
	assert.LessOrEqual(t, handsUp(smallest-512), int64(120*mi), "nothing smaller grows it")

	smallest, ok = smallestGrowingBottom(handsUp, 130*mi, 120*mi, 4*mi)
	require.True(t, ok)
	assert.Equal(t, 132*mi, smallest, "a whole extent of the bottom")

	smallest, ok = smallestGrowingBottom(handsUp, 129*mi+7, 1000*mi, 3*mi)
	require.True(t, ok)
	assert.Greater(t, handsUp(smallest), int64(1000*mi))
	assert.LessOrEqual(t, handsUp(smallest-3*mi), int64(1000*mi))
	assert.Zero(t, smallest%(3*mi))

	_, ok = smallestGrowingBottom(func(int64) int64 { return 0 }, 130*mi, 120*mi, 512)
	assert.False(t, ok, "a chain no bottom grows")
}

// The bottom of a chain resized for a size is asked that size, not the
// smallest bottom handing the head the size worked out from it: the volume
// group hands out whole extents of 4m less 4m of metadata, so a bottom of
// 162m hands up 156m, which a bottom of 160m hands up too. The bottom used to
// be asked 160m, 2m short of the size the configuration then recorded at the
// size reached, and the size asked could never be reached (card 300).
func TestTheBottomIsAskedTheSizeGiven(t *testing.T) {
	p := bottomPlan(t, drbdPoolChain(128*mi), "162m")
	bottom := stepOf(t, p, "disk#1")
	assert.Equal(t, 162*mi, bottom.To)
	assert.False(t, bottom.Skip)
	assert.Equal(t, 162*mi, p.Bottom)
}

// The bottom is asked the size given even when a level above holds what is
// needed: a bottom a former resize left short of the size configured is
// grown to it, and what spans it grown onto it. A bottom holding the size
// changes nothing, as asking a volume for the size configured never did.
func TestABottomLeftShortIsGrownToTheSizeGiven(t *testing.T) {
	chain := drbdPoolChain(128 * mi)
	chain[len(chain)-1] = &fakeLink{rid: "disk#1", has: 126 * mi}
	ask := func(size string) ResizePlan {
		change, err := sizeconv.ParseChange(size)
		require.NoError(t, err)
		p, err := buildResizePlan(context.Background(), links(chain...), change, naming.Path{}, ResizeOptions{Bottom: true, Configured: 128 * mi})
		require.NoError(t, err)
		return p
	}
	p := ask("128m")
	bottom := stepOf(t, p, "disk#1")
	assert.False(t, bottom.Skip, "the bottom left short is grown")
	assert.Equal(t, 128*mi, bottom.To)
	assert.False(t, stepOf(t, p, "disk#2").Skip, "the volume group grown onto it")

	chain[len(chain)-1] = &fakeLink{rid: "disk#1", has: 128 * mi}
	assert.False(t, ask("128m").HasWork(), "a bottom holding the size changes nothing")
}
