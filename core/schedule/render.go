package schedule

import (
	"time"

	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/render/tree"
)

func SprintTime(t time.Time) string {
	if t == time.Unix(0, 0) || t.IsZero() {
		return "-"
	}
	layout := "2006-01-02 15:04:05 Z07:00"
	return t.Local().Format(layout)
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
