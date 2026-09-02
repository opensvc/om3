package api

import "encoding/json"

// The tab renderer walks the api types by their json tags, so a type
// whose fields are its content needs nothing here. ObjectData is not
// one: it is a union, a raw message the generator gives no fields, so
// the object listings would have nothing to select from. It composes
// the map its content deserves, and the two types above it carry that
// map up to the renderer.

func (t ObjectItem) Unstructured() map[string]any {
	return map[string]any{
		"kind": t.Kind,
		"meta": t.Meta.Unstructured(),
		"data": t.Data.Unstructured(),
	}
}

func (t ObjectMeta) Unstructured() map[string]any {
	return map[string]any{
		"object": t.Object,
	}
}

func (t ObjectData) Unstructured() map[string]any {
	m := make(map[string]any)
	b, err := t.MarshalJSON()
	if err != nil {
		return m
	}
	_ = json.Unmarshal(b, &m)
	return m
}
