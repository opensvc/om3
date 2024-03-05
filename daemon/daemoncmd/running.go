package daemoncmd

type (
	PingItems []PingItem

	PingItem struct {
		Data Ping     `json:"data"`
		Meta NodeMeta `json:"meta"`
	}

	NodeMeta struct {
		Node string `json:"node"`
	}

	Ping struct {
		Ping   bool   `json:"ping"`
		Detail string `json:"detail"`
	}
)

func (t PingItem) Unstructured() map[string]any {
	return map[string]any{
		"meta": t.Meta.Unstructured(),
		"data": t.Data.Unstructured(),
	}
}

func (t Ping) Unstructured() map[string]any {
	return map[string]any{
		"ping":   t.Ping,
		"detail": t.Detail,
	}
}

func (t NodeMeta) Unstructured() map[string]any {
	return map[string]any{
		"node": t.Node,
	}
}
