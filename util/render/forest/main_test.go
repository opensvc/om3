package forest

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func TestForest(t *testing.T) {
	ref := "svc1          \n" +
		"`- avail            up  \n" +
		"   `- res#id  ....  up  label  \n" +
		"                        warn   \n" +
		"                        err    \n"
	tree := New()
	tree.head.AddColumn().AddText("svc1").SetColor(color.Bold)
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
	col.AddText("warn").SetColor(color.FgYellow).SetAlign(AlignLeft)
	col.AddText("err").SetColor(color.FgRed).SetAlign(AlignLeft)
	s := tree.Render()
	//fmt.Println(s)
	t.Log("programatic forest")
	t.Log(s)
	assert.Equal(t, ref, s)
}
