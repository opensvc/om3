package commoncmd

// List is a listing a command builds itself, from what the node holds or
// from the status of the cluster, printed in the machine formats as the
// lists the api answers with, {kind, items}: a script reads every listing,
// from om or ox, the same way. Kind is the one of the api list of the same
// things, when there is one.
type List[T any] struct {
	Kind  string `json:"kind"`
	Items []T    `json:"items"`
}

// NewList returns the list of the items, empty rather than nil.
func NewList[T any](kind string, items []T) List[T] {
	if items == nil {
		items = make([]T, 0)
	}
	return List[T]{Kind: kind, Items: items}
}

// GetItems hands the items to the renderer, which sorts them.
func (t List[T]) GetItems() any {
	return t.Items
}
