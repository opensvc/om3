package tree

import (
	"fmt"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func TestForest(t *testing.T) {
	widthToExpected := map[int]string{
		// Only 13 columns are left for the label column, less than
		// minWrapWidth, so it wraps at minWrapWidth and overflows.
		35: "svc1        \n" +
			"└ avail           up  \n" +
			"  └ res#id  ....  up  label               \n" +
			"                      warn: some long warn\n" +
			"                      ing description     \n" +
			"                      err                 \n",
		55: "svc1        \n" +
			"└ avail           up  \n" +
			"  └ res#id  ....  up  label                                \n" +
			"                      warn: some long warning description  \n" +
			"                      err                                  \n",
	}
	for width, expected := range widthToExpected {

		tree := New()
		tree.ForcedWidth = width
		tree.AddColumn().AddText("svc1").SetColor(color.Bold)
		node := tree.AddNode()
		node.AddColumn().AddText("avail")
		node.AddColumn()
		node.AddColumn().AddText("up").SetColor(color.FgGreen)
		node = node.AddNode()
		node.AddColumn().AddText("res#id")
		node.AddColumn().AddText("....")
		node.AddColumn().AddText("up").SetColor(color.FgGreen)
		col := node.AddColumn()
		col.AddText("label")
		col.AddText("warn: some long warning description").SetColor(color.FgYellow).SetAlign(AlignLeft)
		col.AddText("err").SetColor(color.FgRed).SetAlign(AlignLeft)
		s := tree.Render()
		fmt.Println(s)
		t.Log("programmatic tree")
		t.Log(s)
		assert.Equal(t, expected, s)
	}
}

// TestForestNarrowWidth verifies Render doesn't panic when the terminal is
// too narrow for the tree prefix and the non-wrappable columns, leaving no
// room for the oversized columns.
func TestForestNarrowWidth(t *testing.T) {
	for width := 1; width <= 120; width++ {
		for depth := 1; depth <= 4; depth++ {
			tree := New()
			tree.ForcedWidth = width
			tree.AddColumn().AddText("svc1")
			node := tree.AddNode()
			for i := 0; i < depth; i++ {
				node.AddColumn().AddText("a_rather_long_resource_id#0")
				node.AddColumn().AddText("....")
				node.AddColumn().AddText("up")
				node.AddColumn().AddText("a long label that will need wrapping")
				node = node.AddNode()
			}
			assert.NotPanics(t, func() { _ = tree.Render() }, "width=%d depth=%d", width, depth)
		}
	}
}

// TestForestNoTerminal verifies Render doesn't wrap when stdout is not a
// terminal and COLUMNS is not set, and wraps to COLUMNS when set.
func TestForestNoTerminal(t *testing.T) {
	label := "a long label that would need wrapping on a narrow terminal"
	newTree := func() *Tree {
		tree := New()
		tree.AddColumn().AddText("svc1")
		node := tree.AddNode()
		node.AddColumn().AddText("a_rather_long_resource_id#0")
		node.AddColumn().AddText("....")
		node.AddColumn().AddText("up")
		node.AddColumn().AddText(label)
		return tree
	}

	t.Setenv("COLUMNS", "")
	assert.Contains(t, newTree().Render(), label, "unexpected wrapping")

	t.Setenv("COLUMNS", "40")
	assert.NotContains(t, newTree().Render(), label, "expected wrapping")
}
