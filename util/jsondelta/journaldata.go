package jsondelta

import (
	"encoding/json"
	"sort"

	"github.com/pkg/errors"
)

type (
	JournalData struct {
		c    container
		hook interface{}
	}

	patchWatcher interface {
		patchEvent(Patch)
	}
)

func New(eventHook interface{}) *JournalData {
	p := make(partialDoc)
	return &JournalData{
		c:    &p,
		hook: eventHook,
	}
}

func newLazyNodeFromInterface(v interface{}) (*lazyNode, error) {
	var r json.RawMessage
	r, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return newLazyNode(&r), nil
}

func (p *JournalData) Keys(path OperationPath) (keys []string, err error) {
	o, s := findObject(&p.c, path)
	if o == nil {
		err = errors.Wrapf(ErrMissing, "Set operation does not apply: doc is missing path: %s", path)
		return
	}
	var node *lazyNode
	if node, err = o.get(s); err != nil {
		return
	} else {
		for k := range node.doc {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return
	}
}

func (p *JournalData) Set(path OperationPath, value interface{}) error {
	var (
		oldNode, oldNodeCopy, newNode *lazyNode
		err, getOldErr                error
	)
	o, s := findObject(&p.c, path)
	if o == nil {
		return errors.Wrapf(ErrMissing, "Set operation does not apply: doc is missing path: %s", path)
	}
	oldNode, getOldErr = o.get(s)
	if newNode, err = newLazyNodeFromInterface(value); err != nil {
		return err
	}
	if oldNode == nil || getOldErr != nil {
		err = o.add(s, newNode)
		if err != nil {
			return err
		}
		if ev, ok := p.hook.(patchWatcher); ok {
			ev.patchEvent([]Operation{
				{
					OpPath:  path,
					OpValue: newNode.raw,
					OpKind:  "replace",
				},
			})
		}
	} else {
		err = o.set(s, newNode)
		if err != nil {
			return err
		}
		oldNodeCopy, _, err = deepCopy(oldNode)
		if err != nil {
			return err
		}
		if ev, ok := p.hook.(patchWatcher); ok {
			ev.patchEvent(newNode.diff(path, oldNodeCopy))
		}
	}
	return nil
}

func (p *JournalData) Unset(path OperationPath) error {
	o, s := findObject(&p.c, path)
	if o == nil {
		return errors.Wrapf(ErrMissing, "Unset operation does not apply: doc is missing path: %s", path)
	}
	if err := o.remove(s); err != nil {
		return err
	}
	if ev, ok := p.hook.(patchWatcher); ok {
		ev.patchEvent([]Operation{{OpPath: path, OpKind: "remove"}})
	}
	return nil
}

func (p *JournalData) MarshalPath(path OperationPath) (b []byte, err error) {
	var l *lazyNode
	c, s := findObject(&p.c, path)
	if l, err = c.get(s); err != nil {
		return
	}
	return json.Marshal(l)
}

func (n *lazyNode) diff(path OperationPath, previous *lazyNode) (patch Patch) {
	basePath := append(OperationPath{}, path...)
	if previous == nil {
		patch = append(patch, Operation{
			OpPath:  basePath,
			OpValue: n.raw,
			OpKind:  "replace",
		})
		return
	}
	if !n.tryDoc() {
		n.tryAry()
	}
	switch n.which {
	case eAry:
		if !previous.tryAry() {
			patch = append(patch, Operation{
				OpPath:  basePath,
				OpValue: n.raw,
				OpKind:  "replace",
			})
			return
		}
		lenPrevious := len(previous.ary)
		lenCurrent := len(n.ary)
		if lenCurrent < lenPrevious {
			for relId := range previous.ary[lenCurrent:lenPrevious] {
				idx := relId + lenCurrent
				patch = append(patch, Operation{
					OpPath: append(append(OperationPath{}, basePath...), idx),
					OpKind: "remove",
				})
			}
			for idx := range n.ary[0:lenCurrent] {
				patch = append(patch, n.ary[idx].diff(
					append(append(OperationPath{}, basePath...), idx),
					previous.ary[idx])...)
			}
		} else {
			for id := range n.ary[:lenPrevious] {
				patch = append(patch, n.ary[id].diff(
					append(append(OperationPath{}, basePath...), id),
					previous.ary[id])...)
			}
			for relId := range n.ary[lenPrevious:] {
				id := relId + lenPrevious
				patch = append(patch, Operation{
					OpPath:  append(append(OperationPath{}, basePath...), id),
					OpValue: n.ary[id].raw,
					OpKind:  "replace",
				})
			}
		}
	case eDoc:
		if !previous.tryDoc() {
			// new value
			patch = append(patch, Operation{
				OpPath:  basePath,
				OpValue: n.raw,
				OpKind:  "replace",
			})
			return
		}
		done := make(map[string]bool)
		for idx := range previous.doc {
			if _, ok := n.doc[idx]; !ok {
				op := Operation{
					OpPath: append(append(OperationPath{}, basePath...), idx),
					OpKind: "remove",
				}
				patch = append(patch, op)
				done[idx] = true
			}
		}
		for idx, currentNode := range n.doc {
			if _, ok := done[idx]; ok {
				continue
			}
			patch = append(patch, currentNode.diff(
				append(append(OperationPath{}, basePath...), idx),
				previous.doc[idx])...)
		}
	case eRaw:
		if n.equal(previous) {
			return
		}
		patch = append(patch, Operation{
			OpPath:  basePath,
			OpValue: n.raw,
			OpKind:  "replace",
		})
	}
	return
}
