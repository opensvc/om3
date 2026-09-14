package object

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
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

	// refuseShrink is the xfs case: it can grow and never shrink.
	refuseShrink bool
}

func (t *fakeLink) RID() string           { return t.rid }
func (t *fakeLink) Manifest() *manifest.T { return nil }

func (t *fakeLink) CurrentSize(_ context.Context) (int64, error) { return t.has, nil }

func (t *fakeLink) ResizePlan(_ context.Context, to int64) (int64, error) {
	if t.refuseShrink && to < t.has {
		return 0, fmt.Errorf("a %s cannot shrink", t.rid)
	}
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

// links wraps fake resources into a chain of one object, which is what a
// chain that does not cross into another object is.
func links(l ...resource.Driver) []resizeLink {
	chain := make([]resizeLink, len(l))
	for i, r := range l {
		chain[i] = resizeLink{r: r}
	}
	return chain
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
	p, err := buildResizePlan(context.Background(), links(chain...), change, naming.Path{})
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
	assert.False(t, p.IsShrink)
	assert.Equal(t, []string{"disk#1", "fs#1"}, rids(p))
	for _, step := range p.Steps {
		assert.Equal(t, int64(11*g), step.To)
	}
}

// A chain shrinks from the top down: a filesystem gives the space back before
// the device under it is taken away, or what is mounted is larger than what
// holds it.
func TestAShrinkIsAppliedFromTheTopDown(t *testing.T) {
	chain := []resource.Driver{
		&fakeLink{rid: "fs#1", has: 10 * g},
		&fakeLink{rid: "disk#1", has: 10 * g},
	}
	p := plan(t, chain, "-1g")
	assert.True(t, p.IsShrink)
	assert.Equal(t, []string{"fs#1", "disk#1"}, rids(p))
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
	_, err = buildResizePlan(context.Background(), links(chain...), change, naming.Path{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk#vg")
	assert.Contains(t, err.Error(), "cannot resize")
	assert.Contains(t, err.Error(), "fs#1")
}

// A filesystem that cannot shrink says so while planning, before the device
// under it has moved. Finding it out afterwards is data loss.
func TestAShrinkIsRefusedBeforeAnythingMoves(t *testing.T) {
	chain := []resource.Driver{
		&fakeLink{rid: "fs#1", has: 10 * g, refuseShrink: true},
		&fakeLink{rid: "disk#1", has: 10 * g},
	}
	change, err := sizeconv.ParseChange("-1g")
	require.NoError(t, err)
	_, err = buildResizePlan(context.Background(), links(chain...), change, naming.Path{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fs#1")
	assert.Contains(t, err.Error(), "cannot shrink")
}

// A size that leaves nothing is refused rather than applied.
func TestAResizeToNothingIsRefused(t *testing.T) {
	chain := []resource.Driver{&fakeLink{rid: "fs#1", has: 10 * g}}
	change, err := sizeconv.ParseChange("-10g")
	require.NoError(t, err)
	_, err = buildResizePlan(context.Background(), links(chain...), change, naming.Path{})
	assert.ErrorContains(t, err, "leaves nothing")
}
