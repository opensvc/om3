package schedule

import (
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/render/tree"
	"opensvc.com/opensvc/util/timestamp"
)

func SprintTime(t timestamp.T) string {
	if t.IsZero() {
		return "-"
	}
	return t.Render()
}

func (t Table) Render() string {
	tree := tree.New()
	tree.AddColumn().AddText("Node").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Object").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Action").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Last").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Next").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Keyword").SetColor(rawconfig.Node.Color.Bold)
	tree.AddColumn().AddText("Schedule").SetColor(rawconfig.Node.Color.Bold)
	for _, e := range t {
		n := tree.AddNode()
		n.AddColumn().AddText(e.Node).SetColor(rawconfig.Node.Color.Primary)
		n.AddColumn().AddText(e.Path.String()).SetColor(rawconfig.Node.Color.Primary)
		n.AddColumn().AddText(e.Action).SetColor(rawconfig.Node.Color.Primary)
		n.AddColumn().AddText(SprintTime(e.Last))
		n.AddColumn().AddText(SprintTime(e.Next))
		n.AddColumn().AddText(e.Key)
		n.AddColumn().AddText(e.Definition)
	}
	return tree.Render()
}
