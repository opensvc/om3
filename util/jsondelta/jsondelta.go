package jsondelta

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

type (
	// OperationPath is a list of keys and indices like ["a", "b", 1]
	OperationPath []any

	// Operation describes a dataset change
	Operation struct {
		OpPath  OperationPath
		OpValue *json.RawMessage
		OpKind  string
	}

	// Patch is a list of Operation
	Patch []Operation
)

// NewPatch allocates and initializes a patchset.
func NewPatch(b []byte) (Patch, error) {
	ps := make(Patch, 0)
	var data []*json.RawMessage
	if err := json.Unmarshal(b, &data); err != nil {
		return ps, err
	}
	for _, v := range data {
		ps = append(ps, NewOperation(v))
	}
	return ps, nil
}

func NewPatchFromOperations(ops []Operation) *Patch {
	patch := make(Patch, 0)
	for _, op := range ops {
		patch = append(patch, op)
	}
	return &patch
}

func (o *Operation) UnmarshalJSON(b []byte) error {
	var data []*json.RawMessage
	json.Unmarshal(b, &data)
	json.Unmarshal(*data[0], &o.OpPath)
	if len(data) == 2 {
		o.OpValue = data[1]
		o.OpKind = "replace"
	} else {
		o.OpKind = "remove"
	}
	return nil
}

// MarshalJSON implements json.Marshaler interface for Operation
func (o Operation) MarshalJSON() ([]byte, error) {
	data := make([]json.RawMessage, 0)
	b, err := json.Marshal(o.OpPath)
	if err != nil {
		return nil, err
	}
	data = append(data, b)
	if o.OpKind != "remove" {
		b, err := json.Marshal(o.OpValue)
		if err != nil {
			return nil, err
		}
		data = append(data, b)
	}
	return json.Marshal(data)
}

// NewOperation allocates and initializes a patch
func NewOperation(b *json.RawMessage) Operation {
	o := Operation{}
	var data []*json.RawMessage
	json.Unmarshal(*b, &data)
	json.Unmarshal(*data[0], &o.OpPath)
	if len(data) == 2 {
		o.OpValue = data[1]
		o.OpKind = "replace"
	} else {
		o.OpKind = "remove"
	}
	return o
}

// NewOperationPath allocates and initializes an OperationPath
func NewOperationPath(data []any) OperationPath {
	p := OperationPath{}
	for _, v := range data {
		p = append(p, v)
	}
	return p
}

// NewOptValue allocates and initializes an OpValue
func NewOptValue(v any) *json.RawMessage {
	var b json.RawMessage
	b, _ = json.Marshal(v)
	return &b
}

// Kind returns the kind of operation, like remove or replace.
func (o Operation) Kind() string {
	return o.OpKind
}

// Path returns the path to the data to operate on in a deep dataset.
func (o Operation) Path() (OperationPath, error) {
	return o.OpPath, nil
}

// From is not implemented
func (o Operation) From() (OperationPath, error) {
	return nil, errors.New("From() not implemented")
}

// Value returns the value at the specified path
func (o Operation) value() *lazyNode {
	if o.OpKind != "remove" {
		return newLazyNode(o.OpValue)
	}
	return nil
}

// ValueInterface decodes the operation value into an interface.
func (o Operation) ValueInterface() (any, error) {
	if obj := o.OpValue; obj != nil {
		var v any
		err := json.Unmarshal(*obj, &v)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
	return nil, errors.Wrapf(ErrMissing, "operation, missing value field")
}

func findObject(pd *container, parts OperationPath) (container, string) {
	doc := *pd
	key := fmt.Sprint(parts[len(parts)-1])

	var err error
	if len(parts) < 1 {
		return nil, ""
	}

	for _, part := range parts[:len(parts)-1] {
		partStr := fmt.Sprint(part)
		next, ok := doc.get(partStr)

		if next == nil || ok != nil {
			return nil, ""
		}

		if isArray(*next.raw) {
			doc, err = next.intoAry()
			if err != nil {
				return nil, ""
			}
		} else {
			doc, err = next.intoDoc()
			if err != nil {
				return nil, ""
			}
		}
	}

	return doc, key
}
