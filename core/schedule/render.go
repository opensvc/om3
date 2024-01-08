package schedule

import (
	"time"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/render/tree"
)

func SprintTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

func (t Table) Render() string {
	tree := tree.New()
	tree.AddColumn().AddText("Node").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Object").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Action").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Last").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Next").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Keyword").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("Schedule").SetColor(rawconfig.Color.Bold)
	for _, e := range t {
		n := tree.AddNode()
		n.AddColumn().AddText(e.Node).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(e.Path.String()).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(e.Action).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(SprintTime(e.LastRunAt))
		n.AddColumn().AddText(SprintTime(e.NextRunAt))
		n.AddColumn().AddText(e.Key)
		n.AddColumn().AddText(e.Schedule)
	}
	return tree.Render()
}
